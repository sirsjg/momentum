package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sirsjg/momentum/agent"
	"github.com/sirsjg/momentum/client"
	"github.com/sirsjg/momentum/selection"
	"github.com/sirsjg/momentum/sse"
	"github.com/sirsjg/momentum/ui"
	"github.com/sirsjg/momentum/workflow"
)

// sseEventData represents the structure of SSE event payloads
type sseEventData struct {
	Epic *struct {
		Auto bool `json:"auto"`
	} `json:"epic,omitempty"`
}

// runningAgents tracks which tasks have active agents
type runningAgents struct {
	mu            sync.Mutex
	tasks         map[string]bool
	runners       map[string]*agent.Runner
	stoppedByUser map[string]bool
	doneCh        chan string
}

func newRunningAgents() *runningAgents {
	return &runningAgents{
		tasks:         make(map[string]bool),
		runners:       make(map[string]*agent.Runner),
		stoppedByUser: make(map[string]bool),
		doneCh:        make(chan string, 100),
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
	delete(r.stoppedByUser, taskID)
	select {
	case r.doneCh <- taskID:
	default:
	}
}

func (r *runningAgents) markStoppedByUser(taskID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stoppedByUser[taskID] = true
}

func (r *runningAgents) wasStoppedByUser(taskID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.stoppedByUser[taskID]
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

func (r *runningAgents) hasRunning() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.tasks) > 0
}

func (r *runningAgents) done() <-chan string {
	return r.doneCh
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
	log.SetOutput(io.Discard)

	mode, err := parseExecutionMode(executionMode)
	if err != nil {
		return err
	}

	// Build criteria string for display
	criteria := buildCriteriaString()

	// Create the TUI model
	modeUpdates := make(chan ui.ExecutionMode, 10)
	stopUpdates := make(chan string, 10)
	model := ui.NewModel(criteria, mode, modeUpdates, stopUpdates)

	// Create the bubbletea program
	p := tea.NewProgram(&model, tea.WithAltScreen())

	// Create context for cancellation
	ctx, cancel := context.WithCancel(context.Background())

	// Track running agents for cleanup
	agents := newRunningAgents()

	// Start the background worker
	go runWorker(ctx, p, agents, mode, modeUpdates, stopUpdates)

	// Run the TUI
	_, err = p.Run()

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

func parseExecutionMode(value string) (ui.ExecutionMode, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "async":
		return ui.ExecutionModeAsync, nil
	case "sync":
		return ui.ExecutionModeSync, nil
	default:
		return ui.ExecutionModeAsync, fmt.Errorf("invalid execution mode %q (use async or sync)", value)
	}
}

