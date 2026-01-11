package selection

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stevegrehan/momentum/client"
)

// mockServer creates a test server that responds with the given data.
type mockServer struct {
	projects []client.Project
	epics    map[string][]client.Epic // projectID -> epics
	tasks    map[string][]client.Task // projectID -> tasks
}

func newMockServer() *mockServer {
	return &mockServer{
		projects: []client.Project{},
		epics:    make(map[string][]client.Epic),
		tasks:    make(map[string][]client.Task),
	}
}

func (m *mockServer) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Parse the path
		path := r.URL.Path

		switch {
		case path == "/api/projects" && r.Method == http.MethodGet:
			json.NewEncoder(w).Encode(m.projects)

		case len(path) > len("/api/projects/") && r.Method == http.MethodGet:
			// Extract project ID and check for epics/tasks
			remaining := path[len("/api/projects/"):]

			// Check for /projects/{id}/epics
			for projectID, epics := range m.epics {
				if remaining == projectID+"/epics" {
					json.NewEncoder(w).Encode(epics)
					return
				}
			}

			// Check for /projects/{id}/tasks
			for projectID, tasks := range m.tasks {
				if remaining == projectID+"/tasks" || hasPrefix(remaining, projectID+"/tasks?") {
					// Filter by epic_id if provided
					epicID := r.URL.Query().Get("epic_id")
					status := r.URL.Query().Get("status")

					filteredTasks := []client.Task{}
					for _, task := range tasks {
						if epicID != "" && task.EpicID != epicID {
							continue
						}
						if status != "" && task.Status != status {
							continue
						}
						filteredTasks = append(filteredTasks, task)
					}
					json.NewEncoder(w).Encode(filteredTasks)
					return
				}
			}

			// Check for /projects/{id}/epics for unknown project
			for _, p := range m.projects {
				if remaining == p.ID+"/epics" {
					json.NewEncoder(w).Encode([]client.Epic{})
					return
				}
				if remaining == p.ID+"/tasks" || hasPrefix(remaining, p.ID+"/tasks?") {
					json.NewEncoder(w).Encode([]client.Task{})
					return
				}
			}

			w.WriteHeader(http.StatusNotFound)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// setupTest creates a mock server and client for testing.
func setupTest(m *mockServer) (*httptest.Server, *client.Client) {
	server := httptest.NewServer(m.handler())
	c := client.NewClient(server.URL)
	return server, c
}

// --- Table-Driven Tests for Selection Rules ---

func TestSelectByTaskID(t *testing.T) {
	tests := []struct {
		name        string
		taskID      string
		projects    []client.Project
		tasks       map[string][]client.Task
		expectedID  string
		expectError bool
	}{
		{
			name:   "find existing task by ID",
			taskID: "task-2",
			projects: []client.Project{
				{ID: "proj-1", Name: "Project 1"},
			},
			tasks: map[string][]client.Task{
				"proj-1": {
					{ID: "task-1", Title: "Task 1", Status: "todo", ProjectID: "proj-1"},
					{ID: "task-2", Title: "Task 2", Status: "in_progress", ProjectID: "proj-1"},
					{ID: "task-3", Title: "Task 3", Status: "done", ProjectID: "proj-1"},
				},
			},
			expectedID:  "task-2",
			expectError: false,
		},
		{
			name:   "find task across multiple projects",
			taskID: "task-5",
			projects: []client.Project{
				{ID: "proj-1", Name: "Project 1"},
				{ID: "proj-2", Name: "Project 2"},
			},
			tasks: map[string][]client.Task{
				"proj-1": {
					{ID: "task-1", Title: "Task 1", Status: "todo", ProjectID: "proj-1"},
				},
				"proj-2": {
					{ID: "task-5", Title: "Task 5", Status: "todo", ProjectID: "proj-2"},
				},
			},
			expectedID:  "task-5",
			expectError: false,
		},
		{
			name:   "task not found",
			taskID: "non-existent",
			projects: []client.Project{
				{ID: "proj-1", Name: "Project 1"},
			},
			tasks: map[string][]client.Task{
				"proj-1": {
					{ID: "task-1", Title: "Task 1", Status: "todo", ProjectID: "proj-1"},
				},
			},
			expectedID:  "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newMockServer()
			m.projects = tt.projects
			m.tasks = tt.tasks

			server, c := setupTest(m)
			defer server.Close()

			selector := NewSelector(c, "", "", tt.taskID)
			task, err := selector.SelectTask()

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				if !errors.Is(err, ErrNoTaskAvailable) {
					t.Errorf("expected ErrNoTaskAvailable, got %v", err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if task.ID != tt.expectedID {
					t.Errorf("expected task ID %s, got %s", tt.expectedID, task.ID)
				}
			}
		})
	}
}

func TestSelectByEpicID(t *testing.T) {
	tests := []struct {
		name        string
		epicID      string
		projects    []client.Project
		epics       map[string][]client.Epic
		tasks       map[string][]client.Task
		expectedID  string
		expectError bool
	}{
		{
			name:   "find unblocked todo task from epic",
			epicID: "epic-1",
			projects: []client.Project{
				{ID: "proj-1", Name: "Project 1"},
			},
			epics: map[string][]client.Epic{
				"proj-1": {
					{ID: "epic-1", Title: "Epic 1", Status: "todo", ProjectID: "proj-1", Auto: true},
				},
			},
			tasks: map[string][]client.Task{
				"proj-1": {
					{ID: "task-1", Title: "Task 1", Status: "todo", EpicID: "epic-1", ProjectID: "proj-1", Blocked: false},
					{ID: "task-2", Title: "Task 2", Status: "in_progress", EpicID: "epic-1", ProjectID: "proj-1", Blocked: false},
				},
			},
			expectedID:  "task-1",
			expectError: false,
		},
		{
			name:   "epic not found",
			epicID: "non-existent",
			projects: []client.Project{
				{ID: "proj-1", Name: "Project 1"},
			},
			epics: map[string][]client.Epic{
				"proj-1": {
					{ID: "epic-1", Title: "Epic 1", Status: "todo", ProjectID: "proj-1", Auto: true},
				},
			},
			tasks: map[string][]client.Task{
				"proj-1": {},
			},
			expectedID:  "",
			expectError: true,
		},
		{
			name:   "epic found but no tasks",
			epicID: "epic-1",
			projects: []client.Project{
				{ID: "proj-1", Name: "Project 1"},
			},
			epics: map[string][]client.Epic{
				"proj-1": {
					{ID: "epic-1", Title: "Epic 1", Status: "todo", ProjectID: "proj-1", Auto: true},
				},
			},
			tasks: map[string][]client.Task{
				"proj-1": {
					{ID: "task-1", Title: "Task 1", Status: "todo", EpicID: "epic-2", ProjectID: "proj-1", Blocked: false},
				},
			},
			expectedID:  "",
			expectError: true,
		},
		{
			name:   "epic has auto=false",
			epicID: "epic-1",
			projects: []client.Project{
				{ID: "proj-1", Name: "Project 1"},
			},
			epics: map[string][]client.Epic{
				"proj-1": {
					{ID: "epic-1", Title: "Epic 1", Status: "todo", ProjectID: "proj-1", Auto: false},
				},
			},
			tasks: map[string][]client.Task{
				"proj-1": {
					{ID: "task-1", Title: "Task 1", Status: "todo", EpicID: "epic-1", ProjectID: "proj-1", Blocked: false},
				},
			},
			expectedID:  "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newMockServer()
			m.projects = tt.projects
			m.epics = tt.epics
			m.tasks = tt.tasks

			server, c := setupTest(m)
			defer server.Close()

			selector := NewSelector(c, "", tt.epicID, "")
			task, err := selector.SelectTask()

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if task.ID != tt.expectedID {
					t.Errorf("expected task ID %s, got %s", tt.expectedID, task.ID)
				}
			}
		})
	}
}

