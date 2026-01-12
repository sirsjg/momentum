package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"atomicgo.dev/keyboard"
	"atomicgo.dev/keyboard/keys"
	"github.com/pterm/pterm"
	"github.com/sirsjg/momentum/agent"
	"github.com/sirsjg/momentum/version"
)

// Event is a UI event emitted by the worker.
type Event interface{}

// ListenerConnected signals the listener is connected.
type ListenerConnected struct{}

// ListenerError signals a listener error.
type ListenerError struct{ Err error }

// AddAgent adds a new agent panel.
type AddAgent struct {
	TaskID    string
	TaskTitle string
	AgentName string
	Runner    *agent.Runner
}

// AgentOutput sends output to an agent panel.
type AgentOutput struct {
	TaskID string
	Line   agent.OutputLine
}

// AgentCompleted signals an agent has finished.
type AgentCompleted struct {
	TaskID string
	Result agent.Result
}

type versionCheck struct {
	latestVersion   string
	updateAvailable bool
}

// AgentPanel represents a single agent's output panel.
type AgentPanel struct {
	ID        string
	TaskID    string
	TaskTitle string
	AgentName string
	Runner    *agent.Runner
	Output    []agent.OutputLine
	StartTime time.Time
	EndTime   time.Time
	Result    *agent.Result
	ScrollPos int
	Follow    bool
	Focused   bool
	Closed    bool
	Stopping  bool
	PID       int
}

// IsRunning returns whether the agent is still running.
func (p *AgentPanel) IsRunning() bool {
	return p.Runner != nil && p.Runner.IsRunning()
}

// IsFinished returns whether the agent has finished (success or failure).
func (p *AgentPanel) IsFinished() bool {
	return p.Result != nil
}

type inputAction int

type inputEvent struct {
	action inputAction
	value  int
}

const (
	actionNone inputAction = iota
	actionQuit
	actionToggleMode
	actionSelectNext
	actionSelectPrev
	actionStop
	actionClose
	actionScrollLines
	actionScrollPage
	actionScrollTop
	actionScrollBottom
	actionFollow
)

// Dashboard renders the headless UI using pterm.
type Dashboard struct {
	criteria string
	mode     ExecutionMode

	modeUpdates chan<- ExecutionMode
	stopUpdates chan<- string

	events chan Event
	inputs chan inputEvent

	listening bool
	connected bool
	lastError error
	taskCount int

	panels       []*AgentPanel
	focusedPanel int
	nextPanelID  int
	progressTick int

	updateAvailable bool
	latestVersion   string

	width  int
	height int

	area *pterm.AreaPrinter
}

// NewDashboard creates a new dashboard UI.
func NewDashboard(criteria string, mode ExecutionMode, modeUpdates chan<- ExecutionMode, stopUpdates chan<- string) *Dashboard {
	return &Dashboard{
		criteria:     criteria,
		mode:         mode,
		modeUpdates:  modeUpdates,
		stopUpdates:  stopUpdates,
		events:       make(chan Event, 200),
		inputs:       make(chan inputEvent, 50),
		panels:       make([]*AgentPanel, 0),
		focusedPanel: -1,
	}
}

// Events returns the channel for sending events into the dashboard.
func (d *Dashboard) Events() chan<- Event {
	return d.events
}

// Run starts the dashboard render loop.
func (d *Dashboard) Run(ctx context.Context) error {
	area, err := pterm.DefaultArea.WithFullscreen().WithRemoveWhenDone().Start()
	if err != nil {
		return err
	}
	d.area = area
	defer d.area.Stop()

	d.refreshSize()

	go d.listenForKeys(ctx)
	go d.checkVersion()

	refresh := time.NewTicker(200 * time.Millisecond)
	defer refresh.Stop()

	d.render()

	for {
		select {
		case <-ctx.Done():
			return nil
		case ev := <-d.events:
			d.handleEvent(ev)
			d.render()
		case input := <-d.inputs:
			if d.handleInput(input) {
				return nil
			}
			d.render()
		case <-refresh.C:
			d.progressTick++
			d.refreshSize()
			d.render()
		}
	}
}

