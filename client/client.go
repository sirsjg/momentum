// Package client provides a REST client for the Flux API.
package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client is a REST client for the Flux API.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new Flux API client with the given base URL.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Project represents a Flux project.
type Project struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// Epic represents a Flux epic within a project.
type Epic struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Notes     string   `json:"notes,omitempty"`
	Status    string   `json:"status"`
	DependsOn []string `json:"depends_on,omitempty"`
	ProjectID string   `json:"project_id"`
	Auto      bool     `json:"auto,omitempty"`
}

// Guardrail represents a numbered instruction for agents.
type Guardrail struct {
	ID     string `json:"id"`
	Number int    `json:"number"`
	Text   string `json:"text"`
}

// Task represents a Flux task within a project.
type Task struct {
	ID                 string      `json:"id"`
	Title              string      `json:"title"`
	Notes              string      `json:"notes,omitempty"`
	Status             string      `json:"status"`
	DependsOn          []string    `json:"depends_on,omitempty"`
	ProjectID          string      `json:"project_id"`
	EpicID             string      `json:"epic_id,omitempty"`
	Blocked            bool        `json:"blocked"`
	Priority           *int        `json:"priority,omitempty"`
	AcceptanceCriteria []string    `json:"acceptance_criteria,omitempty"`
	Guardrails         []Guardrail `json:"guardrails,omitempty"`
}

// ProjectContext contains project-level context for agents.
type ProjectContext struct {
	Problem       string   `json:"problem,omitempty"`
	BusinessRules []string `json:"businessRules,omitempty"`
	Notes         string   `json:"notes,omitempty"`
}

// EpicUpdate contains optional fields for updating an epic.
type EpicUpdate struct {
	Title     *string   `json:"title,omitempty"`
	Notes     *string   `json:"notes,omitempty"`
	Status    *string   `json:"status,omitempty"`
	DependsOn *[]string `json:"depends_on,omitempty"`
}

// TaskUpdate contains optional fields for updating a task.
type TaskUpdate struct {
	Title     *string   `json:"title,omitempty"`
	Notes     *string   `json:"notes,omitempty"`
	Status    *string   `json:"status,omitempty"`
	EpicID    *string   `json:"epic_id,omitempty"`
	DependsOn *[]string `json:"depends_on,omitempty"`
}

// TaskFilters contains optional filters for listing tasks.
type TaskFilters struct {
	EpicID *string
	Status *string
}

// APIError represents an error response from the Flux API.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("flux api error (status %d): %s", e.StatusCode, e.Message)
}

// doRequest performs an HTTP request and handles the response.
func (c *Client) doRequest(method, path string, body interface{}, result interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequest(method, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message := string(respBody)
		if message == "" {
			message = http.StatusText(resp.StatusCode)
		}
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    message,
		}
	}

	if result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}

	return nil
}

// --- Project Operations ---

// ListProjects returns all Flux projects.
func (c *Client) ListProjects() ([]Project, error) {
	var projects []Project
	if err := c.doRequest(http.MethodGet, "/api/projects", nil, &projects); err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}
	return projects, nil
}

// CreateProject creates a new project with the given name and description.
func (c *Client) CreateProject(name, description string) (*Project, error) {
	body := map[string]string{
		"name": name,
	}
	if description != "" {
		body["description"] = description
	}

	var project Project
	if err := c.doRequest(http.MethodPost, "/api/projects", body, &project); err != nil {
		return nil, fmt.Errorf("failed to create project: %w", err)
	}
	return &project, nil
}

// UpdateProject updates an existing project's name and/or description.
func (c *Client) UpdateProject(projectID, name, description string) (*Project, error) {
	body := make(map[string]string)
	if name != "" {
		body["name"] = name
	}
	if description != "" {
		body["description"] = description
	}

	var project Project
	path := fmt.Sprintf("/api/projects/%s", url.PathEscape(projectID))
	if err := c.doRequest(http.MethodPatch, path, body, &project); err != nil {
		return nil, fmt.Errorf("failed to update project %s: %w", projectID, err)
	}
	return &project, nil
}

// DeleteProject deletes a project and all its epics and tasks.
func (c *Client) DeleteProject(projectID string) error {
	path := fmt.Sprintf("/api/projects/%s", url.PathEscape(projectID))
	if err := c.doRequest(http.MethodDelete, path, nil, nil); err != nil {
		return fmt.Errorf("failed to delete project %s: %w", projectID, err)
	}
	return nil
}

// GetProjectContext retrieves the project context (problem, business rules, notes).
func (c *Client) GetProjectContext(projectID string) (*ProjectContext, error) {
	var ctx ProjectContext
	path := fmt.Sprintf("/api/projects/%s/context", url.PathEscape(projectID))
	if err := c.doRequest(http.MethodGet, path, nil, &ctx); err != nil {
		return nil, fmt.Errorf("failed to get project context for %s: %w", projectID, err)
	}
	// Return nil if context is empty
	if ctx.Problem == "" && len(ctx.BusinessRules) == 0 && ctx.Notes == "" {
		return nil, nil
	}
	return &ctx, nil
}

