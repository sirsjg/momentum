package agent

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
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
	c.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

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
	defer c.mu.Unlock()

	if !c.running || c.cmd == nil || c.cmd.Process == nil {
		return nil
	}

	// Cancel the context first - this signals exec.CommandContext to kill the process
	if c.cancel != nil {
		c.cancel()
	}

	// Also send SIGINT to process group for graceful shutdown of any children
	// Use negative PID to signal the process group
	if err := syscall.Kill(-c.cmd.Process.Pid, syscall.SIGINT); err != nil {
		// If process group signal fails, try direct signal
		c.cmd.Process.Signal(os.Interrupt)
	}

	// Give it 3 seconds to shutdown gracefully, then force kill
	done := make(chan struct{})
	go func() {
		c.cmd.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(3 * time.Second):
		// Force kill the process group
		syscall.Kill(-c.cmd.Process.Pid, syscall.SIGKILL)
		return c.cmd.Process.Kill()
	}
}

// IsRunning returns whether the agent is currently executing
func (c *ClaudeCode) IsRunning() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running
}