func (d *Dashboard) checkVersion() {
	latest, available := version.CheckForUpdate()
	select {
	case d.events <- versionCheck{latestVersion: latest, updateAvailable: available}:
	default:
	}
}

func (d *Dashboard) listenForKeys(ctx context.Context) {
	_ = keyboard.Listen(func(key keys.Key) (bool, error) {
		if ctx.Err() != nil {
			return true, nil
		}
		if event, ok := mapKeyToInputEvent(key); ok {
			select {
			case d.inputs <- event:
			default:
			}
			if event.action == actionQuit {
				return true, nil
			}
		}
		return false, nil
	})
}

func mapKeyToInputEvent(key keys.Key) (inputEvent, bool) {
	switch key.Code {
	case keys.CtrlC, keys.Escape:
		return inputEvent{action: actionQuit}, true
	case keys.Tab, keys.Down:
		return inputEvent{action: actionSelectNext}, true
	case keys.ShiftTab, keys.Up:
		return inputEvent{action: actionSelectPrev}, true
	case keys.PgUp:
		return inputEvent{action: actionScrollPage, value: -1}, true
	case keys.PgDown:
		return inputEvent{action: actionScrollPage, value: 1}, true
	case keys.Home:
		return inputEvent{action: actionScrollTop}, true
	case keys.End:
		return inputEvent{action: actionScrollBottom}, true
	case keys.RuneKey:
		if len(key.Runes) == 0 {
			return inputEvent{}, false
		}
		switch key.Runes[0] {
		case 'q':
			return inputEvent{action: actionQuit}, true
		case 'm':
			return inputEvent{action: actionToggleMode}, true
		case 'j':
			return inputEvent{action: actionSelectNext}, true
		case 'k':
			return inputEvent{action: actionSelectPrev}, true
		case 's':
			return inputEvent{action: actionStop}, true
		case 'x', 'c':
			return inputEvent{action: actionClose}, true
		case 'f':
			return inputEvent{action: actionFollow}, true
		case 'g':
			return inputEvent{action: actionScrollTop}, true
		case 'G':
			return inputEvent{action: actionScrollBottom}, true
		case 'u':
			return inputEvent{action: actionScrollLines, value: -3}, true
		case 'd':
			return inputEvent{action: actionScrollLines, value: 3}, true
		}
	}
	return inputEvent{}, false
}

func (d *Dashboard) handleInput(input inputEvent) bool {
	switch input.action {
	case actionQuit:
		return true
	case actionToggleMode:
		d.mode = d.mode.Toggle()
		if d.modeUpdates != nil {
			select {
			case d.modeUpdates <- d.mode:
			default:
			}
		}
	case actionSelectNext:
		if len(d.panels) == 0 {
			return false
		}
		if d.focusedPanel < len(d.panels)-1 {
			d.focusedPanel++
		} else {
			d.focusedPanel = 0
		}
		d.ensureFollow(d.focusedPanel)
	case actionSelectPrev:
		if len(d.panels) == 0 {
			return false
		}
		if d.focusedPanel > 0 {
			d.focusedPanel--
		} else {
			d.focusedPanel = len(d.panels) - 1
		}
		d.ensureFollow(d.focusedPanel)
	case actionStop:
		d.stopFocusedPanel()
	case actionClose:
		d.closeFocusedPanel()
	case actionScrollLines:
		d.scrollOutput(input.value)
	case actionScrollPage:
		d.scrollOutput(input.value * d.outputViewHeight())
	case actionScrollTop:
		d.scrollToTop()
	case actionScrollBottom:
		d.scrollToBottom()
	case actionFollow:
		d.setFollow(true)
	}
	return false
}

func (d *Dashboard) handleEvent(ev Event) {
	switch msg := ev.(type) {
	case ListenerConnected:
		d.connected = true
		d.listening = true
		d.lastError = nil
	case ListenerError:
		d.lastError = msg.Err
	case AddAgent:
		d.addAgentPanel(msg.TaskID, msg.TaskTitle, msg.AgentName, msg.Runner)
	case AgentOutput:
		d.appendAgentOutput(msg.TaskID, msg.Line)
	case AgentCompleted:
		d.completeAgent(msg.TaskID, msg.Result)
	case versionCheck:
		d.updateAvailable = msg.updateAvailable
		d.latestVersion = msg.latestVersion
	}
}

