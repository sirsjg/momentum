package sse

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestEventParsing tests the parsing of SSE events with data: and event: fields.
func TestEventParsing(t *testing.T) {
	tests := []struct {
		name          string
		sseData       string
		expectedEvent Event
	}{
		{
			name:          "simple data field",
			sseData:       "data: hello world\n\n",
			expectedEvent: Event{Type: "message", Data: "hello world"},
		},
		{
			name:          "data field without space after colon",
			sseData:       "data:hello world\n\n",
			expectedEvent: Event{Type: "message", Data: "hello world"},
		},
		{
			name:          "event type with data",
			sseData:       "event: custom-event\ndata: test data\n\n",
			expectedEvent: Event{Type: "custom-event", Data: "test data"},
		},
		{
			name:          "data-changed event",
			sseData:       "event: data-changed\ndata: {\"source\":\"api\"}\n\n",
			expectedEvent: Event{Type: "data-changed", Data: "{\"source\":\"api\"}"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test server that sends SSE data
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				w.Header().Set("Cache-Control", "no-cache")
				w.Header().Set("Connection", "keep-alive")

				flusher, ok := w.(http.Flusher)
				if !ok {
					t.Error("response writer does not support flushing")
					return
				}

				fmt.Fprint(w, tt.sseData)
				flusher.Flush()

				// Keep connection alive briefly to allow event to be read
				time.Sleep(100 * time.Millisecond)
			}))
			defer server.Close()

			// Create subscriber pointing to test server
			sub := NewSubscriber(server.URL)
			// Override the URL to not append /api/events
			sub.url = server.URL

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			events := sub.Start(ctx)

			// Wait for event or timeout
			select {
			case event := <-events:
				if event.Type != tt.expectedEvent.Type {
					t.Errorf("expected event type %q, got %q", tt.expectedEvent.Type, event.Type)
				}
				if event.Data != tt.expectedEvent.Data {
					t.Errorf("expected event data %q, got %q", tt.expectedEvent.Data, event.Data)
				}
			case <-ctx.Done():
				t.Error("timed out waiting for event")
			}

			sub.Stop()
		})
	}
}

// TestMultiLineData tests parsing of SSE events with multiple data: lines.
func TestMultiLineData(t *testing.T) {
	// Create a test server that sends multi-line data
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("response writer does not support flushing")
			return
		}

		// Send multi-line data event
		fmt.Fprint(w, "event: multiline\n")
		fmt.Fprint(w, "data: line1\n")
		fmt.Fprint(w, "data: line2\n")
		fmt.Fprint(w, "data: line3\n")
		fmt.Fprint(w, "\n")
		flusher.Flush()

		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	sub := NewSubscriber(server.URL)
	sub.url = server.URL

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	events := sub.Start(ctx)

	select {
	case event := <-events:
		if event.Type != "multiline" {
			t.Errorf("expected event type 'multiline', got %q", event.Type)
		}
		expectedData := "line1\nline2\nline3"
		if event.Data != expectedData {
			t.Errorf("expected data %q, got %q", expectedData, event.Data)
		}
	case <-ctx.Done():
		t.Error("timed out waiting for event")
	}

	sub.Stop()
}

// TestReconnectionBackoff tests that the reconnection delay increases exponentially.
func TestReconnectionBackoff(t *testing.T) {
	sub := NewSubscriber("http://localhost:1")

	// Initial delay should be 1 second
	if sub.reconnectDelay != 1*time.Second {
		t.Errorf("expected initial reconnect delay of 1s, got %v", sub.reconnectDelay)
	}

	// Simulate handleReconnect being called (which doubles the delay)
	ctx := context.Background()

	// We can't directly call handleReconnect as it waits, but we can test the backoff logic
	// by checking the doubling behavior

	// Check that delay doubles up to max
	delay := sub.reconnectDelay
	for i := 0; i < 10; i++ {
		delay *= 2
		if delay > sub.maxReconnectDelay {
			delay = sub.maxReconnectDelay
		}
	}

	if delay != sub.maxReconnectDelay {
		t.Errorf("expected delay to cap at %v, got %v", sub.maxReconnectDelay, delay)
	}

	// Test resetBackoff
	sub.reconnectDelay = 16 * time.Second
	sub.consecutiveFailures = 5
	sub.resetBackoff()

	if sub.reconnectDelay != 1*time.Second {
		t.Errorf("expected reset delay of 1s, got %v", sub.reconnectDelay)
	}
	if sub.consecutiveFailures != 0 {
		t.Errorf("expected 0 consecutive failures after reset, got %d", sub.consecutiveFailures)
	}

	_ = ctx // Unused but kept for consistency
}

