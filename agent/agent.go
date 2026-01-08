// Package agent provides subprocess management for AI coding agents.
package agent

import (
	"context"
	"io"
	"time"
)

// Agent defines the interface for AI coding agents.
type Agent interface {
	// Name returns the agent's display name
	Name() string

	// Start begins the agent subprocess with the given prompt
	Start(ctx context.Context, prompt string) error

	// Stdout returns a reader for the agent's stdout
	Stdout() io.Reader

	// Stderr returns a reader for the agent's stderr
	Stderr() io.Reader

	// Wait blocks until the agent completes and returns the exit code
	Wait() (exitCode int, err error)

	// Cancel terminates the agent subprocess
	Cancel() error

	// IsRunning returns whether the agent is currently executing
	IsRunning() bool
}

// Config holds agent configuration
type Config struct {
	// WorkDir is the working directory for the agent
	WorkDir string

	// Env contains additional environment variables
	Env map[string]string

	// Timeout is the maximum execution time (0 = no timeout)
	Timeout time.Duration
}

// Result represents the outcome of an agent execution
type Result struct {
	ExitCode int
	Duration time.Duration
	Error    error
}

// OutputLine represents a single line of agent output
type OutputLine struct {
	Text      string
	IsStderr  bool
	Timestamp time.Time
}