func (d *Dashboard) addAgentPanel(taskID, taskTitle, agentName string, runner *agent.Runner) {
	d.nextPanelID++
	id := fmt.Sprintf("agent-%d", d.nextPanelID)

	pid := 0
	if runner != nil {
		pid = runner.PID()
	}

	panel := &AgentPanel{
		ID:        id,
		TaskID:    taskID,
		TaskTitle: taskTitle,
		AgentName: agentName,
		Runner:    runner,
		Output:    make([]agent.OutputLine, 0),
		StartTime: time.Now(),
		PID:       pid,
		Follow:    true,
	}

	d.panels = append(d.panels, panel)
	if len(d.panels) == 1 {
		d.focusedPanel = 0
	}
}

func (d *Dashboard) appendAgentOutput(taskID string, line agent.OutputLine) {
	for _, panel := range d.panels {
		if panel.TaskID != taskID {
			continue
		}
		parsed := parseClaudeOutput(line.Text)
		if parsed == "" {
			return
		}
		parsedLine := agent.OutputLine{
			Text:      parsed,
			IsStderr:  line.IsStderr,
			Timestamp: line.Timestamp,
		}
		panel.Output = append(panel.Output, parsedLine)
		if panel.Follow {
			panel.ScrollPos = clampScroll(len(panel.Output), d.outputViewHeight(), panel.ScrollPos, true)
		}
		return
	}
}

func (d *Dashboard) completeAgent(taskID string, result agent.Result) {
	for _, panel := range d.panels {
		if panel.TaskID != taskID {
			continue
		}
		panel.Result = &result
		panel.EndTime = time.Now()
		panel.Runner = nil
		d.taskCount++
		return
	}
}

func (d *Dashboard) stopFocusedPanel() {
	if d.focusedPanel < 0 || d.focusedPanel >= len(d.panels) {
		return
	}
	panel := d.panels[d.focusedPanel]
	if panel.IsRunning() && panel.Runner != nil && !panel.Stopping {
		panel.Stopping = true
		_ = panel.Runner.Cancel()
		if d.stopUpdates != nil {
			select {
			case d.stopUpdates <- panel.TaskID:
			default:
			}
		}
	}
}

func (d *Dashboard) closeFocusedPanel() {
	if d.focusedPanel < 0 || d.focusedPanel >= len(d.panels) {
		return
	}
	d.panels = append(d.panels[:d.focusedPanel], d.panels[d.focusedPanel+1:]...)
	if len(d.panels) == 0 {
		d.focusedPanel = -1
		return
	}
	if d.focusedPanel >= len(d.panels) {
		d.focusedPanel = len(d.panels) - 1
	}
}

func (d *Dashboard) ensureFollow(index int) {
	if index < 0 || index >= len(d.panels) {
		return
	}
	panel := d.panels[index]
	panel.Follow = true
	panel.ScrollPos = clampScroll(len(panel.Output), d.outputViewHeight(), panel.ScrollPos, true)
}

func (d *Dashboard) scrollOutput(delta int) {
	if d.focusedPanel < 0 || d.focusedPanel >= len(d.panels) {
		return
	}
	panel := d.panels[d.focusedPanel]
	panel.Follow = false
	panel.ScrollPos = clampScroll(len(panel.Output), d.outputViewHeight(), panel.ScrollPos+delta, false)
}

func (d *Dashboard) scrollToTop() {
	if d.focusedPanel < 0 || d.focusedPanel >= len(d.panels) {
		return
	}
	panel := d.panels[d.focusedPanel]
	panel.Follow = false
	panel.ScrollPos = 0
}

func (d *Dashboard) scrollToBottom() {
	if d.focusedPanel < 0 || d.focusedPanel >= len(d.panels) {
		return
	}
	panel := d.panels[d.focusedPanel]
	panel.Follow = true
	panel.ScrollPos = clampScroll(len(panel.Output), d.outputViewHeight(), panel.ScrollPos, true)
}

