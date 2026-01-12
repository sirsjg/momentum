package ui

import "github.com/charmbracelet/lipgloss"

// Color palette
var (
	GlowGreen = lipgloss.Color("#7CFF8A")
	Cyan      = lipgloss.Color("#5EE6FF")
	Green     = lipgloss.Color("#44D27E")
	Amber     = lipgloss.Color("#F4C857")
	Orange    = lipgloss.Color("#F49B5A")
	Red       = lipgloss.Color("#FF6B6B")
	Gray      = lipgloss.Color("#8B949E")
	DarkGray  = lipgloss.Color("#2D333B")
	Charcoal  = lipgloss.Color("#1C2128")
	LightGray = lipgloss.Color("#C9D1D9")
	White     = lipgloss.Color("#F5F7FA")
)

// Common styles
var (
	// Logo and header
	LogoStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(GlowGreen)

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
				BorderForeground(GlowGreen)

	// Title styles
	TitleStyle = lipgloss.NewStyle().
			Foreground(White).
			Background(Charcoal).
			Padding(0, 1).
			Bold(true)

	ListHeaderStyle = lipgloss.NewStyle().
			Foreground(Gray).
			Bold(true)

	SelectedRowStyle = lipgloss.NewStyle().
				Foreground(White).
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
			Foreground(GlowGreen).
			Bold(true)

	AgentFailed = lipgloss.NewStyle().
			Foreground(Red).
			Bold(true)

	ProgressTrackStyle = lipgloss.NewStyle().
				Foreground(DarkGray)

	ProgressPulseStyle = lipgloss.NewStyle().
				Foreground(GlowGreen).
				Bold(true)

	ProgressCompleteStyle = lipgloss.NewStyle().
				Foreground(Green).
				Bold(true)

	ProgressFailedStyle = lipgloss.NewStyle().
				Foreground(Red).
				Bold(true)

	PidStyle = lipgloss.NewStyle().
			Foreground(Gray)

	TaskIDStyle = lipgloss.NewStyle().
			Foreground(Cyan)

	TaskNameStyle = lipgloss.NewStyle().
			Foreground(White)

	TimeStyle = lipgloss.NewStyle().
			Foreground(LightGray)

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
				Background(GlowGreen).
				Padding(0, 1).
				Bold(true)

	// Help text
	HelpStyle = lipgloss.NewStyle().
			Foreground(Gray)

	HelpKeyStyle = lipgloss.NewStyle().
			Foreground(Cyan)

	// Update notification
	UpdateAvailableStyle = lipgloss.NewStyle().
				Foreground(Orange).
				Bold(true)

	ConsoleOverlayStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(GlowGreen)

	ConsoleTitleStyle = lipgloss.NewStyle().
				Foreground(White).
				Background(Charcoal).
				Padding(0, 1).
				Bold(true)
)
