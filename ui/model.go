package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sirsjg/momentum/agent"
	"github.com/sirsjg/momentum/version"
)

// AgentUpdate represents an update from an agent
type AgentUpdate struct {
	PanelID string
	Type    string // "output", "completed", "error"
	Line    *agent.OutputLine
	Result  *agent.Result
	Error   error
}

// AgentPanel represents a single agent's output panel
type AgentPanel struct {
	ID        string
	TaskID    string
	TaskTitle string
	AgentName string
	Runner    *agent.Runner
	Output    []agent.OutputLine
	StartTime time.Time
	EndTime   time.Time // Set when agent completes
	Result    *agent.Result
	ScrollPos int
	Focused   bool
	Closed    bool
	Stopping  bool // Set when stop is requested but process hasn't exited yet
	PID       int
}

// IsRunning returns whether the agent is still running
func (p *AgentPanel) IsRunning() bool {
	return p.Runner != nil && p.Runner.IsRunning()
}

// IsFinished returns whether the agent has finished (success or failure)
func (p *AgentPanel) IsFinished() bool {
	return p.Result != nil
}

// Model is the main TUI model
type Model struct {
	// Dimensions
	width           int
	height          int
	consoleWidth    int
	consoleHeight   int
	listPanelHeight int
	listBodyHeight  int

	// Listener state
	listening    bool
	connected    bool
	lastError    error
	criteria     string
	spinner      spinner.Model
	taskCount    int
	lastTaskTime time.Time
	mode         ExecutionMode

	// Agent panels
	panels       []*AgentPanel
	focusedPanel int
	scrollIndex  int
	nextPanelID  int

	// List and detail view components
	viewport      viewport.Model
	consoleOpen   bool
	progressFrame int

	// Agent updates channel
	agentUpdates chan AgentUpdate

	// Update notification
	updateAvailable bool
	latestVersion   string

	modeUpdates chan<- ExecutionMode
	stopUpdates chan<- string // sends taskID when user stops an agent

	// WorkDir settings
	workDir           string
	workDirUpdates    chan<- string
	workDirMenuOpen   bool
	workDirInputMode  bool
	workDirInput      textinput.Model
	promptPreviewOpen bool
	claudeMdFiles     []claudeMdFile
	promptViewport    viewport.Model
}

// claudeMdFile represents a CLAUDE.md file and its content
type claudeMdFile struct {
	Path    string
	Content string
}

// NewModel creates a new TUI model
func NewModel(criteria string, mode ExecutionMode, workDir string, modeUpdates chan<- ExecutionMode, stopUpdates chan<- string, workDirUpdates chan<- string) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(GlowGreen)

	// Initialize viewport for detail view
	vp := viewport.New(0, 0)

	// Initialize viewport for prompt preview
	promptVp := viewport.New(0, 0)

	// Initialize text input for workdir
	ti := textinput.New()
	ti.Placeholder = "Enter path..."
	ti.CharLimit = 256

	return Model{
		criteria:       criteria,
		mode:           mode,
		workDir:        workDir,
		spinner:        s,
		panels:         make([]*AgentPanel, 0),
		viewport:       vp,
		promptViewport: promptVp,
		workDirInput:   ti,
		agentUpdates:   make(chan AgentUpdate, 100),
		modeUpdates:    modeUpdates,
		stopUpdates:    stopUpdates,
		workDirUpdates: workDirUpdates,
	}
}

// Messages
type tickMsg time.Time
type agentUpdateMsg AgentUpdate
type versionCheckMsg struct {
	latestVersion   string
	updateAvailable bool
}

// ListenerConnectedMsg signals the listener is connected
type ListenerConnectedMsg struct{}

// ListenerErrorMsg signals a listener error
type ListenerErrorMsg struct{ Err error }

// AddAgentMsg requests adding a new agent panel
type AddAgentMsg struct {
	TaskID    string
	TaskTitle string
	AgentName string
	Runner    *agent.Runner
}

// AgentOutputMsg sends output to an agent panel
type AgentOutputMsg struct {
	TaskID string
	Line   agent.OutputLine
}

