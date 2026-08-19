// Package sse provides Server-Sent Events (SSE) subscription functionality
// for the Flux API. It handles automatic reconnection with exponential backoff
// and falls back to polling if SSE connections fail repeatedly.
package sse

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Event represents a Server-Sent Event received from the Flux API.
type Event struct {
	// Type is the event type (e.g., "data-changed", "message")
	Type string
	// Data is the event payload
	Data string
}

// Subscriber manages an SSE connection to the Flux API.
// It automatically handles reconnection with exponential backoff
// and can fall back to polling if SSE fails repeatedly.
type Subscriber struct {
	// url is the SSE endpoint URL
	url string
	// reconnectDelay is the current delay before attempting reconnection
	reconnectDelay time.Duration
	// maxReconnectDelay is the maximum delay between reconnection attempts
	maxReconnectDelay time.Duration
	// events is the channel where received events are sent
	events chan Event
	// done is used to signal graceful shutdown
	done chan struct{}
	// mu protects the running state
	mu sync.Mutex
	// running indicates whether the subscriber is active
	running bool
	// started prevents reusing the one-shot event channel after shutdown.
	started bool
	// cancel stops an active HTTP request when Stop is called.
	cancel context.CancelFunc
	// consecutiveFailures tracks SSE connection failures for fallback logic
	consecutiveFailures int
	// maxFailuresBeforePolling is the threshold before falling back to polling
	maxFailuresBeforePolling int
	// pollingInterval is the interval for polling fallback
	pollingInterval time.Duration
	// client is the HTTP client used for connections
	client *http.Client
	// apiKey authenticates private-project event streams.
	apiKey string
}

// Option configures a Subscriber.
type Option func(*Subscriber)

// WithAPIKey configures Flux authentication. Flux accepts the token as a
// query parameter for SSE clients and as a Bearer token.
func WithAPIKey(apiKey string) Option {
	return func(s *Subscriber) {
		s.apiKey = strings.TrimSpace(apiKey)
	}
}

// WithPollingInterval configures the refresh interval used after SSE errors.
func WithPollingInterval(interval time.Duration) Option {
	return func(s *Subscriber) {
		if interval > 0 {
			s.pollingInterval = interval
		}
	}
}

// NewSubscriber creates a new SSE Subscriber for the Flux API.
// The baseURL should be the root URL of the Flux server (e.g., "http://localhost:3000").
func NewSubscriber(baseURL string, options ...Option) *Subscriber {
	// Ensure baseURL doesn't have a trailing slash
	baseURL = strings.TrimSuffix(baseURL, "/")

	s := &Subscriber{
		url:                      fmt.Sprintf("%s/api/events", baseURL),
		reconnectDelay:           1 * time.Second,
		maxReconnectDelay:        30 * time.Second,
		events:                   make(chan Event, 100),
		done:                     make(chan struct{}),
		maxFailuresBeforePolling: 5,
		pollingInterval:          5 * time.Second,
		client: &http.Client{
			Timeout: 0, // No timeout for SSE connections
		},
	}
	for _, option := range options {
		option(s)
	}
	if s.apiKey != "" {
		if endpoint, err := url.Parse(s.url); err == nil {
			query := endpoint.Query()
			query.Set("token", s.apiKey)
			endpoint.RawQuery = query.Encode()
			s.url = endpoint.String()
		}
	}
	return s
}

// Start begins the SSE subscription and returns a channel for receiving events.
// The subscription will automatically reconnect on connection loss.
// Use the provided context or call Stop() to terminate the subscription.
func (s *Subscriber) Start(ctx context.Context) <-chan Event {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return s.events
	}
	runCtx, cancel := context.WithCancel(ctx)
	s.started = true
	s.running = true
	s.cancel = cancel
	s.mu.Unlock()

	go s.run(runCtx)

	return s.events
}

