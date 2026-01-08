package agent

import "errors"

var (
	// ErrAgentAlreadyRunning is returned when attempting to start an agent that is already running
	ErrAgentAlreadyRunning = errors.New("agent is already running")

	// ErrAgentNotStarted is returned when attempting to wait on an agent that hasn't been started
	ErrAgentNotStarted = errors.New("agent has not been started")

	// ErrAgentNotFound is returned when the agent executable cannot be found
	ErrAgentNotFound = errors.New("agent executable not found")

	// ErrAgentTimeout is returned when the agent execution times out
	ErrAgentTimeout = errors.New("agent execution timed out")

	// ErrAgentCancelled is returned when the agent execution was cancelled
	ErrAgentCancelled = errors.New("agent execution was cancelled")
)