func TestSelectByProjectID(t *testing.T) {
	tests := []struct {
		name        string
		projectID   string
		projects    []client.Project
		epics       map[string][]client.Epic
		tasks       map[string][]client.Task
		expectedID  string
		expectError bool
	}{
		{
			name:      "find unblocked todo task from project with auto epic",
			projectID: "proj-1",
			projects: []client.Project{
				{ID: "proj-1", Name: "Project 1"},
			},
			epics: map[string][]client.Epic{
				"proj-1": {
					{ID: "epic-1", Title: "Epic 1", Status: "todo", ProjectID: "proj-1", Auto: true},
				},
			},
			tasks: map[string][]client.Task{
				"proj-1": {
					{ID: "task-1", Title: "Task 1", Status: "todo", EpicID: "epic-1", ProjectID: "proj-1", Blocked: false},
					{ID: "task-2", Title: "Task 2", Status: "todo", EpicID: "epic-1", ProjectID: "proj-1", Blocked: true},
				},
			},
			expectedID:  "task-1",
			expectError: false,
		},
		{
			name:      "project has no tasks",
			projectID: "proj-1",
			projects: []client.Project{
				{ID: "proj-1", Name: "Project 1"},
			},
			epics: map[string][]client.Epic{
				"proj-1": {
					{ID: "epic-1", Title: "Epic 1", Status: "todo", ProjectID: "proj-1", Auto: true},
				},
			},
			tasks: map[string][]client.Task{
				"proj-1": {},
			},
			expectedID:  "",
			expectError: true,
		},
		{
			name:      "project has tasks but no auto epics",
			projectID: "proj-1",
			projects: []client.Project{
				{ID: "proj-1", Name: "Project 1"},
			},
			epics: map[string][]client.Epic{
				"proj-1": {
					{ID: "epic-1", Title: "Epic 1", Status: "todo", ProjectID: "proj-1", Auto: false},
				},
			},
			tasks: map[string][]client.Task{
				"proj-1": {
					{ID: "task-1", Title: "Task 1", Status: "todo", EpicID: "epic-1", ProjectID: "proj-1", Blocked: false},
				},
			},
			expectedID:  "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newMockServer()
			m.projects = tt.projects
			m.epics = tt.epics
			m.tasks = tt.tasks

			server, c := setupTest(m)
			defer server.Close()

			selector := NewSelector(c, tt.projectID, "", "")
			task, err := selector.SelectTask()

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if task.ID != tt.expectedID {
					t.Errorf("expected task ID %s, got %s", tt.expectedID, task.ID)
				}
			}
		})
	}
}

