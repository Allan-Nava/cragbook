// Package diary is the user's own record of routes: climbed, a project, or one to try, with
// the day and a note. It refers to the catalogue's routes by id and never changes them.
package diary

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Allan-Nava/cragbook/internal/domain/climbing"
)

type Status string

const (
	Done    Status = "done"
	Project Status = "project"
	Todo    Status = "todo"
)

// ErrInvalid is wrapped by every reason an ascent is refused.
var ErrInvalid = errors.New("invalid ascent")

func ParseStatus(s string) (Status, error) {
	switch st := Status(s); st {
	case Done, Project, Todo:
		return st, nil
	}
	return "", fmt.Errorf(`%w: status %q is not one of "done", "project", "todo"`, ErrInvalid, s)
}

// MaxNotes is the longest note, in characters.
const MaxNotes = 2000

// Ascent is the diary's entry for one route. Build it with NewAscent, which keeps the rules:
// a day only for a route climbed, never after today, and a note of at most MaxNotes.
type Ascent struct {
	route     climbing.RouteID
	status    Status
	climbedOn *time.Time
	notes     string
	updatedAt time.Time
}

// NewAscent makes an entry stamped now. climbedOn is a day (its time of day is dropped);
// "today" is now's date in now's location.
func NewAscent(route climbing.RouteID, status Status, climbedOn *time.Time, notes string, now time.Time) (Ascent, error) {
	if route == "" {
		return Ascent{}, fmt.Errorf("%w: no route", ErrInvalid)
	}
	if _, err := ParseStatus(string(status)); err != nil {
		return Ascent{}, err
	}
	notes = strings.TrimSpace(notes)
	if utf8.RuneCountInString(notes) > MaxNotes {
		return Ascent{}, fmt.Errorf("%w: notes of more than %d characters", ErrInvalid, MaxNotes)
	}
	var day *time.Time
	if climbedOn != nil {
		if status != Done {
			return Ascent{}, fmt.Errorf("%w: a day only for a route climbed", ErrInvalid)
		}
		d := time.Date(climbedOn.Year(), climbedOn.Month(), climbedOn.Day(), 0, 0, 0, 0, time.UTC)
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		if d.After(today) {
			return Ascent{}, fmt.Errorf("%w: climbed on %s, after today", ErrInvalid, d.Format(time.DateOnly))
		}
		day = &d
	}
	return Ascent{route: route, status: status, climbedOn: day, notes: notes, updatedAt: now}, nil
}

// Restore rebuilds an entry read back from storage, which only ever held valid ones.
func Restore(route climbing.RouteID, status Status, climbedOn *time.Time, notes string, updatedAt time.Time) Ascent {
	return Ascent{route: route, status: status, climbedOn: climbedOn, notes: notes, updatedAt: updatedAt}
}

func (a Ascent) Route() climbing.RouteID { return a.route }
func (a Ascent) Status() Status          { return a.status }
func (a Ascent) ClimbedOn() *time.Time   { return a.climbedOn }
func (a Ascent) Notes() string           { return a.notes }
func (a Ascent) UpdatedAt() time.Time    { return a.updatedAt }

// Compare orders the diary: the climbed first, the latest day first (undated after), then the
// projects, then the routes to try, each by when it was last changed.
func Compare(a, b Ascent) int {
	rank := map[Status]int{Done: 0, Project: 1, Todo: 2}
	if c := cmp.Compare(rank[a.status], rank[b.status]); c != 0 {
		return c
	}
	switch {
	case a.climbedOn != nil && b.climbedOn == nil:
		return -1
	case a.climbedOn == nil && b.climbedOn != nil:
		return 1
	case a.climbedOn != nil && !a.climbedOn.Equal(*b.climbedOn):
		return b.climbedOn.Compare(*a.climbedOn)
	}
	if c := b.updatedAt.Compare(a.updatedAt); c != 0 {
		return c
	}
	return cmp.Compare(a.route, b.route)
}

// Repository keeps the diary. Delete of an entry that is not there is no error.
type Repository interface {
	Save(ctx context.Context, a Ascent) error
	Delete(ctx context.Context, route climbing.RouteID) error
	ForRoutes(ctx context.Context, routes []climbing.RouteID) (map[climbing.RouteID]Ascent, error)
	All(ctx context.Context) ([]Ascent, error)
}
