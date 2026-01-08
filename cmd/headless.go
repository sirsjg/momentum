package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/stevegrehan/momentum/agent"
	"github.com/stevegrehan/momentum/client"
	"github.com/stevegrehan/momentum/selection"
	"github.com/stevegrehan/momentum/workflow"
)

var (
	// headless mode flags
	taskID    string
	epicID    string
	projectID string
)

// headlessCmd represents the headless command
var headlessCmd = &cobra.Command{
	Use:   "headless",
	Short: "Run Momentum in headless mode for automation",
	Long: `Run Momentum in headless mode without a user interface.

This mode is designed for automation, scripting, and CI/CD pipelines.
Use flags to specify which project, epic, or task to work with.

If no flags are specified, the newest unblocked todo task across all projects
will be selected automatically.

The selected task will be executed by the Claude Code agent, which will:
1. Mark the task as 'in_progress'
2. Execute the task using Claude Code
3. Mark the task as 'done' on successful completion

Examples:
  # Auto-select newest unblocked todo task from any project
  momentum headless

  # Work with a specific project
  momentum headless --project proj-123

  # Work with a specific epic in a project
  momentum headless --epic epic-456

  # Work with a specific task
  momentum headless --task task-789

  # Combine with custom server URL
  momentum --base-url http://flux.example.com:3000 headless --project myproject`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runHeadless()
	},
}

func init() {
	rootCmd.AddCommand(headlessCmd)

	// Headless mode specific flags
	headlessCmd.Flags().StringVar(&taskID, "task", "", "Task ID to work with")
	headlessCmd.Flags().StringVar(&epicID, "epic", "", "Epic ID to work with")
	headlessCmd.Flags().StringVar(&projectID, "project", "", "Project ID to work with")
}

// runHeadless executes the headless mode logic
func runHeadless() error {
	fmt.Printf("Running in headless mode...\n")
	fmt.Printf("Connecting to Flux server at: %s\n", GetBaseURL())
	fmt.Println()

	// Create the REST client
	c := client.NewClient(GetBaseURL())

	// Create workflow for status updates
	wf := workflow.NewWorkflow(c)

	// Create the selector with the provided filters
	selector := selection.NewSelector(c, projectID, epicID, taskID)

	// Log the selection criteria
	if taskID != "" {
		fmt.Printf("Selection criteria: specific task %s\n", taskID)
	} else if epicID != "" {
		fmt.Printf("Selection criteria: first unblocked todo from epic %s\n", epicID)
	} else if projectID != "" {
		fmt.Printf("Selection criteria: first unblocked todo from project %s\n", projectID)
	} else {
		fmt.Printf("Selection criteria: newest unblocked todo across all projects\n")
	}
	fmt.Println()

	// Select a task
	task, err := selector.SelectTask()
	if err != nil {
		if errors.Is(err, selection.ErrNoTaskAvailable) {
			fmt.Println("No task available matching the selection criteria.")
			return nil
		}
		return fmt.Errorf("failed to select task: %w", err)
	}

	// Print the selected task details
	fmt.Println("Selected task:")
	fmt.Println("==============")
	fmt.Printf("  ID:        %s\n", task.ID)
	fmt.Printf("  Title:     %s\n", task.Title)
	fmt.Printf("  Status:    %s\n", task.Status)
	fmt.Printf("  Blocked:   %t\n", task.Blocked)
	fmt.Printf("  Project:   %s\n", task.ProjectID)
	if task.EpicID != "" {
		fmt.Printf("  Epic:      %s\n", task.EpicID)
	}
	if task.Notes != "" {
		fmt.Printf("  Notes:     %s\n", task.Notes)
	}
	if len(task.DependsOn) > 0 {
		fmt.Printf("  Depends on: %v\n", task.DependsOn)
	}
	fmt.Println()

	// Mark task as in_progress
	fmt.Printf("Starting task %s...\n", task.ID)
	if err := wf.StartWorking([]string{task.ID}); err != nil {
		return fmt.Errorf("failed to start task: %w", err)
	}

	// Build prompt for the agent
	prompt := buildHeadlessPrompt(task)

	// Create and run agent
	fmt.Println("Spawning Claude Code agent...")
	fmt.Println()

	ag := agent.NewClaudeCode(agent.Config{
		WorkDir: ".",
	})

	runner := agent.NewRunner(ag)

	ctx := context.Background()
	if err := runner.Run(ctx, prompt); err != nil {
		return fmt.Errorf("failed to start agent: %w", err)
	}

	// Stream output to console
	go func() {
		for line := range runner.Output() {
			if line.IsStderr {
				fmt.Fprintf(os.Stderr, "%s\n", line.Text)
			} else {
				fmt.Println(line.Text)
			}
		}
	}()

	// Wait for completion
	result := <-runner.Done()

	fmt.Println()
	if result.ExitCode == 0 {
		fmt.Printf("Agent completed successfully in %v\n", result.Duration)

		// Mark task as done
		if err := wf.MarkComplete([]string{task.ID}); err != nil {
			return fmt.Errorf("failed to mark task complete: %w", err)
		}
		fmt.Printf("Task %s marked as done.\n", task.ID)
	} else {
		fmt.Printf("Agent failed with exit code %d\n", result.ExitCode)
		fmt.Printf("Task %s remains in progress for investigation.\n", task.ID)
		if result.Error != nil {
			return result.Error
		}
	}

	return nil
}

// buildHeadlessPrompt constructs the prompt for the agent in headless mode
func buildHeadlessPrompt(task *client.Task) string {
	var b strings.Builder

	b.WriteString("You are working on a task from a project management system.\n\n")

	b.WriteString(fmt.Sprintf("Task ID: %s\n", task.ID))
	b.WriteString(fmt.Sprintf("Task: %s\n", task.Title))

	if task.Notes != "" {
		b.WriteString(fmt.Sprintf("\nDetails:\n%s\n", task.Notes))
	}

	b.WriteString("\nPlease complete this task. When finished, provide a summary of what was done.")

	return b.String()
}