func TestSelectNewestTodoAcrossAllProjects(t *testing.T) {
	tests := []struct {
		name        string
		projects    []client.Project
		epics       map[string][]client.Epic
		tasks       map[string][]client.Task
		expectedID  string
		expectError bool
	}{
		{
			name: "select newest unblocked todo across projects from auto epics",
			projects: []client.Project{
				{ID: "proj-1", Name: "Project 1"},
				{ID: "proj-2", Name: "Project 2"},
			},
			epics: map[string][]client.Epic{
				"proj-1": {
					{ID: "epic-1", Title: "Epic 1", Status: "todo", ProjectID: "proj-1", Auto: true},
				},
				"proj-2": {
					{ID: "epic-2", Title: "Epic 2", Status: "todo", ProjectID: "proj-2", Auto: true},
				},
			},
			tasks: map[string][]client.Task{
				"proj-1": {
					{ID: "task-a", Title: "Task A", Status: "todo", EpicID: "epic-1", ProjectID: "proj-1", Blocked: false},
				},
				"proj-2": {
					{ID: "task-z", Title: "Task Z", Status: "todo", EpicID: "epic-2", ProjectID: "proj-2", Blocked: false},
				},
			},
			// "task-z" > "task-a" lexicographically, so task-z is "newer"
			expectedID:  "task-z",
			expectError: false,
		},
		{
			name:        "no projects available",
			projects:    []client.Project{},
			epics:       map[string][]client.Epic{},
			tasks:       map[string][]client.Task{},
			expectedID:  "",
			expectError: true,
		},
		{
			name: "all projects have no tasks",
			projects: []client.Project{
				{ID: "proj-1", Name: "Project 1"},
				{ID: "proj-2", Name: "Project 2"},
			},
			epics: map[string][]client.Epic{
				"proj-1": {
					{ID: "epic-1", Title: "Epic 1", Status: "todo", ProjectID: "proj-1", Auto: true},
				},
				"proj-2": {
					{ID: "epic-2", Title: "Epic 2", Status: "todo", ProjectID: "proj-2", Auto: true},
				},
			},
			tasks: map[string][]client.Task{
				"proj-1": {},
				"proj-2": {},
			},
			expectedID:  "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newMockServer()
			m.projects = tt.projects
			m.epics = tt.epics
			m.tasks = tt.tasks

			server, c := setupTest(m)
			defer server.Close()

			selector := NewSelector(c, "", "", "")
			task, err := selector.SelectTask()

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if task.ID != tt.expectedID {
					t.Errorf("expected task ID %s, got %s", tt.expectedID, task.ID)
				}
			}
		})
	}
}

