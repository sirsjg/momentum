package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/stevegrehan/momentum/agent"
	"github.com/stevegrehan/momentum/version"
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
	ID         string
	TaskID     string
	TaskTitle  string
	AgentName  string
	Runner     *agent.Runner
	Output     []agent.OutputLine
	StartTime  time.Time
	EndTime    time.Time // Set when agent completes
	Result     *agent.Result
	ScrollPos  int
	Focused    bool
	Closed     bool
	Stopping   bool // Set when stop is requested but process hasn't exited yet
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
	width  int
	height int

	// Listener state
	listening    bool
	connected    bool
	lastError    error
	criteria     string
	spinner      spinner.Model
	taskCount    int
	lastTaskTime time.Time

	// Agent panels
	panels       []*AgentPanel
	focusedPanel int
	nextPanelID  int

	// Agent updates channel
	agentUpdates chan AgentUpdate
}


// NewModel creates a new TUI model
func NewModel(criteria string) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(Purple)

	return Model{
		criteria:     criteria,
		spinner:      s,
		panels:       make([]*AgentPanel, 0),
		agentUpdates: make(chan AgentUpdate, 100),
	}
}

// Messages
type tickMsg time.Time
type agentUpdateMsg AgentUpdate

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
	)
}

func tickCmd() tea.Cmd {
	return tea.Tick(200*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// Update handles messages
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tickMsg:
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
	}

	return m, nil
}

func (m *Model) addAgentPanel(taskID, taskTitle, agentName string, runner *agent.Runner) {
	m.nextPanelID++
	id := fmt.Sprintf("agent-%d", m.nextPanelID)

	panel := &AgentPanel{
		ID:        id,
		TaskID:    taskID,
		TaskTitle: taskTitle,
		AgentName: agentName,
		Runner:    runner,
		Output:    make([]agent.OutputLine, 0),
		StartTime: time.Now(),
		Focused:   len(m.panels) == 0,
	}

	m.panels = append(m.panels, panel)
	if panel.Focused {
		m.focusedPanel = len(m.panels) - 1
	}
}

func (m *Model) appendAgentOutput(taskID string, line agent.OutputLine) {
	for _, panel := range m.panels {
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
			// Auto-scroll
			visibleLines := m.getPanelHeight() - 4
			if visibleLines < 1 {
				visibleLines = 10
			}
			if len(panel.Output) > visibleLines {
				panel.ScrollPos = len(panel.Output) - visibleLines
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
			return
		}
	}
}

func (m *Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit

	case "tab":
		// Cycle through panels
		if len(m.panels) > 0 {
			// Clear current focus
			if m.focusedPanel >= 0 && m.focusedPanel < len(m.panels) {
				m.panels[m.focusedPanel].Focused = false
			}
			m.focusedPanel = (m.focusedPanel + 1) % len(m.panels)
			m.panels[m.focusedPanel].Focused = true
		}
		return m, nil

	case "shift+tab":
		// Cycle backwards through panels
		if len(m.panels) > 0 {
			if m.focusedPanel >= 0 && m.focusedPanel < len(m.panels) {
				m.panels[m.focusedPanel].Focused = false
			}
			m.focusedPanel = (m.focusedPanel - 1 + len(m.panels)) % len(m.panels)
			m.panels[m.focusedPanel].Focused = true
		}
		return m, nil

	case "x", "c":
		// Close focused panel if finished
		if m.focusedPanel >= 0 && m.focusedPanel < len(m.panels) {
			panel := m.panels[m.focusedPanel]
			if panel.IsFinished() {
				m.panels = append(m.panels[:m.focusedPanel], m.panels[m.focusedPanel+1:]...)
				if m.focusedPanel >= len(m.panels) {
					m.focusedPanel = len(m.panels) - 1
				}
				if m.focusedPanel >= 0 && m.focusedPanel < len(m.panels) {
					m.panels[m.focusedPanel].Focused = true
				}
			}
		}
		return m, nil

	case "s", "esc":
		// Stop focused panel's agent if running
		if m.focusedPanel >= 0 && m.focusedPanel < len(m.panels) {
			panel := m.panels[m.focusedPanel]
			if panel.IsRunning() && panel.Runner != nil && !panel.Stopping {
				panel.Stopping = true
				panel.Runner.Cancel()
			}
		}
		return m, nil

	case "up", "k":
		// Scroll up in focused panel
		if m.focusedPanel >= 0 && m.focusedPanel < len(m.panels) {
			panel := m.panels[m.focusedPanel]
			panel.ScrollPos -= 3
			if panel.ScrollPos < 0 {
				panel.ScrollPos = 0
			}
		}
		return m, nil

	case "down", "j":
		// Scroll down in focused panel
		if m.focusedPanel >= 0 && m.focusedPanel < len(m.panels) {
			panel := m.panels[m.focusedPanel]
			maxScroll := len(panel.Output) - (m.getPanelHeight() - 4)
			if maxScroll < 0 {
				maxScroll = 0
			}
			panel.ScrollPos += 3
			if panel.ScrollPos > maxScroll {
				panel.ScrollPos = maxScroll
			}
		}
		return m, nil
	}

	return m, nil
}

