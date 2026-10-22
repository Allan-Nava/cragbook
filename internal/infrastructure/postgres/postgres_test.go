package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Allan-Nava/cragbook/internal/domain/climbing"
	"github.com/Allan-Nava/cragbook/internal/domain/diary"
	"github.com/Allan-Nava/cragbook/internal/infrastructure/contract"
)

// Against a real PostgreSQL: CRAGBOOK_TEST_DATABASE_URL, which CI sets; skipped without it.
func open(t *testing.T) *Store {
	t.Helper()
	url := os.Getenv("CRAGBOOK_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("CRAGBOOK_TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	s, err := Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	for range 2 { // a second start applies nothing twice
		if err := s.Migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.pool.Exec(ctx, `TRUNCATE ascent, route, crag`); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestTheContract(t *testing.T) {
	contract.Run(t, func(t *testing.T) contract.Store { return open(t) })
}

// The checks the domain already makes, kept by the database too: a row written by hand or by a
// future bug cannot hold an entry the domain would refuse.
func TestTheSchemaRefusesWhatTheDomainRefuses(t *testing.T) {
	s, ctx := open(t), context.Background()
	if err := s.SaveCrag(ctx, climbing.Crag{ID: "c1", Name: "Test Wall"}, []climbing.Route{{ID: "r1"}}, time.Now()); err != nil {
		t.Fatal(err)
	}
	day := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	for name, a := range map[string]diary.Ascent{
		"a day on a todo": diary.Restore("r1", diary.Todo, &day, "", time.Now()),
		"unknown status":  diary.Restore("r1", "maybe", nil, "", time.Now()),
	} {
		if err := s.Save(ctx, a); err == nil {
			t.Errorf("%s: saved", name)
		}
	}
}
