// Package importing is the use case of refreshing the catalogue from cragcast: every crag
// listed, a few at a time, each written whole or not at all.
package importing

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Allan-Nava/cragbook/internal/domain/climbing"
)

// Listing is a crag as the source lists it, with how to fetch its routes.
type Listing struct {
	Crag climbing.Crag
	Ref  string
}

// Source is where the catalogue comes from; the cragcast adapter is one.
type Source interface {
	List(ctx context.Context) ([]Listing, error)
	Routes(ctx context.Context, l Listing) ([]climbing.Route, error)
}

type Summary struct {
	Crags  int
	Routes int
	Failed []string // crags left as they were, each with its error
}

var (
	ErrAlreadyRunning = errors.New("an import is already running")
	// ErrSource wraps a failure to read the listing: the source, not cragbook, is at fault.
	ErrSource = errors.New("the source failed")
)

// Observer hears how each run ended: when, what it imported, and its error (nil when it went
// through). A run refused with ErrAlreadyRunning is no run, and is not heard. Metrics are one.
type Observer interface {
	Imported(at time.Time, sum Summary, err error)
}

// Config bounds an import: how many crags are fetched at once, and how long the whole may take.
type Config struct {
	Workers int
	Timeout time.Duration
}

var DefaultConfig = Config{Workers: 4, Timeout: 5 * time.Minute}

type Service struct {
	src     Source
	dst     climbing.CatalogWriter
	now     func() time.Time
	log     *slog.Logger
	cfg     Config
	obs     Observer
	running atomic.Bool
}

func NewService(src Source, dst climbing.CatalogWriter, now func() time.Time, log *slog.Logger, cfg Config, obs Observer) *Service {
	if cfg.Workers < 1 {
		cfg.Workers = 1
	}
	return &Service{src: src, dst: dst, now: now, log: log, cfg: cfg, obs: obs}
}

// Run imports every crag the source lists; one run at a time (ErrAlreadyRunning). A crag whose
// routes cannot be read or saved is left as it was — never saved without its routes, which
// would mark them gone — and listed in Failed; the rest go on. Only the listing failing, or ctx
// ending, fails the run.
func (s *Service) Run(ctx context.Context) (Summary, error) {
	if !s.running.CompareAndSwap(false, true) {
		return Summary{}, ErrAlreadyRunning
	}
	defer s.running.Store(false)
	sum, err := s.run(ctx)
	s.obs.Imported(s.now(), sum, err)
	return sum, err
}

func (s *Service) run(ctx context.Context) (Summary, error) {
	if s.cfg.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.cfg.Timeout)
		defer cancel()
	}
	listings, err := s.src.List(ctx)
	if err != nil {
		return Summary{}, fmt.Errorf("import: %w: %w", ErrSource, err)
	}
	at := s.now()
	jobs := make(chan Listing)
	var (
		mu  sync.Mutex
		sum = Summary{Failed: []string{}}
		wg  sync.WaitGroup
	)
	for range s.cfg.Workers {
		wg.Go(func() {
			for l := range jobs {
				n, err := s.one(ctx, l, at)
				mu.Lock()
				if err != nil {
					sum.Failed = append(sum.Failed, fmt.Sprintf("%s: %v", l.Crag.ID, err))
					s.log.Warn("crag not imported", "crag", l.Crag.ID, "err", err)
				} else {
					sum.Crags++
					sum.Routes += n
				}
				mu.Unlock()
			}
		})
	}
send:
	for _, l := range listings {
		select {
		case jobs <- l:
		case <-ctx.Done():
			break send
		}
	}
	close(jobs)
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return sum, fmt.Errorf("import: %w", err)
	}
	s.log.Info("import done", "crags", sum.Crags, "routes", sum.Routes, "failed", len(sum.Failed))
	return sum, nil
}

func (s *Service) one(ctx context.Context, l Listing, at time.Time) (int, error) {
	routes, err := s.src.Routes(ctx, l)
	if err != nil {
		return 0, err
	}
	for i := range routes {
		if routes[i].ID == "" {
			return 0, errors.New("a route without an id")
		}
		routes[i].CragID = l.Crag.ID
	}
	if err := s.dst.SaveCrag(ctx, l.Crag, routes, at); err != nil {
		return 0, err
	}
	return len(routes), nil
}