// AgentCompletedMsg signals an agent has finished
type AgentCompletedMsg struct {
	TaskID string
	Result agent.Result
}

// Init initializes the model
func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		tickCmd(),
		checkVersionCmd(),
	)
}

func tickCmd() tea.Cmd {
	return tea.Tick(200*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func checkVersionCmd() tea.Cmd {
	return func() tea.Msg {
		latest, available := version.CheckForUpdate()
		return versionCheckMsg{
			latestVersion:   latest,
			updateAvailable: available,
		}
	}
}

// Update handles messages
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateLayoutDimensions()
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tickMsg:
		m.progressFrame++
		return m, tickCmd()

	case ListenerConnectedMsg:
		m.connected = true
		m.listening = true
		m.lastError = nil
		return m, nil

	case ListenerErrorMsg:
		m.lastError = msg.Err
		return m, nil

	case AddAgentMsg:
		m.addAgentPanel(msg.TaskID, msg.TaskTitle, msg.AgentName, msg.Runner)
		return m, nil

	case AgentOutputMsg:
		m.appendAgentOutput(msg.TaskID, msg.Line)
		return m, nil

	case AgentCompletedMsg:
		m.completeAgent(msg.TaskID, msg.Result)
		return m, nil

	case versionCheckMsg:
		m.updateAvailable = msg.updateAvailable
		m.latestVersion = msg.latestVersion
		return m, nil
	}

	return m, nil
}

func (m *Model) addAgentPanel(taskID, taskTitle, agentName string, runner *agent.Runner) {
	m.nextPanelID++
	id := fmt.Sprintf("agent-%d", m.nextPanelID)

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
	}

	m.panels = append(m.panels, panel)

	// Auto-select first panel
	if len(m.panels) == 1 {
		m.focusedPanel = 0
	}

	m.clampSelection()
	m.updateConsoleContent()
}

func (m *Model) appendAgentOutput(taskID string, line agent.OutputLine) {
	for i, panel := range m.panels {
		if panel.TaskID == taskID {
			// Parse JSON output to extract meaningful content
			parsed := parseClaudeOutput(line.Text)
			if parsed == "" {
				return // Skip empty/uninteresting messages
			}

			parsedLine := agent.OutputLine{
				Text:      parsed,
				IsStderr:  line.IsStderr,
				Timestamp: line.Timestamp,
			}

			panel.Output = append(panel.Output, parsedLine)

			// Update viewport if this is the selected panel
			if i == m.focusedPanel {
				m.updateConsoleContent()
			}
			return
		}
	}
}

func (m *Model) completeAgent(taskID string, result agent.Result) {
	for _, panel := range m.panels {
		if panel.TaskID == taskID {
			panel.Result = &result
			panel.EndTime = time.Now()
			panel.Runner = nil
			m.taskCount++
			m.lastTaskTime = time.Now()
			m.clampSelection()
			m.updateConsoleContent()
			return
		}
	}
}

