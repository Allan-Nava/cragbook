package cragcast

import (
	"context"
	"fmt"

	"github.com/Allan-Nava/cragbook/internal/application/importing"
	"github.com/Allan-Nava/cragbook/internal/domain/climbing"
)

var _ importing.Source = (*Client)(nil)

// List is the import's listing: every crag of the index, with the path of its file.
func (c *Client) List(ctx context.Context) ([]importing.Listing, error) {
	idx, err := c.Index(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]importing.Listing, 0, len(idx.Crags))
	for _, e := range idx.Crags {
		out = append(out, importing.Listing{Ref: e.URL, Crag: climbing.Crag{
			ID: climbing.CragID(e.ID), Name: e.Name,
			Place: climbing.Place{Municipality: e.Municipality, ProvinceCode: e.ProvinceCode, Latitude: e.Latitude,
				Longitude: e.Longitude, Aspect: e.Aspect, Elevation: e.Elevation},
		}})
	}
	return out, nil
}

// Routes reads a crag's file into domain routes: a grade that is no French grade is dropped, the
// sector's id becomes its name. A file that is about another crag is refused.
func (c *Client) Routes(ctx context.Context, l importing.Listing) ([]climbing.Route, error) {
	file, err := c.Crag(ctx, l.Ref)
	if err != nil {
		return nil, err
	}
	if climbing.CragID(file.ID) != l.Crag.ID {
		return nil, fmt.Errorf("the file %s is for crag %q", l.Ref, file.ID)
	}
	sectors := make(map[string]*string, len(file.Sectors))
	for _, s := range file.Sectors {
		sectors[s.ID] = s.Name
	}
	out := make([]climbing.Route, 0, len(file.Routes))
	for _, r := range file.Routes {
		route := climbing.Route{ID: climbing.RouteID(r.ID), CragID: l.Crag.ID, Name: r.Name, Length: r.Length,
			Bolts: r.Bolts, Pitches: r.Pitches}
		if r.Grade != nil {
			if g, ok := climbing.ParseGrade(*r.Grade); ok {
				route.Grade = &g
			}
		}
		if r.Sector != nil {
			route.Sector = sectors[*r.Sector]
		}
		out = append(out, route)
	}
	return out, nil
}