func (m *Model) getPanelHeight() int {
	// Reserve space for header and help
	return (m.height - 8) // Listener panel takes ~4 lines, help takes ~2
}

// View renders the UI
func (m *Model) View() string {
	if m.width == 0 {
		return ""
	}

	var b strings.Builder

	// Header - ASCII art logo
	logo := ` __  __                            _
|  \/  | ___  _ __ ___   ___ _ __ | |_ _   _ _ __ ___
| |\/| |/ _ \| '_ ` + "`" + ` _ \ / _ \ '_ \| __| | | | '_ ` + "`" + ` _ \
| |  | | (_) | | | | | |  __/ | | | |_| |_| | | | | | |
|_|  |_|\___/|_| |_| |_|\___|_| |_|\__|\__,_|_| |_| |_|`
	b.WriteString(LogoStyle.Render(logo))
	b.WriteString("\n")
	b.WriteString(TaglineStyle.Render("keep the board moving"))
	b.WriteString("  ")
	b.WriteString(VersionStyle.Render("v" + version.Short()))
	b.WriteString("\n\n")

	// Listener panel
	b.WriteString(m.renderListenerPanel())
	b.WriteString("\n")

	// Agent panels (tiled)
	if len(m.panels) > 0 {
		b.WriteString(m.renderAgentPanels())
	}
	b.WriteString("\n")

	// Help
	help := HelpKeyStyle.Render("Tab") + HelpStyle.Render(" focus  ") +
		HelpKeyStyle.Render("j/k") + HelpStyle.Render(" scroll  ") +
		HelpKeyStyle.Render("s") + HelpStyle.Render(" stop  ") +
		HelpKeyStyle.Render("x") + HelpStyle.Render(" close  ") +
		HelpKeyStyle.Render("q") + HelpStyle.Render(" quit")
	b.WriteString(help)

	return b.String()
}

func (m *Model) renderListenerPanel() string {
	var status string
	if m.lastError != nil {
		status = StatusError.Render(fmt.Sprintf("Error: %v", m.lastError))
	} else if m.connected {
		status = StatusConnected.Render("Connected") + " " + m.spinner.View() + " " +
			StatusWaiting.Render("Watching for tasks...")
	} else {
		status = m.spinner.View() + " " + StatusWaiting.Render("Connecting...")
	}

	content := fmt.Sprintf("%s\n%s: %s\nTasks completed: %d",
		status,
		lipgloss.NewStyle().Foreground(Gray).Render("Filter"),
		m.criteria,
		m.taskCount,
	)

	return PanelStyle.Width(m.width - 4).Render(content)
}

