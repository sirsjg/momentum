package agent

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"
)

// ClaudeCode implements the Agent interface for Claude Code CLI
type ClaudeCode struct {
	config    Config
	cmd       *exec.Cmd
	stdout    io.ReadCloser
	stderr    io.ReadCloser
	ctx       context.Context
	cancel    context.CancelFunc
	mu        sync.Mutex
	running   bool
	startTime time.Time
}

// NewClaudeCode creates a new Claude Code agent instance
func NewClaudeCode(config Config) *ClaudeCode {
	return &ClaudeCode{
		config: config,
	}
}

// Name returns the agent's display name
func (c *ClaudeCode) Name() string {
	return "Claude Code"
}

// Start begins the agent subprocess with the given prompt
func (c *ClaudeCode) Start(ctx context.Context, prompt string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		return ErrAgentAlreadyRunning
	}

	// Create cancellable context
	if c.config.Timeout > 0 {
		c.ctx, c.cancel = context.WithTimeout(ctx, c.config.Timeout)
	} else {
		c.ctx, c.cancel = context.WithCancel(ctx)
	}

	// Build command: claude -p --output-format stream-json --verbose --dangerously-skip-permissions "prompt"
	// Using stream-json for real-time output instead of --print which buffers
	c.cmd = exec.CommandContext(c.ctx, "claude",
		"-p",
		"--output-format", "stream-json",
		"--verbose",
		"--dangerously-skip-permissions",
		prompt,
	)

	// Create a new process group so we can signal all children
	setProcAttr(c.cmd)

	// Set working directory
	if c.config.WorkDir != "" {
		c.cmd.Dir = c.config.WorkDir
	}

	// Set environment
	if len(c.config.Env) > 0 {
		c.cmd.Env = os.Environ()
		for k, v := range c.config.Env {
			c.cmd.Env = append(c.cmd.Env, k+"="+v)
		}
	}

	// Capture stdout/stderr
	var err error
	c.stdout, err = c.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	c.stderr, err = c.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the process
	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start claude: %w", err)
	}

	c.running = true
	c.startTime = time.Now()
	return nil
}

// Stdout returns a reader for the agent's stdout
func (c *ClaudeCode) Stdout() io.Reader {
	return c.stdout
}

// Stderr returns a reader for the agent's stderr
func (c *ClaudeCode) Stderr() io.Reader {
	return c.stderr
}

// Wait blocks until the agent completes and returns the exit code
func (c *ClaudeCode) Wait() (int, error) {
	if c.cmd == nil {
		return -1, ErrAgentNotStarted
	}

	err := c.cmd.Wait()

	c.mu.Lock()
	c.running = false
	c.mu.Unlock()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), nil
		}
		return -1, err
	}
	return 0, nil
}

// Cancel terminates the agent subprocess
func (c *ClaudeCode) Cancel() error {
	c.mu.Lock()
	if !c.running || c.cmd == nil || c.cmd.Process == nil {
		c.mu.Unlock()
		return nil
	}

	// Capture what we need before releasing lock
	pid := c.cmd.Process.Pid
	process := c.cmd.Process
	c.mu.Unlock()

	// Send interrupt signal to process tree for graceful shutdown
	killProcessTree(pid, process, false)

	// Schedule a force kill after 3 seconds if process is still running
	// Don't call Wait() here - the Runner's Wait() goroutine handles that
	go func() {
		time.Sleep(3 * time.Second)

		c.mu.Lock()
		stillRunning := c.running
		c.mu.Unlock()

		if stillRunning {
			// Force kill the process tree
			killProcessTree(pid, process, true)
		}
	}()

	return nil
}

// IsRunning returns whether the agent is currently executing
func (c *ClaudeCode) IsRunning() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running
}

// PID returns the process ID for the running agent, or 0 if unavailable.
func (c *ClaudeCode) PID() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cmd == nil || c.cmd.Process == nil {
		return 0
	}
	return c.cmd.Process.Pid
}
