package importing_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Allan-Nava/cragbook/internal/application/importing"
	"github.com/Allan-Nava/cragbook/internal/domain/climbing"
	"github.com/Allan-Nava/cragbook/internal/infrastructure/memory"
)

// A made-up source of n crags with two routes each, not real places.
type source struct {
	n        int
	fail     map[climbing.CragID]error
	delay    time.Duration
	inFlight atomic.Int32
	peak     atomic.Int32
	block    chan struct{} // List waits on it, after closing entered
	entered  chan struct{}
	listErr  error
}

func (s *source) List(ctx context.Context) ([]importing.Listing, error) {
	if s.block != nil {
		close(s.entered)
		<-s.block
	}
	if s.listErr != nil {
		return nil, s.listErr
	}
	out := []importing.Listing{}
	for i := range s.n {
		id := climbing.CragID(fmt.Sprintf("osm-node-%d", i))
		out = append(out, importing.Listing{Crag: climbing.Crag{ID: id, Name: "Wall " + string(id)}, Ref: string(id)})
	}
	return out, nil
}

func (s *source) Routes(ctx context.Context, l importing.Listing) ([]climbing.Route, error) {
	n := s.inFlight.Add(1)
	defer s.inFlight.Add(-1)
	for p := s.peak.Load(); n > p; p = s.peak.Load() {
		if s.peak.CompareAndSwap(p, n) {
			break
		}
	}
	select {
	case <-time.After(s.delay):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if err := s.fail[l.Crag.ID]; err != nil {
		return nil, err
	}
	g, _ := climbing.ParseGrade("6a+")
	return []climbing.Route{{ID: climbing.RouteID(l.Ref + "-r1"), Grade: &g}, {ID: climbing.RouteID(l.Ref + "-r2")}}, nil
}

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

// newLogger writes text logs to w, under mu.
func newLogger(mu *sync.Mutex, w io.Writer) *slog.Logger {
	return slog.New(slog.NewTextHandler(lockedWriter{mu, w}, nil))
}

type lockedWriter struct {
	mu *sync.Mutex
	w  io.Writer
}

func (l lockedWriter) Write(b []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(b)
}

// heard is an Observer that keeps what it hears.
type heard struct {
	mu   sync.Mutex
	runs []string
}

func (h *heard) Imported(_ time.Time, sum importing.Summary, err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.runs = append(h.runs, fmt.Sprintf("%d crags, %d failed, err %v", sum.Crags, len(sum.Failed), err))
}

func service(src importing.Source, m *memory.Store, cfg importing.Config) *importing.Service {
	return importing.NewService(src, m, time.Now, quiet, cfg, &heard{})
}

func TestImportsEveryCragWithItsRoutes(t *testing.T) {
	m := memory.New()
	sum, err := service(&source{n: 20}, m, importing.DefaultConfig).Run(context.Background())
	if err != nil || sum.Crags != 20 || sum.Routes != 40 || len(sum.Failed) != 0 {
		t.Fatalf("%+v, %v", sum, err)
	}
	if r, _ := m.Route(context.Background(), "osm-node-3-r1"); r.CragID != "osm-node-3" || r.Grade.Rank() != 37 {
		t.Fatalf("route %+v", r)
	}
}

func TestAtMostWorkersCragsAtOnce(t *testing.T) {
	src := &source{n: 30, delay: 5 * time.Millisecond}
	if _, err := service(src, memory.New(), importing.Config{Workers: 3}).Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if peak := src.peak.Load(); peak < 2 || peak > 3 {
		t.Fatalf("peak %d at once, want 2..3", peak)
	}
}

func TestACragThatFailsIsLeftAsItWas(t *testing.T) {
	m := memory.New()
	ctx := context.Background()
	if err := m.SaveCrag(ctx, climbing.Crag{ID: "osm-node-2", Name: "Wall"}, []climbing.Route{{ID: "kept"}}, time.Now()); err != nil {
		t.Fatal(err)
	}
	src := &source{n: 5, fail: map[climbing.CragID]error{"osm-node-2": errors.New("502 Bad Gateway")}}
	sum, err := service(src, m, importing.DefaultConfig).Run(ctx)
	if err != nil || sum.Crags != 4 || len(sum.Failed) != 1 || !strings.Contains(sum.Failed[0], "osm-node-2: 502") {
		t.Fatalf("%+v, %v", sum, err)
	}
	if r, _ := m.Route(ctx, "kept"); r.Gone {
		t.Fatal("the failed crag's route was marked gone")
	}
}

func TestOneRunAtATime(t *testing.T) {
	src := &source{n: 1, block: make(chan struct{}), entered: make(chan struct{})}
	s := service(src, memory.New(), importing.DefaultConfig)
	done := make(chan error)
	go func() { _, err := s.Run(context.Background()); done <- err }()
	<-src.entered // the first run is inside List
	if _, err := s.Run(context.Background()); !errors.Is(err, importing.ErrAlreadyRunning) {
		t.Fatalf("second run: %v, want ErrAlreadyRunning", err)
	}
	close(src.block)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	src.block = nil
	if _, err := s.Run(context.Background()); err != nil {
		t.Fatalf("a run after the first: %v", err)
	}
}

func TestATimedOutImportStopsAndSaysSo(t *testing.T) {
	src := &source{n: 200, delay: 20 * time.Millisecond}
	start := time.Now()
	_, err := service(src, memory.New(), importing.Config{Workers: 4, Timeout: 30 * time.Millisecond}).Run(context.Background())
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > time.Second {
		t.Fatalf("%v after %s", err, time.Since(start))
	}
}

func TestEveryRunIsHeardAndARefusedOneIsNot(t *testing.T) {
	h := &heard{}
	src := &source{n: 2, fail: map[climbing.CragID]error{"osm-node-1": errors.New("502 Bad Gateway")}}
	s := importing.NewService(src, memory.New(), time.Now, quiet, importing.DefaultConfig, h)
	if _, err := s.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	src.listErr = errors.New("crags.json: 503")
	if _, err := s.Run(context.Background()); !errors.Is(err, importing.ErrSource) {
		t.Fatalf("%v, want ErrSource", err)
	}
	src.listErr, src.block, src.entered = nil, make(chan struct{}), make(chan struct{})
	go func() { <-src.entered; _, _ = s.Run(context.Background()); close(src.block) }()
	if _, err := s.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"1 crags, 1 failed, err <nil>",
		"0 crags, 0 failed, err import: the source failed: crags.json: 503",
		"1 crags, 1 failed, err <nil>", // and the run refused meanwhile, not heard
	}
	if strings.Join(h.runs, "\n") != strings.Join(want, "\n") {
		t.Fatalf("heard\n%s\nwant\n%s", strings.Join(h.runs, "\n"), strings.Join(want, "\n"))
	}
}