func (m *Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle prompt preview mode
	if m.promptPreviewOpen {
		switch msg.String() {
		case "esc":
			m.promptPreviewOpen = false
			return m, nil
		case "up", "k", "down", "j", "pgup", "pgdown", "home", "end":
			var cmd tea.Cmd
			m.promptViewport, cmd = m.promptViewport.Update(msg)
			return m, cmd
		}
		return m, nil
	}

	// Handle workdir text input mode
	if m.workDirInputMode {
		switch msg.String() {
		case "esc":
			m.workDirInputMode = false
			m.workDirInput.Reset()
			return m, nil
		case "enter":
			newPath := m.workDirInput.Value()
			if newPath != "" {
				m.workDir = expandHomePath(newPath)
				if m.workDirUpdates != nil {
					select {
					case m.workDirUpdates <- m.workDir:
					default:
					}
				}
			}
			m.workDirInputMode = false
			m.workDirInput.Reset()
			return m, nil
		default:
			var cmd tea.Cmd
			m.workDirInput, cmd = m.workDirInput.Update(msg)
			return m, cmd
		}
	}

	// Handle workdir menu mode
	if m.workDirMenuOpen {
		switch msg.String() {
		case "esc":
			m.workDirMenuOpen = false
			return m, nil
		case "1":
			m.workDirMenuOpen = false
			m.workDirInputMode = true
			m.workDirInput.SetValue(m.workDir)
			m.workDirInput.Focus()
			return m, nil
		case "2":
			m.workDirMenuOpen = false
			m.saveWorkDirToEnv()
			return m, nil
		}
		return m, nil
	}

	if m.consoleOpen {
		switch msg.String() {
		case "esc":
			m.consoleOpen = false
			m.updateLayoutDimensions()
			return m, nil
		case "up", "k", "down", "j", "pgup", "pgdown", "home", "end":
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		case "enter":
			m.consoleOpen = false
			m.updateLayoutDimensions()
			return m, nil
		}
	}

	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit

	case "enter":
		if len(m.panels) > 0 {
			m.consoleOpen = true
			m.updateConsoleContent()
			m.updateLayoutDimensions()
		}
		return m, nil

	case "x", "c":
		// Close selected panel
		if m.focusedPanel >= 0 && m.focusedPanel < len(m.panels) {
			m.panels = append(m.panels[:m.focusedPanel], m.panels[m.focusedPanel+1:]...)
			m.clampSelection()
			m.updateConsoleContent()
		}
		return m, nil

	case "s":
		// Stop selected panel's agent if running
		if m.focusedPanel >= 0 && m.focusedPanel < len(m.panels) {
			panel := m.panels[m.focusedPanel]
			if panel.IsRunning() && panel.Runner != nil && !panel.Stopping {
				panel.Stopping = true
				panel.Runner.Cancel()
				if m.stopUpdates != nil {
					select {
					case m.stopUpdates <- panel.TaskID:
					default:
					}
				}
			}
		}
		return m, nil

	case "up", "k":
		if m.focusedPanel > 0 {
			m.focusedPanel--
			m.clampSelection()
			m.updateConsoleContent()
		}
		return m, nil

	case "down", "j":
		if m.focusedPanel < len(m.panels)-1 {
			m.focusedPanel++
			m.clampSelection()
			m.updateConsoleContent()
		}
		return m, nil

	case "m":
		m.mode = m.mode.Toggle()
		if m.modeUpdates != nil {
			select {
			case m.modeUpdates <- m.mode:
			default:
			}
		}
		return m, nil

	case "w":
		m.workDirMenuOpen = true
		return m, nil

	case "p":
		m.loadClaudeMdFiles()
		m.promptPreviewOpen = true
		m.updatePromptPreviewContent()
		return m, nil
	}

	return m, nil
}

func (m *Model) updateLayoutDimensions() {
	headerHeight := lipgloss.Height(m.renderHeader())
	helpHeight := lipgloss.Height(m.renderHelp())
	gaps := 2
	if m.consoleOpen {
		gaps = 3
	}

	available := m.height - headerHeight - helpHeight - gaps
	if available < 0 {
		available = 0
	}

	listWidth := m.width - 4
	if listWidth < 20 {
		listWidth = 20
	}

	consoleHeight := 0
	minListPanelHeight := 6
	if available < minListPanelHeight {
		minListPanelHeight = available
	}
	if m.consoleOpen {
		consoleHeight = available / 3
		if consoleHeight < 8 {
			consoleHeight = 8
		}
		if consoleHeight > 14 {
			consoleHeight = 14
		}

		maxConsole := available - minListPanelHeight
		if maxConsole < 0 {
			maxConsole = 0
		}
		if consoleHeight > maxConsole {
			consoleHeight = maxConsole
		}
	}

	listPanelHeight := available - consoleHeight
	if listPanelHeight < minListPanelHeight {
		listPanelHeight = minListPanelHeight
		consoleHeight = available - listPanelHeight
		if consoleHeight < 0 {
			consoleHeight = 0
		}
	}

	listBodyHeight := listPanelHeight - 3
	if listBodyHeight < 1 {
		if listPanelHeight == 0 {
			listBodyHeight = 0
		} else {
			listBodyHeight = 1
		}
	}
	m.listPanelHeight = listPanelHeight
	m.listBodyHeight = listBodyHeight
	m.clampSelection()

	// Update console dimensions
	m.consoleWidth = listWidth - 4
	if m.consoleWidth < 40 {
		m.consoleWidth = listWidth - 2
	}
	if m.consoleWidth > 120 {
		m.consoleWidth = 120
	}
	m.consoleHeight = consoleHeight

	if m.consoleHeight > 0 {
		m.viewport.Width = m.consoleWidth - 4
		m.viewport.Height = m.consoleHeight - 4
	} else {
		m.viewport.Width = listWidth - 4
		m.viewport.Height = 1
	}
}

