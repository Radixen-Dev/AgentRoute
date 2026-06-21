// SPDX-License-Identifier: GPL-3.0-only

package gateway

import (
	"sync"
	"time"
)

// RequestEntry records one proxied request, for display on the TUI's
// Gateway / Live Log screen.
type RequestEntry struct {
	Time       time.Time
	Wire       Wire
	Alias      string // the model alias the client requested
	Model      string // the upstream OpenRouter model it was mapped to
	StatusCode int
	Duration   time.Duration
	Err        string // non-empty if the request failed before a status code was received
}

// RequestLog is a fixed-capacity, thread-safe ring buffer of recent
// requests. Capacity 0 means unbounded (not recommended outside tests).
type RequestLog struct {
	mu       sync.Mutex
	entries  []RequestEntry
	capacity int
}

// NewRequestLog returns a RequestLog that retains at most capacity entries,
// dropping the oldest when full.
func NewRequestLog(capacity int) *RequestLog {
	return &RequestLog{capacity: capacity}
}

// Add appends e, evicting the oldest entry if at capacity.
func (l *RequestLog) Add(e RequestEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.entries = append(l.entries, e)
	if l.capacity > 0 && len(l.entries) > l.capacity {
		l.entries = l.entries[len(l.entries)-l.capacity:]
	}
}

// Recent returns up to n of the most recent entries, oldest first. n <= 0
// returns all retained entries.
func (l *RequestLog) Recent(n int) []RequestEntry {
	l.mu.Lock()
	defer l.mu.Unlock()

	if n <= 0 || n >= len(l.entries) {
		out := make([]RequestEntry, len(l.entries))
		copy(out, l.entries)
		return out
	}
	out := make([]RequestEntry, n)
	copy(out, l.entries[len(l.entries)-n:])
	return out
}
