package applog

import (
	"strings"
	"sync"
	"testing"
)

// reset clears the package-level state so tests don't leak into each other.
func reset(t *testing.T) {
	t.Helper()
	mu.Lock()
	entries = nil
	mu.Unlock()
}

func TestPrintfAndDrain(t *testing.T) {
	reset(t)

	Printf("hello %s", "world")
	Printf("n=%d", 42)

	got := Drain()
	if len(got) != 2 {
		t.Fatalf("Drain() returned %d entries, want 2", len(got))
	}
	if got[0].Text != "hello world" {
		t.Errorf("entries[0].Text = %q, want %q", got[0].Text, "hello world")
	}
	if got[1].Text != "n=42" {
		t.Errorf("entries[1].Text = %q, want %q", got[1].Text, "n=42")
	}
	if got[0].At.IsZero() {
		t.Error("At should be set to the log time")
	}
}

func TestDrainClearsBuffer(t *testing.T) {
	reset(t)

	Printf("first")
	_ = Drain()

	got := Drain()
	if got != nil {
		t.Errorf("second Drain() = %v, want nil", got)
	}
}

func TestDrainEmpty(t *testing.T) {
	reset(t)

	got := Drain()
	if got != nil {
		t.Errorf("Drain() on empty buffer = %v, want nil", got)
	}
}

func TestPrintfRingBufferCap(t *testing.T) {
	reset(t)

	// maxEntries = 4; write 6 entries and assert we keep the last 4.
	for i := 0; i < 6; i++ {
		Printf("msg-%d", i)
	}
	got := Drain()
	if len(got) != maxEntries {
		t.Fatalf("len(entries) = %d, want %d", len(got), maxEntries)
	}
	// Oldest two messages (msg-0, msg-1) should be dropped.
	for i, e := range got {
		want := "msg-" + string(rune('2'+i))
		if e.Text != want {
			t.Errorf("entries[%d].Text = %q, want %q", i, e.Text, want)
		}
	}
}

func TestPrintfConcurrent(t *testing.T) {
	reset(t)

	const goroutines = 20
	const perGoroutine = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				Printf("g-log")
			}
		}()
	}
	wg.Wait()

	got := Drain()
	// Ring buffer caps at maxEntries regardless of concurrent writers.
	if len(got) > maxEntries {
		t.Errorf("len(entries) = %d, exceeds cap %d", len(got), maxEntries)
	}
	for _, e := range got {
		if !strings.HasPrefix(e.Text, "g-log") {
			t.Errorf("unexpected entry text %q", e.Text)
		}
	}
}
