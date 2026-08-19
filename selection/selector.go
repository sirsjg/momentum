// Package selection provides task selection logic for the Momentum headless mode.
package selection

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/sirsjg/momentum/client"
)

// ErrNoTaskAvailable is returned when no suitable task can be found.
var ErrNoTaskAvailable = errors.New("no task available matching the selection criteria")

// Selector handles task selection logic for headless mode.
// It supports filtering by project, epic, or specific task ID.
type Selector struct {
	client    *client.Client
	projectID string
	epicID    string
	taskID    string
}

// NewSelector creates a new Selector with the given filters.
// All filter parameters are optional - pass empty strings if not needed.
func NewSelector(c *client.Client, projectID, epicID, taskID string) *Selector {
	return &Selector{
		client:    c,
		projectID: projectID,
		epicID:    epicID,
		taskID:    taskID,
	}
}

// SelectTask selects a task based on the configured filters.
// The selection logic follows this priority:
//  1. If taskID is provided, fetch that specific task
//  2. If epicID is provided, get the first unblocked todo task from that epic (if epic has auto=true)
//  3. If projectID is provided, get the first unblocked todo task from that project (only from auto epics)
//  4. If nothing is provided, get the newest unblocked todo task across ALL projects (only from auto epics)
//
// Only tasks meeting ALL of these criteria are considered:
//   - Task belongs to an epic with auto=true
//   - Task has status "todo"
//   - Task is unblocked (blocked=false)
//
// Within the qualifying tasks, newer tasks (by created_at) come first.
func (s *Selector) SelectTask() (*client.Task, error) {
	return s.SelectTaskExcluding(nil)
}

// SelectTaskExcluding selects a task while skipping any task IDs in excluded.
func (s *Selector) SelectTaskExcluding(excluded map[string]bool) (*client.Task, error) {
	// Case 1: Specific task ID provided
	if s.taskID != "" {
		return s.fetchSpecificTask(excluded)
	}

	// Case 2: Epic ID provided - get tasks from that epic's project filtered by epic
	if s.epicID != "" {
		return s.selectFromEpic(excluded)
	}

	// Case 3: Project ID provided - get tasks from that project
	if s.projectID != "" {
		return s.selectFromProject(s.projectID, excluded)
	}

	// Case 4: No filters - search across all projects
	return s.selectFromAllProjects(excluded)
}

// fetchSpecificTask fetches a task by its ID.
func (s *Selector) fetchSpecificTask(excluded map[string]bool) (*client.Task, error) {
	if excluded != nil && excluded[s.taskID] {
		return nil, fmt.Errorf("task %s excluded: %w", s.taskID, ErrNoTaskAvailable)
	}

	task, err := s.client.GetTask(s.taskID)
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
			return nil, fmt.Errorf("task %s not found: %w", s.taskID, ErrNoTaskAvailable)
		}
		return nil, fmt.Errorf("failed to get task %s: %w", s.taskID, err)
	}
	return task, nil
}

// selectFromEpic selects the best task from the specified epic.
func (s *Selector) selectFromEpic(excluded map[string]bool) (*client.Task, error) {
	epic, err := s.client.GetEpic(s.epicID)
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
			return nil, fmt.Errorf("epic %s not found: %w", s.epicID, ErrNoTaskAvailable)
		}
		return nil, fmt.Errorf("failed to get epic %s: %w", s.epicID, err)
	}

	// Only process epics with auto=true
	if !epic.Auto {
		return nil, fmt.Errorf("epic %s has auto=false: %w", s.epicID, ErrNoTaskAvailable)
	}

	// Get tasks filtered by epic
	filters := client.TaskFilters{
		EpicID: client.StringPtr(s.epicID),
	}
	tasks, err := s.client.ListTasks(epic.ProjectID, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks for epic %s: %w", s.epicID, err)
	}

	// Build auto epic IDs map (just this epic since we already verified it's auto)
	autoEpicIDs := map[string]bool{s.epicID: true}

	return s.selectBestTask(tasks, autoEpicIDs, excluded)
}

