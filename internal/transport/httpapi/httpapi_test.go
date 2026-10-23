package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Allan-Nava/cragbook/internal/application/catalogue"
	"github.com/Allan-Nava/cragbook/internal/application/importing"
	"github.com/Allan-Nava/cragbook/internal/application/journal"
	"github.com/Allan-Nava/cragbook/internal/domain/climbing"
	"github.com/Allan-Nava/cragbook/internal/infrastructure/memory"
	"github.com/Allan-Nava/cragbook/internal/transport/httpapi"
)

// The real use cases over the memory adapter; made-up crags and routes, not real places.
var now = time.Date(2026, 10, 23, 15, 0, 0, 0, time.UTC)

type importer func(context.Context) (importing.Summary, error)

func (f importer) Run(ctx context.Context) (importing.Summary, error) { return f(ctx) }

func p[T any](v T) *T { return &v }

func setup(t *testing.T, imp importer) http.Handler {
	t.Helper()
	m := memory.New()
	g, _ := climbing.ParseGrade("6a+")
	m.SaveCrag(context.Background(), climbing.Crag{ID: "osm-node-1", Name: "Test Wall", Place: climbing.Place{Municipality: p("Lecco")}},
		[]climbing.Route{{ID: "osm-node-2", Name: p("Diedro"), Grade: &g, Sector: p("Settore Alto")}, {ID: "osm-way-3", Name: p("Placca")}}, now)
	m.SaveCrag(context.Background(), climbing.Crag{ID: "osm-node-9", Name: "Empty Wall"}, nil, now)
	if imp == nil {
		imp = func(context.Context) (importing.Summary, error) {
			return importing.Summary{Crags: 2, Failed: []string{}}, nil
		}
	}
	clock := func() time.Time { return now }
	h := httpapi.New("v0.1.0-test", catalogue.NewService(m, m, m), journal.NewService(m, m, clock), imp, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return h.Routes(nil)
}

func do(t *testing.T, h http.Handler, method, path, body string) (int, any) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
	var v any
	json.Unmarshal(rec.Body.Bytes(), &v)
	return rec.Code, v
}

func obj(v any) map[string]any { m, _ := v.(map[string]any); return m }
func list(v any) []any         { l, _ := v.([]any); return l }

func TestListsAndSearchesCrags(t *testing.T) {
	h := setup(t, nil)
	if code, v := do(t, h, "GET", "/api/crags", ""); code != 200 || len(list(v)) != 2 {
		t.Fatalf("%d %v", code, v)
	}
	if _, v := do(t, h, "GET", "/api/crags?withRoutes=true", ""); len(list(v)) != 1 || obj(list(v)[0])["routes"] != 2.0 {
		t.Fatalf("with routes: %v", v)
	}
	if _, v := do(t, h, "GET", "/api/crags?q=empty", ""); len(list(v)) != 1 {
		t.Fatalf("search: %v", v)
	}
	if code, v := do(t, h, "GET", "/api/crags?q="+strings.Repeat("x", 101), ""); code != 400 || !strings.Contains(obj(v)["error"].(string), "100") {
		t.Fatalf("a long query: %d %v", code, v)
	}
}

func TestACragComesWithItsRoutesLevelsAndOSMLinks(t *testing.T) {
	h := setup(t, nil)
	code, v := do(t, h, "GET", "/api/crags/osm-node-1", "")
	c := obj(v)
	if code != 200 || c["name"] != "Test Wall" || c["osm"] != "https://www.openstreetmap.org/node/1" {
		t.Fatalf("%d %v", code, v)
	}
	routes := list(c["routeList"])
	first, second := obj(routes[0]), obj(routes[1])
	if first["grade"] != "6a+" || first["level"] != "6" || first["osm"] != "https://www.openstreetmap.org/node/2" || first["ascent"] != nil {
		t.Fatalf("first %v", first)
	}
	if second["level"] != "" || second["grade"] != nil {
		t.Fatalf("second %v", second)
	}
	if code, v := do(t, h, "GET", "/api/crags/nowhere", ""); code != 404 || obj(v)["error"] != "crag not found" {
		t.Fatalf("unknown crag: %d %v", code, v)
	}
}