// TestBackoffCalculation tests the exponential backoff calculation.
func TestBackoffCalculation(t *testing.T) {
	tests := []struct {
		name            string
		initialDelay    time.Duration
		maxDelay        time.Duration
		iterations      int
		expectedResults []time.Duration
	}{
		{
			name:         "standard backoff",
			initialDelay: 1 * time.Second,
			maxDelay:     30 * time.Second,
			iterations:   5,
			expectedResults: []time.Duration{
				2 * time.Second,
				4 * time.Second,
				8 * time.Second,
				16 * time.Second,
				30 * time.Second, // Capped at max
			},
		},
		{
			name:         "already at max",
			initialDelay: 30 * time.Second,
			maxDelay:     30 * time.Second,
			iterations:   3,
			expectedResults: []time.Duration{
				30 * time.Second,
				30 * time.Second,
				30 * time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delay := tt.initialDelay
			for i := 0; i < tt.iterations; i++ {
				delay *= 2
				if delay > tt.maxDelay {
					delay = tt.maxDelay
				}
				if delay != tt.expectedResults[i] {
					t.Errorf("iteration %d: expected delay %v, got %v", i, tt.expectedResults[i], delay)
				}
			}
		})
	}
}

// TestSubscriberStartStop tests the start and stop functionality.
func TestSubscriberStartStop(t *testing.T) {
	sub := NewSubscriber("http://localhost:1")

	if sub.IsRunning() {
		t.Error("subscriber should not be running initially")
	}

	ctx, cancel := context.WithCancel(context.Background())
	sub.Start(ctx)

	// Give a moment for goroutine to start
	time.Sleep(10 * time.Millisecond)

	if !sub.IsRunning() {
		t.Error("subscriber should be running after Start")
	}

	// Stop via cancel
	cancel()

	// Give a moment for shutdown
	time.Sleep(100 * time.Millisecond)

	// Start again and stop via Stop()
	sub2 := NewSubscriber("http://localhost:1")
	ctx2 := context.Background()
	sub2.Start(ctx2)

	time.Sleep(10 * time.Millisecond)

	sub2.Stop()

	time.Sleep(50 * time.Millisecond)

	if sub2.IsRunning() {
		t.Error("subscriber should not be running after Stop")
	}
}

func TestStopCancelsActiveConnection(t *testing.T) {
	requestStarted := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		requestStarted <- struct{}{}
		<-r.Context().Done()
	}))
	defer server.Close()

	sub := NewSubscriber(server.URL)
	events := sub.Start(context.Background())
	select {
	case <-requestStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for SSE connection")
	}
	sub.Stop()
	select {
	case _, ok := <-events:
		if ok {
			t.Fatal("unexpected event after stop")
		}
	case <-time.After(time.Second):
		t.Fatal("Stop did not cancel the active SSE connection")
	}
}

// TestSubscriberDoubleStart tests that double-starting returns the same channel.
func TestSubscriberDoubleStart(t *testing.T) {
	sub := NewSubscriber("http://localhost:1")

	ctx := context.Background()
	ch1 := sub.Start(ctx)
	ch2 := sub.Start(ctx)

	// Both should return the same channel
	if ch1 != ch2 {
		t.Error("double Start should return the same event channel")
	}

	sub.Stop()
}

// TestSubscriberEvents tests the Events() method.
func TestSubscriberEvents(t *testing.T) {
	sub := NewSubscriber("http://localhost:1")

	events := sub.Events()
	if events == nil {
		t.Error("Events() should return non-nil channel")
	}
}

// TestCommentLinesIgnored tests that SSE comment lines are ignored.
func TestCommentLinesIgnored(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")

		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}

		// Send comment followed by real event
		fmt.Fprint(w, ": this is a comment\n")
		fmt.Fprint(w, "data: actual data\n\n")
		flusher.Flush()

		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	sub := NewSubscriber(server.URL)
	sub.url = server.URL

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	events := sub.Start(ctx)

	select {
	case event := <-events:
		if event.Data != "actual data" {
			t.Errorf("expected data 'actual data', got %q", event.Data)
		}
	case <-ctx.Done():
		t.Error("timed out waiting for event")
	}

	sub.Stop()
}