// runWorker runs the background task selection and agent spawning
func runWorker(ctx context.Context, p *tea.Program, agents *runningAgents, mode ui.ExecutionMode, modeUpdates <-chan ui.ExecutionMode, stopUpdates <-chan string) {
	// Create the REST client
	c := client.NewClient(GetBaseURL())

	// Create workflow for status updates
	wf := workflow.NewWorkflow(c)
	wf.SetOutput(io.Discard)

	// Create the selector
	selector := selection.NewSelector(c, projectID, epicID, taskID)

	// Start SSE subscriber
	subscriber := sse.NewSubscriber(GetBaseURL())
	sseEvents := subscriber.Start(ctx)
	defer subscriber.Stop()

	// Signal connected
	p.Send(ui.ListenerConnectedMsg{})

	// Process stop requests even when the main loop blocks waiting for SSE.
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case taskID := <-stopUpdates:
				agents.markStoppedByUser(taskID)
			}
		}
	}()

	pending := make([]*client.Task, 0)
	queued := make(map[string]bool)

	startTask := func(task *client.Task) {
		delete(queued, task.ID)
		if err := wf.StartWorking([]string{task.ID}); err != nil {
			p.Send(ui.ListenerErrorMsg{Err: err})
			return
		}
		spawnAgent(ctx, p, c, task, wf, agents)
	}

	queueTask := func(task *client.Task) {
		if queued[task.ID] {
			return
		}
		queued[task.ID] = true
		pending = append(pending, task)
	}

	startNextPending := func() {
		if len(pending) == 0 || agents.hasRunning() {
			return
		}
		next := pending[0]
		pending = pending[1:]
		startTask(next)
	}

	startAllPending := func() {
		if len(pending) == 0 {
			return
		}
		tasks := pending
		pending = nil
		for _, task := range tasks {
			startTask(task)
		}
	}

	// Main loop
	for {
		select {
		case <-ctx.Done():
			return
		case <-agents.done():
		case newMode := <-modeUpdates:
			mode = newMode
			if mode == ui.ExecutionModeAsync {
				startAllPending()
			}
		default:
		}

		if mode == ui.ExecutionModeSync && len(pending) > 0 && !agents.hasRunning() {
			startNextPending()
			time.Sleep(250 * time.Millisecond)
			continue
		}

		// Try to select a task
		task, err := selector.SelectTaskExcluding(queued)
		if err != nil {
			if errors.Is(err, selection.ErrNoTaskAvailable) {
				if len(pending) > 0 {
					time.Sleep(250 * time.Millisecond)
					continue
				}
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

		if mode == ui.ExecutionModeSync && agents.hasRunning() {
			queueTask(task)
			time.Sleep(250 * time.Millisecond)
			continue
		}

		// Skip if agent already running for this task
		if agents.isRunning(task.ID) {
			time.Sleep(1 * time.Second)
			continue
		}

		if mode == ui.ExecutionModeSync {
			queueTask(task)
			startNextPending()
			continue
		}

		startTask(task)
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
func spawnAgent(ctx context.Context, p *tea.Program, c *client.Client, task *client.Task, wf *workflow.Workflow, agents *runningAgents) {
	// Create agent
	ag := agent.NewClaudeCode(agent.Config{
		WorkDir: ".",
	})

	runner := agent.NewRunner(ag)

	// Mark task as having a running agent (with runner reference for cleanup)
	agents.markRunning(task.ID, runner)

	// Fetch full task context if PRD context is enabled (non-fatal if it fails)
	var taskCtx *client.TaskContext
	if prdContext {
		var err error
		taskCtx, err = c.GetTaskContext(task.ID)
		if err != nil {
			// Log warning but continue - agent will work with basic task info
			p.Send(ui.ListenerErrorMsg{Err: fmt.Errorf("PRD context unavailable (continuing without): %w", err)})
		}
	}

	// Build prompt with context
	prompt := buildHeadlessPrompt(task, taskCtx)

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

		// Check if stopped by user before marking done (which clears the flag)
		stoppedByUser := agents.wasStoppedByUser(task.ID)

		// Mark agent as done
		agents.markDone(task.ID)

		p.Send(ui.AgentCompletedMsg{
			TaskID: task.ID,
			Result: result,
		})

		// Update task status
		if stoppedByUser {
			// User stopped the agent, reset task to planning
			wf.ResetToPlanning([]string{task.ID})
		} else if result.ExitCode == 0 {
			wf.MarkComplete([]string{task.ID})
		}
		// On failure (not stopped by user), leave as in_progress for investigation
	}()
}

// buildHeadlessPrompt constructs the prompt for the agent
func buildHeadlessPrompt(task *client.Task, ctx *client.TaskContext) string {
	var b strings.Builder

	b.WriteString(`Goal: complete a single Flux task end-to-end, verify it works, and mark the task as done in Flux.

Process:
1) Find the task to work on (use the given task ID/title, or select the highest-priority todo in the target project).
2) Inspect relevant files; keep changes minimal and aligned with existing patterns.
3) Implement the task.
4) Verify the change:
   - If task has a verification command (shown below), run it via MCP: mcp__flux__run_task_verify with {"task_id":"<id>"}
   - Otherwise prefer existing tests/scripts. If none, run a reasonable check (build/typecheck or a minimal manual check).
   - Report what you ran and the result.
   - Add a comment to the task via MCP using mcp__flux__add_task_comment.
     Example: {"task_id":"<id>","body":"What you did + verification results + any notes."}
5) Mark the task as done using Flux MCP (mcp__flux__move_task_status with status "done") and mention the task ID in your final message.

Constraints:
- Do not modify unrelated files.
- Do not reset/revert unrelated git changes.
- Be concise in explanations.

If anything blocks completion, stop and report the blocker instead of guessing, and set the task status back to "planning", and add a comment explaining the issue.

Task context:
`)

	b.WriteString(fmt.Sprintf("- Task ID: %s\n", task.ID))
	b.WriteString(fmt.Sprintf("- Task: %s\n", task.Title))

	if task.Notes != "" {
		b.WriteString(fmt.Sprintf("- Details:\n%s\n", task.Notes))
	}

	// Include verification command if set
	if task.Verify != "" {
		b.WriteString(fmt.Sprintf("- Verification command: %s\n", task.Verify))
		b.WriteString("  (Run via mcp__flux__run_task_verify after implementation)\n")
	}

	// Include acceptance criteria if available
	if len(task.AcceptanceCriteria) > 0 {
		b.WriteString("\n## Acceptance Criteria\n")
		for _, ac := range task.AcceptanceCriteria {
			b.WriteString(fmt.Sprintf("- %s\n", ac))
		}
	}

	// Include guardrails if available (numbered constraints)
	if len(task.Guardrails) > 0 {
		b.WriteString("\n## Guardrails (must follow)\n")
		for _, g := range task.Guardrails {
			b.WriteString(fmt.Sprintf("- [%d] %s\n", g.Number, g.Text))
		}
	}

	// Include linked requirements from PRD if context available
	if ctx != nil && len(ctx.LinkedRequirements) > 0 {
		b.WriteString("\n## Linked Requirements\n")
		for _, req := range ctx.LinkedRequirements {
			b.WriteString(fmt.Sprintf("- %s [%s]: %s\n", req.ID, req.Priority, req.Description))
			if req.Acceptance != "" {
				b.WriteString(fmt.Sprintf("  Acceptance: %s\n", req.Acceptance))
			}
		}
	}

	// Include phase info
	if ctx != nil && ctx.Phase != nil {
		b.WriteString(fmt.Sprintf("\n## Phase: %s - %s\n", ctx.Phase.ID, ctx.Phase.Name))
	}

	// Include relevant PRD context (summary, problem, relevant business rules)
	if ctx != nil && ctx.Prd != nil {
		prd := ctx.Prd
		b.WriteString("\n## Project PRD Context\n")
		if prd.Summary != "" {
			b.WriteString(fmt.Sprintf("Summary: %s\n", prd.Summary))
		}
		b.WriteString(fmt.Sprintf("Problem: %s\n", prd.Problem))

		// Include relevant business rules if any
		if len(prd.BusinessRules) > 0 {
			b.WriteString("\nBusiness Rules:\n")
			for _, br := range prd.BusinessRules {
				scope := ""
				if br.Scope != "" {
					scope = fmt.Sprintf(" [%s]", br.Scope)
				}
				b.WriteString(fmt.Sprintf("- %s%s: %s\n", br.ID, scope, br.Description))
			}
		}

		// Include unresolved open questions that might be relevant
		unresolvedQuestions := 0
		for _, q := range prd.OpenQuestions {
			if q.Resolved == "" {
				unresolvedQuestions++
			}
		}
		if unresolvedQuestions > 0 {
			b.WriteString(fmt.Sprintf("\nNote: %d open questions in PRD remain unresolved\n", unresolvedQuestions))
		}

		// Include session notes (previous learnings)
		if prd.Notes != "" {
			b.WriteString("\n## Session Notes (previous learnings)\n")
			b.WriteString(prd.Notes)
			b.WriteString("\n")
		}
	}

	// Instruction to save decisions
	if ctx != nil && ctx.Prd != nil {
		b.WriteString(`
If you discover important decisions, blockers, or learnings that future tasks should know,
save them with: mcp__flux__add_prd_note(project_id, note)
`)
	}

	return b.String()
}
