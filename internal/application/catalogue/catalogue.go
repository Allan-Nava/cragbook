// Package catalogue is the use case of reading the crags and their routes, each route with the
// user's diary entry when there is one.
package catalogue

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/Allan-Nava/cragbook/internal/domain/climbing"
	"github.com/Allan-Nava/cragbook/internal/domain/diary"
)

// MaxQuery is the longest search, in characters.
const MaxQuery = 100

var ErrInvalidQuery = errors.New("invalid query")

type Filter struct {
	Query      string // in the crag's name or municipality, any case
	WithRoutes bool   // only crags with routes still on OpenStreetMap
}

// CragSummary is a crag with its counts, for a list: the routes still on OpenStreetMap and,
// of those, the ones climbed.
type CragSummary struct {
	Crag   climbing.Crag
	Routes int
	Done   int
}

// Summaries is the read model the crag list comes from: it crosses the catalogue and the diary
// in one query, which a list of 220 crags should not do route by route.
type Summaries interface {
	CragSummaries(ctx context.Context, f Filter) ([]CragSummary, error)
}

type RouteEntry struct {
	Route  climbing.Route
	Ascent *diary.Ascent
}

type CragDetail struct {
	CragSummary
	Routes []RouteEntry
}

type Service struct {
	summaries Summaries
	catalog   climbing.Catalog
	ascents   diary.Repository
}

func NewService(s Summaries, c climbing.Catalog, a diary.Repository) *Service {
	return &Service{summaries: s, catalog: c, ascents: a}
}

func (s *Service) Crags(ctx context.Context, f Filter) ([]CragSummary, error) {
	f.Query = strings.TrimSpace(f.Query)
	if utf8.RuneCountInString(f.Query) > MaxQuery {
		return nil, fmt.Errorf("%w: a search of more than %d characters", ErrInvalidQuery, MaxQuery)
	}
	return s.summaries.CragSummaries(ctx, f)
}

// Crag is one crag with its routes, easiest first. A route gone from OpenStreetMap is listed
// only while the diary has an entry for it, and counted in neither total.
func (s *Service) Crag(ctx context.Context, id climbing.CragID) (CragDetail, error) {
	crag, err := s.catalog.Crag(ctx, id)
	if err != nil {
		return CragDetail{}, err
	}
	routes, err := s.catalog.Routes(ctx, id)
	if err != nil {
		return CragDetail{}, err
	}
	ids := make([]climbing.RouteID, 0, len(routes))
	for _, r := range routes {
		ids = append(ids, r.ID)
	}
	ascents, err := s.ascents.ForRoutes(ctx, ids)
	if err != nil {
		return CragDetail{}, err
	}
	climbing.SortRoutes(routes)
	out := CragDetail{CragSummary: CragSummary{Crag: crag}, Routes: make([]RouteEntry, 0, len(routes))}
	for _, r := range routes {
		a, has := ascents[r.ID]
		if r.Gone && !has {
			continue
		}
		e := RouteEntry{Route: r}
		if has {
			e.Ascent = &a
		}
		out.Routes = append(out.Routes, e)
		if !r.Gone {
			out.CragSummary.Routes++
			if has && a.Status() == diary.Done {
				out.Done++
			}
		}
	}
	return out, nil
}
