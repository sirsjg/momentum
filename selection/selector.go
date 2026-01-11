// Package selection provides task selection logic for the Momentum headless mode.
package selection

import (
	"errors"
	"fmt"
	"sort"

	"github.com/stevegrehan/momentum/client"
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
// Within the qualifying tasks, newer tasks (by ID) come first.
func (s *Selector) SelectTask() (*client.Task, error) {
	// Case 1: Specific task ID provided
	if s.taskID != "" {
		return s.fetchSpecificTask()
	}

	// Case 2: Epic ID provided - get tasks from that epic's project filtered by epic
	if s.epicID != "" {
		return s.selectFromEpic()
	}

	// Case 3: Project ID provided - get tasks from that project
	if s.projectID != "" {
		return s.selectFromProject(s.projectID)
	}

	// Case 4: No filters - search across all projects
	return s.selectFromAllProjects()
}

// fetchSpecificTask fetches a task by its ID.
// Since the client doesn't have a GetTask method, we need to find it
// by listing tasks from all projects.
func (s *Selector) fetchSpecificTask() (*client.Task, error) {
	// We need to find the task across all projects since we don't know which project it belongs to
	projects, err := s.client.ListProjects()
	if err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}

	for _, project := range projects {
		tasks, err := s.client.ListTasks(project.ID, client.TaskFilters{})
		if err != nil {
			// Log but continue searching other projects
			continue
		}

		for i := range tasks {
			if tasks[i].ID == s.taskID {
				return &tasks[i], nil
			}
		}
	}

	return nil, fmt.Errorf("task %s not found: %w", s.taskID, ErrNoTaskAvailable)
}

// selectFromEpic selects the best task from the specified epic.
func (s *Selector) selectFromEpic() (*client.Task, error) {
	// First, we need to find which project this epic belongs to
	projects, err := s.client.ListProjects()
	if err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}

	// Find the project containing the epic and check if it's auto-enabled
	var targetProjectID string
	var epicIsAuto bool
	for _, project := range projects {
		epics, err := s.client.ListEpics(project.ID)
		if err != nil {
			continue
		}

		for _, epic := range epics {
			if epic.ID == s.epicID {
				targetProjectID = project.ID
				epicIsAuto = epic.Auto
				break
			}
		}
		if targetProjectID != "" {
			break
		}
	}

	if targetProjectID == "" {
		return nil, fmt.Errorf("epic %s not found: %w", s.epicID, ErrNoTaskAvailable)
	}

	// Only process epics with auto=true
	if !epicIsAuto {
		return nil, fmt.Errorf("epic %s has auto=false: %w", s.epicID, ErrNoTaskAvailable)
	}

	// Get tasks filtered by epic
	filters := client.TaskFilters{
		EpicID: client.StringPtr(s.epicID),
	}
	tasks, err := s.client.ListTasks(targetProjectID, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks for epic %s: %w", s.epicID, err)
	}

	// Build auto epic IDs map (just this epic since we already verified it's auto)
	autoEpicIDs := map[string]bool{s.epicID: true}

	return s.selectBestTask(tasks, autoEpicIDs)
}

// selectFromProject selects the best task from the specified project.
func (s *Selector) selectFromProject(projectID string) (*client.Task, error) {
	tasks, err := s.client.ListTasks(projectID, client.TaskFilters{})
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks for project %s: %w", projectID, err)
	}

	// Get auto epic IDs for this project
	autoEpicIDs, err := s.getAutoEpicIDs(projectID)
	if err != nil {
		return nil, err
	}

	return s.selectBestTask(tasks, autoEpicIDs)
}

// selectFromAllProjects selects the best task across all projects.
func (s *Selector) selectFromAllProjects() (*client.Task, error) {
	projects, err := s.client.ListProjects()
	if err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}

	if len(projects) == 0 {
		return nil, fmt.Errorf("no projects found: %w", ErrNoTaskAvailable)
	}

	var allTasks []client.Task
	allAutoEpicIDs := make(map[string]bool)

	for _, project := range projects {
		tasks, err := s.client.ListTasks(project.ID, client.TaskFilters{})
		if err != nil {
			// Log but continue with other projects
			continue
		}
		allTasks = append(allTasks, tasks...)

		// Get auto epic IDs for this project
		autoEpicIDs, err := s.getAutoEpicIDs(project.ID)
		if err != nil {
			continue
		}
		for epicID := range autoEpicIDs {
			allAutoEpicIDs[epicID] = true
		}
	}

	return s.selectBestTask(allTasks, allAutoEpicIDs)
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
// Tasks are sorted by ID descending (newer first).
func (s *Selector) selectBestTask(tasks []client.Task, autoEpicIDs map[string]bool) (*client.Task, error) {
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
	candidates := filterAndSortTasks(autoTasks)

	if len(candidates) == 0 {
		return nil, ErrNoTaskAvailable
	}

	return &candidates[0], nil
}

// filterAndSortTasks filters tasks to only include unblocked tasks with status "todo",
// sorted by ID descending (newer first).
func filterAndSortTasks(tasks []client.Task) []client.Task {
	var unblockedTodos []client.Task

	for _, task := range tasks {
		if !task.Blocked && task.Status == "todo" {
			unblockedTodos = append(unblockedTodos, task)
		}
	}

	// Sort by ID descending (newer first)
	sort.Slice(unblockedTodos, func(i, j int) bool {
		return unblockedTodos[i].ID > unblockedTodos[j].ID
	})

	return unblockedTodos
}
