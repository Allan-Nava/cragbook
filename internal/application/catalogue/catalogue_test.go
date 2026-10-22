package catalogue_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Allan-Nava/cragbook/internal/application/catalogue"
	"github.com/Allan-Nava/cragbook/internal/domain/climbing"
	"github.com/Allan-Nava/cragbook/internal/domain/diary"
	"github.com/Allan-Nava/cragbook/internal/infrastructure/memory"
)

// Made-up crags and routes, not real places.
var t0 = time.Date(2026, 10, 23, 10, 0, 0, 0, time.UTC)

func p[T any](v T) *T { return &v }

func grade(s string) *climbing.Grade { g, _ := climbing.ParseGrade(s); return &g }

func setup(t *testing.T) (*catalogue.Service, *memory.Store) {
	t.Helper()
	m := memory.New()
	ctx := context.Background()
	m.SaveCrag(ctx, climbing.Crag{ID: "c1", Name: "Test Wall"}, []climbing.Route{
		{ID: "hard", Name: p("Tetto"), Grade: grade("7a")},
		{ID: "easy", Name: p("Placca"), Grade: grade("5a")},
		{ID: "none", Name: p("Senza grado")},
		{ID: "old", Name: p("Vecchia"), Grade: grade("6a")},
		{ID: "older", Name: p("Ancora più vecchia"), Grade: grade("6b")},
	}, t0)
	done, _ := diary.NewAscent("old", diary.Done, nil, "", t0)
	m.Save(ctx, done)
	hardDone, _ := diary.NewAscent("hard", diary.Done, nil, "", t0)
	m.Save(ctx, hardDone)
	// "old" and "older" leave OpenStreetMap; "old" has an ascent, "older" none.
	m.SaveCrag(ctx, climbing.Crag{ID: "c1", Name: "Test Wall"}, []climbing.Route{
		{ID: "hard", Name: p("Tetto"), Grade: grade("7a")}, {ID: "easy", Name: p("Placca"), Grade: grade("5a")},
		{ID: "none", Name: p("Senza grado")},
	}, t0.Add(time.Hour))
	return catalogue.NewService(m, m, m), m
}

func TestACragListsItsRoutesEasiestFirstAndKeepsGoneOnesWithAnAscent(t *testing.T) {
	s, _ := setup(t)
	d, err := s.Crag(context.Background(), "c1")
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, e := range d.Routes {
		ids = append(ids, string(e.Route.ID))
	}
	if strings.Join(ids, ",") != "easy,old,hard,none" {
		t.Fatalf("routes %v: want easy, old (gone, with its ascent), hard, none; older left out", ids)
	}
	if d.Routes[1].Ascent == nil || !d.Routes[1].Route.Gone || d.Routes[0].Ascent != nil {
		t.Fatalf("entries %+v", d.Routes)
	}
	if d.CragSummary.Routes != 3 || d.Done != 1 {
		t.Fatalf("counts %d routes, %d done: the gone route is in neither", d.CragSummary.Routes, d.Done)
	}
}

func TestAnUnknownCragIsNotFound(t *testing.T) {
	s, _ := setup(t)
	if _, err := s.Crag(context.Background(), "nowhere"); !errors.Is(err, climbing.ErrCragNotFound) {
		t.Fatalf("%v", err)
	}
}

func TestTheListAndItsQuery(t *testing.T) {
	s, _ := setup(t)
	list, err := s.Crags(context.Background(), catalogue.Filter{Query: "  test  "})
	if err != nil || len(list) != 1 || list[0].Routes != 3 || list[0].Done != 1 {
		t.Fatalf("%+v, %v", list, err)
	}
	if _, err := s.Crags(context.Background(), catalogue.Filter{Query: strings.Repeat("x", catalogue.MaxQuery+1)}); !errors.Is(err, catalogue.ErrInvalidQuery) {
		t.Fatalf("a long query: %v", err)
	}
}
