// Package contract is the behaviour every storage adapter owes the use cases, as tests: the
// memory adapter and the PostgreSQL one run the same cases, so either can stand in for the
// other. Made-up crags and routes, not real places.
package contract

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Allan-Nava/cragbook/internal/application/catalogue"
	"github.com/Allan-Nava/cragbook/internal/domain/climbing"
	"github.com/Allan-Nava/cragbook/internal/domain/diary"
)

// Store is every port a storage adapter implements.
type Store interface {
	climbing.Catalog
	climbing.CatalogWriter
	diary.Repository
	catalogue.Summaries
}

func ptr[T any](v T) *T { return &v }

var t0 = time.Date(2026, 10, 23, 10, 0, 0, 0, time.UTC)

func wall(id, name string) climbing.Crag {
	return climbing.Crag{ID: climbing.CragID(id), Name: name,
		Place: climbing.Place{Municipality: ptr("Lecco"), ProvinceCode: ptr("LC"), Latitude: 45.8, Longitude: 9.4, Aspect: ptr("S"), Elevation: ptr(600)}}
}

func route(id, name, g string) climbing.Route {
	r := climbing.Route{ID: climbing.RouteID(id), Name: ptr(name), Length: ptr(25.5), Bolts: ptr(11), Pitches: ptr(1), Sector: ptr("Settore Alto")}
	if gr, ok := climbing.ParseGrade(g); ok {
		r.Grade = &gr
	}
	return r
}

