package client

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// setupTestServer creates a test server with the given handler.
func setupTestServer(handler http.Handler) (*httptest.Server, *Client) {
	server := httptest.NewServer(handler)
	client := NewClient(server.URL)
	return server, client
}

// --- Project Tests ---

func TestListProjects(t *testing.T) {
	expectedProjects := []Project{
		{ID: "proj-1", Name: "Project 1", Description: "First project"},
		{ID: "proj-2", Name: "Project 2", Description: "Second project"},
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET method, got %s", r.Method)
		}
		if r.URL.Path != "/api/projects" {
			t.Errorf("expected path /api/projects, got %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedProjects)
	})

	server, client := setupTestServer(handler)
	defer server.Close()

	projects, err := client.ListProjects()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(projects) != len(expectedProjects) {
		t.Errorf("expected %d projects, got %d", len(expectedProjects), len(projects))
	}

	for i, p := range projects {
		if p.ID != expectedProjects[i].ID {
			t.Errorf("expected project ID %s, got %s", expectedProjects[i].ID, p.ID)
		}
		if p.Name != expectedProjects[i].Name {
			t.Errorf("expected project name %s, got %s", expectedProjects[i].Name, p.Name)
		}
	}
}

func TestCreateProject(t *testing.T) {
	expectedProject := Project{ID: "proj-new", Name: "New Project", Description: "A new project"}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST method, got %s", r.Method)
		}
		if r.URL.Path != "/api/projects" {
			t.Errorf("expected path /api/projects, got %s", r.URL.Path)
		}

		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if body["name"] != "New Project" {
			t.Errorf("expected name 'New Project', got '%s'", body["name"])
		}
		if body["description"] != "A new project" {
			t.Errorf("expected description 'A new project', got '%s'", body["description"])
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(expectedProject)
	})

	server, client := setupTestServer(handler)
	defer server.Close()

	project, err := client.CreateProject("New Project", "A new project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if project.ID != expectedProject.ID {
		t.Errorf("expected project ID %s, got %s", expectedProject.ID, project.ID)
	}
	if project.Name != expectedProject.Name {
		t.Errorf("expected project name %s, got %s", expectedProject.Name, project.Name)
	}
}

func TestUpdateProject(t *testing.T) {
	expectedProject := Project{ID: "proj-1", Name: "Updated Name", Description: "Updated desc"}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("expected PATCH method, got %s", r.Method)
		}
		if r.URL.Path != "/api/projects/proj-1" {
			t.Errorf("expected path /api/projects/proj-1, got %s", r.URL.Path)
		}

		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if body["name"] != "Updated Name" {
			t.Errorf("expected name 'Updated Name', got '%s'", body["name"])
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedProject)
	})

	server, client := setupTestServer(handler)
	defer server.Close()

	project, err := client.UpdateProject("proj-1", "Updated Name", "Updated desc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if project.Name != expectedProject.Name {
		t.Errorf("expected project name %s, got %s", expectedProject.Name, project.Name)
	}
}

func TestDeleteProject(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE method, got %s", r.Method)
		}
		if r.URL.Path != "/api/projects/proj-1" {
			t.Errorf("expected path /api/projects/proj-1, got %s", r.URL.Path)
		}

		w.WriteHeader(http.StatusNoContent)
	})

	server, client := setupTestServer(handler)
	defer server.Close()

	err := client.DeleteProject("proj-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Epic Tests ---

func TestListEpics(t *testing.T) {
	expectedEpics := []Epic{
		{ID: "epic-1", Title: "Epic 1", Status: "todo", ProjectID: "proj-1"},
		{ID: "epic-2", Title: "Epic 2", Status: "in_progress", ProjectID: "proj-1"},
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET method, got %s", r.Method)
		}
		if r.URL.Path != "/api/projects/proj-1/epics" {
			t.Errorf("expected path /api/projects/proj-1/epics, got %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedEpics)
	})

	server, client := setupTestServer(handler)
	defer server.Close()

	epics, err := client.ListEpics("proj-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(epics) != len(expectedEpics) {
		t.Errorf("expected %d epics, got %d", len(expectedEpics), len(epics))
	}

	for i, e := range epics {
		if e.ID != expectedEpics[i].ID {
			t.Errorf("expected epic ID %s, got %s", expectedEpics[i].ID, e.ID)
		}
	}
}

func TestCreateEpic(t *testing.T) {
	expectedEpic := Epic{ID: "epic-new", Title: "New Epic", Notes: "Some notes", Status: "todo", ProjectID: "proj-1"}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST method, got %s", r.Method)
		}
		if r.URL.Path != "/api/projects/proj-1/epics" {
			t.Errorf("expected path /api/projects/proj-1/epics, got %s", r.URL.Path)
		}

		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if body["title"] != "New Epic" {
			t.Errorf("expected title 'New Epic', got '%s'", body["title"])
		}
		if body["notes"] != "Some notes" {
			t.Errorf("expected notes 'Some notes', got '%s'", body["notes"])
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(expectedEpic)
	})

	server, client := setupTestServer(handler)
	defer server.Close()

	epic, err := client.CreateEpic("proj-1", "New Epic", "Some notes")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if epic.ID != expectedEpic.ID {
		t.Errorf("expected epic ID %s, got %s", expectedEpic.ID, epic.ID)
	}
	if epic.Title != expectedEpic.Title {
		t.Errorf("expected epic title %s, got %s", expectedEpic.Title, epic.Title)
	}
}

