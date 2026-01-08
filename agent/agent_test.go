package agent

import (
	"context"
	"io"
	"testing"
	"time"
)

func TestNewClaudeCode(t *testing.T) {
	config := Config{
		WorkDir: "/tmp",
		Timeout: 30 * time.Second,
	}

	agent := NewClaudeCode(config)

	if agent == nil {
		t.Fatal("expected non-nil agent")
	}

	if agent.Name() != "Claude Code" {
		t.Errorf("expected name 'Claude Code', got '%s'", agent.Name())
	}

	if agent.IsRunning() {
		t.Error("expected agent to not be running before Start")
	}
}

func TestAgentNotStarted(t *testing.T) {
	agent := NewClaudeCode(Config{})

	_, err := agent.Wait()
	if err != ErrAgentNotStarted {
		t.Errorf("expected ErrAgentNotStarted, got %v", err)
	}
}

func TestNewRunner(t *testing.T) {
	agent := NewClaudeCode(Config{})
	runner := NewRunner(agent)

	if runner == nil {
		t.Fatal("expected non-nil runner")
	}

	if runner.IsRunning() {
		t.Error("expected runner to not be running before Run")
	}

	if runner.Agent() != agent {
		t.Error("expected runner.Agent() to return the wrapped agent")
	}
}

func TestOutputLine(t *testing.T) {
	line := OutputLine{
		Text:      "test output",
		IsStderr:  false,
		Timestamp: time.Now(),
	}

	if line.Text != "test output" {
		t.Errorf("expected text 'test output', got '%s'", line.Text)
	}

	if line.IsStderr {
		t.Error("expected IsStderr to be false")
	}
}

func TestResult(t *testing.T) {
	result := Result{
		ExitCode: 0,
		Duration: 5 * time.Second,
		Error:    nil,
	}

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}

	if result.Duration != 5*time.Second {
		t.Errorf("expected duration 5s, got %v", result.Duration)
	}
}

func TestRegistry(t *testing.T) {
	reg := NewRegistry()

	// Check default claude agent is registered
	if !reg.Has("claude") {
		t.Error("expected 'claude' agent to be registered by default")
	}

	// Check available agents
	available := reg.Available()
	found := false
	for _, name := range available {
		if name == "claude" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'claude' in Available()")
	}

	// Test Create
	agent, err := reg.Create("claude", Config{})
	if err != nil {
		t.Errorf("unexpected error creating claude agent: %v", err)
	}
	if agent == nil {
		t.Error("expected non-nil agent")
	}
	if agent.Name() != "Claude Code" {
		t.Errorf("expected name 'Claude Code', got '%s'", agent.Name())
	}

	// Test unknown agent
	_, err = reg.Create("unknown", Config{})
	if err == nil {
		t.Error("expected error for unknown agent")
	}
}

func TestRegistryCustomAgent(t *testing.T) {
	reg := NewRegistry()

	// Register a custom mock agent
	reg.Register("mock", func(cfg Config) Agent {
		return &mockAgent{name: "Mock Agent"}
	})

	if !reg.Has("mock") {
		t.Error("expected 'mock' agent to be registered")
	}

	agent, err := reg.Create("mock", Config{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if agent.Name() != "Mock Agent" {
		t.Errorf("expected name 'Mock Agent', got '%s'", agent.Name())
	}

	// Unregister
	reg.Unregister("mock")
	if reg.Has("mock") {
		t.Error("expected 'mock' agent to be unregistered")
	}
}

func TestDefaultRegistry(t *testing.T) {
	// Test the global registry functions
	available := AvailableAgents()
	found := false
	for _, name := range available {
		if name == "claude" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'claude' in AvailableAgents()")
	}

	agent, err := CreateAgent("claude", Config{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if agent == nil {
		t.Error("expected non-nil agent")
	}
}

// mockAgent is a simple mock implementation of Agent for testing
type mockAgent struct {
	name    string
	running bool
}

func (m *mockAgent) Name() string                                   { return m.name }
func (m *mockAgent) Start(ctx context.Context, prompt string) error { return nil }
func (m *mockAgent) Stdout() io.Reader                              { return nil }
func (m *mockAgent) Stderr() io.Reader                              { return nil }
func (m *mockAgent) Wait() (int, error)                             { return 0, nil }
func (m *mockAgent) Cancel() error                                  { return nil }
func (m *mockAgent) IsRunning() bool                                { return m.running }