// Stop gracefully stops the subscriber and closes the event channel.
func (s *Subscriber) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}

	s.running = false
	cancel := s.cancel
	s.cancel = nil
	close(s.done)
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// run is the main loop that manages the SSE connection or polling fallback.
func (s *Subscriber) run(ctx context.Context) {
	defer func() {
		s.mu.Lock()
		s.running = false
		s.cancel = nil
		s.mu.Unlock()
		close(s.events)
	}()

	for {
		select {
		case <-ctx.Done():
			log.Println("SSE subscriber: context cancelled, shutting down")
			return
		case <-s.done:
			log.Println("SSE subscriber: stop requested, shutting down")
			return
		default:
			// Check if we should fall back to polling
			if s.consecutiveFailures >= s.maxFailuresBeforePolling {
				s.pollOnce()
				s.waitWithContext(ctx, s.pollingInterval)
				// Periodically retry SSE instead of getting stuck in fallback mode.
				s.consecutiveFailures = s.maxFailuresBeforePolling - 1
				continue
			}

			// Attempt SSE connection
			err := s.connect(ctx)
			if err != nil {
				select {
				case <-ctx.Done():
					return
				case <-s.done:
					return
				default:
				}
				s.consecutiveFailures++
				log.Printf("SSE subscriber: connection error (attempt %d): %v", s.consecutiveFailures, err)

				if s.consecutiveFailures >= s.maxFailuresBeforePolling {
					log.Printf("SSE subscriber: falling back to polling (every %v)", s.pollingInterval)
				}

				s.handleReconnect(ctx)
			}
		}
	}
}

// connect establishes an SSE connection and processes incoming events.
// It returns when the connection is closed or an error occurs.
func (s *Subscriber) connect(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set SSE-specific headers
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")
	if s.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.apiKey)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		if urlErr, ok := err.(*url.Error); ok {
			return fmt.Errorf("failed to connect: %w", urlErr.Err)
		}
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	if contentType := resp.Header.Get("Content-Type"); !strings.HasPrefix(strings.ToLower(contentType), "text/event-stream") {
		return fmt.Errorf("unexpected content type %q", contentType)
	}

	// Reset the delay on a successful connection. The failure counter resets
	// after an event is actually received, avoiding tight loops on endpoints
	// that return 200 and immediately close.
	s.reconnectDelay = time.Second
	log.Printf("SSE subscriber: connected to %s", urlWithoutQuery(s.url))

	// Read and parse SSE events
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	var currentEvent Event

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-s.done:
			return nil
		default:
		}

		line := scanner.Text()

		// Empty line indicates end of event
		if line == "" {
			if currentEvent.Data != "" {
				// Default event type if none specified
				if currentEvent.Type == "" {
					currentEvent.Type = "message"
				}
				s.sendEvent(currentEvent)
				s.resetBackoff()
				currentEvent = Event{}
			}
			continue
		}

		// Parse SSE fields
		if strings.HasPrefix(line, "data:") {
			data := strings.TrimPrefix(line, "data:")
			data = strings.TrimSpace(data)
			if currentEvent.Data != "" {
				currentEvent.Data += "\n" + data
			} else {
				currentEvent.Data = data
			}
		} else if strings.HasPrefix(line, "event:") {
			currentEvent.Type = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %w", err)
	}

	return fmt.Errorf("connection closed by server")
}

func urlWithoutQuery(rawURL string) string {
	endpoint, err := url.Parse(rawURL)
	if err != nil {
		return "Flux SSE endpoint"
	}
	endpoint.RawQuery = ""
	endpoint.Fragment = ""
	return endpoint.String()
}

// handleReconnect implements exponential backoff for reconnection attempts.
func (s *Subscriber) handleReconnect(ctx context.Context) {
	s.waitWithContext(ctx, s.reconnectDelay)

	// Exponential backoff: double the delay up to max
	s.reconnectDelay *= 2
	if s.reconnectDelay > s.maxReconnectDelay {
		s.reconnectDelay = s.maxReconnectDelay
	}
}

// resetBackoff resets the reconnection delay and failure counter.
func (s *Subscriber) resetBackoff() {
	s.reconnectDelay = 1 * time.Second
	s.consecutiveFailures = 0
}

// waitWithContext waits for the specified duration or until context is cancelled.
func (s *Subscriber) waitWithContext(ctx context.Context, duration time.Duration) {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-s.done:
	case <-timer.C:
	}
}

// sendEvent sends an event to the events channel without blocking.
func (s *Subscriber) sendEvent(event Event) {
	select {
	case s.events <- event:
		// Successfully sent
	default:
		// Channel full, log warning but don't block
		log.Printf("SSE subscriber: event channel full, dropping event of type %q", event.Type)
	}
}

// pollOnce emits a refresh signal used by callers to perform their normal
// REST selection pass when SSE connections fail repeatedly.
func (s *Subscriber) pollOnce() {
	// Emit a synthetic data-changed event to trigger a refresh
	s.sendEvent(Event{
		Type: "data-changed",
		Data: `{"source":"polling","message":"periodic refresh"}`,
	})
}

// IsRunning returns whether the subscriber is currently active.
func (s *Subscriber) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// Events returns the event channel for receiving SSE events.
// This is useful if you need to access the channel after calling Start().
func (s *Subscriber) Events() <-chan Event {
	return s.events
}
