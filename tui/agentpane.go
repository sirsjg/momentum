package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/stevegrehan/momentum/agent"
)

// Agent pane styles
var (
	agentPaneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(cyan)

	agentTitleStyle = lipgloss.NewStyle().
			Foreground(white).
			Background(cyan).
			Padding(0, 1).
			Bold(true)

	agentOutputStyle = lipgloss.NewStyle().
				Foreground(lightGray)

	agentStderrStyle = lipgloss.NewStyle().
				Foreground(amber)

	agentRunningStyle = lipgloss.NewStyle().
				Foreground(green).
				Bold(true)

	agentCompletedStyle = lipgloss.NewStyle().
				Foreground(gray)

	agentFailedStyle = lipgloss.NewStyle().
				Foreground(red).
				Bold(true)
)

// AgentState represents the current state of the agent pane
type AgentState struct {
	Runner      *agent.Runner
	Output      []agent.OutputLine
	TaskID      string
	TaskTitle   string
	PaneHeight  int
	PaneOpen    bool
	ScrollPos   int
	LastResult  *agent.Result
}

// NewAgentState creates a new agent state with defaults
func NewAgentState() *AgentState {
	return &AgentState{
		PaneHeight: 12,
		Output:     make([]agent.OutputLine, 0),
	}
}

// IsRunning returns whether an agent is currently running
func (s *AgentState) IsRunning() bool {
	return s.Runner != nil && s.Runner.IsRunning()
}

// Clear resets the agent state for a new run
func (s *AgentState) Clear() {
	s.Output = make([]agent.OutputLine, 0)
	s.ScrollPos = 0
	s.LastResult = nil
}

// AppendOutput adds a new output line and auto-scrolls
func (s *AgentState) AppendOutput(line agent.OutputLine) {
	s.Output = append(s.Output, line)
	// Auto-scroll to bottom
	visibleLines := s.PaneHeight - 3
	if len(s.Output) > visibleLines {
		s.ScrollPos = len(s.Output) - visibleLines
	}
}

// ScrollUp scrolls the output up by the given amount
func (s *AgentState) ScrollUp(amount int) {
	s.ScrollPos -= amount
	if s.ScrollPos < 0 {
		s.ScrollPos = 0
	}
}

// ScrollDown scrolls the output down by the given amount
func (s *AgentState) ScrollDown(amount int) {
	maxScroll := len(s.Output) - (s.PaneHeight - 3)
	if maxScroll < 0 {
		maxScroll = 0
	}
	s.ScrollPos += amount
	if s.ScrollPos > maxScroll {
		s.ScrollPos = maxScroll
	}
}

// RenderAgentPane renders the agent output pane
func RenderAgentPane(state *AgentState, width int) string {
	if !state.PaneOpen {
		return ""
	}

	var b strings.Builder

	// Title bar with status
	var statusIndicator string
	if state.IsRunning() {
		statusIndicator = agentRunningStyle.Render(" [Running]")
	} else if state.LastResult != nil {
		if state.LastResult.ExitCode == 0 {
			statusIndicator = agentCompletedStyle.Render(" [Completed]")
		} else {
			statusIndicator = agentFailedStyle.Render(fmt.Sprintf(" [Failed: exit %d]", state.LastResult.ExitCode))
		}
	}

	title := fmt.Sprintf("Agent: %s%s", state.TaskTitle, statusIndicator)
	titleBar := agentTitleStyle.Width(width - 6).Render(title)
	b.WriteString(titleBar)
	b.WriteString("\n")

	// Output lines with scrolling
	visibleLines := state.PaneHeight - 3 // Account for title and borders
	if visibleLines < 1 {
		visibleLines = 1
	}

	startIdx := state.ScrollPos
	endIdx := startIdx + visibleLines

	if startIdx > len(state.Output) {
		startIdx = len(state.Output)
	}
	if endIdx > len(state.Output) {
		endIdx = len(state.Output)
	}

	for i := startIdx; i < endIdx; i++ {
		line := state.Output[i]
		style := agentOutputStyle
		if line.IsStderr {
			style = agentStderrStyle
		}
		text := truncateString(line.Text, width-8)
		b.WriteString(style.Render(text))
		b.WriteString("\n")
	}

	// Pad remaining space
	for i := endIdx - startIdx; i < visibleLines; i++ {
		b.WriteString("\n")
	}

	// Add scroll indicator if needed
	if len(state.Output) > visibleLines {
		scrollInfo := fmt.Sprintf("Lines %d-%d of %d (PgUp/PgDn to scroll)",
			startIdx+1, endIdx, len(state.Output))
		b.WriteString(lipgloss.NewStyle().Foreground(gray).Render(scrollInfo))
	}

	return agentPaneStyle.Width(width - 4).Height(state.PaneHeight).Render(b.String())
}

func truncateString(s string, maxLen int) string {
	if maxLen <= 3 {
		return s
	}
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
