package clock

import (
	"time"

	"github.com/jonboulle/clockwork"
)

type Clock interface {
	Now() time.Time
}

type System struct{}

func (System) Now() time.Time { return time.Now() }

type Manual struct {
	fake *clockwork.FakeClock
}

func NewManual(start time.Time) *Manual {
	return &Manual{fake: clockwork.NewFakeClockAt(start)}
}

func (m *Manual) Now() time.Time { return m.fake.Now() }

func (m *Manual) Advance(d time.Duration) { m.fake.Advance(d) }
