// Package journal is the use case of keeping the diary: recording what was climbed, tried or
// planned on a route, clearing it, and reading it back.
package journal

import (
	"context"
	"slices"
	"time"

	"github.com/Allan-Nava/cragbook/internal/domain/climbing"
	"github.com/Allan-Nava/cragbook/internal/domain/diary"
)

// Record is the command of writing a route's entry, replacing the one it had.
type Record struct {
	Route     climbing.RouteID
	Status    string
	ClimbedOn *time.Time
	Notes     string
}

// Entry is a diary entry with the route and crag it is about, to be read on its own.
type Entry struct {
	Ascent diary.Ascent
	Route  climbing.Route
	Crag   climbing.Crag
}

type Service struct {
	catalog climbing.Catalog
	ascents diary.Repository
	now     func() time.Time
}

func NewService(c climbing.Catalog, a diary.Repository, now func() time.Time) *Service {
	return &Service{catalog: c, ascents: a, now: now}
}

// Record writes the entry; the route must be in the catalogue (climbing.ErrRouteNotFound), and
// the entry follows the diary's rules (diary.ErrInvalid).
func (s *Service) Record(ctx context.Context, cmd Record) (climbing.Route, diary.Ascent, error) {
	route, err := s.catalog.Route(ctx, cmd.Route)
	if err != nil {
		return climbing.Route{}, diary.Ascent{}, err
	}
	status, err := diary.ParseStatus(cmd.Status)
	if err != nil {
		return climbing.Route{}, diary.Ascent{}, err
	}
	a, err := diary.NewAscent(route.ID, status, cmd.ClimbedOn, cmd.Notes, s.now())
	if err != nil {
		return climbing.Route{}, diary.Ascent{}, err
	}
	if err := s.ascents.Save(ctx, a); err != nil {
		return climbing.Route{}, diary.Ascent{}, err
	}
	return route, a, nil
}

// Clear removes a route's entry; clearing one that has none is no error.
func (s *Service) Clear(ctx context.Context, id climbing.RouteID) error {
	if _, err := s.catalog.Route(ctx, id); err != nil {
		return err
	}
	return s.ascents.Delete(ctx, id)
}

// Entries is the whole diary, in diary.Compare's order.
func (s *Service) Entries(ctx context.Context) ([]Entry, error) {
	all, err := s.ascents.All(ctx)
	if err != nil {
		return nil, err
	}
	slices.SortFunc(all, diary.Compare)
	routeIDs := make([]climbing.RouteID, 0, len(all))
	for _, a := range all {
		routeIDs = append(routeIDs, a.Route())
	}
	routes, err := s.catalog.RoutesByID(ctx, routeIDs)
	if err != nil {
		return nil, err
	}
	byRoute := make(map[climbing.RouteID]climbing.Route, len(routes))
	var cragIDs []climbing.CragID
	for _, r := range routes {
		byRoute[r.ID] = r
		cragIDs = append(cragIDs, r.CragID)
	}
	crags, err := s.catalog.CragsByID(ctx, cragIDs)
	if err != nil {
		return nil, err
	}
	byCrag := make(map[climbing.CragID]climbing.Crag, len(crags))
	for _, c := range crags {
		byCrag[c.ID] = c
	}
	out := make([]Entry, 0, len(all))
	for _, a := range all {
		r, ok := byRoute[a.Route()]
		if !ok {
			continue // routes are never deleted, so this is a race with nothing to show
		}
		out = append(out, Entry{Ascent: a, Route: r, Crag: byCrag[r.CragID]})
	}
	return out, nil
}
