package cmd

import (
	"sync"
	"testing"

	"github.com/stevegrehan/momentum/client"
	"github.com/stevegrehan/momentum/sse"
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
	if !contains(result, "Please complete this task") {
		t.Error("prompt should contain completion instruction")
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
