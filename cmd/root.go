package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/sirsjg/momentum/version"
)

var (
	// baseURL is the Flux server base URL
	baseURL       string
	executionMode string
	prdContext    bool
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
  momentum --base-url http://flux.example.com:3589 --project myproject`,
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
	rootCmd.PersistentFlags().StringVar(&baseURL, "base-url", "http://localhost:3589", "Flux server base URL")

	// Task selection flags (on root command now)
	rootCmd.Flags().StringVar(&taskID, "task", "", "Specific task ID to work with")
	rootCmd.Flags().StringVar(&epicID, "epic", "", "Filter tasks by epic ID")
	rootCmd.Flags().StringVar(&projectID, "project", "", "Filter tasks by project ID")
	rootCmd.Flags().StringVar(&executionMode, "execution-mode", "async", "Task execution mode: async or sync")

	// Context flags
	rootCmd.Flags().BoolVar(&prdContext, "prd-context", true, "Include PRD context in agent prompts (problem, business rules, notes)")
}

// GetBaseURL returns the configured base URL for the Flux server
func GetBaseURL() string {
	return baseURL
}

// exitWithError prints an error message to stderr and exits with code 1
func exitWithError(msg string) {
	fmt.Fprintln(os.Stderr, "Error:", msg)
	os.Exit(1)
}
