package cragcast

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Made-up crags and routes, not real places.
const index = `{"meta":{"version":1,"generatedAt":"2026-10-22T10:00:00Z","cragData":"© OpenStreetMap contributors"},
 "crags":[{"id":"osm-node-1","name":"Test Wall","municipality":"Lecco","provinceCode":"LC","latitude":45.8,
 "longitude":9.4,"aspect":"S","elevation":600,"url":"crags/osm-node-1.json"}]}`

const crag = `{"meta":{"version":1},"crag":{"id":"osm-node-1","name":"Test Wall",
 "sectors":[{"id":"osm-way-2","name":"Test Wall - Settore Alto","aspect":"S","gradeMin":"5b","gradeMax":"7a"}],
 "routes":[{"id":"osm-node-3","name":"Diedro","grade":"6a+","length":25,"bolts":11,"pitches":1,"sector":"osm-way-2"},
           {"id":"osm-node-4","name":null,"grade":null,"length":null,"bolts":null,"pitches":null,"sector":null}]}}`

func serve(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c, err := New(srv.URL+"/api/v1", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestReadsTheIndexAndACragFile(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/crags.json":
			fmt.Fprint(w, index)
		case "/api/v1/crags/osm-node-1.json":
			fmt.Fprint(w, crag)
		default:
			http.NotFound(w, r)
		}
	})
	idx, err := c.Index(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Crags) != 1 || idx.Crags[0].Name != "Test Wall" || *idx.Crags[0].ProvinceCode != "LC" {
		t.Fatalf("index %+v", idx)
	}
	got, err := c.Crag(context.Background(), idx.Crags[0].URL)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Routes) != 2 || *got.Routes[0].Grade != "6a+" || got.Routes[1].Name != nil || *got.Sectors[0].Name != "Test Wall - Settore Alto" {
		t.Fatalf("crag %+v", got)
	}
}

func TestAnotherAPIVersionIsRefused(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"meta":{"version":2},"crags":[]}`) })
	if _, err := c.Index(context.Background()); err == nil || !strings.Contains(err.Error(), "version 2") {
		t.Fatalf("err = %v, want a version error", err)
	}
}

func TestAnErrorStatusIsAnError(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) { http.Error(w, "down", http.StatusBadGateway) })
	if _, err := c.Index(context.Background()); err == nil || !strings.Contains(err.Error(), "502") {
		t.Fatalf("err = %v, want the status", err)
	}
}

func TestAStalledServerTimesOut(t *testing.T) {
	stop := make(chan struct{})
	t.Cleanup(func() { close(stop) })
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-stop:
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(srv.Close)
	c, _ := New(srv.URL, 50*time.Millisecond)
	start := time.Now()
	if _, err := c.Index(context.Background()); err == nil {
		t.Fatal("no error from a server that never answers")
	}
	if time.Since(start) > 2*time.Second {
		t.Fatalf("took %s", time.Since(start))
	}
}

func TestAPathOutsideTheAPIIsRefused(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) { t.Errorf("requested %s", r.URL) })
	for _, p := range []string{"https://evil.example/x.json", "/etc/passwd", "//evil.example/x"} {
		if _, err := c.Crag(context.Background(), p); err == nil {
			t.Errorf("Crag(%q) fetched", p)
		}
	}
}

func TestABaseThatIsNoHTTPURLIsRefused(t *testing.T) {
	for _, b := range []string{"ftp://x/", "not a url", "file:///tmp/"} {
		if _, err := New(b, time.Second); err == nil {
			t.Errorf("New(%q) accepted", b)
		}
	}
}

func TestTheAdapterGivesTheDomainItsCragsAndRoutes(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/crags.json":
			fmt.Fprint(w, index)
		case "/api/v1/crags/osm-node-1.json":
			fmt.Fprint(w, crag)
		}
	})
	ctx := context.Background()
	list, err := c.List(ctx)
	if err != nil || len(list) != 1 || list[0].Crag.Name != "Test Wall" || *list[0].Crag.Place.Elevation != 600 {
		t.Fatalf("%+v, %v", list, err)
	}
	routes, err := c.Routes(ctx, list[0])
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 2 || routes[0].Grade.String() != "6a+" || *routes[0].Sector != "Test Wall - Settore Alto" || routes[0].CragID != "osm-node-1" {
		t.Fatalf("routes %+v", routes)
	}
	if routes[1].Grade != nil || routes[1].Sector != nil {
		t.Fatalf("second route %+v", routes[1])
	}
	other := list[0]
	other.Crag.ID = "osm-node-9"
	if _, err := c.Routes(ctx, other); err == nil || !strings.Contains(err.Error(), "is for crag") {
		t.Fatalf("a file for another crag: %v", err)
	}
}