func (d *Dashboard) setFollow(follow bool) {
	if d.focusedPanel < 0 || d.focusedPanel >= len(d.panels) {
		return
	}
	panel := d.panels[d.focusedPanel]
	panel.Follow = follow
	panel.ScrollPos = clampScroll(len(panel.Output), d.outputViewHeight(), panel.ScrollPos, follow)
}

func (d *Dashboard) outputViewHeight() int {
	if d.height <= 0 {
		return 8
	}
	view := d.height / 3
	if view < 8 {
		view = 8
	}
	if view > 18 {
		view = 18
	}
	return view
}

func (d *Dashboard) refreshSize() {
	width := pterm.GetTerminalWidth()
	height := pterm.GetTerminalHeight()
	if width > 0 {
		d.width = width
	}
	if height > 0 {
		d.height = height
	}
}

func (d *Dashboard) render() {
	if d.area == nil {
		return
	}

	sections := []string{
		d.renderHeader(),
		d.renderStatus(),
		d.renderAgentTable(),
		d.renderOutput(),
		d.renderHelp(),
	}

	content := strings.Join(sections, "\n")
	d.area.Update(content)
}

func (d *Dashboard) renderHeader() string {
	logo := "" +
		"                                     ██\n" +
		"███▄███▄ ▄███▄ ███▄███▄ ▄█▀█▄ ████▄ ▀██▀▀ ██ ██ ███▄███▄\n" +
		"██ ██ ██ ██ ██ ██ ██ ██ ██▄█▀ ██ ██  ██   ██ ██ ██ ██ ██\n" +
		"██ ██ ██ ▀███▀ ██ ██ ██ ▀█▄▄▄ ██ ██  ██   ▀██▀█ ██ ██ ██"

	logoStyle := pterm.NewStyle(pterm.FgLightGreen, pterm.Bold)
	taglineStyle := pterm.NewStyle(pterm.FgGray)
	versionStyle := pterm.NewStyle(pterm.FgDarkGray)

	var b strings.Builder
	b.WriteString(logoStyle.Sprint(logo))
	b.WriteString("\n")
	b.WriteString(taglineStyle.Sprint("keep the board moving"))
	b.WriteString("  ")
	b.WriteString(versionStyle.Sprint("v" + version.Short()))
	return b.String()
}

func (d *Dashboard) renderStatus() string {
	statusLine := d.statusLine()

	rows := []string{
		statusLine,
		fmt.Sprintf("Filter: %s", d.criteria),
		fmt.Sprintf("Mode: %s", d.mode.String()),
		fmt.Sprintf("Tasks completed: %d", d.taskCount),
	}

	if d.updateAvailable {
		rows = append(rows, fmt.Sprintf("Update available: v%s (brew upgrade momentum)", d.latestVersion))
	}

	box := pterm.DefaultBox.WithTitle("Status")
	return box.Sprint(strings.Join(rows, "\n"))
}

func (d *Dashboard) statusLine() string {
	if d.lastError != nil {
		errStyle := pterm.NewStyle(pterm.FgRed, pterm.Bold)
		return errStyle.Sprintf("Error: %v", d.lastError)
	}

	spinner := []string{"-", "\\", "|", "/"}[d.progressTick%4]
	if d.connected {
		okStyle := pterm.NewStyle(pterm.FgGreen, pterm.Bold)
		return okStyle.Sprintf("Connected %s watching for tasks", spinner)
	}

	waitStyle := pterm.NewStyle(pterm.FgYellow)
	return waitStyle.Sprintf("Connecting %s", spinner)
}

