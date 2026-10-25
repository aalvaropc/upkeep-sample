package notes_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aalvaropc/upkeep-sample/clock"
	"github.com/aalvaropc/upkeep-sample/notes"
	"github.com/aalvaropc/upkeep-sample/store"
)

func newService(t *testing.T) (*notes.Service, *clock.Manual) {
	t.Helper()
	c := clock.NewManual(time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC))
	return notes.NewService(store.NewMemory(), c), c
}

func TestCreateAndGet(t *testing.T) {
	svc, _ := newService(t)
	ctx := t.Context()
	n, err := svc.Create(ctx, "alice", notes.Input{Title: "  Groceries ", Body: "milk", Tags: []string{"Home", "home", "errands"}})
	if err != nil {
		t.Fatal(err)
	}
	if n.ID == "" || n.Owner != "alice" || n.Title != "Groceries" {
		t.Fatalf("created = %+v", n)
	}
	if strings.Join(n.Tags, ",") != "home,errands" {
		t.Fatalf("tags = %v, want lowercase and deduplicated", n.Tags)
	}
	got, err := svc.Get(ctx, "alice", n.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != n.Title || !got.CreatedAt.Equal(n.CreatedAt) {
		t.Fatalf("got = %+v", got)
	}
}

func TestOwnersAreIsolated(t *testing.T) {
	svc, _ := newService(t)
	ctx := t.Context()
	n, err := svc.Create(ctx, "alice", notes.Input{Title: "private"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Get(ctx, "bob", n.ID); !errors.Is(err, notes.ErrNotFound) {
		t.Fatalf("bob reading alice's note: error = %v, want ErrNotFound", err)
	}
	if err := svc.Delete(ctx, "bob", n.ID); !errors.Is(err, notes.ErrNotFound) {
		t.Fatalf("bob deleting alice's note: error = %v, want ErrNotFound", err)
	}
	list, err := svc.List(ctx, "bob", notes.Page{})
	if err != nil {
		t.Fatal(err)
	}
	if list.Total != 0 || len(list.Notes) != 0 {
		t.Fatalf("bob's list = %+v", list)
	}
}

func TestUpdateSetsUpdatedAt(t *testing.T) {
	svc, c := newService(t)
	ctx := t.Context()
	n, err := svc.Create(ctx, "alice", notes.Input{Title: "draft"})
	if err != nil {
		t.Fatal(err)
	}
	c.Advance(time.Minute)
	updated, err := svc.Update(ctx, "alice", n.ID, notes.Input{Title: "final", Body: "done"})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Title != "final" || !updated.CreatedAt.Equal(n.CreatedAt) || !updated.UpdatedAt.Equal(n.CreatedAt.Add(time.Minute)) {
		t.Fatalf("updated = %+v", updated)
	}
	if _, err := svc.Update(ctx, "alice", "missing", notes.Input{Title: "x"}); !errors.Is(err, notes.ErrNotFound) {
		t.Fatalf("update of a missing note: error = %v", err)
	}
}

func TestValidation(t *testing.T) {
	svc, _ := newService(t)
	tooManyTags := make([]string, notes.MaxTags+1)
	for i := range tooManyTags {
		tooManyTags[i] = "t" + strings.Repeat("x", i)
	}
	for name, in := range map[string]notes.Input{
		"empty title":    {Title: "   "},
		"long title":     {Title: strings.Repeat("a", notes.MaxTitleLength+1)},
		"long body":      {Title: "x", Body: strings.Repeat("b", notes.MaxBodyLength+1)},
		"too many tags":  {Title: "x", Tags: tooManyTags},
		"tag with space": {Title: "x", Tags: []string{"two words"}},
		"empty tag":      {Title: "x", Tags: []string{""}},
		"long tag":       {Title: "x", Tags: []string{strings.Repeat("t", notes.MaxTagLength+1)}},
	} {
		if _, err := svc.Create(t.Context(), "alice", in); !errors.Is(err, notes.ErrInvalid) {
			t.Errorf("%s: error = %v, want ErrInvalid", name, err)
		}
	}
	if _, err := svc.Create(t.Context(), "alice", notes.Input{Title: strings.Repeat("é", notes.MaxTitleLength)}); err != nil {
		t.Errorf("a title of exactly %d characters was rejected: %v", notes.MaxTitleLength, err)
	}
}

func TestListPagination(t *testing.T) {
	svc, c := newService(t)
	ctx := t.Context()
	for i := range 25 {
		if _, err := svc.Create(ctx, "alice", notes.Input{Title: "note " + string(rune('a'+i))}); err != nil {
			t.Fatal(err)
		}
		c.Advance(time.Second)
	}
	first, err := svc.List(ctx, "alice", notes.Page{})
	if err != nil {
		t.Fatal(err)
	}
	if first.Total != 25 || len(first.Notes) != notes.DefaultPageSize || first.Limit != notes.DefaultPageSize {
		t.Fatalf("first page: total %d, %d notes, limit %d", first.Total, len(first.Notes), first.Limit)
	}
	if first.Notes[0].Title != "note a" {
		t.Fatalf("first note = %q, want the oldest", first.Notes[0].Title)
	}
	rest, err := svc.List(ctx, "alice", notes.Page{Offset: 20, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(rest.Notes) != 5 || rest.Notes[0].Title != "note u" {
		t.Fatalf("second page = %d notes starting at %q", len(rest.Notes), rest.Notes[0].Title)
	}
	clamped, err := svc.List(ctx, "alice", notes.Page{Offset: -3, Limit: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if clamped.Offset != 0 || clamped.Limit != notes.MaxPageSize {
		t.Fatalf("clamped page = offset %d limit %d", clamped.Offset, clamped.Limit)
	}
	past, err := svc.List(ctx, "alice", notes.Page{Offset: 100})
	if err != nil {
		t.Fatal(err)
	}
	if past.Notes == nil || len(past.Notes) != 0 || past.Total != 25 {
		t.Fatalf("page past the end = %+v", past)
	}
}

func TestDelete(t *testing.T) {
	svc, _ := newService(t)
	ctx := t.Context()
	n, err := svc.Create(ctx, "alice", notes.Input{Title: "temp"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctx, "alice", n.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Get(ctx, "alice", n.ID); !errors.Is(err, notes.ErrNotFound) {
		t.Fatalf("get after delete: error = %v", err)
	}
}

func TestNewID(t *testing.T) {
	seen := map[string]bool{}
	for range 100 {
		id := notes.NewID()
		if len(id) != 16 || seen[id] {
			t.Fatalf("NewID() = %q", id)
		}
		seen[id] = true
	}
}
