package metrics_test

import (
	"errors"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Allan-Nava/cragbook/internal/application/importing"
	"github.com/Allan-Nava/cragbook/internal/infrastructure/metrics"
)

func scrape(t *testing.T, m *metrics.Metrics) string {
	t.Helper()
	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	b, _ := io.ReadAll(rec.Body)
	return string(b)
}

func has(t *testing.T, text string, lines ...string) {
	t.Helper()
	for _, l := range lines {
		if !strings.Contains(text, "\n"+l+"\n") {
			t.Errorf("no line %q", l)
		}
	}
}

func TestRequestsByRouteAndStatus(t *testing.T) {
	m := metrics.New("v1.2.3")
	m.Served("GET", "GET /api/crags/{id}", 200, 20*time.Millisecond)
	m.Served("GET", "GET /api/crags/{id}", 200, 30*time.Millisecond)
	m.Served("GET", "GET /api/crags/{id}", 404, time.Millisecond)
	text := scrape(t, m)
	has(t, text,
		`cragbook_http_requests_total{method="GET",route="GET /api/crags/{id}",status="200"} 2`,
		`cragbook_http_requests_total{method="GET",route="GET /api/crags/{id}",status="404"} 1`,
		`cragbook_http_request_duration_seconds_count{method="GET",route="GET /api/crags/{id}"} 3`,
		`cragbook_build_info{version="v1.2.3"} 1`,
	)
	if !strings.Contains(text, "go_goroutines") || !strings.Contains(text, "process_") {
		t.Error("no runtime or process metrics")
	}
}

func TestTheLastImportThatWentThrough(t *testing.T) {
	m := metrics.New("dev")
	ok := time.Unix(1_800_000_000, 0)
	m.Imported(ok, importing.Summary{Crags: 218, Routes: 116, Failed: []string{"a: 502", "b: 502"}}, nil)
	m.Imported(ok.Add(time.Hour), importing.Summary{}, errors.New("import: the source failed"))
	has(t, scrape(t, m),
		`cragbook_imports_total{result="ok"} 1`,
		`cragbook_imports_total{result="error"} 1`,
		`cragbook_import_last_run_timestamp_seconds 1.8000036e+09`,
		`cragbook_import_last_success_timestamp_seconds 1.8e+09`,
		`cragbook_import_crags 218`,
		`cragbook_import_routes 116`,
		`cragbook_import_failed_crags 2`,
	)
}
