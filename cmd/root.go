package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirsjg/momentum/version"
	"github.com/spf13/cobra"
)

var (
	// baseURL is the Flux server base URL
	baseURL       string
	apiKey        string
	pollInterval  time.Duration
	executionMode string
	workDir       string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:     "momentum",
	Short:   "Momentum - Headless agent runner for Flux project management",
	Version: version.Short(),
	Long: `Momentum is a headless agent runner for the Flux project management system.
It watches for tasks and automatically executes them using Claude Code.

Because once the board starts moving, it shouldn't stop.

Examples:
  # Watch for tasks from a specific project
  momentum --project myproject

  # Watch for tasks from a specific epic
  momentum --epic epic-456

  # Work with a specific task
  momentum --task task-789

  # Use a custom Flux server URL
  momentum --base-url http://flux.example.com:3000 --project myproject`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runHeadless()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVar(&baseURL, "base-url", "http://localhost:3000", "Flux server base URL")
	rootCmd.PersistentFlags().StringVar(&apiKey, "api-key", "", "Flux API key (sent as a Bearer token)")
	rootCmd.PersistentFlags().DurationVar(&pollInterval, "poll-interval", 5*time.Second, "REST polling interval when waiting for tasks")

	// Task selection flags (on root command now)
	rootCmd.Flags().StringVar(&taskID, "task", "", "Specific task ID to work with")
	rootCmd.Flags().StringVar(&epicID, "epic", "", "Filter tasks by epic ID")
	rootCmd.Flags().StringVar(&projectID, "project", "", "Filter tasks by project ID")
	rootCmd.Flags().StringVar(&executionMode, "execution-mode", "async", "Task execution mode: async or sync")
	rootCmd.Flags().StringVar(&workDir, "workdir", "", "Working directory for agents (inherits CLAUDE.md)")
}

// GetBaseURL returns the configured base URL for the Flux server
func GetBaseURL() string {
	return baseURL
}

// GetAPIKey returns the configured Flux API key.
func GetAPIKey() string {
	return apiKey
}

// GetWorkDir returns the current workdir setting
func GetWorkDir() string {
	return workDir
}

// SetWorkDir allows runtime updates from TUI
func SetWorkDir(dir string) {
	workDir = dir
}

// InitWorkDir sets initial workdir from CLI flag > env var > "."
func InitWorkDir() {
	if workDir != "" {
		return // CLI flag already set
	}
	if dir := os.Getenv("MOMENTUM_WORKDIR"); dir != "" {
		workDir = expandHome(dir)
		return
	}
	workDir = "."
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}
