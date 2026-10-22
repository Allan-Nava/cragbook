// Package climbing is the catalogue: crags and their single routes, as OpenStreetMap maps them
// (© OpenStreetMap contributors, ODbL 1.0) and cragcast publishes them. cragbook copies it and
// never edits it; the user's own data is in package diary.
package climbing

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"strings"
	"time"
)

// CragID and RouteID are OSM ids as cragcast writes them: osm-node-123, osm-way-4, osm-relation-5.
type CragID string
type RouteID string

// OSMURL is the object's page on OpenStreetMap, or "" for an id not in that form.
func OSMURL(id string) string {
	parts := strings.SplitN(id, "-", 3)
	if len(parts) != 3 || parts[0] != "osm" || parts[2] == "" {
		return ""
	}
	switch parts[1] {
	case "node", "way", "relation":
		return "https://www.openstreetmap.org/" + parts[1] + "/" + parts[2]
	}
	return ""
}

// Place is where a crag is: optional parts are nil where OSM has none.
type Place struct {
	Municipality *string
	ProvinceCode *string
	Latitude     float64
	Longitude    float64
	Aspect       *string // the wall's aspect, N … NW
	Elevation    *int    // metres
}

type Crag struct {
	ID    CragID
	Name  string
	Place Place
}

// Route is one single route of a crag. Gone is set once an import no longer finds it on
// OpenStreetMap: the route is kept, never deleted, so a diary entry about it stays readable.
type Route struct {
	ID      RouteID
	CragID  CragID
	Name    *string
	Grade   *Grade
	Length  *float64 // metres
	Bolts   *int
	Pitches *int
	Sector  *string // the sector's name
	Gone    bool
}

// Level is the route's grade level, LevelNone without a grade.
func (r Route) Level() Level {
	if r.Grade == nil {
		return LevelNone
	}
	return r.Grade.Level()
}

// SortRoutes orders routes easiest first, the ungraded last, then by name and id, so a list
// reads the same on every request.
func SortRoutes(routes []Route) {
	slices.SortStableFunc(routes, func(a, b Route) int {
		if c := cmp.Compare(rankOrMax(a), rankOrMax(b)); c != 0 {
			return c
		}
		if c := cmp.Compare(nameOrMax(a), nameOrMax(b)); c != 0 {
			return c
		}
		return cmp.Compare(a.ID, b.ID)
	})
}

func rankOrMax(r Route) int {
	if r.Grade == nil {
		return 1 << 30
	}
	return r.Grade.Rank()
}

func nameOrMax(r Route) string {
	if r.Name == nil {
		return "￿"
	}
	return strings.ToLower(*r.Name)
}

var (
	ErrCragNotFound  = errors.New("crag not found")
	ErrRouteNotFound = errors.New("route not found")
)

// Catalog reads the catalogue. Crag and Route return ErrCragNotFound and ErrRouteNotFound.
type Catalog interface {
	Crag(ctx context.Context, id CragID) (Crag, error)
	CragsByID(ctx context.Context, ids []CragID) ([]Crag, error)
	Routes(ctx context.Context, crag CragID) ([]Route, error) // gone ones included
	Route(ctx context.Context, id RouteID) (Route, error)
	RoutesByID(ctx context.Context, ids []RouteID) ([]Route, error)
}

// CatalogWriter is how an import replaces one crag's copy: the crag and the routes listed are
// written, routes no longer listed are marked gone, a gone route listed again comes back.
type CatalogWriter interface {
	SaveCrag(ctx context.Context, c Crag, routes []Route, at time.Time) error
}
