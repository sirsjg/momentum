package cmd

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sirsjg/momentum/client"
	"github.com/sirsjg/momentum/sse"
)

func TestNewRunningAgents(t *testing.T) {
	agents := newRunningAgents()
	if agents == nil {
		t.Fatal("newRunningAgents returned nil")
	}
	if agents.tasks == nil {
		t.Error("tasks map not initialized")
	}
}

func TestRunningAgents_MarkRunningAndIsRunning(t *testing.T) {
	agents := newRunningAgents()

	// Initially should not be running
	if agents.isRunning("task-1") {
		t.Error("expected task-1 to not be running initially")
	}

	// Mark as running (nil runner for unit tests)
	agents.markRunning("task-1", nil)
	if !agents.isRunning("task-1") {
		t.Error("expected task-1 to be running after markRunning")
	}

	// Other tasks should still not be running
	if agents.isRunning("task-2") {
		t.Error("expected task-2 to not be running")
	}
}

func TestRunningAgents_MarkDone(t *testing.T) {
	agents := newRunningAgents()

	agents.markRunning("task-1", nil)
	if !agents.isRunning("task-1") {
		t.Fatal("expected task-1 to be running")
	}

	agents.markDone("task-1")
	if agents.isRunning("task-1") {
		t.Error("expected task-1 to not be running after markDone")
	}
}

func TestRunningAgents_MarkDoneNonExistent(t *testing.T) {
	agents := newRunningAgents()

	// Should not panic when marking non-existent task as done
	agents.markDone("non-existent")
	if agents.isRunning("non-existent") {
		t.Error("expected non-existent task to not be running")
	}
}

func TestRunningAgents_MultipleTasks(t *testing.T) {
	agents := newRunningAgents()

	agents.markRunning("task-1", nil)
	agents.markRunning("task-2", nil)
	agents.markRunning("task-3", nil)

	if !agents.isRunning("task-1") || !agents.isRunning("task-2") || !agents.isRunning("task-3") {
		t.Error("expected all tasks to be running")
	}

	agents.markDone("task-2")
	if !agents.isRunning("task-1") || agents.isRunning("task-2") || !agents.isRunning("task-3") {
		t.Error("only task-2 should be done")
	}
}

func TestRunningAgents_ConcurrentAccess(t *testing.T) {
	agents := newRunningAgents()
	var wg sync.WaitGroup

	// Spawn multiple goroutines marking and checking tasks
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			taskID := "task-" + string(rune('a'+id%26))
			agents.markRunning(taskID, nil)
			agents.isRunning(taskID)
			agents.markDone(taskID)
		}(i)
	}

	wg.Wait()
}

func TestIsAutoEpicEvent_AutoTrue(t *testing.T) {
	event := sse.Event{
		Type: "task.created",
		Data: `{"epic": {"auto": true}}`,
	}
	if !isAutoEpicEvent(event) {
		t.Error("expected event with epic.auto=true to return true")
	}
}

func TestIsAutoEpicEvent_AutoFalse(t *testing.T) {
	event := sse.Event{
		Type: "task.created",
		Data: `{"epic": {"auto": false}}`,
	}
	if isAutoEpicEvent(event) {
		t.Error("expected event with epic.auto=false to return false")
	}
}

func TestIsAutoEpicEvent_NoEpicField(t *testing.T) {
	event := sse.Event{
		Type: "task.created",
		Data: `{"task": {"id": "123"}}`,
	}
	if isAutoEpicEvent(event) {
		t.Error("expected event without epic field to return false")
	}
}

func TestIsAutoEpicEvent_EmptyEpic(t *testing.T) {
	event := sse.Event{
		Type: "task.created",
		Data: `{"epic": {}}`,
	}
	if isAutoEpicEvent(event) {
		t.Error("expected event with empty epic to return false")
	}
}

func TestIsAutoEpicEvent_InvalidJSON(t *testing.T) {
	event := sse.Event{
		Type: "task.created",
		Data: `not valid json`,
	}
	if isAutoEpicEvent(event) {
		t.Error("expected invalid JSON to return false")
	}
}

