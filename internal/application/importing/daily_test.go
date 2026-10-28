package importing_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Allan-Nava/cragbook/internal/application/importing"
	"github.com/Allan-Nava/cragbook/internal/infrastructure/memory"
)

func TestTimeOfDay(t *testing.T) {
	rome, err := time.LoadLocation("Europe/Rome")
	if err != nil {
		t.Fatal(err)
	}
	at := importing.TimeOfDay{Hour: 6, Minute: 30}
	for _, c := range []struct{ now, want string }{
		{"2026-10-28T05:00:00", "2026-10-28T06:30:00+01:00"}, // still to come today
		{"2026-10-28T06:30:00", "2026-10-29T06:30:00+01:00"}, // now is not after now
		{"2026-10-28T23:59:00", "2026-10-29T06:30:00+01:00"},
		{"2026-12-31T07:00:00", "2027-01-01T06:30:00+01:00"},
		{"2026-03-28T07:00:00", "2026-03-29T06:30:00+02:00"}, // the night clocks go forward
		{"2026-10-24T07:00:00", "2026-10-25T06:30:00+01:00"}, // and back
	} {
		now, _ := time.ParseInLocation("2006-01-02T15:04:05", c.now, rome)
		if got := at.Next(now).Format(time.RFC3339); got != c.want {
			t.Errorf("after %s: %s, want %s", c.now, got, c.want)
		}
	}
	if got, err := importing.ParseTimeOfDay("04:05"); err != nil || got.String() != "04:05" {
		t.Errorf("04:05: %v %v", got, err)
	}
	for _, bad := range []string{"", "6", "24:00", "06:60", "6.30", "06:30:00"} {
		if _, err := importing.ParseTimeOfDay(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

// clock is a time the test moves, and a wait it releases.
type clock struct {
	mu    sync.Mutex
	now   time.Time
	waits chan time.Duration
	fire  chan time.Time
}

func (c *clock) Now() time.Time  { c.mu.Lock(); defer c.mu.Unlock(); return c.now }
func (c *clock) set(t time.Time) { c.mu.Lock(); c.now = t; c.mu.Unlock() }
func (c *clock) wait(d time.Duration) <-chan time.Time {
	c.waits <- d
	return c.fire
}

func TestDailyRunsAtItsHourEveryDayUntilStopped(t *testing.T) {
	start := time.Date(2026, 10, 28, 5, 0, 0, 0, time.UTC)
	c := &clock{now: start, waits: make(chan time.Duration), fire: make(chan time.Time)}
	h := &heard{}
	var logs strings.Builder
	var logMu sync.Mutex
	log := newLogger(&logMu, &logs)
	src := &source{n: 1}
	s := importing.NewService(src, memory.New(), c.Now, log, importing.DefaultConfig, h)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.Daily(ctx, importing.TimeOfDay{Hour: 6, Minute: 30}, c.wait); close(done) }()

	if d := <-c.waits; d != 90*time.Minute {
		t.Fatalf("first wait %s, want 1h30m", d)
	}
	c.set(start.Add(90*time.Minute - time.Second)) // the wall clock a second behind the timer
	c.fire <- c.Now()
	if d := <-c.waits; d != 24*time.Hour+time.Second {
		t.Fatalf("second wait %s: today's run was run again", d)
	}
	src.listErr = errors.New("crags.json: 503")
	c.set(start.Add(90*time.Minute + 24*time.Hour))
	c.fire <- c.Now()
	if d := <-c.waits; d != 24*time.Hour {
		t.Fatalf("after a failed run, wait %s, want a day", d)
	}
	cancel()
	<-done
	if len(h.runs) != 2 || !strings.Contains(h.runs[1], "503") {
		t.Fatalf("runs %v", h.runs)
	}
	logMu.Lock()
	defer logMu.Unlock()
	if !strings.Contains(logs.String(), `level=ERROR msg="daily import"`) || !strings.Contains(logs.String(), "next import") {
		t.Fatalf("logs:\n%s", logs.String())
	}
}
