package ui

import (
	"testing"
	"time"

	"github.com/sirsjg/momentum/agent"
)

func TestTruncate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{"short string", "hello", 10, "hello"},
		{"exact length", "hello", 5, "hello"},
		{"needs truncation", "hello world", 8, "hello..."},
		{"very short max", "hello", 3, "hel"},
		{"max length 4", "hello world", 4, "h..."},
		{"empty string", "", 10, ""},
		{"zero max", "hello", 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncate(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	panel := &AgentPanel{
		StartTime: start,
		EndTime:   start.Add(65 * time.Second),
		Result:    &agent.Result{ExitCode: 0},
	}

	if got := formatDuration(panel); got != "01:05" {
		t.Errorf("expected duration 01:05, got %q", got)
	}

	panel = &AgentPanel{
		StartTime: start,
		EndTime:   start.Add(2*time.Hour + 3*time.Minute + 4*time.Second),
		Result:    &agent.Result{ExitCode: 0},
	}

	if got := formatDuration(panel); got != "02:03:04" {
		t.Errorf("expected duration 02:03:04, got %q", got)
	}
}

func TestClampScroll(t *testing.T) {
	if got := clampScroll(0, 10, 0, true); got != 0 {
		t.Errorf("expected 0, got %d", got)
	}
	if got := clampScroll(5, 3, 0, true); got != 2 {
		t.Errorf("expected 2, got %d", got)
	}
	if got := clampScroll(5, 3, -1, false); got != 0 {
		t.Errorf("expected 0, got %d", got)
	}
	if got := clampScroll(5, 3, 10, false); got != 2 {
		t.Errorf("expected 2, got %d", got)
	}
}