func TestTheDiaryIsWrittenReadAndCleared(t *testing.T) {
	h := setup(t, nil)
	code, v := do(t, h, "PUT", "/api/routes/osm-node-2/ascent", `{"status":"done","climbedOn":"2026-10-20","notes":"  a vista  "}`)
	a := obj(obj(v)["ascent"])
	if code != 200 || a["status"] != "done" || a["climbedOn"] != "2026-10-20" || a["notes"] != "a vista" || a["updatedAt"] != "2026-10-23T15:00:00Z" {
		t.Fatalf("%d %v", code, v)
	}
	if _, v := do(t, h, "GET", "/api/crags/osm-node-1", ""); obj(v)["done"] != 1.0 {
		t.Fatalf("done count: %v", obj(v)["done"])
	}
	if _, v := do(t, h, "GET", "/api/diary", ""); len(list(v)) != 1 || obj(list(v)[0])["cragName"] != "Test Wall" {
		t.Fatalf("diary %v", v)
	}
	if code, v := do(t, h, "PUT", "/api/routes/osm-node-2/ascent", `{"status":"project","climbedOn":""}`); code != 200 || obj(obj(v)["ascent"])["climbedOn"] != nil {
		t.Fatalf("an empty date is no date: %d %v", code, v)
	}
	if code, _ := do(t, h, "DELETE", "/api/routes/osm-node-2/ascent", ""); code != 204 {
		t.Fatalf("delete: %d", code)
	}
	if _, v := do(t, h, "GET", "/api/diary", ""); len(list(v)) != 0 {
		t.Fatalf("diary after delete: %v", v)
	}
}

func TestWhatTheAPIRefuses(t *testing.T) {
	h := setup(t, nil)
	cases := map[string]string{
		`{"status":"maybe"}`:                                            "status",
		`{"status":"todo","climbedOn":"2026-10-01"}`:                    "only for a route climbed",
		`{"status":"done","climbedOn":"20/10/2026"}`:                    "YYYY-MM-DD",
		`{"status":"done","climbedOn":"2026-10-24"}`:                    "after today",
		`{"status":"done","notes":"` + strings.Repeat("é", 2001) + `"}`: "2000",
		`{"status":"done","rating":5}`:                                  "unknown field",
		`not json`:                                                      "body",
	}
	for body, want := range cases {
		code, v := do(t, h, "PUT", "/api/routes/osm-node-2/ascent", body)
		if msg, _ := obj(v)["error"].(string); code != 400 || !strings.Contains(msg, want) {
			t.Errorf("%.40s: %d %v, want 400 about %q", body, code, v, want)
		}
	}
	if code, _ := do(t, h, "PUT", "/api/routes/nowhere/ascent", `{"status":"done"}`); code != 404 {
		t.Errorf("unknown route: %d", code)
	}
	if code, _ := do(t, h, "DELETE", "/api/routes/nowhere/ascent", ""); code != 404 {
		t.Errorf("delete unknown route: %d", code)
	}
}

func TestImportStatuses(t *testing.T) {
	for _, c := range []struct {
		err  error
		code int
		msg  string
	}{
		{nil, 200, ""},
		{importing.ErrAlreadyRunning, 409, "already running"},
		{fmt.Errorf("import: %w: %w", importing.ErrSource, errors.New("GET crags.json: 502 Bad Gateway")), 502, "502"},
		{errors.New("disk full at /var/lib/x"), 500, "internal error"},
	} {
		h := setup(t, func(context.Context) (importing.Summary, error) {
			return importing.Summary{Crags: 2, Failed: []string{}}, c.err
		})
		code, v := do(t, h, "POST", "/api/import", "")
		if code != c.code || (c.msg != "" && !strings.Contains(fmt.Sprint(v), c.msg)) {
			t.Errorf("%v: %d %v, want %d", c.err, code, v, c.code)
		}
		if c.code == 500 && strings.Contains(fmt.Sprint(v), "disk") {
			t.Errorf("an internal error showed its cause: %v", v)
		}
	}
}

func TestUnknownEndpointsAndMethods(t *testing.T) {
	h := setup(t, nil)
	if code, _ := do(t, h, "GET", "/api/nothing", ""); code != 404 {
		t.Errorf("unknown: %d", code)
	}
	if code, _ := do(t, h, "DELETE", "/api/crags/osm-node-1", ""); code != 405 {
		t.Errorf("wrong method: %d", code)
	}
	if code, v := do(t, h, "GET", "/healthz", ""); code != 200 || obj(v)["status"] != "ok" || obj(v)["version"] != "v0.1.0-test" {
		t.Errorf("healthz: %d %v", code, v)
	}
}
