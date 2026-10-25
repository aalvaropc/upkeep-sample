package clock

import (
	"testing"
	"time"
)

func TestManual(t *testing.T) {
	start := time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC)
	m := NewManual(start)
	if !m.Now().Equal(start) {
		t.Fatalf("Now() = %v, want %v", m.Now(), start)
	}
	m.Advance(90 * time.Minute)
	if want := start.Add(90 * time.Minute); !m.Now().Equal(want) {
		t.Fatalf("Now() after Advance = %v, want %v", m.Now(), want)
	}
}

func TestSystemIsNow(t *testing.T) {
	before := time.Now()
	got := System{}.Now()
	if got.Before(before) || got.After(time.Now()) {
		t.Fatalf("System.Now() = %v is not the current time", got)
	}
}