func (d *Dashboard) renderAgentTable() string {
	if len(d.panels) == 0 {
		return pterm.DefaultBox.WithTitle("Agents").Sprint("No running tasks yet.")
	}

	maxTitle := 32
	if d.width > 140 {
		maxTitle = 48
	} else if d.width < 90 {
		maxTitle = 24
	}

	data := pterm.TableData{{"Sel", "Progress", "Status", "Task", "ID", "Time"}}

	for i, panel := range d.panels {
		statusText, statusStyle := statusForPanel(panel)
		bar := renderProgressBar(16, panel, d.progressTick)
		taskTitle := truncate(panel.TaskTitle, maxTitle)
		elapsed := formatDuration(panel)
		marker := " "
		rowStyle := pterm.NewStyle(pterm.FgWhite)
		if i == d.focusedPanel {
			marker = ">"
			rowStyle = pterm.NewStyle(pterm.FgLightWhite, pterm.Bold)
		}

		data = append(data, []string{
			rowStyle.Sprint(marker),
			bar,
			statusStyle.Sprint(statusText),
			rowStyle.Sprint(taskTitle),
			pterm.NewStyle(pterm.FgLightCyan).Sprint(panel.TaskID),
			pterm.NewStyle(pterm.FgGray).Sprint(elapsed),
		})
	}

	table := pterm.DefaultTable.WithHasHeader().WithData(data)
	content, err := table.Srender()
	if err != nil {
		content = ""
	}
	return pterm.DefaultBox.WithTitle("Agents").Sprint(strings.TrimRight(content, "\n"))
}

func (d *Dashboard) renderOutput() string {
	box := pterm.DefaultBox.WithTitle("Output")
	if d.focusedPanel < 0 || d.focusedPanel >= len(d.panels) {
		return box.Sprint("Select a task to view output.")
	}

	panel := d.panels[d.focusedPanel]
	statusText, statusStyle := statusForPanel(panel)
	followText := "follow"
	if !panel.Follow {
		followText = "paused"
	}

	lines := make([]string, 0)
	lines = append(lines, fmt.Sprintf("Task: %s", panel.TaskTitle))
	lines = append(lines, fmt.Sprintf("Status: %s | PID: %d | Scroll: %s", statusStyle.Sprint(statusText), panel.PID, followText))
	lines = append(lines, strings.Repeat("-", 60))

	viewHeight := d.outputViewHeight()
	start := panel.ScrollPos
	if panel.Follow {
		start = clampScroll(len(panel.Output), viewHeight, panel.ScrollPos, true)
		panel.ScrollPos = start
	}
	end := start + viewHeight
	if end > len(panel.Output) {
		end = len(panel.Output)
	}

	outputStyle := pterm.NewStyle(pterm.FgLightWhite)
	errStyle := pterm.NewStyle(pterm.FgYellow)

	for _, line := range panel.Output[start:end] {
		if line.IsStderr {
			lines = append(lines, errStyle.Sprint(line.Text))
		} else {
			lines = append(lines, outputStyle.Sprint(line.Text))
		}
	}

	for len(lines) < viewHeight+3 {
		lines = append(lines, "")
	}

	lineStart := 0
	lineEnd := 0
	if len(panel.Output) > 0 {
		lineStart = start + 1
		lineEnd = end
	}
	footer := fmt.Sprintf("Lines %d-%d of %d", lineStart, lineEnd, len(panel.Output))
	lines = append(lines, pterm.NewStyle(pterm.FgGray).Sprint(footer))

	return box.Sprint(strings.Join(lines, "\n"))
}

func (d *Dashboard) renderHelp() string {
	helpStyle := pterm.NewStyle(pterm.FgGray)
	help := "Keys: j/k or up/down select | tab/shift+tab cycle | m mode | s stop | x close | pgup/pgdn scroll | f follow | q quit"
	return helpStyle.Sprint(help)
}

func statusForPanel(panel *AgentPanel) (string, pterm.Style) {
	switch {
	case panel.Stopping && panel.IsRunning():
		return "stopping", *pterm.NewStyle(pterm.FgYellow, pterm.Bold)
	case panel.IsRunning():
		return "running", *pterm.NewStyle(pterm.FgGreen, pterm.Bold)
	case panel.Result != nil:
		if panel.Result.ExitCode == 0 {
			return "complete", *pterm.NewStyle(pterm.FgLightGreen, pterm.Bold)
		}
		if panel.Stopping {
			return "stopped", *pterm.NewStyle(pterm.FgGray)
		}
		return fmt.Sprintf("failed %d", panel.Result.ExitCode), *pterm.NewStyle(pterm.FgRed, pterm.Bold)
	default:
		return "pending", *pterm.NewStyle(pterm.FgYellow)
	}
}