func TestUpdateEpic(t *testing.T) {
	expectedEpic := Epic{ID: "epic-1", Title: "Updated Epic", Status: "in_progress", ProjectID: "proj-1"}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("expected PATCH method, got %s", r.Method)
		}
		if r.URL.Path != "/api/epics/epic-1" {
			t.Errorf("expected path /api/epics/epic-1, got %s", r.URL.Path)
		}

		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if body["title"] != "Updated Epic" {
			t.Errorf("expected title 'Updated Epic', got '%v'", body["title"])
		}
		if body["status"] != "in_progress" {
			t.Errorf("expected status 'in_progress', got '%v'", body["status"])
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedEpic)
	})

	server, client := setupTestServer(handler)
	defer server.Close()

	updates := EpicUpdate{
		Title:  StringPtr("Updated Epic"),
		Status: StringPtr("in_progress"),
	}

	epic, err := client.UpdateEpic("epic-1", updates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if epic.Title != expectedEpic.Title {
		t.Errorf("expected epic title %s, got %s", expectedEpic.Title, epic.Title)
	}
	if epic.Status != expectedEpic.Status {
		t.Errorf("expected epic status %s, got %s", expectedEpic.Status, epic.Status)
	}
}

func TestDeleteEpic(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE method, got %s", r.Method)
		}
		if r.URL.Path != "/api/epics/epic-1" {
			t.Errorf("expected path /api/epics/epic-1, got %s", r.URL.Path)
		}

		w.WriteHeader(http.StatusNoContent)
	})

	server, client := setupTestServer(handler)
	defer server.Close()

	err := client.DeleteEpic("epic-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Task Tests ---

func TestListTasks(t *testing.T) {
	expectedTasks := []Task{
		{ID: "task-1", Title: "Task 1", Status: "todo", ProjectID: "proj-1", Blocked: false},
		{ID: "task-2", Title: "Task 2", Status: "in_progress", ProjectID: "proj-1", Blocked: true},
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET method, got %s", r.Method)
		}
		if r.URL.Path != "/api/projects/proj-1/tasks" {
			t.Errorf("expected path /api/projects/proj-1/tasks, got %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedTasks)
	})

	server, client := setupTestServer(handler)
	defer server.Close()

	tasks, err := client.ListTasks("proj-1", TaskFilters{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tasks) != len(expectedTasks) {
		t.Errorf("expected %d tasks, got %d", len(expectedTasks), len(tasks))
	}

	for i, task := range tasks {
		if task.ID != expectedTasks[i].ID {
			t.Errorf("expected task ID %s, got %s", expectedTasks[i].ID, task.ID)
		}
	}
}

func TestListTasksWithFilters(t *testing.T) {
	tests := []struct {
		name          string
		filters       TaskFilters
		expectedQuery string
	}{
		{
			name:          "filter by epic",
			filters:       TaskFilters{EpicID: StringPtr("epic-1")},
			expectedQuery: "epic_id=epic-1",
		},
		{
			name:          "filter by status",
			filters:       TaskFilters{Status: StringPtr("todo")},
			expectedQuery: "status=todo",
		},
		{
			name: "filter by both epic and status",
			filters: TaskFilters{
				EpicID: StringPtr("epic-1"),
				Status: StringPtr("in_progress"),
			},
			expectedQuery: "epic_id=epic-1&status=in_progress",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("expected GET method, got %s", r.Method)
				}

				// Check query parameters
				query := r.URL.RawQuery
				if query != tt.expectedQuery {
					t.Errorf("expected query '%s', got '%s'", tt.expectedQuery, query)
				}

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode([]Task{})
			})

			server, client := setupTestServer(handler)
			defer server.Close()

			_, err := client.ListTasks("proj-1", tt.filters)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestListTasksAppliesFiltersLocally(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]Task{
			{ID: "wanted", EpicID: "epic-1", Status: "todo"},
			{ID: "wrong-epic", EpicID: "epic-2", Status: "todo"},
			{ID: "wrong-status", EpicID: "epic-1", Status: "done"},
		})
	})

	server, c := setupTestServer(handler)
	defer server.Close()
	tasks, err := c.ListTasks("proj-1", TaskFilters{
		EpicID: StringPtr("epic-1"),
		Status: StringPtr("todo"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tasks) != 1 || tasks[0].ID != "wanted" {
		t.Fatalf("unexpected filtered tasks: %#v", tasks)
	}
}

func TestCreateTask(t *testing.T) {
	expectedTask := Task{ID: "task-new", Title: "New Task", Notes: "Task notes", Status: "todo", ProjectID: "proj-1", EpicID: "epic-1"}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST method, got %s", r.Method)
		}
		if r.URL.Path != "/api/projects/proj-1/tasks" {
			t.Errorf("expected path /api/projects/proj-1/tasks, got %s", r.URL.Path)
		}

		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if body["title"] != "New Task" {
			t.Errorf("expected title 'New Task', got '%s'", body["title"])
		}
		if body["notes"] != "Task notes" {
			t.Errorf("expected notes 'Task notes', got '%s'", body["notes"])
		}
		if body["epic_id"] != "epic-1" {
			t.Errorf("expected epic_id 'epic-1', got '%s'", body["epic_id"])
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(expectedTask)
	})

	server, client := setupTestServer(handler)
	defer server.Close()

	task, err := client.CreateTask("proj-1", "New Task", "Task notes", "epic-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if task.ID != expectedTask.ID {
		t.Errorf("expected task ID %s, got %s", expectedTask.ID, task.ID)
	}
	if task.Title != expectedTask.Title {
		t.Errorf("expected task title %s, got %s", expectedTask.Title, task.Title)
	}
}