func (m *Model) renderAgentPanels() string {
	if len(m.panels) == 0 {
		return ""
	}

	panelHeight := m.getPanelHeight()

	// Calculate tile layout
	numPanels := len(m.panels)
	var cols int
	switch {
	case numPanels == 1:
		cols = 1
	case numPanels <= 4:
		cols = 2
	default:
		cols = 3
	}

	panelWidth := (m.width - 4) / cols
	if panelWidth < 40 {
		panelWidth = m.width - 4
		cols = 1
	}

	var rows []string
	for i := 0; i < len(m.panels); i += cols {
		var rowPanels []string
		for j := 0; j < cols && i+j < len(m.panels); j++ {
			panel := m.panels[i+j]
			rendered := m.renderSinglePanel(panel, panelWidth-2, panelHeight)
			rowPanels = append(rowPanels, rendered)
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, rowPanels...))
	}

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func (m *Model) renderSinglePanel(panel *AgentPanel, width, height int) string {
	var b strings.Builder

	// Title with status
	var elapsed time.Duration
	if panel.IsFinished() && !panel.EndTime.IsZero() {
		elapsed = panel.EndTime.Sub(panel.StartTime).Round(time.Second)
	} else {
		elapsed = time.Since(panel.StartTime).Round(time.Second)
	}

	var statusStr string
	if panel.Stopping && panel.IsRunning() {
		statusStr = AgentStopping.Render(" [Stopping...]")
	} else if panel.IsRunning() {
		statusStr = AgentRunning.Render(" [Running]")
	} else if panel.Result != nil {
		if panel.Result.ExitCode == 0 {
			statusStr = AgentCompleted.Render(" [Done]")
		} else if panel.Stopping {
			statusStr = AgentStopped.Render(" [Stopped]")
		} else {
			statusStr = AgentFailed.Render(fmt.Sprintf(" [Failed:%d]", panel.Result.ExitCode))
		}
	}

	title := fmt.Sprintf("%s: %s %s (%s)", panel.AgentName, truncate(panel.TaskTitle, 20), statusStr, elapsed)
	b.WriteString(TitleStyle.Width(width - 4).Render(title))
	b.WriteString("\n")

	// Output lines
	visibleLines := height - 4
	if visibleLines < 1 {
		visibleLines = 1
	}

	startIdx := panel.ScrollPos
	endIdx := startIdx + visibleLines
	if endIdx > len(panel.Output) {
		endIdx = len(panel.Output)
	}

	for i := startIdx; i < endIdx; i++ {
		line := panel.Output[i]
		text := truncate(line.Text, width-6)
		if line.IsStderr {
			b.WriteString(StderrStyle.Render(text))
		} else {
			b.WriteString(OutputStyle.Render(text))
		}
		b.WriteString("\n")
	}

	// Pad remaining lines
	for i := endIdx - startIdx; i < visibleLines; i++ {
		b.WriteString("\n")
	}

	// Close button if finished
	if panel.IsFinished() {
		closeBtn := "[x] Close"
		if panel.Focused {
			b.WriteString(ButtonFocusedStyle.Render(closeBtn))
		} else {
			b.WriteString(ButtonStyle.Render(closeBtn))
		}
	}

	// Choose panel style based on focus
	style := PanelStyle
	if panel.Focused {
		style = FocusedPanelStyle
	}

	return style.Width(width).Height(height).Render(b.String())
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

	panel := &AgentPanel{
		ID:        id,
		TaskID:    taskID,
		TaskTitle: taskTitle,
		AgentName: agentName,
		Runner:    runner,
		Output:    make([]agent.OutputLine, 0),
		StartTime: time.Now(),
		Focused:   len(m.panels) == 0, // Focus first panel
	}

	m.panels = append(m.panels, panel)
	if panel.Focused {
		m.focusedPanel = len(m.panels) - 1
	}

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