func renderProgressBar(width int, panel *AgentPanel, frame int) string {
	inner := width - 2
	if inner < 3 {
		inner = 3
	}

	trackStyle := pterm.NewStyle(pterm.FgDarkGray)
	pulseStyle := pterm.NewStyle(pterm.FgLightGreen, pterm.Bold)
	completeStyle := pterm.NewStyle(pterm.FgGreen, pterm.Bold)
	failedStyle := pterm.NewStyle(pterm.FgRed, pterm.Bold)

	if panel.IsFinished() {
		fill := strings.Repeat("=", inner)
		style := completeStyle
		if panel.Result != nil && panel.Result.ExitCode != 0 {
			style = failedStyle
		}
		return "[" + style.Sprint(fill) + "]"
	}

	if panel.Stopping && panel.IsRunning() {
		return "[" + trackStyle.Sprint(strings.Repeat("-", inner)) + "]"
	}

	segLen := inner / 4
	if segLen < 3 {
		segLen = 3
	}
	if segLen > inner {
		segLen = inner
	}

	pos := frame % (inner + segLen)
	start := pos - segLen

	var b strings.Builder
	b.WriteString("[")
	for i := 0; i < inner; i++ {
		if i >= start && i < start+segLen {
			b.WriteString(pulseStyle.Sprint("="))
		} else {
			b.WriteString(trackStyle.Sprint("-"))
		}
	}
	b.WriteString("]")
	return b.String()
}

func truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func formatDuration(panel *AgentPanel) string {
	var elapsed time.Duration
	if panel.IsFinished() && !panel.EndTime.IsZero() {
		elapsed = panel.EndTime.Sub(panel.StartTime)
	} else {
		elapsed = time.Since(panel.StartTime)
	}
	elapsed = elapsed.Round(time.Second)

	h := int(elapsed.Hours())
	m := int(elapsed.Minutes()) % 60
	s := int(elapsed.Seconds()) % 60

	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

func clampScroll(totalLines, viewHeight, current int, follow bool) int {
	if viewHeight <= 0 {
		return 0
	}
	maxStart := totalLines - viewHeight
	if maxStart < 0 {
		maxStart = 0
	}
	if follow {
		return maxStart
	}
	if current < 0 {
		return 0
	}
	if current > maxStart {
		return maxStart
	}
	return current
}

// SetListening sets the listening state.
func (d *Dashboard) SetListening(listening bool) {
	d.listening = listening
}

// SetConnected sets the connection state.
func (d *Dashboard) SetConnected(connected bool) {
	d.connected = connected
}

// SetError sets the last error.
func (d *Dashboard) SetError(err error) {
	if err != nil {
		d.lastError = err
		return
	}
	d.lastError = nil
}

// AddAgent adds a new agent panel and returns its ID.
func (d *Dashboard) AddAgent(taskID, taskTitle, agentName string, runner *agent.Runner) string {
	d.addAgentPanel(taskID, taskTitle, agentName, runner)
	if len(d.panels) > 0 {
		return d.panels[len(d.panels)-1].ID
	}
	return ""
}

// GetOpenPanelCount returns the number of open panels.
func (d *Dashboard) GetOpenPanelCount() int {
	return len(d.panels)
}

// HasRunningAgents returns true if any agent is still running.
func (d *Dashboard) HasRunningAgents() bool {
	for _, p := range d.panels {
		if p.IsRunning() {
			return true
		}
	}
	return false
}

// CancelAllAgents cancels all running agents.
func (d *Dashboard) CancelAllAgents() {
	for _, p := range d.panels {
		if p.IsRunning() && p.Runner != nil && !p.Stopping {
			p.Stopping = true
			_ = p.Runner.Cancel()
		}
	}
}

// GetUpdateChannel returns the channel for sending agent updates.
func (d *Dashboard) GetUpdateChannel() chan<- Event {
	return d.events
}

// Send sends an event to the dashboard if possible.
func (d *Dashboard) Send(event Event) error {
	select {
	case d.events <- event:
		return nil
	default:
		return errors.New("dashboard event queue is full")
	}
}