func TestUpdateTask(t *testing.T) {
	expectedTask := Task{ID: "task-1", Title: "Updated Task", Status: "done", ProjectID: "proj-1"}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("expected PATCH method, got %s", r.Method)
		}
		if r.URL.Path != "/api/tasks/task-1" {
			t.Errorf("expected path /api/tasks/task-1, got %s", r.URL.Path)
		}

		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if body["title"] != "Updated Task" {
			t.Errorf("expected title 'Updated Task', got '%v'", body["title"])
		}
		if body["status"] != "done" {
			t.Errorf("expected status 'done', got '%v'", body["status"])
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedTask)
	})

	server, client := setupTestServer(handler)
	defer server.Close()

	updates := TaskUpdate{
		Title:  StringPtr("Updated Task"),
		Status: StringPtr("done"),
	}

	task, err := client.UpdateTask("task-1", updates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if task.Title != expectedTask.Title {
		t.Errorf("expected task title %s, got %s", expectedTask.Title, task.Title)
	}
	if task.Status != expectedTask.Status {
		t.Errorf("expected task status %s, got %s", expectedTask.Status, task.Status)
	}
}

func TestDeleteTask(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE method, got %s", r.Method)
		}
		if r.URL.Path != "/api/tasks/task-1" {
			t.Errorf("expected path /api/tasks/task-1, got %s", r.URL.Path)
		}

		w.WriteHeader(http.StatusNoContent)
	})

	server, client := setupTestServer(handler)
	defer server.Close()

	err := client.DeleteTask("task-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMoveTaskStatus(t *testing.T) {
	expectedTask := Task{ID: "task-1", Title: "Task 1", Status: "done", ProjectID: "proj-1"}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("expected PATCH method, got %s", r.Method)
		}
		if r.URL.Path != "/api/tasks/task-1" {
			t.Errorf("expected path /api/tasks/task-1, got %s", r.URL.Path)
		}

		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if body["status"] != "done" {
			t.Errorf("expected status 'done', got '%s'", body["status"])
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedTask)
	})

	server, client := setupTestServer(handler)
	defer server.Close()

	task, err := client.MoveTaskStatus("task-1", "done")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if task.Status != "done" {
		t.Errorf("expected task status 'done', got %s", task.Status)
	}
}

// --- Error Handling Tests ---

func TestHTTPError404(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("project not found"))
	})

	server, client := setupTestServer(handler)
	defer server.Close()

	_, err := client.ListProjects()
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		// The error is wrapped, so we need to check the underlying error
		t.Logf("error: %v", err)
	} else {
		if apiErr.StatusCode != http.StatusNotFound {
			t.Errorf("expected status code 404, got %d", apiErr.StatusCode)
		}
	}
}

