package workflow

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stevegrehan/momentum/client"
)

func setupTestServer(handler http.HandlerFunc) (*httptest.Server, *client.Client) {
	server := httptest.NewServer(handler)
	c := client.NewClient(server.URL)
	return server, c
}

func TestNewWorkflow(t *testing.T) {
	c := client.NewClient("http://localhost:3000")
	wf := NewWorkflow(c)

	if wf == nil {
		t.Fatal("NewWorkflow returned nil")
	}
	if wf.client != c {
		t.Error("client not set correctly")
	}
}

func TestWorkflow_StartWorking_EmptyList(t *testing.T) {
	c := client.NewClient("http://localhost:3000")
	wf := NewWorkflow(c)

	err := wf.StartWorking([]string{})
	if err != nil {
		t.Errorf("expected no error for empty list, got %v", err)
	}
}

func TestWorkflow_StartWorking_SingleTask(t *testing.T) {
	server, c := setupTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("expected PATCH, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/tasks/task-1") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		// Verify request body
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["status"] != "in_progress" {
			t.Errorf("expected status 'in_progress', got %q", body["status"])
		}

		// Return updated task
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "task-1",
			"title":  "Test Task",
			"status": "in_progress",
		})
	})
	defer server.Close()

	wf := NewWorkflow(c)
	err := wf.StartWorking([]string{"task-1"})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestWorkflow_StartWorking_MultipleTasks(t *testing.T) {
	callCount := 0
	server, c := setupTestServer(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "task-" + string(rune('0'+callCount)),
			"title":  "Test Task",
			"status": "in_progress",
		})
	})
	defer server.Close()

	wf := NewWorkflow(c)
	err := wf.StartWorking([]string{"task-1", "task-2", "task-3"})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if callCount != 3 {
		t.Errorf("expected 3 API calls, got %d", callCount)
	}
}

func TestWorkflow_StartWorking_PartialFailure(t *testing.T) {
	callCount := 0
	server, c := setupTestServer(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if strings.Contains(r.URL.Path, "task-2") {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "task-1",
			"title":  "Test Task",
			"status": "in_progress",
		})
	})
	defer server.Close()

	wf := NewWorkflow(c)
	err := wf.StartWorking([]string{"task-1", "task-2", "task-3"})
	if err == nil {
		t.Error("expected error for partial failure")
	}
	if !strings.Contains(err.Error(), "task-2") {
		t.Errorf("error should mention failed task: %v", err)
	}
	// Should still have called all 3
	if callCount != 3 {
		t.Errorf("expected 3 API calls despite failure, got %d", callCount)
	}
}

func TestWorkflow_MarkComplete_EmptyList(t *testing.T) {
	c := client.NewClient("http://localhost:3000")
	wf := NewWorkflow(c)

	err := wf.MarkComplete([]string{})
	if err != nil {
		t.Errorf("expected no error for empty list, got %v", err)
	}
}

func TestWorkflow_MarkComplete_SingleTask(t *testing.T) {
	server, c := setupTestServer(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["status"] != "done" {
			t.Errorf("expected status 'done', got %q", body["status"])
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "task-1",
			"title":  "Test Task",
			"status": "done",
		})
	})
	defer server.Close()

	wf := NewWorkflow(c)
	err := wf.MarkComplete([]string{"task-1"})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestWorkflow_MarkComplete_AllFail(t *testing.T) {
	server, c := setupTestServer(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "server error", http.StatusInternalServerError)
	})
	defer server.Close()

	wf := NewWorkflow(c)
	err := wf.MarkComplete([]string{"task-1", "task-2"})
	if err == nil {
		t.Error("expected error when all tasks fail")
	}
	if !strings.Contains(err.Error(), "task-1") || !strings.Contains(err.Error(), "task-2") {
		t.Errorf("error should mention all failed tasks: %v", err)
	}
}

func TestWorkflow_ResetTask_EmptyList(t *testing.T) {
	c := client.NewClient("http://localhost:3000")
	wf := NewWorkflow(c)

	err := wf.ResetTask([]string{})
	if err != nil {
		t.Errorf("expected no error for empty list, got %v", err)
	}
}

func TestWorkflow_ResetTask_SingleTask(t *testing.T) {
	server, c := setupTestServer(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["status"] != "todo" {
			t.Errorf("expected status 'todo', got %q", body["status"])
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "task-1",
			"title":  "Test Task",
			"status": "todo",
		})
	})
	defer server.Close()

	wf := NewWorkflow(c)
	err := wf.ResetTask([]string{"task-1"})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestWorkflow_ResetTask_MultipleTasks(t *testing.T) {
	callCount := 0
	server, c := setupTestServer(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "task",
			"title":  "Test Task",
			"status": "todo",
		})
	})
	defer server.Close()

	wf := NewWorkflow(c)
	err := wf.ResetTask([]string{"task-1", "task-2"})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if callCount != 2 {
		t.Errorf("expected 2 API calls, got %d", callCount)
	}
}

func TestWorkflow_ErrorAggregation(t *testing.T) {
	callCount := 0
	server, c := setupTestServer(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		// All requests fail
		http.Error(w, "error", http.StatusInternalServerError)
	})
	defer server.Close()

	wf := NewWorkflow(c)
	err := wf.StartWorking([]string{"task-1", "task-2", "task-3"})

	if err == nil {
		t.Fatal("expected error")
	}

	// Error should contain all task IDs
	errStr := err.Error()
	if !strings.Contains(errStr, "task-1") {
		t.Error("error should contain task-1")
	}
	if !strings.Contains(errStr, "task-2") {
		t.Error("error should contain task-2")
	}
	if !strings.Contains(errStr, "task-3") {
		t.Error("error should contain task-3")
	}

	// All tasks should have been attempted
	if callCount != 3 {
		t.Errorf("expected 3 calls, got %d", callCount)
	}
}
