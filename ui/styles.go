package ui

import "github.com/charmbracelet/lipgloss"

// Color palette
var (
	Purple    = lipgloss.Color("#7C3AED")
	Cyan      = lipgloss.Color("#06B6D4")
	Green     = lipgloss.Color("#10B981")
	Amber     = lipgloss.Color("#F59E0B")
	Red       = lipgloss.Color("#EF4444")
	Gray      = lipgloss.Color("#6B7280")
	DarkGray  = lipgloss.Color("#374151")
	LightGray = lipgloss.Color("#9CA3AF")
	White     = lipgloss.Color("#F9FAFB")
)

// Common styles
var (
	// Logo and header
	LogoStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(Purple)

	TaglineStyle = lipgloss.NewStyle().
			Foreground(Gray).
			Italic(true)

	VersionStyle = lipgloss.NewStyle().
			Foreground(DarkGray)

	// Panel styles
	PanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(DarkGray)

	FocusedPanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(Purple)

	// Title styles
	TitleStyle = lipgloss.NewStyle().
			Foreground(White).
			Background(Purple).
			Padding(0, 1).
			Bold(true)

	// Status styles
	StatusConnected = lipgloss.NewStyle().
			Foreground(Green).
			Bold(true)

	StatusWaiting = lipgloss.NewStyle().
			Foreground(Amber)

	StatusError = lipgloss.NewStyle().
			Foreground(Red).
			Bold(true)

	// Agent status styles
	AgentRunning = lipgloss.NewStyle().
			Foreground(Green).
			Bold(true)

	AgentStopping = lipgloss.NewStyle().
			Foreground(Amber).
			Bold(true)

	AgentStopped = lipgloss.NewStyle().
			Foreground(Gray)

	AgentCompleted = lipgloss.NewStyle().
			Foreground(Cyan)

	AgentFailed = lipgloss.NewStyle().
			Foreground(Red).
			Bold(true)

	// Output styles
	OutputStyle = lipgloss.NewStyle().
			Foreground(LightGray)

	StderrStyle = lipgloss.NewStyle().
			Foreground(Amber)

	// Button styles
	ButtonStyle = lipgloss.NewStyle().
			Foreground(White).
			Background(DarkGray).
			Padding(0, 1)

	ButtonFocusedStyle = lipgloss.NewStyle().
				Foreground(White).
				Background(Purple).
				Padding(0, 1).
				Bold(true)

	// Help text
	HelpStyle = lipgloss.NewStyle().
			Foreground(Gray)

	HelpKeyStyle = lipgloss.NewStyle().
			Foreground(Cyan)
)
