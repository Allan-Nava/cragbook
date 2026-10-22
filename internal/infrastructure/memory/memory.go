// Package memory keeps the catalogue and the diary in memory, with the same contract as the
// PostgreSQL adapter: for tests of the use cases, and for trying cragbook without a database.
package memory

import (
	"context"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Allan-Nava/cragbook/internal/application/catalogue"
	"github.com/Allan-Nava/cragbook/internal/domain/climbing"
	"github.com/Allan-Nava/cragbook/internal/domain/diary"
)

type Store struct {
	mu      sync.RWMutex
	crags   map[climbing.CragID]climbing.Crag
	order   []climbing.CragID
	routes  map[climbing.RouteID]climbing.Route
	ascents map[climbing.RouteID]diary.Ascent
}

func New() *Store {
	return &Store{crags: map[climbing.CragID]climbing.Crag{}, routes: map[climbing.RouteID]climbing.Route{},
		ascents: map[climbing.RouteID]diary.Ascent{}}
}

var (
	_ climbing.Catalog       = (*Store)(nil)
	_ climbing.CatalogWriter = (*Store)(nil)
	_ diary.Repository       = (*Store)(nil)
	_ catalogue.Summaries    = (*Store)(nil)
)

func (s *Store) SaveCrag(ctx context.Context, c climbing.Crag, routes []climbing.Route, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.crags[c.ID]; !ok {
		s.order = append(s.order, c.ID)
	}
	s.crags[c.ID] = c
	listed := map[climbing.RouteID]bool{}
	for _, r := range routes {
		r.CragID, r.Gone = c.ID, false
		s.routes[r.ID] = r
		listed[r.ID] = true
	}
	for id, r := range s.routes {
		if r.CragID == c.ID && !listed[id] {
			r.Gone = true
			s.routes[id] = r
		}
	}
	return nil
}

func (s *Store) Crag(ctx context.Context, id climbing.CragID) (climbing.Crag, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.crags[id]
	if !ok {
		return climbing.Crag{}, climbing.ErrCragNotFound
	}
	return c, nil
}

func (s *Store) CragsByID(ctx context.Context, ids []climbing.CragID) ([]climbing.Crag, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []climbing.Crag
	for _, id := range ids {
		if c, ok := s.crags[id]; ok && !slices.ContainsFunc(out, func(o climbing.Crag) bool { return o.ID == id }) {
			out = append(out, c)
		}
	}
	return out, nil
}

func (s *Store) Routes(ctx context.Context, crag climbing.CragID) ([]climbing.Route, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []climbing.Route
	for _, r := range s.routes {
		if r.CragID == crag {
			out = append(out, r)
		}
	}
	return out, nil
}

func (s *Store) Route(ctx context.Context, id climbing.RouteID) (climbing.Route, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.routes[id]
	if !ok {
		return climbing.Route{}, climbing.ErrRouteNotFound
	}
	return r, nil
}

func (s *Store) RoutesByID(ctx context.Context, ids []climbing.RouteID) ([]climbing.Route, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []climbing.Route
	for _, id := range ids {
		if r, ok := s.routes[id]; ok {
			out = append(out, r)
		}
	}
	return out, nil
}

func (s *Store) Save(ctx context.Context, a diary.Ascent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.routes[a.Route()]; !ok {
		return climbing.ErrRouteNotFound
	}
	s.ascents[a.Route()] = a
	return nil
}

func (s *Store) Delete(ctx context.Context, route climbing.RouteID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.ascents, route)
	return nil
}

func (s *Store) ForRoutes(ctx context.Context, routes []climbing.RouteID) (map[climbing.RouteID]diary.Ascent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := map[climbing.RouteID]diary.Ascent{}
	for _, id := range routes {
		if a, ok := s.ascents[id]; ok {
			out[id] = a
		}
	}
	return out, nil
}

func (s *Store) All(ctx context.Context) ([]diary.Ascent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]diary.Ascent, 0, len(s.ascents))
	for _, a := range s.ascents {
		out = append(out, a)
	}
	return out, nil
}

func (s *Store) CragSummaries(ctx context.Context, f catalogue.Filter) ([]catalogue.CragSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	q := strings.ToLower(f.Query)
	var out []catalogue.CragSummary
	for _, id := range s.order {
		c := s.crags[id]
		town := ""
		if c.Place.Municipality != nil {
			town = strings.ToLower(*c.Place.Municipality)
		}
		if q != "" && !strings.Contains(strings.ToLower(c.Name), q) && !strings.Contains(town, q) {
			continue
		}
		sum := catalogue.CragSummary{Crag: c}
		for _, r := range s.routes {
			if r.CragID == id && !r.Gone {
				sum.Routes++
				if a, ok := s.ascents[r.ID]; ok && a.Status() == diary.Done {
					sum.Done++
				}
			}
		}
		if f.WithRoutes && sum.Routes == 0 {
			continue
		}
		out = append(out, sum)
	}
	slices.SortFunc(out, func(a, b catalogue.CragSummary) int {
		return strings.Compare(a.Crag.Name+string(a.Crag.ID), b.Crag.Name+string(b.Crag.ID))
	})
	return out, nil
}