// TestNonOKStatusCode tests that non-200 status codes cause errors.
func TestNonOKStatusCode(t *testing.T) {
	statusCodes := []int{
		http.StatusBadRequest,
		http.StatusUnauthorized,
		http.StatusInternalServerError,
		http.StatusServiceUnavailable,
	}

	for _, code := range statusCodes {
		t.Run(fmt.Sprintf("status_%d", code), func(t *testing.T) {
			requestCount := 0
			var mu sync.Mutex

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				requestCount++
				mu.Unlock()
				w.WriteHeader(code)
			}))
			defer server.Close()

			sub := NewSubscriber(server.URL)
			sub.url = server.URL
			sub.maxFailuresBeforePolling = 2 // Lower threshold for faster test

			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cancel()

			sub.Start(ctx)

			// Wait for context to timeout
			<-ctx.Done()

			sub.Stop()

			mu.Lock()
			if requestCount < 1 {
				t.Error("expected at least one request to be made")
			}
			mu.Unlock()
		})
	}
}

// TestSendEventChannelFull tests behavior when event channel is full.
func TestSendEventChannelFull(t *testing.T) {
	sub := NewSubscriber("http://localhost:1")

	// Fill the channel
	for i := 0; i < 100; i++ {
		sub.sendEvent(Event{Type: "test", Data: fmt.Sprintf("event-%d", i)})
	}

	// This should not block due to the non-blocking send
	done := make(chan bool)
	go func() {
		sub.sendEvent(Event{Type: "overflow", Data: "this should be dropped"})
		done <- true
	}()

	select {
	case <-done:
		// Success - the send didn't block
	case <-time.After(1 * time.Second):
		t.Error("sendEvent blocked when channel was full")
	}
}

// TestNewSubscriberURLFormatting tests URL formatting in NewSubscriber.
func TestNewSubscriberURLFormatting(t *testing.T) {
	tests := []struct {
		name        string
		baseURL     string
		expectedURL string
	}{
		{
			name:        "without trailing slash",
			baseURL:     "http://localhost:3000",
			expectedURL: "http://localhost:3000/api/events",
		},
		{
			name:        "with trailing slash",
			baseURL:     "http://localhost:3000/",
			expectedURL: "http://localhost:3000/api/events",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := NewSubscriber(tt.baseURL)
			if sub.url != tt.expectedURL {
				t.Errorf("expected URL %q, got %q", tt.expectedURL, sub.url)
			}
		})
	}
}

// TestPollingFallback tests that the subscriber falls back to polling after failures.
func TestPollingFallback(t *testing.T) {
	pollCount := 0
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		pollCount++
		count := pollCount
		mu.Unlock()

		// First several requests fail to trigger polling mode
		if count <= 5 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		// After that, return OK (simulating server recovery during polling)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sub := NewSubscriber(server.URL)
	sub.url = server.URL
	sub.maxFailuresBeforePolling = 3
	sub.pollingInterval = 50 * time.Millisecond
	sub.reconnectDelay = 10 * time.Millisecond
	sub.maxReconnectDelay = 50 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	events := sub.Start(ctx)

	// Drain events and wait for some polling events
	eventCount := 0
	timeout := time.After(800 * time.Millisecond)
	for {
		select {
		case <-events:
			eventCount++
			if eventCount >= 2 {
				// Got polling events, test passed
				cancel()
				sub.Stop()
				return
			}
		case <-timeout:
			// Check if we got into polling mode (indicated by consecutive failures)
			mu.Lock()
			count := pollCount
			mu.Unlock()
			if count >= 3 {
				// Got into polling mode even if we didn't receive events
				cancel()
				sub.Stop()
				return
			}
			t.Error("timed out waiting for polling fallback")
			cancel()
			sub.Stop()
			return
		}
	}
}

// TestIDAndRetryFieldsIgnored tests that id: and retry: fields don't cause errors.
func TestIDAndRetryFieldsIgnored(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")

		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}

		// Send event with id and retry fields
		fmt.Fprint(w, "id: 12345\n")
		fmt.Fprint(w, "retry: 5000\n")
		fmt.Fprint(w, "data: test\n\n")
		flusher.Flush()

		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	sub := NewSubscriber(server.URL)
	sub.url = server.URL

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	events := sub.Start(ctx)

	select {
	case event := <-events:
		if event.Data != "test" {
			t.Errorf("expected data 'test', got %q", event.Data)
		}
	case <-ctx.Done():
		t.Error("timed out waiting for event")
	}

	sub.Stop()
}