func TestPrioritizeUnblockedOverBlocked(t *testing.T) {
	tests := []struct {
		name        string
		epics       map[string][]client.Epic
		tasks       []client.Task
		expectedID  string
		expectError bool
	}{
		{
			name: "unblocked todo over blocked todo",
			epics: map[string][]client.Epic{
				"proj-1": {{ID: "epic-1", Title: "Epic 1", ProjectID: "proj-1", Auto: true}},
			},
			tasks: []client.Task{
				{ID: "task-1", Title: "Blocked Todo", Status: "todo", EpicID: "epic-1", Blocked: true},
				{ID: "task-2", Title: "Unblocked Todo", Status: "todo", EpicID: "epic-1", Blocked: false},
			},
			expectedID: "task-2",
		},
		{
			name: "no unblocked todos returns error",
			epics: map[string][]client.Epic{
				"proj-1": {{ID: "epic-1", Title: "Epic 1", ProjectID: "proj-1", Auto: true}},
			},
			tasks: []client.Task{
				{ID: "task-1", Title: "Blocked Todo", Status: "todo", EpicID: "epic-1", Blocked: true},
				{ID: "task-2", Title: "Unblocked In Progress", Status: "in_progress", EpicID: "epic-1", Blocked: false},
			},
			expectedID:  "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newMockServer()
			m.projects = []client.Project{{ID: "proj-1", Name: "Project 1"}}
			m.epics = tt.epics
			m.tasks = map[string][]client.Task{"proj-1": tt.tasks}

			server, c := setupTest(m)
			defer server.Close()

			selector := NewSelector(c, "proj-1", "", "")
			task, err := selector.SelectTask()

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if task.ID != tt.expectedID {
					t.Errorf("expected task ID %s, got %s", tt.expectedID, task.ID)
				}
			}
		})
	}
}

func TestPrioritizeTodoStatusOverOtherStatuses(t *testing.T) {
	tests := []struct {
		name       string
		epics      map[string][]client.Epic
		tasks      []client.Task
		expectedID string
	}{
		{
			name: "selects todo task ignoring non-todo tasks",
			epics: map[string][]client.Epic{
				"proj-1": {{ID: "epic-1", Title: "Epic 1", ProjectID: "proj-1", Auto: true}},
			},
			tasks: []client.Task{
				{ID: "task-1", Title: "In Progress", Status: "in_progress", EpicID: "epic-1", Blocked: false},
				{ID: "task-2", Title: "Todo", Status: "todo", EpicID: "epic-1", Blocked: false},
			},
			expectedID: "task-2",
		},
		{
			name: "newest unblocked todo when multiple exist",
			epics: map[string][]client.Epic{
				"proj-1": {{ID: "epic-1", Title: "Epic 1", ProjectID: "proj-1", Auto: true}},
			},
			tasks: []client.Task{
				{ID: "task-a", Title: "Old Todo", Status: "todo", EpicID: "epic-1", Blocked: false},
				{ID: "task-b", Title: "Newer Todo", Status: "todo", EpicID: "epic-1", Blocked: false},
				{ID: "task-c", Title: "Newest Todo", Status: "todo", EpicID: "epic-1", Blocked: false},
			},
			// task-c > task-b > task-a lexicographically
			expectedID: "task-c",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newMockServer()
			m.projects = []client.Project{{ID: "proj-1", Name: "Project 1"}}
			m.epics = tt.epics
			m.tasks = map[string][]client.Task{"proj-1": tt.tasks}

			server, c := setupTest(m)
			defer server.Close()

			selector := NewSelector(c, "proj-1", "", "")
			task, err := selector.SelectTask()

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if task.ID != tt.expectedID {
				t.Errorf("expected task ID %s, got %s", tt.expectedID, task.ID)
			}
		})
	}
}

// --- Error Cases ---

func TestNoTasksAvailable(t *testing.T) {
	m := newMockServer()
	m.projects = []client.Project{{ID: "proj-1", Name: "Project 1"}}
	m.epics = map[string][]client.Epic{
		"proj-1": {{ID: "epic-1", Title: "Epic 1", ProjectID: "proj-1", Auto: true}},
	}
	m.tasks = map[string][]client.Task{"proj-1": {}}

	server, c := setupTest(m)
	defer server.Close()

	selector := NewSelector(c, "proj-1", "", "")
	_, err := selector.SelectTask()

	if err == nil {
		t.Error("expected error, got nil")
	}

	if !errors.Is(err, ErrNoTaskAvailable) {
		t.Errorf("expected ErrNoTaskAvailable, got %v", err)
	}
}

