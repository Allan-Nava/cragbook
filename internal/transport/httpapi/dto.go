package httpapi

import (
	"time"

	"github.com/Allan-Nava/cragbook/internal/application/catalogue"
	"github.com/Allan-Nava/cragbook/internal/domain/climbing"
	"github.com/Allan-Nava/cragbook/internal/domain/diary"
)

// The API's JSON shapes, apart from the domain's types so either can change alone.

type errorDTO struct {
	Error string `json:"error"`
}

type cragDTO struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Municipality *string `json:"municipality"`
	ProvinceCode *string `json:"provinceCode"`
	Latitude     float64 `json:"latitude"`
	Longitude    float64 `json:"longitude"`
	Aspect       *string `json:"aspect"`
	Elevation    *int    `json:"elevation"`
	Routes       int     `json:"routes"` // still on OpenStreetMap
	Done         int     `json:"done"`   // of those, climbed
	OSM          string  `json:"osm"`
}

type cragDetailDTO struct {
	cragDTO
	RouteList []routeDTO `json:"routeList"`
}

type routeDTO struct {
	ID      string     `json:"id"`
	Name    *string    `json:"name"`
	Grade   *string    `json:"grade"`
	Level   string     `json:"level"` // "4" for 4 and under, "8" for 8 and over, "" ungraded
	Length  *float64   `json:"length"`
	Bolts   *int       `json:"bolts"`
	Pitches *int       `json:"pitches"`
	Sector  *string    `json:"sector"`
	Gone    bool       `json:"gone"` // no longer on OpenStreetMap, kept for its diary entry
	OSM     string     `json:"osm"`
	Ascent  *ascentDTO `json:"ascent"`
}

type ascentDTO struct {
	Status    string  `json:"status"`
	ClimbedOn *string `json:"climbedOn"`
	Notes     string  `json:"notes"`
	UpdatedAt string  `json:"updatedAt"`
}

// ascentIn is the body of PUT /api/routes/{id}/ascent.
type ascentIn struct {
	Status    string  `json:"status"`
	ClimbedOn *string `json:"climbedOn"`
	Notes     string  `json:"notes"`
}

type diaryEntryDTO struct {
	routeDTO
	CragID   string `json:"cragId"`
	CragName string `json:"cragName"`
}

type summaryDTO struct {
	Crags  int      `json:"crags"`
	Routes int      `json:"routes"`
	Failed []string `json:"failed"`
}

func toCrag(s catalogue.CragSummary) cragDTO {
	c, p := s.Crag, s.Crag.Place
	return cragDTO{string(c.ID), c.Name, p.Municipality, p.ProvinceCode, p.Latitude, p.Longitude, p.Aspect, p.Elevation,
		s.Routes, s.Done, climbing.OSMURL(string(c.ID))}
}

func toDetail(d catalogue.CragDetail) cragDetailDTO {
	out := cragDetailDTO{cragDTO: toCrag(d.CragSummary), RouteList: make([]routeDTO, 0, len(d.Routes))}
	for _, e := range d.Routes {
		out.RouteList = append(out.RouteList, toRoute(e.Route, e.Ascent))
	}
	return out
}

func toRoute(r climbing.Route, a *diary.Ascent) routeDTO {
	out := routeDTO{ID: string(r.ID), Name: r.Name, Level: string(r.Level()), Length: r.Length, Bolts: r.Bolts,
		Pitches: r.Pitches, Sector: r.Sector, Gone: r.Gone, OSM: climbing.OSMURL(string(r.ID))}
	if r.Grade != nil {
		g := r.Grade.String()
		out.Grade = &g
	}
	if a != nil {
		dto := &ascentDTO{Status: string(a.Status()), Notes: a.Notes(), UpdatedAt: a.UpdatedAt().UTC().Format(time.RFC3339)}
		if d := a.ClimbedOn(); d != nil {
			s := d.Format(time.DateOnly)
			dto.ClimbedOn = &s
		}
		out.Ascent = dto
	}
	return out
}
