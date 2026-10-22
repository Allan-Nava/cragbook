package journal_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Allan-Nava/cragbook/internal/application/journal"
	"github.com/Allan-Nava/cragbook/internal/domain/climbing"
	"github.com/Allan-Nava/cragbook/internal/domain/diary"
	"github.com/Allan-Nava/cragbook/internal/infrastructure/memory"
)

// Made-up crags and routes, not real places.
var now = time.Date(2026, 10, 23, 15, 0, 0, 0, time.UTC)

func p[T any](v T) *T { return &v }

func setup(t *testing.T) *journal.Service {
	t.Helper()
	m := memory.New()
	ctx := context.Background()
	m.SaveCrag(ctx, climbing.Crag{ID: "c1", Name: "Test Wall"}, []climbing.Route{{ID: "r1", Name: p("Diedro")}, {ID: "r2", Name: p("Placca")}}, now)
	m.SaveCrag(ctx, climbing.Crag{ID: "c2", Name: "Other Wall"}, []climbing.Route{{ID: "r3", Name: p("Tetto")}}, now)
	return journal.NewService(m, m, func() time.Time { return now })
}

func day(s string) *time.Time { d, _ := time.Parse(time.DateOnly, s); return &d }

func TestRecordReadAndClear(t *testing.T) {
	s, ctx := setup(t), context.Background()
	route, a, err := s.Record(ctx, journal.Record{Route: "r1", Status: "done", ClimbedOn: day("2026-10-20"), Notes: "a vista"})
	if err != nil || *route.Name != "Diedro" || a.Status() != diary.Done || !a.UpdatedAt().Equal(now) {
		t.Fatalf("%+v %+v %v", route, a, err)
	}
	s.Record(ctx, journal.Record{Route: "r3", Status: "project"})
	s.Record(ctx, journal.Record{Route: "r2", Status: "done", ClimbedOn: day("2026-10-21")})
	entries, err := s.Entries(ctx)
	if err != nil || len(entries) != 3 {
		t.Fatalf("%d entries, %v", len(entries), err)
	}
	if entries[0].Route.ID != "r2" || entries[1].Route.ID != "r1" || entries[2].Crag.Name != "Other Wall" {
		t.Fatalf("order: %s, %s, %s (%s)", entries[0].Route.ID, entries[1].Route.ID, entries[2].Route.ID, entries[2].Crag.Name)
	}
	if err := s.Clear(ctx, "r1"); err != nil {
		t.Fatal(err)
	}
	if entries, _ := s.Entries(ctx); len(entries) != 2 {
		t.Fatalf("%d entries after clearing", len(entries))
	}
}

func TestWhatTheJournalRefuses(t *testing.T) {
	s, ctx := setup(t), context.Background()
	if _, _, err := s.Record(ctx, journal.Record{Route: "nowhere", Status: "done"}); !errors.Is(err, climbing.ErrRouteNotFound) {
		t.Errorf("unknown route: %v", err)
	}
	if _, _, err := s.Record(ctx, journal.Record{Route: "r1", Status: "maybe"}); !errors.Is(err, diary.ErrInvalid) {
		t.Errorf("unknown status: %v", err)
	}
	if _, _, err := s.Record(ctx, journal.Record{Route: "r1", Status: "done", ClimbedOn: day("2026-10-24")}); !errors.Is(err, diary.ErrInvalid) {
		t.Errorf("tomorrow: %v", err)
	}
	if err := s.Clear(ctx, "nowhere"); !errors.Is(err, climbing.ErrRouteNotFound) {
		t.Errorf("clear unknown: %v", err)
	}
}