func ascent(t *testing.T, id string, st diary.Status, day string, at time.Time) diary.Ascent {
	t.Helper()
	var d *time.Time
	if day != "" {
		v, _ := time.Parse(time.DateOnly, day)
		d = &v
	}
	a, err := diary.NewAscent(climbing.RouteID(id), st, d, "note "+id, at)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// Run runs every case; fresh returns an empty store for each.
func Run(t *testing.T, fresh func(t *testing.T) Store) {
	ctx := context.Background()

	t.Run("an import keeps the diary and marks what left OSM", func(t *testing.T) {
		s := fresh(t)
		must(t, s.SaveCrag(ctx, wall("c1", "Test Wall"), []climbing.Route{route("r1", "Diedro", "6a+"), route("r2", "Placca", "5c")}, t0))
		must(t, s.Save(ctx, ascent(t, "r1", diary.Done, "2026-10-20", t0)))
		next := []climbing.Route{route("r1", "Diedro Giallo", "6a+"), route("r3", "Spigolo", "")}
		for range 2 { // an import is idempotent
			must(t, s.SaveCrag(ctx, wall("c1", "Test Wall"), next, t0.Add(time.Hour)))
		}
		r1, err := s.Route(ctx, "r1")
		must(t, err)
		if *r1.Name != "Diedro Giallo" || r1.Gone || r1.Grade.String() != "6a+" || *r1.Bolts != 11 || *r1.Sector != "Settore Alto" {
			t.Fatalf("r1 = %+v", r1)
		}
		if r3, _ := s.Route(ctx, "r3"); r3.Grade != nil {
			t.Fatalf("r3 has a grade: %+v", r3)
		}
		if r2, err := s.Route(ctx, "r2"); err != nil || !r2.Gone {
			t.Fatalf("r2 = %+v, %v; want kept and gone", r2, err)
		}
		got, _ := s.ForRoutes(ctx, []climbing.RouteID{"r1", "r2", "r3"})
		if len(got) != 1 || got["r1"].Notes() != "note r1" || got["r1"].ClimbedOn().Format(time.DateOnly) != "2026-10-20" {
			t.Fatalf("ascents %+v", got)
		}
		if routes, _ := s.Routes(ctx, "c1"); len(routes) != 3 {
			t.Fatalf("%d routes, want 3 with the gone one", len(routes))
		}
		must(t, s.SaveCrag(ctx, wall("c1", "Test Wall"), append(next, route("r2", "Placca", "5c")), t0.Add(2*time.Hour)))
		if r2, _ := s.Route(ctx, "r2"); r2.Gone {
			t.Fatal("r2 still gone after it came back")
		}
	})

	t.Run("summaries search, filter and count", func(t *testing.T) {
		s := fresh(t)
		must(t, s.SaveCrag(ctx, wall("c1", "Còrna Test"), []climbing.Route{route("r1", "A", "6a"), route("r2", "B", "6b")}, t0))
		must(t, s.SaveCrag(ctx, wall("c2", "Sasso 100%"), nil, t0))
		must(t, s.Save(ctx, ascent(t, "r1", diary.Done, "", t0)))
		must(t, s.Save(ctx, ascent(t, "r2", diary.Project, "", t0)))
		all, _ := s.CragSummaries(ctx, catalogue.Filter{})
		if len(all) != 2 || all[0].Crag.ID != "c1" || all[0].Routes != 2 || all[0].Done != 1 || *all[0].Crag.Place.Elevation != 600 {
			t.Fatalf("summaries %+v", all)
		}
		if got, _ := s.CragSummaries(ctx, catalogue.Filter{WithRoutes: true}); len(got) != 1 {
			t.Fatalf("with routes %+v", got)
		}
		if got, _ := s.CragSummaries(ctx, catalogue.Filter{Query: "CÒRNA"}); len(got) != 1 {
			t.Fatalf("by name %+v", got)
		}
		if got, _ := s.CragSummaries(ctx, catalogue.Filter{Query: "lecco"}); len(got) != 2 {
			t.Fatalf("by municipality %+v", got)
		}
		if got, _ := s.CragSummaries(ctx, catalogue.Filter{Query: "%"}); len(got) != 1 || got[0].Crag.ID != "c2" {
			t.Fatalf("a %% is a literal: %+v", got)
		}
	})

	t.Run("lookups and not found", func(t *testing.T) {
		s := fresh(t)
		must(t, s.SaveCrag(ctx, wall("c1", "Test Wall"), []climbing.Route{route("r1", "A", "6a")}, t0))
		if _, err := s.Crag(ctx, "nowhere"); !errors.Is(err, climbing.ErrCragNotFound) {
			t.Fatalf("crag: %v", err)
		}
		if _, err := s.Route(ctx, "nowhere"); !errors.Is(err, climbing.ErrRouteNotFound) {
			t.Fatalf("route: %v", err)
		}
		if err := s.Save(ctx, ascent(t, "nowhere", diary.Done, "", t0)); !errors.Is(err, climbing.ErrRouteNotFound) {
			t.Fatalf("ascent of an unknown route: %v", err)
		}
		crags, _ := s.CragsByID(ctx, []climbing.CragID{"c1", "c1", "nowhere"})
		routes, _ := s.RoutesByID(ctx, []climbing.RouteID{"r1", "nowhere"})
		if len(crags) != 1 || len(routes) != 1 {
			t.Fatalf("by id: %d crags, %d routes", len(crags), len(routes))
		}
	})

	t.Run("the diary is replaced, listed and cleared", func(t *testing.T) {
		s := fresh(t)
		must(t, s.SaveCrag(ctx, wall("c1", "Test Wall"), []climbing.Route{route("r1", "A", "6a"), route("r2", "B", "6b")}, t0))
		must(t, s.Save(ctx, ascent(t, "r1", diary.Project, "", t0)))
		must(t, s.Save(ctx, ascent(t, "r1", diary.Done, "2026-10-01", t0.Add(time.Hour))))
		must(t, s.Save(ctx, ascent(t, "r2", diary.Todo, "", t0)))
		all, _ := s.All(ctx)
		if len(all) != 2 {
			t.Fatalf("%d entries", len(all))
		}
		got, _ := s.ForRoutes(ctx, []climbing.RouteID{"r1"})
		if got["r1"].Status() != diary.Done || !got["r1"].UpdatedAt().Equal(t0.Add(time.Hour)) {
			t.Fatalf("r1 = %+v", got["r1"])
		}
		must(t, s.Delete(ctx, "r1"))
		must(t, s.Delete(ctx, "r1")) // twice is no error
		if all, _ := s.All(ctx); len(all) != 1 {
			t.Fatalf("%d entries after a delete", len(all))
		}
	})
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