func (m *Model) clampSelection() {
	if len(m.panels) == 0 {
		m.focusedPanel = -1
		m.scrollIndex = 0
		return
	}

	if m.focusedPanel < 0 {
		m.focusedPanel = 0
	}
	if m.focusedPanel >= len(m.panels) {
		m.focusedPanel = len(m.panels) - 1
	}

	maxItems := m.listMaxItems()
	if maxItems <= 0 {
		m.scrollIndex = m.focusedPanel
		return
	}

	if m.focusedPanel < m.scrollIndex {
		m.scrollIndex = m.focusedPanel
	}
	if m.focusedPanel >= m.scrollIndex+maxItems {
		m.scrollIndex = m.focusedPanel - maxItems + 1
	}

	if maxStart := len(m.panels) - maxItems; maxStart >= 0 && m.scrollIndex > maxStart {
		m.scrollIndex = maxStart
	}
	if m.scrollIndex < 0 {
		m.scrollIndex = 0
	}
}

func (m *Model) listMaxItems() int {
	rowHeight := 2
	gap := 1
	if m.listBodyHeight <= 0 {
		return 0
	}
	return (m.listBodyHeight + gap) / (rowHeight + gap)
}

func (m *Model) updateConsoleContent() {
	if m.focusedPanel < 0 || m.focusedPanel >= len(m.panels) {
		m.viewport.SetContent("")
		return
	}

	panel := m.panels[m.focusedPanel]
	var b strings.Builder

	for _, line := range panel.Output {
		text := line.Text
		if line.IsStderr {
			b.WriteString(StderrStyle.Render(text))
		} else {
			b.WriteString(OutputStyle.Render(text))
		}
		b.WriteString("\n")
	}

	m.viewport.SetContent(b.String())

	// Auto-scroll to bottom if running
	if panel.IsRunning() {
		m.viewport.GotoBottom()
	}
}

// View renders the UI
func (m *Model) View() string {
	if m.width == 0 {
		return ""
	}

	// Check for overlay modes first
	if m.promptPreviewOpen {
		return m.renderPromptPreview()
	}
	if m.workDirMenuOpen {
		return m.renderWorkDirMenu()
	}
	if m.workDirInputMode {
		return m.renderWorkDirInput()
	}

	var b strings.Builder

	// Header
	b.WriteString(m.renderHeader())
	b.WriteString("\n")

	b.WriteString(m.renderTaskListPanel())
	b.WriteString("\n")

	if m.consoleOpen {
		b.WriteString(m.renderConsolePanel())
		b.WriteString("\n")
	}

	// Help
	b.WriteString(m.renderHelp())

	view := b.String()
	if m.height > 0 {
		view = clampHeight(view, m.height)
	}
	return view
}

