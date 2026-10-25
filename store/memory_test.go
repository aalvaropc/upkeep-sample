package store

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/aalvaropc/upkeep-sample/notes"
)

func TestReturnedNotesAreCopies(t *testing.T) {
	m := NewMemory()
	ctx := t.Context()
	n := notes.Note{ID: "1", Owner: "alice", Title: "t", Tags: []string{"a"}}
	if err := m.Insert(ctx, n); err != nil {
		t.Fatal(err)
	}
	n.Tags[0] = "changed"
	got, err := m.Get(ctx, "alice", "1")
	if err != nil {
		t.Fatal(err)
	}
	got.Tags[0] = "changed again"
	again, _ := m.Get(ctx, "alice", "1")
	if again.Tags[0] != "a" {
		t.Fatalf("stored tags = %v, want the store to keep its own copy", again.Tags)
	}
}

func TestUpdateAndDeleteMissing(t *testing.T) {
	m := NewMemory()
	ctx := t.Context()
	if err := m.Update(ctx, notes.Note{ID: "x", Owner: "alice"}); !errors.Is(err, notes.ErrNotFound) {
		t.Fatalf("Update missing: %v", err)
	}
	if err := m.Delete(ctx, "alice", "x"); !errors.Is(err, notes.ErrNotFound) {
		t.Fatalf("Delete missing: %v", err)
	}
}

func TestConcurrentUse(t *testing.T) {
	m := NewMemory()
	ctx := t.Context()
	start := time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC)
	var wg sync.WaitGroup
	for w := range 8 {
		wg.Go(func() {
			owner := fmt.Sprintf("user%d", w%2)
			for i := range 200 {
				id := fmt.Sprintf("%d-%d", w, i)
				n := notes.Note{ID: id, Owner: owner, Title: id, CreatedAt: start.Add(time.Duration(i) * time.Second)}
				if err := m.Insert(ctx, n); err != nil {
					t.Error(err)
					return
				}
				if _, _, err := m.List(ctx, owner, 0, 10); err != nil {
					t.Error(err)
					return
				}
				if i%3 == 0 {
					if err := m.Delete(ctx, owner, id); err != nil {
						t.Error(err)
						return
					}
				}
			}
		})
	}
	wg.Wait()
	_, total0, _ := m.List(ctx, "user0", 0, 1)
	_, total1, _ := m.List(ctx, "user1", 0, 1)
	if want := 4 * (200 - 67); total0 != want || total1 != want {
		t.Fatalf("totals = %d and %d, want %d each", total0, total1, want)
	}
}