func TestTaskNotFound(t *testing.T) {
	m := newMockServer()
	m.projects = []client.Project{{ID: "proj-1", Name: "Project 1"}}
	m.tasks = map[string][]client.Task{
		"proj-1": {
			{ID: "task-1", Title: "Task 1", Status: "todo", ProjectID: "proj-1"},
		},
	}

	server, c := setupTest(m)
	defer server.Close()

	selector := NewSelector(c, "", "", "non-existent-task")
	_, err := selector.SelectTask()

	if err == nil {
		t.Error("expected error, got nil")
	}

	if !errors.Is(err, ErrNoTaskAvailable) {
		t.Errorf("expected ErrNoTaskAvailable, got %v", err)
	}
}

// --- Filter and Sort Tests ---

func TestFilterAndSortTasks(t *testing.T) {
	tests := []struct {
		name           string
		tasks          []client.Task
		expectedOrder  []string // Expected order of task IDs
		expectedLength int
	}{
		{
			name: "sort unblocked todos by ID descending",
			tasks: []client.Task{
				{ID: "task-a", Status: "todo", Blocked: false},
				{ID: "task-c", Status: "todo", Blocked: false},
				{ID: "task-b", Status: "todo", Blocked: false},
			},
			expectedOrder:  []string{"task-c", "task-b", "task-a"},
			expectedLength: 3,
		},
		{
			name: "unblocked todos only when mixed with blocked",
			tasks: []client.Task{
				{ID: "task-1", Status: "todo", Blocked: true},
				{ID: "task-2", Status: "todo", Blocked: false},
				{ID: "task-3", Status: "todo", Blocked: true},
			},
			expectedOrder:  []string{"task-2"},
			expectedLength: 1,
		},
		{
			name: "returns empty when no unblocked todos exist",
			tasks: []client.Task{
				{ID: "task-1", Status: "in_progress", Blocked: false},
				{ID: "task-2", Status: "done", Blocked: false},
				{ID: "task-3", Status: "todo", Blocked: true},
			},
			expectedOrder:  []string{},
			expectedLength: 0,
		},
		{
			name: "returns empty when only blocked todos exist",
			tasks: []client.Task{
				{ID: "task-1", Status: "todo", Blocked: true},
				{ID: "task-2", Status: "todo", Blocked: true},
			},
			expectedOrder:  []string{},
			expectedLength: 0,
		},
		{
			name: "returns empty when only non-todo tasks exist",
			tasks: []client.Task{
				{ID: "task-1", Status: "done", Blocked: true},
				{ID: "task-2", Status: "in_progress", Blocked: true},
			},
			expectedOrder:  []string{},
			expectedLength: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterAndSortTasks(tt.tasks)

			if len(result) != tt.expectedLength {
				t.Errorf("expected %d tasks, got %d", tt.expectedLength, len(result))
				return
			}

			for i, expectedID := range tt.expectedOrder {
				if i >= len(result) {
					break
				}
				if result[i].ID != expectedID {
					t.Errorf("position %d: expected task ID %s, got %s", i, expectedID, result[i].ID)
				}
			}
		})
	}
}

func TestFilterAndSortTasksEmpty(t *testing.T) {
	result := filterAndSortTasks([]client.Task{})
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d tasks", len(result))
	}
}

// --- Complex Scenarios ---