func (m *Model) renderWorkDirMenu() string {
	var b strings.Builder

	title := lipgloss.NewStyle().Bold(true).Foreground(GlowGreen).Render("WorkDir")
	b.WriteString(title)
	b.WriteString("\n\n")

	b.WriteString(fmt.Sprintf("Current: %s\n\n", shortenPath(m.workDir)))

	b.WriteString(HelpKeyStyle.Render("[1]") + " Change path...\n")
	b.WriteString(HelpKeyStyle.Render("[2]") + " Save to MOMENTUM_WORKDIR env var\n\n")

	b.WriteString(HelpStyle.Render("Press 1-2 or esc to cancel"))

	content := PanelStyle.Width(60).Render(b.String())

	// Center in screen
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m *Model) renderWorkDirInput() string {
	var b strings.Builder

	title := lipgloss.NewStyle().Bold(true).Foreground(GlowGreen).Render("WorkDir")
	b.WriteString(title)
	b.WriteString("\n\n")

	b.WriteString("Enter path: ")
	b.WriteString(m.workDirInput.View())
	b.WriteString("\n\n")

	b.WriteString(HelpStyle.Render("Press enter to confirm or esc to cancel"))

	content := PanelStyle.Width(60).Render(b.String())

	// Center in screen
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m *Model) renderPromptPreview() string {
	var b strings.Builder

	title := lipgloss.NewStyle().Bold(true).Foreground(GlowGreen).Render("Inherited System Prompt")
	b.WriteString(title)
	b.WriteString("\n\n")

	// Update viewport dimensions
	previewWidth := m.width - 10
	if previewWidth > 100 {
		previewWidth = 100
	}
	previewHeight := m.height - 10
	if previewHeight > 30 {
		previewHeight = 30
	}
	m.promptViewport.Width = previewWidth - 4
	m.promptViewport.Height = previewHeight - 6

	b.WriteString(m.promptViewport.View())
	b.WriteString("\n\n")

	b.WriteString(HelpStyle.Render("esc to close  j/k scroll"))

	content := PanelStyle.Width(previewWidth).Render(b.String())

	// Center in screen
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m *Model) renderListenerPanel() string {
	var status string
	if m.lastError != nil {
		status = StatusError.Render(fmt.Sprintf("Error: %v", m.lastError))
	} else if m.connected {
		status = StatusConnected.Render("Connected and watching for tasks...") + " " + m.spinner.View()
	} else {
		status = m.spinner.View() + " " + StatusWaiting.Render("Connecting...")
	}

	labelWidth := 16
	labelStyle := lipgloss.NewStyle().Foreground(Gray).Width(labelWidth)
	hintStyle := lipgloss.NewStyle().Foreground(Gray).Italic(true)

	// Shorten workdir for display
	displayWorkDir := shortenPath(m.workDir)
	if len(displayWorkDir) > 40 {
		displayWorkDir = "..." + displayWorkDir[len(displayWorkDir)-37:]
	}

	content := fmt.Sprintf("%s\n%s %s\n%s %s\n%s %s\n%s %d\n\n%s",
		status,
		labelStyle.Render("Filter:"),
		m.criteria,
		labelStyle.Render("Mode:"),
		m.mode.String(),
		labelStyle.Render("WorkDir:"),
		displayWorkDir,
		labelStyle.Render("Tasks completed:"),
		m.taskCount,
		hintStyle.Render("Agents inherit CLAUDE.md from WorkDir. Press p to preview."),
	)

	return PanelStyle.Width(m.width - 4).Render(content)
}

func (m *Model) renderHeader() string {
	var b strings.Builder

	logo := `
                                     ██
███▄███▄ ▄███▄ ███▄███▄ ▄█▀█▄ ████▄ ▀██▀▀ ██ ██ ███▄███▄
██ ██ ██ ██ ██ ██ ██ ██ ██▄█▀ ██ ██  ██   ██ ██ ██ ██ ██
██ ██ ██ ▀███▀ ██ ██ ██ ▀█▄▄▄ ██ ██  ██   ▀██▀█ ██ ██ ██
`
	b.WriteString(LogoStyle.Render(logo))
	b.WriteString("\n")
	b.WriteString(TaglineStyle.Render("keep the board moving"))
	b.WriteString("  ")
	b.WriteString(VersionStyle.Render("v" + version.Short()))
	b.WriteString("\n\n")
	b.WriteString(m.renderListenerPanel())

	return b.String()
}