// --- Epic Operations ---

// ListEpics returns all epics in the specified project.
func (c *Client) ListEpics(projectID string) ([]Epic, error) {
	var epics []Epic
	path := fmt.Sprintf("/api/projects/%s/epics", url.PathEscape(projectID))
	if err := c.doRequest(http.MethodGet, path, nil, &epics); err != nil {
		return nil, fmt.Errorf("failed to list epics for project %s: %w", projectID, err)
	}
	return epics, nil
}

// CreateEpic creates a new epic in the specified project.
func (c *Client) CreateEpic(projectID, title, notes string) (*Epic, error) {
	body := map[string]string{
		"title": title,
	}
	if notes != "" {
		body["notes"] = notes
	}

	var epic Epic
	path := fmt.Sprintf("/api/projects/%s/epics", url.PathEscape(projectID))
	if err := c.doRequest(http.MethodPost, path, body, &epic); err != nil {
		return nil, fmt.Errorf("failed to create epic in project %s: %w", projectID, err)
	}
	return &epic, nil
}

// UpdateEpic updates an existing epic with the provided updates.
func (c *Client) UpdateEpic(epicID string, updates EpicUpdate) (*Epic, error) {
	var epic Epic
	path := fmt.Sprintf("/api/epics/%s", url.PathEscape(epicID))
	if err := c.doRequest(http.MethodPatch, path, updates, &epic); err != nil {
		return nil, fmt.Errorf("failed to update epic %s: %w", epicID, err)
	}
	return &epic, nil
}

// DeleteEpic deletes an epic. Tasks will become unassigned (not deleted).
func (c *Client) DeleteEpic(epicID string) error {
	path := fmt.Sprintf("/api/epics/%s", url.PathEscape(epicID))
	if err := c.doRequest(http.MethodDelete, path, nil, nil); err != nil {
		return fmt.Errorf("failed to delete epic %s: %w", epicID, err)
	}
	return nil
}

// --- Task Operations ---

// ListTasks returns all tasks in the specified project, optionally filtered.
func (c *Client) ListTasks(projectID string, filters TaskFilters) ([]Task, error) {
	var tasks []Task
	path := fmt.Sprintf("/api/projects/%s/tasks", url.PathEscape(projectID))

	// Build query parameters
	queryParams := url.Values{}
	if filters.EpicID != nil && *filters.EpicID != "" {
		queryParams.Set("epic_id", *filters.EpicID)
	}
	if filters.Status != nil && *filters.Status != "" {
		queryParams.Set("status", *filters.Status)
	}
	if len(queryParams) > 0 {
		path += "?" + queryParams.Encode()
	}

	if err := c.doRequest(http.MethodGet, path, nil, &tasks); err != nil {
		return nil, fmt.Errorf("failed to list tasks for project %s: %w", projectID, err)
	}
	return tasks, nil
}

// CreateTask creates a new task in the specified project.
func (c *Client) CreateTask(projectID, title, notes, epicID string) (*Task, error) {
	body := map[string]string{
		"title": title,
	}
	if notes != "" {
		body["notes"] = notes
	}
	if epicID != "" {
		body["epic_id"] = epicID
	}

	var task Task
	path := fmt.Sprintf("/api/projects/%s/tasks", url.PathEscape(projectID))
	if err := c.doRequest(http.MethodPost, path, body, &task); err != nil {
		return nil, fmt.Errorf("failed to create task in project %s: %w", projectID, err)
	}
	return &task, nil
}

// UpdateTask updates an existing task with the provided updates.
func (c *Client) UpdateTask(taskID string, updates TaskUpdate) (*Task, error) {
	var task Task
	path := fmt.Sprintf("/api/tasks/%s", url.PathEscape(taskID))
	if err := c.doRequest(http.MethodPatch, path, updates, &task); err != nil {
		return nil, fmt.Errorf("failed to update task %s: %w", taskID, err)
	}
	return &task, nil
}

// DeleteTask deletes a task.
func (c *Client) DeleteTask(taskID string) error {
	path := fmt.Sprintf("/api/tasks/%s", url.PathEscape(taskID))
	if err := c.doRequest(http.MethodDelete, path, nil, nil); err != nil {
		return fmt.Errorf("failed to delete task %s: %w", taskID, err)
	}
	return nil
}

// MoveTaskStatus is a shortcut method to quickly change a task's status.
func (c *Client) MoveTaskStatus(taskID, status string) (*Task, error) {
	updates := TaskUpdate{
		Status: StringPtr(status),
	}
	return c.UpdateTask(taskID, updates)
}

// --- Helper Functions ---

// StringPtr returns a pointer to the given string. Useful for optional fields in updates.
func StringPtr(s string) *string {
	return &s
}

// StringSlicePtr returns a pointer to the given string slice. Useful for optional depends_on fields.
func StringSlicePtr(s []string) *[]string {
	return &s
}