// selectFromProject selects the best task from the specified project.
func (s *Selector) selectFromProject(projectID string, excluded map[string]bool) (*client.Task, error) {
	tasks, err := s.client.ListTasks(projectID, client.TaskFilters{})
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks for project %s: %w", projectID, err)
	}

	// Get auto epic IDs for this project
	autoEpicIDs, err := s.getAutoEpicIDs(projectID)
	if err != nil {
		return nil, err
	}

	return s.selectBestTask(tasks, autoEpicIDs, excluded)
}

// selectFromAllProjects selects the best task across all projects.
func (s *Selector) selectFromAllProjects(excluded map[string]bool) (*client.Task, error) {
	projects, err := s.client.ListProjects()
	if err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}

	if len(projects) == 0 {
		return nil, fmt.Errorf("no projects found: %w", ErrNoTaskAvailable)
	}

	var allTasks []client.Task
	allAutoEpicIDs := make(map[string]bool)
	var firstErr error
	inspectedProjects := 0

	for _, project := range projects {
		tasks, err := s.client.ListTasks(project.ID, client.TaskFilters{})
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}

		// Get auto epic IDs for this project
		autoEpicIDs, err := s.getAutoEpicIDs(project.ID)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		inspectedProjects++
		allTasks = append(allTasks, tasks...)
		for epicID := range autoEpicIDs {
			allAutoEpicIDs[epicID] = true
		}
	}
	if inspectedProjects == 0 && firstErr != nil {
		return nil, fmt.Errorf("failed to inspect Flux projects: %w", firstErr)
	}

	return s.selectBestTask(allTasks, allAutoEpicIDs, excluded)
}

// getAutoEpicIDs returns a map of epic IDs that have auto=true for the given project.
func (s *Selector) getAutoEpicIDs(projectID string) (map[string]bool, error) {
	epics, err := s.client.ListEpics(projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to list epics for project %s: %w", projectID, err)
	}

	autoEpicIDs := make(map[string]bool)
	for _, epic := range epics {
		if epic.Auto {
			autoEpicIDs[epic.ID] = true
		}
	}
	return autoEpicIDs, nil
}

// selectBestTask selects the best task from a list.
// Only tasks belonging to auto-enabled epics with status "todo" and unblocked are considered.
// Tasks are sorted by created_at descending (newer first), with ID as a
// deterministic fallback for records created by older Flux versions.
func (s *Selector) selectBestTask(tasks []client.Task, autoEpicIDs map[string]bool, excluded map[string]bool) (*client.Task, error) {
	if len(tasks) == 0 {
		return nil, ErrNoTaskAvailable
	}

	// Filter to only tasks belonging to auto-enabled epics
	var autoTasks []client.Task
	for _, task := range tasks {
		if task.EpicID != "" && autoEpicIDs[task.EpicID] {
			autoTasks = append(autoTasks, task)
		}
	}

	// Filter and sort tasks
	candidates := filterAndSortTasks(autoTasks, excluded)

	if len(candidates) == 0 {
		return nil, ErrNoTaskAvailable
	}

	return &candidates[0], nil
}

// filterAndSortTasks returns active, unblocked todo tasks with newest creation
// timestamps first.
func filterAndSortTasks(tasks []client.Task, excluded map[string]bool) []client.Task {
	var unblockedTodos []client.Task

	for _, task := range tasks {
		if excluded != nil && excluded[task.ID] {
			continue
		}
		if !task.Archived && !task.Blocked && task.Status == "todo" {
			unblockedTodos = append(unblockedTodos, task)
		}
	}

	// Flux uses random IDs, so they do not encode creation order.
	sort.Slice(unblockedTodos, func(i, j int) bool {
		iCreated, iErr := time.Parse(time.RFC3339Nano, unblockedTodos[i].CreatedAt)
		jCreated, jErr := time.Parse(time.RFC3339Nano, unblockedTodos[j].CreatedAt)
		if iErr == nil && jErr == nil && !iCreated.Equal(jCreated) {
			return iCreated.After(jCreated)
		}
		if iErr == nil && jErr != nil {
			return true
		}
		if iErr != nil && jErr == nil {
			return false
		}
		return unblockedTodos[i].ID > unblockedTodos[j].ID
	})

	return unblockedTodos
}