func (m *Model) renderHelp() string {
	help := HelpKeyStyle.Render("enter") + HelpStyle.Render(" console  ") +
		HelpKeyStyle.Render("j/k") + HelpStyle.Render(" select  ") +
		HelpKeyStyle.Render("m") + HelpStyle.Render(" mode  ") +
		HelpKeyStyle.Render("w") + HelpStyle.Render(" workdir  ") +
		HelpKeyStyle.Render("p") + HelpStyle.Render(" prompt  ") +
		HelpKeyStyle.Render("s") + HelpStyle.Render(" stop  ") +
		HelpKeyStyle.Render("x") + HelpStyle.Render(" remove  ") +
		HelpKeyStyle.Render("q") + HelpStyle.Render(" quit")

	if m.updateAvailable {
		updateMsg := fmt.Sprintf("  Update available: v%s - run: brew upgrade momentum", m.latestVersion)
		help += UpdateAvailableStyle.Render(updateMsg)
	}

	return help
}

func (m *Model) renderTaskListPanel() string {
	listWidth := m.width - 4
	if listWidth < 20 {
		listWidth = 20
	}

	contentWidth := listWidth - 2
	header := m.renderListHeader(contentWidth)

	var b strings.Builder
	b.WriteString(header)
	b.WriteString("\n")
	b.WriteString(m.renderTaskListBody(contentWidth))

	return PanelStyle.Width(listWidth).Height(m.listPanelHeight).Render(b.String())
}

func (m *Model) renderTaskListBody(width int) string {
	if m.listBodyHeight <= 0 {
		return ""
	}

	var b strings.Builder
	linesUsed := 0
	contentWidth := width - 2
	if contentWidth < 10 {
		contentWidth = 10
	}

	if len(m.panels) == 0 {
		empty := lipgloss.NewStyle().Foreground(Gray).Render("No running tasks yet...")
		b.WriteString(empty)
		linesUsed++
		return padLines(b.String(), m.listBodyHeight-linesUsed)
	}

	maxItems := m.listMaxItems()
	if maxItems <= 0 {
		return padLines("", m.listBodyHeight)
	}

	start := m.scrollIndex
	if start < 0 {
		start = 0
	}
	end := start + maxItems
	if end > len(m.panels) {
		end = len(m.panels)
	}

	for i := start; i < end; i++ {
		panel := m.panels[i]
		line1 := renderProgressLine(panel, contentWidth, m.progressFrame)
		line2 := renderMetaLine(panel, contentWidth)
		prefix := "  "
		if i == m.focusedPanel {
			prefix = "> "
		}
		line1 = prefix + line1
		line2 = "  " + line2

		if i == m.focusedPanel {
			line1 = SelectedRowStyle.Width(width).Render(line1)
			line2 = SelectedRowStyle.Width(width).Render(line2)
		} else {
			line1 = lipgloss.NewStyle().Width(width).Render(line1)
			line2 = lipgloss.NewStyle().Width(width).Render(line2)
		}

		b.WriteString(line1)
		b.WriteString("\n")
		b.WriteString(line2)
		linesUsed += 2

		if i != end-1 && linesUsed < m.listBodyHeight {
			b.WriteString("\n")
			linesUsed++
		}
	}

	return padLines(b.String(), m.listBodyHeight-linesUsed)
}

func (m *Model) renderListHeader(width int) string {
	label := "PID"
	task := "TASK"
	name := "NAME"
	timer := "TIME"

	base := fmt.Sprintf("%s  %s  %s", label, task, name)
	padding := width - lipgloss.Width(base) - lipgloss.Width(timer)
	if padding < 1 {
		padding = 1
	}

	return ListHeaderStyle.Width(width).Render(base + strings.Repeat(" ", padding) + timer)
}

