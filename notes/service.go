package notes

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/aalvaropc/upkeep-sample/clock"
)

type Service struct {
	store Store
	clock clock.Clock
	newID func() string
}

func NewService(store Store, c clock.Clock) *Service {
	return &Service{store: store, clock: c, newID: NewID}
}

func NewID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func (s *Service) Create(ctx context.Context, owner string, in Input) (Note, error) {
	in, err := validate(in)
	if err != nil {
		return Note{}, err
	}
	now := s.clock.Now().UTC()
	n := Note{
		ID:        s.newID(),
		Owner:     owner,
		Title:     in.Title,
		Body:      in.Body,
		Tags:      in.Tags,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.store.Insert(ctx, n); err != nil {
		return Note{}, err
	}
	return n, nil
}

func (s *Service) Get(ctx context.Context, owner, id string) (Note, error) {
	return s.store.Get(ctx, owner, id)
}

func (s *Service) List(ctx context.Context, owner string, page Page) (List, error) {
	page = clampPage(page)
	notes, total, err := s.store.List(ctx, owner, page.Offset, page.Limit)
	if err != nil {
		return List{}, err
	}
	if notes == nil {
		notes = []Note{}
	}
	return List{Notes: notes, Total: total, Offset: page.Offset, Limit: page.Limit}, nil
}

func (s *Service) Update(ctx context.Context, owner, id string, in Input) (Note, error) {
	in, err := validate(in)
	if err != nil {
		return Note{}, err
	}
	n, err := s.store.Get(ctx, owner, id)
	if err != nil {
		return Note{}, err
	}
	n.Title, n.Body, n.Tags = in.Title, in.Body, in.Tags
	n.UpdatedAt = s.clock.Now().UTC()
	if err := s.store.Update(ctx, n); err != nil {
		return Note{}, err
	}
	return n, nil
}

func (s *Service) Delete(ctx context.Context, owner, id string) error {
	return s.store.Delete(ctx, owner, id)
}

func clampPage(p Page) Page {
	if p.Offset < 0 {
		p.Offset = 0
	}
	switch {
	case p.Limit <= 0:
		p.Limit = DefaultPageSize
	case p.Limit > MaxPageSize:
		p.Limit = MaxPageSize
	}
	return p
}

func validate(in Input) (Input, error) {
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		return Input{}, fmt.Errorf("%w: title is required", ErrInvalid)
	}
	if utf8.RuneCountInString(in.Title) > MaxTitleLength {
		return Input{}, fmt.Errorf("%w: title is longer than %d characters", ErrInvalid, MaxTitleLength)
	}
	if utf8.RuneCountInString(in.Body) > MaxBodyLength {
		return Input{}, fmt.Errorf("%w: body is longer than %d characters", ErrInvalid, MaxBodyLength)
	}
	if len(in.Tags) > MaxTags {
		return Input{}, fmt.Errorf("%w: at most %d tags", ErrInvalid, MaxTags)
	}
	tags := make([]string, 0, len(in.Tags))
	seen := make(map[string]bool, len(in.Tags))
	for _, tag := range in.Tags {
		tag = strings.ToLower(strings.TrimSpace(tag))
		if !validTag(tag) {
			return Input{}, fmt.Errorf("%w: tag %q must be 1 to %d lowercase letters, digits or dashes", ErrInvalid, tag, MaxTagLength)
		}
		if !seen[tag] {
			seen[tag] = true
			tags = append(tags, tag)
		}
	}
	in.Tags = tags
	return in, nil
}

func validTag(tag string) bool {
	if tag == "" || len(tag) > MaxTagLength {
		return false
	}
	for _, r := range tag {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
		default:
			return false
		}
	}
	return true
}