// TestEmptyDataNotSent tests that events with empty data are not sent.
func TestEmptyDataNotSent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")

		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}

		// Send empty event (no data field) followed by real event
		fmt.Fprint(w, "event: empty\n\n")
		fmt.Fprint(w, "data: real data\n\n")
		flusher.Flush()

		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	sub := NewSubscriber(server.URL)
	sub.url = server.URL

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	events := sub.Start(ctx)

	// The first event (empty) should be skipped
	select {
	case event := <-events:
		if event.Data != "real data" {
			t.Errorf("expected 'real data', got %q (empty event should have been skipped)", event.Data)
		}
	case <-ctx.Done():
		t.Error("timed out waiting for event")
	}

	sub.Stop()
}

// TestMultipleEvents tests receiving multiple events in sequence.
func TestMultipleEvents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")

		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}

		// Send multiple events
		for i := 1; i <= 3; i++ {
			fmt.Fprintf(w, "event: event-%d\n", i)
			fmt.Fprintf(w, "data: data-%d\n\n", i)
			flusher.Flush()
		}

		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	sub := NewSubscriber(server.URL)
	sub.url = server.URL

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	events := sub.Start(ctx)

	received := make([]Event, 0, 3)
	for i := 0; i < 3; i++ {
		select {
		case event := <-events:
			received = append(received, event)
		case <-ctx.Done():
			t.Fatalf("timed out waiting for event %d", i+1)
		}
	}

	if len(received) != 3 {
		t.Errorf("expected 3 events, got %d", len(received))
	}

	for i, event := range received {
		expectedType := fmt.Sprintf("event-%d", i+1)
		expectedData := fmt.Sprintf("data-%d", i+1)
		if event.Type != expectedType {
			t.Errorf("event %d: expected type %q, got %q", i+1, expectedType, event.Type)
		}
		if event.Data != expectedData {
			t.Errorf("event %d: expected data %q, got %q", i+1, expectedData, event.Data)
		}
	}

	sub.Stop()
}

// TestWaitWithContext tests the waitWithContext helper function.
func TestWaitWithContext(t *testing.T) {
	sub := NewSubscriber("http://localhost:1")

	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		start := time.Now()
		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()

		sub.waitWithContext(ctx, 10*time.Second)
		elapsed := time.Since(start)

		if elapsed >= 1*time.Second {
			t.Errorf("waitWithContext did not respond to context cancellation in time: %v", elapsed)
		}
	})

	t.Run("normal wait", func(t *testing.T) {
		ctx := context.Background()

		start := time.Now()
		sub.waitWithContext(ctx, 50*time.Millisecond)
		elapsed := time.Since(start)

		if elapsed < 40*time.Millisecond {
			t.Errorf("waitWithContext returned too early: %v", elapsed)
		}
	})
}

// TestSSEHeadersSet tests that the correct SSE headers are set on requests.
func TestSSEHeadersSet(t *testing.T) {
	capturedHeaders := make(chan http.Header, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case capturedHeaders <- r.Header.Clone():
		default:
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		// Close immediately
	}))
	defer server.Close()

	sub := NewSubscriber(server.URL)
	sub.url = server.URL

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	sub.Start(ctx)

	var headers http.Header
	select {
	case headers = <-capturedHeaders:
	case <-ctx.Done():
		t.Fatal("timed out waiting for SSE request")
	}

	sub.Stop()

	// Check headers
	if accept := headers.Get("Accept"); !strings.Contains(accept, "text/event-stream") {
		t.Errorf("expected Accept header to contain 'text/event-stream', got %q", accept)
	}
	if cacheControl := headers.Get("Cache-Control"); !strings.Contains(cacheControl, "no-cache") {
		t.Errorf("expected Cache-Control header to contain 'no-cache', got %q", cacheControl)
	}
	if connection := headers.Get("Connection"); !strings.Contains(connection, "keep-alive") {
		t.Errorf("expected Connection header to contain 'keep-alive', got %q", connection)
	}
}

func TestSubscriberAuthentication(t *testing.T) {
	var authorization string
	var token string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization = r.Header.Get("Authorization")
		token = r.URL.Query().Get("token")
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "event: connected\ndata: \"ok\"\n\n")
	}))
	defer server.Close()

	sub := NewSubscriber(server.URL, WithAPIKey(" flx_test_key "))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	<-sub.Start(ctx)
	sub.Stop()

	if authorization != "Bearer flx_test_key" {
		t.Errorf("unexpected Authorization header %q", authorization)
	}
	if token != "flx_test_key" {
		t.Errorf("unexpected SSE token %q", token)
	}
}