func (m *Model) renderConsolePanel() string {
	if m.focusedPanel < 0 || m.focusedPanel >= len(m.panels) {
		return ""
	}

	panel := m.panels[m.focusedPanel]
	statusText, statusStyle := statusForPanel(panel)
	title := fmt.Sprintf("Console: %s · %s · %s", panel.TaskTitle, statusStyle.Render(statusText), formatDuration(panel))

	content := ConsoleTitleStyle.Width(m.consoleWidth-2).Render(title) + "\n"
	content += m.viewport.View()
	content += "\n" + HelpStyle.Render("esc to close")

	if m.consoleHeight <= 0 {
		return ""
	}

	return ConsoleOverlayStyle.Width(m.consoleWidth).Height(m.consoleHeight).Render(content)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func renderProgressLine(panel *AgentPanel, width int, frame int) string {
	statusText, statusStyle := statusForPanel(panel)

	barWidth := width - lipgloss.Width(statusText) - 1
	if barWidth < 10 {
		barWidth = 10
	}
	bar := renderProgressBar(barWidth, panel, frame)

	return bar + " " + statusStyle.Render(statusText)
}

func renderMetaLine(panel *AgentPanel, width int) string {
	pidText := "pid:-"
	if panel.PID > 0 {
		pidText = fmt.Sprintf("pid:%d", panel.PID)
	}
	taskIDText := fmt.Sprintf("task:%s", panel.TaskID)
	elapsed := formatDuration(panel)
	timeWidth := lipgloss.Width(elapsed)

	baseWidth := lipgloss.Width(pidText) + 2 + lipgloss.Width(taskIDText) + 2
	nameMax := width - baseWidth - timeWidth - 2
	if nameMax < 8 {
		nameMax = 8
	}

	nameText := truncate(panel.TaskTitle, nameMax)

	baseRaw := fmt.Sprintf("%s  %s  %s", pidText, taskIDText, nameText)
	padding := width - lipgloss.Width(baseRaw) - timeWidth
	if padding < 1 {
		padding = 1
	}

	return fmt.Sprintf(
		"%s  %s  %s%s%s",
		PidStyle.Render(pidText),
		TaskIDStyle.Render(taskIDText),
		TaskNameStyle.Render(nameText),
		strings.Repeat(" ", padding),
		TimeStyle.Render(elapsed),
	)
}

func renderProgressBar(width int, panel *AgentPanel, frame int) string {
	inner := width - 2
	if inner < 3 {
		inner = 3
	}

	if panel.IsFinished() {
		fill := strings.Repeat("=", inner)
		style := ProgressCompleteStyle
		if panel.Result != nil && panel.Result.ExitCode != 0 {
			style = ProgressFailedStyle
		}
		return "[" + style.Render(fill) + "]"
	}

	if panel.Stopping && panel.IsRunning() {
		return "[" + ProgressTrackStyle.Render(strings.Repeat("-", inner)) + "]"
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
			b.WriteString(ProgressPulseStyle.Render("="))
		} else {
			b.WriteString(ProgressTrackStyle.Render("-"))
		}
	}
	b.WriteString("]")
	return b.String()
}

func statusForPanel(panel *AgentPanel) (string, lipgloss.Style) {
	switch {
	case panel.Stopping && panel.IsRunning():
		return "stopping", AgentStopping
	case panel.IsRunning():
		return "running", AgentRunning
	case panel.Result != nil:
		if panel.Result.ExitCode == 0 {
			return "complete 100%", AgentCompleted
		}
		if panel.Stopping {
			return "stopped", AgentStopped
		}
		return fmt.Sprintf("failed %d", panel.Result.ExitCode), AgentFailed
	default:
		return "pending", StatusWaiting
	}
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

func padLines(s string, count int) string {
	if count <= 0 {
		return s
	}
	return s + strings.Repeat("\n", count)
}

func clampHeight(s string, height int) string {
	if height <= 0 {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) > height {
		lines = lines[:height]
		return strings.Join(lines, "\n")
	}
	if len(lines) < height {
		return s + strings.Repeat("\n", height-len(lines))
	}
	return s
}

// Public methods for external control

// SetListening sets the listening state
func (m *Model) SetListening(listening bool) {
	m.listening = listening
}

// SetConnected sets the connection state
func (m *Model) SetConnected(connected bool) {
	m.connected = connected
}

// SetError sets the last error
func (m *Model) SetError(err error) {
	m.lastError = err
}

// AddAgent adds a new agent panel and returns its ID
func (m *Model) AddAgent(taskID, taskTitle, agentName string, runner *agent.Runner) string {
	m.nextPanelID++
	id := fmt.Sprintf("agent-%d", m.nextPanelID)

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
	}

	m.panels = append(m.panels, panel)

	// Auto-select first panel
	if len(m.panels) == 1 {
		m.focusedPanel = 0
	}

	m.clampSelection()
	m.updateConsoleContent()

	return id
}

