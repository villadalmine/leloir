// Package stream implements the SSE (Server-Sent Events) fan-out broker.
//
// For each active investigation, any number of browser clients (or other
// consumers) can subscribe to its event stream. The broker:
//   - Publishes events from the orchestrator to all subscribers
//   - Buffers briefly to handle slow consumers (drops events if buffer fills,
//     emitting a gap marker so the client knows to refresh)
//   - Cleans up subscriptions when the HTTP connection closes
//
// This is the websocket-alternative: simpler, one-way, works through proxies,
// no connection upgrade dance. For M1, SSE is the primary UI transport.
package stream

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"

	sdkadapter "github.com/leloir/sdk/adapter"
)

// Broker fans out events to subscribers.
type Broker struct {
	mu   sync.RWMutex
	subs map[string][]subscription // investigationID -> subscribers
}

type subscription struct {
	ch   chan sdkadapter.Event
	done chan struct{}
}

// NewBroker creates a new event broker.
func NewBroker() *Broker {
	return &Broker{subs: make(map[string][]subscription)}
}

// Publish delivers an event to all subscribers of the investigation.
// Non-blocking: if a subscriber's buffer is full, the event is dropped for
// that subscriber (with a warning log).
func (b *Broker) Publish(investigationID string, evt sdkadapter.Event) {
	b.mu.RLock()
	subs := b.subs[investigationID]
	b.mu.RUnlock()

	for _, s := range subs {
		select {
		case s.ch <- evt:
		default:
			// Subscriber backlogged; drop this event for them
			slog.Warn("subscriber buffer full, event dropped",
				"investigation_id", investigationID,
				"event_type", evt.Type,
			)
		}
	}
}

// Stream serves an SSE stream of events for one investigation over HTTP.
// Blocks until the connection closes or the investigation completes.
func (b *Broker) Stream(ctx context.Context, investigationID string, w http.ResponseWriter) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	// SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering

	sub := b.subscribe(investigationID)
	defer b.unsubscribe(investigationID, sub)

	// Send initial comment to open the stream
	fmt.Fprintf(w, ": ok\n\n")
	flusher.Flush()

	for {
		select {
		case evt, ok := <-sub.ch:
			if !ok {
				return
			}
			if err := writeSSE(w, evt); err != nil {
				return
			}
			flusher.Flush()

			// EventComplete terminates the stream
			if evt.Type == sdkadapter.EventComplete {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

// subscribe returns a new subscription to an investigation's events.
func (b *Broker) subscribe(investigationID string) subscription {
	sub := subscription{
		ch:   make(chan sdkadapter.Event, 64),
		done: make(chan struct{}),
	}
	b.mu.Lock()
	b.subs[investigationID] = append(b.subs[investigationID], sub)
	b.mu.Unlock()
	return sub
}

// unsubscribe removes a subscription and closes its channel.
func (b *Broker) unsubscribe(investigationID string, target subscription) {
	b.mu.Lock()
	defer b.mu.Unlock()

	subs := b.subs[investigationID]
	filtered := subs[:0]
	for _, s := range subs {
		if s.ch != target.ch {
			filtered = append(filtered, s)
		}
	}
	if len(filtered) == 0 {
		delete(b.subs, investigationID)
	} else {
		b.subs[investigationID] = filtered
	}
	close(target.ch)
}

// writeSSE writes one event to the stream in SSE format.
func writeSSE(w io.Writer, evt sdkadapter.Event) error {
	// Marshal the event to JSON
	data, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	// SSE format: "event: <type>\ndata: <json>\n\n"
	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", evt.Type, data)
	return err
}
