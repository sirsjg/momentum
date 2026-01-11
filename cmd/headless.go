package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stevegrehan/momentum/agent"
	"github.com/stevegrehan/momentum/client"
	"github.com/stevegrehan/momentum/selection"
	"github.com/stevegrehan/momentum/sse"
	"github.com/stevegrehan/momentum/ui"
	"github.com/stevegrehan/momentum/workflow"
)

// sseEventData represents the structure of SSE event payloads
type sseEventData struct {
	Epic *struct {
		Auto bool `json:"auto"`
	} `json:"epic,omitempty"`
}

// runningAgents tracks which tasks have active agents
type runningAgents struct {
	mu      sync.Mutex
	tasks   map[string]bool
	runners map[string]*agent.Runner
}

func newRunningAgents() *runningAgents {
	return &runningAgents{
		tasks:   make(map[string]bool),
		runners: make(map[string]*agent.Runner),
	}
}

func (r *runningAgents) isRunning(taskID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.tasks[taskID]
}

func (r *runningAgents) markRunning(taskID string, runner *agent.Runner) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tasks[taskID] = true
	r.runners[taskID] = runner
}

func (r *runningAgents) markDone(taskID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.tasks, taskID)
	delete(r.runners, taskID)
}

func (r *runningAgents) cancelAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, runner := range r.runners {
		if runner != nil {
			runner.Cancel()
		}
	}
}

// isAutoEpicEvent checks if the SSE event contains an epic with auto=true
func isAutoEpicEvent(event sse.Event) bool {
	var data sseEventData
	if err := json.Unmarshal([]byte(event.Data), &data); err != nil {
		return false
	}
	return data.Epic != nil && data.Epic.Auto
}

var (
	// Task selection flags (defined here, registered in root.go)
	taskID    string
	epicID    string
	projectID string
)

// runHeadless executes the headless mode logic with TUI
func runHeadless() error {
	// Build criteria string for display
	criteria := buildCriteriaString()

	// Create the TUI model
	model := ui.NewModel(criteria)

	// Create the bubbletea program
	p := tea.NewProgram(&model, tea.WithAltScreen())

	// Create context for cancellation
	ctx, cancel := context.WithCancel(context.Background())

	// Track running agents for cleanup
	agents := newRunningAgents()

	// Start the background worker
	go runWorker(ctx, p, agents)

	// Run the TUI
	_, err := p.Run()

	// Cancel all running agents and context on exit
	agents.cancelAll()
	cancel()

	if err != nil {
		return fmt.Errorf("error running UI: %w", err)
	}

	return nil
}

func buildCriteriaString() string {
	if taskID != "" {
		return fmt.Sprintf("Task: %s", taskID)
	}
	if epicID != "" {
		return fmt.Sprintf("Epic: %s", epicID)
	}
	if projectID != "" {
		return fmt.Sprintf("Project: %s", projectID)
	}
	return "All projects"
}

// runWorker runs the background task selection and agent spawning
func runWorker(ctx context.Context, p *tea.Program, agents *runningAgents) {
	// Create the REST client
	c := client.NewClient(GetBaseURL())

	// Create workflow for status updates
	wf := workflow.NewWorkflow(c)

	// Create the selector
	selector := selection.NewSelector(c, projectID, epicID, taskID)

	// Start SSE subscriber
	subscriber := sse.NewSubscriber(GetBaseURL())
	sseEvents := subscriber.Start(ctx)
	defer subscriber.Stop()

	// Signal connected
	p.Send(ui.ListenerConnectedMsg{})

	// Main loop
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Try to select a task
		task, err := selector.SelectTask()
		if err != nil {
			if errors.Is(err, selection.ErrNoTaskAvailable) {
				// Wait for a task to become available (only from auto epics)
				if err := waitForTaskWithSSE(ctx, sseEvents, selector); err != nil {
					if errors.Is(err, context.Canceled) {
						return
					}
					p.Send(ui.ListenerErrorMsg{Err: err})
					time.Sleep(5 * time.Second)
				}
				continue
			}
			p.Send(ui.ListenerErrorMsg{Err: err})
			time.Sleep(5 * time.Second)
			continue
		}

		// Skip if agent already running for this task
		if agents.isRunning(task.ID) {
			time.Sleep(1 * time.Second)
			continue
		}

		// Mark task as in_progress
		if err := wf.StartWorking([]string{task.ID}); err != nil {
			p.Send(ui.ListenerErrorMsg{Err: err})
			continue
		}

		// Spawn agent
		spawnAgent(ctx, p, task, wf, agents)
	}
}

// waitForTaskWithSSE waits for a task to become available using SSE.
// Only processes events where the epic has auto=true.
func waitForTaskWithSSE(ctx context.Context, sseEvents <-chan sse.Event, selector *selection.Selector) error {
	pollTicker := time.NewTicker(5 * time.Second)
	defer pollTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case event, ok := <-sseEvents:
			if !ok {
				continue
			}
			// Only process events from auto-enabled epics
			if !isAutoEpicEvent(event) {
				continue
			}
			if event.Type == "task.created" ||
				event.Type == "task.updated" ||
				event.Type == "task.status_changed" ||
				event.Type == "data-changed" {
				if _, err := selector.SelectTask(); err == nil {
					return nil
				}
			}

		case <-pollTicker.C:
			if _, err := selector.SelectTask(); err == nil {
				return nil
			}
		}
	}
}

// spawnAgent spawns a new agent for the given task
func spawnAgent(ctx context.Context, p *tea.Program, task *client.Task, wf *workflow.Workflow, agents *runningAgents) {
	// Create agent
	ag := agent.NewClaudeCode(agent.Config{
		WorkDir: ".",
	})

	runner := agent.NewRunner(ag)

	// Mark task as having a running agent (with runner reference for cleanup)
	agents.markRunning(task.ID, runner)

	// Build prompt
	prompt := buildHeadlessPrompt(task)

	// Start the agent
	if err := runner.Run(ctx, prompt); err != nil {
		agents.markDone(task.ID)
		p.Send(ui.ListenerErrorMsg{Err: err})
		return
	}

	// Add panel to UI via message
	p.Send(ui.AddAgentMsg{
		TaskID:    task.ID,
		TaskTitle: task.Title,
		AgentName: "Claude",
		Runner:    runner,
	})

	// Stream output in background
	go func() {
		for line := range runner.Output() {
			p.Send(ui.AgentOutputMsg{
				TaskID: task.ID,
				Line:   line,
			})
		}
	}()

	// Wait for completion in background
	go func() {
		result := <-runner.Done()

		// Mark agent as done
		agents.markDone(task.ID)

		p.Send(ui.AgentCompletedMsg{
			TaskID: task.ID,
			Result: result,
		})

		// Update task status
		if result.ExitCode == 0 {
			wf.MarkComplete([]string{task.ID})
		}
		// On failure, leave as in_progress for investigation
	}()
}

// buildHeadlessPrompt constructs the prompt for the agent
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