func TestIsAutoEpicEvent_EmptyData(t *testing.T) {
	event := sse.Event{
		Type: "task.created",
		Data: "",
	}
	if isAutoEpicEvent(event) {
		t.Error("expected empty data to return false")
	}
}

func TestIsAutoEpicEvent_NullEpic(t *testing.T) {
	event := sse.Event{
		Type: "task.created",
		Data: `{"epic": null}`,
	}
	if isAutoEpicEvent(event) {
		t.Error("expected null epic to return false")
	}
}

func TestBuildCriteriaString_TaskID(t *testing.T) {
	// Save and restore package variables
	oldTaskID, oldEpicID, oldProjectID := taskID, epicID, projectID
	defer func() {
		taskID, epicID, projectID = oldTaskID, oldEpicID, oldProjectID
	}()

	taskID = "task-123"
	epicID = ""
	projectID = ""

	result := buildCriteriaString()
	expected := "Task: task-123"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestBuildCriteriaString_EpicID(t *testing.T) {
	oldTaskID, oldEpicID, oldProjectID := taskID, epicID, projectID
	defer func() {
		taskID, epicID, projectID = oldTaskID, oldEpicID, oldProjectID
	}()

	taskID = ""
	epicID = "epic-456"
	projectID = ""

	result := buildCriteriaString()
	expected := "Epic: epic-456"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestBuildCriteriaString_ProjectID(t *testing.T) {
	oldTaskID, oldEpicID, oldProjectID := taskID, epicID, projectID
	defer func() {
		taskID, epicID, projectID = oldTaskID, oldEpicID, oldProjectID
	}()

	taskID = ""
	epicID = ""
	projectID = "project-789"

	result := buildCriteriaString()
	expected := "Project: project-789"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestBuildCriteriaString_NoFilters(t *testing.T) {
	oldTaskID, oldEpicID, oldProjectID := taskID, epicID, projectID
	defer func() {
		taskID, epicID, projectID = oldTaskID, oldEpicID, oldProjectID
	}()

	taskID = ""
	epicID = ""
	projectID = ""

	result := buildCriteriaString()
	expected := "All projects"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestBuildCriteriaString_PriorityOrder(t *testing.T) {
	// Task ID takes priority over epic and project
	oldTaskID, oldEpicID, oldProjectID := taskID, epicID, projectID
	defer func() {
		taskID, epicID, projectID = oldTaskID, oldEpicID, oldProjectID
	}()

	taskID = "task-123"
	epicID = "epic-456"
	projectID = "project-789"

	result := buildCriteriaString()
	if result != "Task: task-123" {
		t.Errorf("expected task ID to take priority, got %q", result)
	}

	// Epic takes priority over project
	taskID = ""
	result = buildCriteriaString()
	if result != "Epic: epic-456" {
		t.Errorf("expected epic ID to take priority over project, got %q", result)
	}
}

func TestBuildHeadlessPrompt_BasicTask(t *testing.T) {
	task := &client.Task{
		ID:    "task-123",
		Title: "Fix the bug",
	}

	result := buildHeadlessPrompt(task)

	if !contains(result, "Task ID: task-123") {
		t.Error("prompt should contain task ID")
	}
	if !contains(result, "Task: Fix the bug") {
		t.Error("prompt should contain task title")
	}
	if !contains(result, "Goal: complete a single Flux task") {
		t.Error("prompt should contain goal instruction")
	}
}

func TestBuildHeadlessPrompt_WithNotes(t *testing.T) {
	task := &client.Task{
		ID:    "task-123",
		Title: "Fix the bug",
		Notes: "The bug is in the auth module. Check line 42.",
	}

	result := buildHeadlessPrompt(task)

	if !contains(result, "Details:") {
		t.Error("prompt should contain Details section")
	}
	if !contains(result, "The bug is in the auth module") {
		t.Error("prompt should contain notes content")
	}
}

func TestBuildHeadlessPrompt_EmptyNotes(t *testing.T) {
	task := &client.Task{
		ID:    "task-123",
		Title: "Fix the bug",
		Notes: "",
	}

	result := buildHeadlessPrompt(task)

	if contains(result, "Details:") {
		t.Error("prompt should not contain Details section when notes are empty")
	}
}

func TestBuildHeadlessPrompt_WithAcceptanceCriteria(t *testing.T) {
	task := &client.Task{
		ID:    "task-123",
		Title: "Implement feature",
		AcceptanceCriteria: []string{
			"Feature works correctly",
			"Tests pass",
		},
	}

	result := buildHeadlessPrompt(task)

	if !contains(result, "Acceptance Criteria:") {
		t.Error("prompt should contain Acceptance Criteria section")
	}
	if !contains(result, "- [ ] Feature works correctly") {
		t.Error("prompt should contain first criterion as checkbox")
	}
	if !contains(result, "- [ ] Tests pass") {
		t.Error("prompt should contain second criterion as checkbox")
	}
}

func TestBuildHeadlessPrompt_WithGuardrails(t *testing.T) {
	task := &client.Task{
		ID:    "task-123",
		Title: "Implement feature",
		Guardrails: []client.Guardrail{
			{ID: "g1", Number: 1, Text: "Low priority rule"},
			{ID: "g2", Number: 10, Text: "Critical rule"},
			{ID: "g3", Number: 5, Text: "Medium priority rule"},
		},
	}

	result := buildHeadlessPrompt(task)

	if !contains(result, "Guardrails:") {
		t.Error("prompt should contain Guardrails section")
	}
	critIdx := strings.Index(result, "Critical rule")
	medIdx := strings.Index(result, "Medium priority rule")
	lowIdx := strings.Index(result, "Low priority rule")
	if critIdx == -1 || medIdx == -1 || lowIdx == -1 {
		t.Fatal("prompt should contain all guardrail texts")
	}
	if critIdx > medIdx || medIdx > lowIdx {
		t.Error("guardrails should be sorted by number descending")
	}
}

func TestBuildHeadlessPrompt_EmptyAcceptanceCriteria(t *testing.T) {
	task := &client.Task{
		ID:                 "task-123",
		Title:              "Fix bug",
		AcceptanceCriteria: []string{},
	}

	result := buildHeadlessPrompt(task)

	if contains(result, "Acceptance Criteria:") {
		t.Error("prompt should not contain Acceptance Criteria when empty")
	}
}

func TestBuildHeadlessPrompt_EmptyGuardrails(t *testing.T) {
	task := &client.Task{
		ID:         "task-123",
		Title:      "Fix bug",
		Guardrails: []client.Guardrail{},
	}

	result := buildHeadlessPrompt(task)

	if contains(result, "Guardrails:") {
		t.Error("prompt should not contain Guardrails when empty")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// =============================================================================
// SSE Reconnection Timing Tests
// =============================================================================
// These tests verify behavior when SSE disconnects/reconnects while tasks are
// queued or running, ensuring no duplicate task starts occur.

// TestSSEReconnect_NoDuplicateStartsForRunningTasks verifies that isRunning()
// check prevents duplicate task starts when SSE reconnects and sends duplicate events.
func TestSSEReconnect_NoDuplicateStartsForRunningTasks(t *testing.T) {
	agents := newRunningAgents()

	// Simulate a task already running (started before reconnect)
	agents.markRunning("task-1", nil)

	// After SSE reconnect, the same task event might arrive again
	// The isRunning check should prevent duplicate starts
	if !agents.isRunning("task-1") {
		t.Error("task should still be marked as running after simulated reconnect")
	}

	// Verify the pattern used in runWorker: check before starting
	shouldStart := !agents.isRunning("task-1")
	if shouldStart {
		t.Error("should not start task that is already running")
	}
}

// TestSSEReconnect_QueuedTasksPreservedDuringReconnect verifies that the queued
// map state is preserved and tasks aren't re-queued during reconnect.
func TestSSEReconnect_QueuedTasksPreservedDuringReconnect(t *testing.T) {
	// Simulate the queued map used in runWorker
	queued := make(map[string]bool)

	// Queue a task before "reconnect"
	queued["task-1"] = true
	queued["task-2"] = true

	// Simulate SSE reconnect - queued state should persist in memory
	// and prevent duplicate queueing
	if !queued["task-1"] {
		t.Error("task-1 should still be in queue after reconnect")
	}
	if !queued["task-2"] {
		t.Error("task-2 should still be in queue after reconnect")
	}

	// Simulate the queueTask pattern - should not re-queue
	queueTask := func(taskID string) bool {
		if queued[taskID] {
			return false // Already queued
		}
		queued[taskID] = true
		return true
	}

	if queueTask("task-1") {
		t.Error("should not re-queue task-1")
	}
	if queueTask("task-2") {
		t.Error("should not re-queue task-2")
	}

	// New task should be queued
	if !queueTask("task-3") {
		t.Error("should queue new task-3")
	}
}

// TestSSEReconnect_ConcurrentMarkRunningIdempotent tests that concurrent calls
// to markRunning for the same task (simulating duplicate events from reconnect)
// are handled safely.
func TestSSEReconnect_ConcurrentMarkRunningIdempotent(t *testing.T) {
	agents := newRunningAgents()
	var wg sync.WaitGroup

	// Simulate multiple goroutines trying to mark the same task as running
	// (as might happen with duplicate events after SSE reconnect)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			agents.markRunning("task-1", nil)
		}()
	}

	wg.Wait()

	// Should still be running (no corruption)
	if !agents.isRunning("task-1") {
		t.Error("task should be running after concurrent markRunning calls")
	}

	// Mark done once
	agents.markDone("task-1")

	if agents.isRunning("task-1") {
		t.Error("task should not be running after markDone")
	}
}

// TestSSEReconnect_HasRunningPersistsDuringReconnect verifies that hasRunning()
// correctly reports running tasks through a simulated reconnect.
func TestSSEReconnect_HasRunningPersistsDuringReconnect(t *testing.T) {
	agents := newRunningAgents()

	if agents.hasRunning() {
		t.Error("should have no running tasks initially")
	}

	// Start a task before "reconnect"
	agents.markRunning("task-1", nil)

	if !agents.hasRunning() {
		t.Error("should have running task")
	}

	// Simulate reconnect - hasRunning should still work
	if !agents.hasRunning() {
		t.Error("hasRunning should persist through reconnect")
	}

	// In sync mode, this prevents starting new tasks
	if agents.hasRunning() {
		// Should wait, not start new task
	}

	agents.markDone("task-1")

	if agents.hasRunning() {
		t.Error("should have no running tasks after markDone")
	}
}

// TestSSEReconnect_DoneChannelNotBlockedOnRapidCompletions tests that rapid
// task completions (as might happen when catching up after reconnect) don't
// block due to channel buffer.
func TestSSEReconnect_DoneChannelNotBlockedOnRapidCompletions(t *testing.T) {
	agents := newRunningAgents()

	// Start many tasks
	for i := 0; i < 50; i++ {
		agents.markRunning(string(rune('a'+i)), nil)
	}

	// Complete them all rapidly (simulates catching up after reconnect)
	done := make(chan bool)
	go func() {
		for i := 0; i < 50; i++ {
			agents.markDone(string(rune('a' + i)))
		}
		done <- true
	}()

	// Should complete without blocking (channel has capacity 100)
	select {
	case <-done:
		// Success
	case <-timeAfter(2):
		t.Error("markDone blocked - channel buffer overflow")
	}
}

// TestSSEReconnect_MultipleRunningTasksTrackedCorrectly tests that multiple
// running tasks are all tracked correctly through a reconnect scenario.
func TestSSEReconnect_MultipleRunningTasksTrackedCorrectly(t *testing.T) {
	agents := newRunningAgents()

	// Start multiple tasks before "reconnect"
	tasks := []string{"task-a", "task-b", "task-c", "task-d"}
	for _, taskID := range tasks {
		agents.markRunning(taskID, nil)
	}

	// Verify all are running (simulating state check after reconnect)
	for _, taskID := range tasks {
		if !agents.isRunning(taskID) {
			t.Errorf("task %s should be running", taskID)
		}
	}

	// Complete some tasks
	agents.markDone("task-b")
	agents.markDone("task-d")

	// Verify correct state
	if !agents.isRunning("task-a") {
		t.Error("task-a should still be running")
	}
	if agents.isRunning("task-b") {
		t.Error("task-b should not be running")
	}
	if !agents.isRunning("task-c") {
		t.Error("task-c should still be running")
	}
	if agents.isRunning("task-d") {
		t.Error("task-d should not be running")
	}
}

// TestSSEReconnect_QueueBehaviorPreventsDuplicates simulates the queueTask
// logic from runWorker to ensure duplicates are prevented.
func TestSSEReconnect_QueueBehaviorPreventsDuplicates(t *testing.T) {
	queued := make(map[string]bool)
	pending := make([]*client.Task, 0)

	// Simulate queueTask from runWorker
	queueTask := func(task *client.Task) {
		if queued[task.ID] {
			return
		}
		queued[task.ID] = true
		pending = append(pending, task)
	}

	// Queue initial tasks
	queueTask(&client.Task{ID: "task-1", Title: "Task 1"})
	queueTask(&client.Task{ID: "task-2", Title: "Task 2"})

	if len(pending) != 2 {
		t.Errorf("expected 2 pending tasks, got %d", len(pending))
	}

	// Simulate SSE reconnect - same events arrive again
	queueTask(&client.Task{ID: "task-1", Title: "Task 1"})
	queueTask(&client.Task{ID: "task-2", Title: "Task 2"})

	// Should still only have 2 tasks (no duplicates)
	if len(pending) != 2 {
		t.Errorf("expected 2 pending tasks after reconnect, got %d", len(pending))
	}

	// New task should be added
	queueTask(&client.Task{ID: "task-3", Title: "Task 3"})
	if len(pending) != 3 {
		t.Errorf("expected 3 pending tasks, got %d", len(pending))
	}
}

// TestSSEReconnect_IsRunningCheckPreventsDuplicateStart tests the full pattern
// used in runWorker to prevent duplicate task starts.
func TestSSEReconnect_IsRunningCheckPreventsDuplicateStart(t *testing.T) {
	agents := newRunningAgents()
	startCount := 0

	// Simulate the pattern from runWorker
	tryStartTask := func(taskID string) bool {
		if agents.isRunning(taskID) {
			return false // Skip - already running
		}
		agents.markRunning(taskID, nil)
		startCount++
		return true
	}

	// First start should succeed
	if !tryStartTask("task-1") {
		t.Error("first start should succeed")
	}

	// Duplicate attempts (from reconnect) should be blocked
	if tryStartTask("task-1") {
		t.Error("duplicate start should be blocked")
	}
	if tryStartTask("task-1") {
		t.Error("duplicate start should be blocked")
	}

	if startCount != 1 {
		t.Errorf("expected 1 start, got %d", startCount)
	}
}

// TestSSEReconnect_ConcurrentQueueAndStartOperations tests concurrent queue
// and start operations that might occur during SSE event processing.
func TestSSEReconnect_ConcurrentQueueAndStartOperations(t *testing.T) {
	agents := newRunningAgents()
	var mu sync.Mutex
	queued := make(map[string]bool)
	var wg sync.WaitGroup

	// Simulate concurrent event processing after reconnect
	for i := 0; i < 100; i++ {
		wg.Add(1)
		taskID := "task-" + string(rune('a'+i%10))
		go func(id string) {
			defer wg.Done()

			// Check queue
			mu.Lock()
			isQueued := queued[id]
			if !isQueued {
				queued[id] = true
			}
			mu.Unlock()

			// Try to start if not already running
			if !agents.isRunning(id) {
				agents.markRunning(id, nil)
			}
		}(taskID)
	}

	wg.Wait()

	// All 10 unique tasks should be tracked
	runningCount := 0
	for i := 0; i < 10; i++ {
		taskID := "task-" + string(rune('a'+i))
		if agents.isRunning(taskID) {
			runningCount++
		}
	}

	if runningCount != 10 {
		t.Errorf("expected 10 running tasks, got %d", runningCount)
	}
}

// TestSSEReconnect_QueueAndRunningStateIndependent verifies that queued and
// running states are tracked independently.
func TestSSEReconnect_QueueAndRunningStateIndependent(t *testing.T) {
	agents := newRunningAgents()
	queued := make(map[string]bool)

	// Queue a task
	queued["task-1"] = true

	// Start the task (removes from queue in real code, marks as running)
	delete(queued, "task-1")
	agents.markRunning("task-1", nil)

	// After reconnect, task should be running but not queued
	if queued["task-1"] {
		t.Error("task should not be in queue after starting")
	}
	if !agents.isRunning("task-1") {
		t.Error("task should be running")
	}

	// Complete the task
	agents.markDone("task-1")

	// Now it can be queued again if needed
	if agents.isRunning("task-1") {
		t.Error("task should not be running after completion")
	}
}

// TestSSEReconnect_SimulatedReconnectScenario provides an end-to-end simulation
// of an SSE reconnect scenario.
func TestSSEReconnect_SimulatedReconnectScenario(t *testing.T) {
	agents := newRunningAgents()
	queued := make(map[string]bool)
	pending := make([]*client.Task, 0)
	startedTasks := make([]string, 0)

	queueTask := func(task *client.Task) {
		if queued[task.ID] {
			return
		}
		queued[task.ID] = true
		pending = append(pending, task)
	}

	startTask := func(task *client.Task) {
		delete(queued, task.ID)
		agents.markRunning(task.ID, nil)
		startedTasks = append(startedTasks, task.ID)
	}

	// Phase 1: Normal operation - receive and start task-1
	task1 := &client.Task{ID: "task-1", Title: "Task 1"}
	if !agents.isRunning(task1.ID) {
		startTask(task1)
	}

	// Phase 2: Task-2 arrives and gets queued (sync mode, task-1 running)
	task2 := &client.Task{ID: "task-2", Title: "Task 2"}
	queueTask(task2)

	// Phase 3: SSE disconnects and reconnects
	// Same events arrive again

	// task-1 event arrives again - should NOT restart
	if !agents.isRunning(task1.ID) {
		startTask(task1)
	}

	// task-2 event arrives again - should NOT re-queue
	queueTask(task2)

	// Verify state
	if len(startedTasks) != 1 {
		t.Errorf("expected 1 started task, got %d", len(startedTasks))
	}
	if len(pending) != 1 {
		t.Errorf("expected 1 pending task, got %d", len(pending))
	}

	// Phase 4: task-1 completes
	agents.markDone(task1.ID)

	// Phase 5: Start pending task-2
	if len(pending) > 0 && !agents.hasRunning() {
		next := pending[0]
		pending = pending[1:]
		startTask(next)
	}

	if len(startedTasks) != 2 {
		t.Errorf("expected 2 total started tasks, got %d", len(startedTasks))
	}
}

// TestSSEReconnect_RapidReconnectBurst tests handling of a burst of events
// that might arrive after SSE reconnects.
func TestSSEReconnect_RapidReconnectBurst(t *testing.T) {
	agents := newRunningAgents()
	queued := make(map[string]bool)
	var mu sync.Mutex
	startCount := 0

	// Simulate burst of events after reconnect
	events := []string{
		"task-1", "task-2", "task-1", "task-3", "task-2",
		"task-1", "task-4", "task-3", "task-2", "task-1",
	}

	var wg sync.WaitGroup
	for _, taskID := range events {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()

			mu.Lock()
			// Check queue
			if !queued[id] {
				queued[id] = true
			}

			// Try to start if not running
			if !agents.isRunning(id) {
				agents.markRunning(id, nil)
				startCount++
			}
			mu.Unlock()
		}(taskID)
	}

	wg.Wait()

	// Should have exactly 4 unique tasks started
	if startCount != 4 {
		t.Errorf("expected 4 unique task starts, got %d", startCount)
	}

	// Verify all 4 are running
	for _, id := range []string{"task-1", "task-2", "task-3", "task-4"} {
		if !agents.isRunning(id) {
			t.Errorf("task %s should be running", id)
		}
	}
}

// timeAfter returns a channel that receives after n seconds.
func timeAfter(seconds int) <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		time.Sleep(time.Duration(seconds) * time.Second)
		close(ch)
	}()
	return ch
}