func TestComplexSelectionScenario(t *testing.T) {
	// Scenario: Multiple projects with various task states
	m := newMockServer()
	m.projects = []client.Project{
		{ID: "proj-1", Name: "Project 1"},
		{ID: "proj-2", Name: "Project 2"},
		{ID: "proj-3", Name: "Project 3"},
	}
	m.epics = map[string][]client.Epic{
		"proj-1": {{ID: "epic-1", Title: "Epic 1", ProjectID: "proj-1", Auto: true}},
		"proj-2": {{ID: "epic-2", Title: "Epic 2", ProjectID: "proj-2", Auto: true}},
		"proj-3": {{ID: "epic-3", Title: "Epic 3", ProjectID: "proj-3", Auto: true}},
	}
	m.tasks = map[string][]client.Task{
		"proj-1": {
			{ID: "task-001", Title: "Old blocked", Status: "todo", EpicID: "epic-1", ProjectID: "proj-1", Blocked: true},
			{ID: "task-002", Title: "Old unblocked todo", Status: "todo", EpicID: "epic-1", ProjectID: "proj-1", Blocked: false},
		},
		"proj-2": {
			{ID: "task-100", Title: "Newer blocked", Status: "todo", EpicID: "epic-2", ProjectID: "proj-2", Blocked: true},
			{ID: "task-101", Title: "Newer unblocked todo", Status: "todo", EpicID: "epic-2", ProjectID: "proj-2", Blocked: false},
		},
		"proj-3": {
			{ID: "task-200", Title: "Newest done", Status: "done", EpicID: "epic-3", ProjectID: "proj-3", Blocked: false},
		},
	}

	server, c := setupTest(m)
	defer server.Close()

	// Test 1: Select across all projects (should get task-101 as newest unblocked todo)
	selector1 := NewSelector(c, "", "", "")
	task1, err := selector1.SelectTask()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if task1.ID != "task-101" {
		t.Errorf("expected task-101, got %s", task1.ID)
	}

	// Test 2: Select from proj-1 only (should get task-002 as only unblocked todo)
	selector2 := NewSelector(c, "proj-1", "", "")
	task2, err := selector2.SelectTask()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if task2.ID != "task-002" {
		t.Errorf("expected task-002, got %s", task2.ID)
	}

	// Test 3: Select from proj-3 only (should error - no todo tasks)
	selector3 := NewSelector(c, "proj-3", "", "")
	_, err = selector3.SelectTask()
	if err == nil {
		t.Error("expected error for proj-3 (no todo tasks), got nil")
	}

	// Test 4: Select specific task by ID (direct lookup, any status)
	selector4 := NewSelector(c, "", "", "task-001")
	task4, err := selector4.SelectTask()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if task4.ID != "task-001" {
		t.Errorf("expected task-001, got %s", task4.ID)
	}
}

// TestSelectionPriorityOrder tests task selection only selects unblocked todo tasks from auto epics
func TestSelectionPriorityOrder(t *testing.T) {
	tests := []struct {
		name        string
		epics       map[string][]client.Epic
		tasks       []client.Task
		expectedID  string
		expectError bool
	}{
		{
			name: "selects unblocked todo from auto epic",
			epics: map[string][]client.Epic{
				"proj-1": {{ID: "epic-1", Title: "Epic 1", ProjectID: "proj-1", Auto: true}},
			},
			tasks: []client.Task{
				{ID: "task-5", Status: "done", EpicID: "epic-1", Blocked: false},
				{ID: "task-4", Status: "in_progress", EpicID: "epic-1", Blocked: false},
				{ID: "task-3", Status: "todo", EpicID: "epic-1", Blocked: true},
				{ID: "task-2", Status: "todo", EpicID: "epic-1", Blocked: false}, // Should win
				{ID: "task-1", Status: "todo", EpicID: "epic-1", Blocked: false},
			},
			expectedID: "task-2", // Highest ID among unblocked todos
		},
		{
			name: "no unblocked todos returns error",
			epics: map[string][]client.Epic{
				"proj-1": {{ID: "epic-1", Title: "Epic 1", ProjectID: "proj-1", Auto: true}},
			},
			tasks: []client.Task{
				{ID: "task-3", Status: "done", EpicID: "epic-1", Blocked: true},
				{ID: "task-2", Status: "in_progress", EpicID: "epic-1", Blocked: false},
				{ID: "task-1", Status: "todo", EpicID: "epic-1", Blocked: true},
			},
			expectError: true,
		},
		{
			name: "ignores tasks from non-auto epics",
			epics: map[string][]client.Epic{
				"proj-1": {
					{ID: "epic-1", Title: "Epic 1", ProjectID: "proj-1", Auto: false},
					{ID: "epic-2", Title: "Epic 2", ProjectID: "proj-1", Auto: true},
				},
			},
			tasks: []client.Task{
				{ID: "task-2", Status: "todo", EpicID: "epic-1", Blocked: false}, // Ignored - non-auto epic
				{ID: "task-1", Status: "todo", EpicID: "epic-2", Blocked: false}, // Should win
			},
			expectedID: "task-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newMockServer()
			m.projects = []client.Project{{ID: "proj-1", Name: "Project 1"}}
			m.epics = tt.epics
			m.tasks = map[string][]client.Task{"proj-1": tt.tasks}

			server, c := setupTest(m)
			defer server.Close()

			selector := NewSelector(c, "proj-1", "", "")
			task, err := selector.SelectTask()

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if task.ID != tt.expectedID {
					t.Errorf("expected task ID %s, got %s", tt.expectedID, task.ID)
				}
			}
		})
	}
}
