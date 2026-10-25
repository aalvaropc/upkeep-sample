package store

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"sync"

	"github.com/aalvaropc/upkeep-sample/notes"
)

type Memory struct {
	mu    sync.RWMutex
	notes map[string]map[string]notes.Note
}

func NewMemory() *Memory {
	return &Memory{notes: make(map[string]map[string]notes.Note)}
}

func (m *Memory) Insert(_ context.Context, n notes.Note) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	owned := m.notes[n.Owner]
	if owned == nil {
		owned = make(map[string]notes.Note)
		m.notes[n.Owner] = owned
	}
	owned[n.ID] = clone(n)
	return nil
}

func (m *Memory) Get(_ context.Context, owner, id string) (notes.Note, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	n, ok := m.notes[owner][id]
	if !ok {
		return notes.Note{}, notes.ErrNotFound
	}
	return clone(n), nil
}

func (m *Memory) List(_ context.Context, owner string, offset, limit int) ([]notes.Note, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	all := make([]notes.Note, 0, len(m.notes[owner]))
	for _, n := range m.notes[owner] {
		all = append(all, clone(n))
	}
	slices.SortFunc(all, func(a, b notes.Note) int {
		return cmp.Or(a.CreatedAt.Compare(b.CreatedAt), strings.Compare(a.ID, b.ID))
	})
	total := len(all)
	if offset >= total {
		return nil, total, nil
	}
	end := min(offset+limit, total)
	return all[offset:end], total, nil
}

func (m *Memory) Update(_ context.Context, n notes.Note) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.notes[n.Owner][n.ID]; !ok {
		return notes.ErrNotFound
	}
	m.notes[n.Owner][n.ID] = clone(n)
	return nil
}

func (m *Memory) Delete(_ context.Context, owner, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.notes[owner][id]; !ok {
		return notes.ErrNotFound
	}
	delete(m.notes[owner], id)
	return nil
}

func clone(n notes.Note) notes.Note {
	n.Tags = slices.Clone(n.Tags)
	return n
}
