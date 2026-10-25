package notes

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound = errors.New("note not found")
	ErrInvalid  = errors.New("invalid note")
)

type Note struct {
	ID        string    `json:"id"`
	Owner     string    `json:"owner"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Tags      []string  `json:"tags"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Input struct {
	Title string   `json:"title"`
	Body  string   `json:"body"`
	Tags  []string `json:"tags"`
}

type Page struct {
	Offset int
	Limit  int
}

type List struct {
	Notes  []Note `json:"notes"`
	Total  int    `json:"total"`
	Offset int    `json:"offset"`
	Limit  int    `json:"limit"`
}

type Store interface {
	Insert(ctx context.Context, n Note) error
	Get(ctx context.Context, owner, id string) (Note, error)
	List(ctx context.Context, owner string, offset, limit int) ([]Note, int, error)
	Update(ctx context.Context, n Note) error
	Delete(ctx context.Context, owner, id string) error
}
