// SPDX-License-Identifier: GPL-3.0-only

package gateway

import (
	"testing"
	"time"
)

func TestRequestLogEvictsOldestWhenFull(t *testing.T) {
	log := NewRequestLog(3)
	for i := 0; i < 5; i++ {
		log.Add(RequestEntry{Alias: string(rune('a' + i)), Time: time.Now()})
	}

	entries := log.Recent(0)
	if len(entries) != 3 {
		t.Fatalf("got %d entries, want 3 (capacity)", len(entries))
	}
	// Oldest two (a, b) should have been evicted; c, d, e remain in order.
	want := []string{"c", "d", "e"}
	for i, e := range entries {
		if e.Alias != want[i] {
			t.Fatalf("entries[%d].Alias = %q, want %q", i, e.Alias, want[i])
		}
	}
}

func TestRequestLogRecentNReturnsLastN(t *testing.T) {
	log := NewRequestLog(0)
	for i := 0; i < 5; i++ {
		log.Add(RequestEntry{Alias: string(rune('a' + i))})
	}

	entries := log.Recent(2)
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if entries[0].Alias != "d" || entries[1].Alias != "e" {
		t.Fatalf("got %+v, want last two entries (d, e)", entries)
	}
}

func TestRequestLogRecentOnEmpty(t *testing.T) {
	log := NewRequestLog(10)
	if entries := log.Recent(5); len(entries) != 0 {
		t.Fatalf("got %d entries, want 0", len(entries))
	}
}
