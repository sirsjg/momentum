package agent

import (
	"bufio"
	"context"
	"io"
	"sync"
	"time"
)

// Runner manages agent execution and output streaming
type Runner struct {
	agent      Agent
	outputChan chan OutputLine
	doneChan   chan Result
	mu         sync.Mutex
	running    bool
	startTime  time.Time
}

// NewRunner creates a new agent runner
func NewRunner(agent Agent) *Runner {
	return &Runner{
		agent:      agent,
		outputChan: make(chan OutputLine, 1000),
		doneChan:   make(chan Result, 1),
	}
}

// Run starts the agent and streams output
func (r *Runner) Run(ctx context.Context, prompt string) error {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return ErrAgentAlreadyRunning
	}
	r.running = true
	r.startTime = time.Now()
	r.mu.Unlock()

	// Start the agent
	if err := r.agent.Start(ctx, prompt); err != nil {
		r.mu.Lock()
		r.running = false
		r.mu.Unlock()
		return err
	}

	// Use WaitGroup to track streaming goroutines
	var wg sync.WaitGroup
	wg.Add(2)

	// Stream stdout
	go func() {
		defer wg.Done()
		r.streamOutput(r.agent.Stdout(), false)
	}()

	// Stream stderr
	go func() {
		defer wg.Done()
		r.streamOutput(r.agent.Stderr(), true)
	}()

	// Wait for completion in background
	go func() {
		exitCode, err := r.agent.Wait()

		// Wait for streaming to complete
		wg.Wait()

		r.mu.Lock()
		duration := time.Since(r.startTime)
		r.running = false
		r.mu.Unlock()

		r.doneChan <- Result{
			ExitCode: exitCode,
			Duration: duration,
			Error:    err,
		}
		close(r.outputChan)
		close(r.doneChan)
	}()

	return nil
}

func (r *Runner) streamOutput(reader io.Reader, isStderr bool) {
	if reader == nil {
		return
	}

	scanner := bufio.NewScanner(reader)
	// Increase buffer size for long lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := OutputLine{
			Text:      scanner.Text(),
			IsStderr:  isStderr,
			Timestamp: time.Now(),
		}

		select {
		case r.outputChan <- line:
		default:
			// Channel full, drop oldest by reading one and adding new
			select {
			case <-r.outputChan:
				r.outputChan <- line
			default:
			}
		}
	}
}

// Output returns the channel for receiving output lines
func (r *Runner) Output() <-chan OutputLine {
	return r.outputChan
}

// Done returns the channel for completion notification
func (r *Runner) Done() <-chan Result {
	return r.doneChan
}

// Cancel terminates the running agent
func (r *Runner) Cancel() error {
	return r.agent.Cancel()
}

// IsRunning returns whether the agent is executing
func (r *Runner) IsRunning() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.running
}

// Agent returns the underlying agent
func (r *Runner) Agent() Agent {
	return r.agent
}
