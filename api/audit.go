package main

import (
	"sync"
	"time"
)

// Entry is one audited action: who did what to which row, and when.
type Entry struct {
	Time    time.Time `json:"time"`
	Role    string    `json:"role"`
	Patient string    `json:"patient,omitempty"` // set when the role is Patient
	Action  string    `json:"action"`            // list, read, raw, delete, delete-refused, compact
	Row     string    `json:"row,omitempty"`
}

// Audit keeps the last 100 entries in memory.
// NOTE: in memory only, so a restart clears it. Write to a table or a log store to keep it.
type Audit struct {
	mu      sync.Mutex
	entries []Entry
}

const auditSize = 100

func (a *Audit) Add(e Entry) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.entries = append(a.entries, e)
	if len(a.entries) > auditSize {
		a.entries = a.entries[len(a.entries)-auditSize:]
	}
}

// Recent returns the entries, newest first.
func (a *Audit) Recent() []Entry {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]Entry, len(a.entries))
	for i, e := range a.entries {
		out[len(out)-1-i] = e
	}
	return out
}