// GetUpdateChannel returns the channel for sending agent updates
func (m *Model) GetUpdateChannel() chan<- AgentUpdate {
	return m.agentUpdates
}

// GetOpenPanelCount returns the number of open (non-closed) panels
func (m *Model) GetOpenPanelCount() int {
	return len(m.panels)
}

// HasRunningAgents returns true if any agent is still running
func (m *Model) HasRunningAgents() bool {
	for _, p := range m.panels {
		if p.IsRunning() {
			return true
		}
	}
	return false
}

// CancelAllAgents cancels all running agents
func (m *Model) CancelAllAgents() {
	for _, p := range m.panels {
		if p.IsRunning() && p.Runner != nil && !p.Stopping {
			p.Stopping = true
			p.Runner.Cancel()
		}
	}
}

// WorkDir helpers

// expandHomePath expands ~ to the user's home directory
func expandHomePath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}

// saveWorkDirToEnv prints instructions for saving workdir to env var
// (actual shell modification not possible from Go, so we inform the user)
func (m *Model) saveWorkDirToEnv() {
	// We can't actually modify the user's shell config from here,
	// but we can show them what to add
	// For now, this is a no-op - the user sees the current workdir and can set it manually
}

// loadClaudeMdFiles finds and loads CLAUDE.md files for preview
func (m *Model) loadClaudeMdFiles() {
	m.claudeMdFiles = nil

	// 1. Global ~/.claude/CLAUDE.md
	home, _ := os.UserHomeDir()
	globalPath := filepath.Join(home, ".claude", "CLAUDE.md")
	if content, err := os.ReadFile(globalPath); err == nil {
		m.claudeMdFiles = append(m.claudeMdFiles, claudeMdFile{
			Path:    globalPath,
			Content: string(content),
		})
	}

	// 2. Walk from workdir up to root, collecting CLAUDE.md files
	absWorkDir := m.workDir
	if !filepath.IsAbs(absWorkDir) {
		if wd, err := os.Getwd(); err == nil {
			absWorkDir = filepath.Join(wd, m.workDir)
		}
	}
	absWorkDir = filepath.Clean(absWorkDir)

	var projectFiles []claudeMdFile
	dir := absWorkDir
	for {
		mdPath := filepath.Join(dir, "CLAUDE.md")
		if content, err := os.ReadFile(mdPath); err == nil {
			// Prepend so parent dirs come first
			projectFiles = append([]claudeMdFile{{
				Path:    mdPath,
				Content: string(content),
			}}, projectFiles...)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	m.claudeMdFiles = append(m.claudeMdFiles, projectFiles...)
}

// updatePromptPreviewContent updates the prompt preview viewport content
func (m *Model) updatePromptPreviewContent() {
	var b strings.Builder

	if len(m.claudeMdFiles) == 0 {
		b.WriteString("No CLAUDE.md files found for current WorkDir.\n\n")
		b.WriteString(fmt.Sprintf("WorkDir: %s\n", m.workDir))
	} else {
		b.WriteString("Sources:\n")
		for _, f := range m.claudeMdFiles {
			b.WriteString(fmt.Sprintf("  %s\n", f.Path))
		}
		b.WriteString("\n")
		b.WriteString(strings.Repeat("─", 60))
		b.WriteString("\n\n")

		for _, f := range m.claudeMdFiles {
			b.WriteString(fmt.Sprintf("# From %s\n", f.Path))
			b.WriteString(f.Content)
			if !strings.HasSuffix(f.Content, "\n") {
				b.WriteString("\n")
			}
			b.WriteString("\n")
		}
	}

	m.promptViewport.SetContent(b.String())
}

// shortenPath shortens a path for display (replaces home with ~)
func shortenPath(path string) string {
	home, _ := os.UserHomeDir()
	if strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}