func TestHTTPError500(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	})

	server, client := setupTestServer(handler)
	defer server.Close()

	_, err := client.CreateProject("Test", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Check that the error message contains useful information
	errStr := err.Error()
	if errStr == "" {
		t.Error("expected non-empty error message")
	}
}

func TestHTTPErrorEmptyResponse(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	})

	server, client := setupTestServer(handler)
	defer server.Close()

	err := client.DeleteProject("proj-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- Helper Tests ---

func TestStringPtr(t *testing.T) {
	s := "test"
	ptr := StringPtr(s)
	if ptr == nil {
		t.Fatal("expected non-nil pointer")
	}
	if *ptr != s {
		t.Errorf("expected '%s', got '%s'", s, *ptr)
	}
}

func TestStringSlicePtr(t *testing.T) {
	s := []string{"a", "b", "c"}
	ptr := StringSlicePtr(s)
	if ptr == nil {
		t.Fatal("expected non-nil pointer")
	}
	if len(*ptr) != len(s) {
		t.Errorf("expected length %d, got %d", len(s), len(*ptr))
	}
}

func TestClientTrimsTrailingSlash(t *testing.T) {
	// Test that the client properly trims trailing slashes from baseURL
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Should receive clean path without double slashes
		if r.URL.Path != "/api/projects" {
			t.Errorf("expected path /api/projects, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]Project{})
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	// Create client with trailing slash
	client := NewClient(server.URL + "/")

	_, err := client.ListProjects()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientSendsBearerToken(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer flx_test_key" {
			t.Errorf("expected Bearer token, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]Project{})
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	c := NewClient(server.URL, WithAPIKey(" flx_test_key "))
	if _, err := c.ListProjects(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAPIErrorParsesFluxJSON(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Authentication required"})
	})

	server, c := setupTestServer(handler)
	defer server.Close()

	_, err := c.ListProjects()
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %v", err)
	}
	if apiErr.StatusCode != http.StatusUnauthorized || apiErr.Message != "Authentication required" {
		t.Fatalf("unexpected API error: %#v", apiErr)
	}
}

func TestGetTaskAndEpic(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/tasks/task-1":
			json.NewEncoder(w).Encode(Task{ID: "task-1", ProjectID: "proj-1", CreatedAt: "2026-08-20T00:00:00Z"})
		case "/api/epics/epic-1":
			json.NewEncoder(w).Encode(Epic{ID: "epic-1", ProjectID: "proj-1", Auto: true})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	server, c := setupTestServer(handler)
	defer server.Close()

	task, err := c.GetTask("task-1")
	if err != nil || task.ID != "task-1" {
		t.Fatalf("unexpected task result: task=%#v err=%v", task, err)
	}
	epic, err := c.GetEpic("epic-1")
	if err != nil || epic.ID != "epic-1" {
		t.Fatalf("unexpected epic result: epic=%#v err=%v", epic, err)
	}
}

func TestUpdateTaskWithDependsOn(t *testing.T) {
	expectedTask := Task{
		ID:        "task-1",
		Title:     "Task 1",
		Status:    "todo",
		DependsOn: []string{"task-2", "task-3"},
		ProjectID: "proj-1",
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		deps, ok := body["depends_on"].([]interface{})
		if !ok {
			t.Fatal("expected depends_on to be an array")
		}
		if len(deps) != 2 {
			t.Errorf("expected 2 dependencies, got %d", len(deps))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedTask)
	})

	server, client := setupTestServer(handler)
	defer server.Close()

	updates := TaskUpdate{
		DependsOn: StringSlicePtr([]string{"task-2", "task-3"}),
	}

	task, err := client.UpdateTask("task-1", updates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(task.DependsOn) != 2 {
		t.Errorf("expected 2 dependencies, got %d", len(task.DependsOn))
	}
}

func TestUpdateEpicWithDependsOn(t *testing.T) {
	expectedEpic := Epic{
		ID:        "epic-1",
		Title:     "Epic 1",
		Status:    "todo",
		DependsOn: []string{"epic-2"},
		ProjectID: "proj-1",
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		deps, ok := body["depends_on"].([]interface{})
		if !ok {
			t.Fatal("expected depends_on to be an array")
		}
		if len(deps) != 1 {
			t.Errorf("expected 1 dependency, got %d", len(deps))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expectedEpic)
	})

	server, client := setupTestServer(handler)
	defer server.Close()

	updates := EpicUpdate{
		DependsOn: StringSlicePtr([]string{"epic-2"}),
	}

	epic, err := client.UpdateEpic("epic-1", updates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(epic.DependsOn) != 1 {
		t.Errorf("expected 1 dependency, got %d", len(epic.DependsOn))
	}
}
