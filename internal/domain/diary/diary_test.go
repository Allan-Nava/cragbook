package diary

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
)

var now = time.Date(2026, 10, 23, 15, 30, 0, 0, time.UTC)

func day(s string) *time.Time { d, _ := time.Parse(time.DateOnly, s); return &d }

func TestAClimbedRouteKeepsItsDayAndATrimmedNote(t *testing.T) {
	a, err := NewAscent("osm-node-1", Done, day("2026-10-20"), "  a vista  ", now)
	if err != nil {
		t.Fatal(err)
	}
	if a.Status() != Done || a.ClimbedOn().Format(time.DateOnly) != "2026-10-20" || a.Notes() != "a vista" || !a.UpdatedAt().Equal(now) {
		t.Fatalf("%+v", a)
	}
	if _, err := NewAscent("osm-node-1", Done, day("2026-10-23"), "", now); err != nil {
		t.Fatalf("today refused: %v", err)
	}
}

func TestWhatADiaryEntryRefuses(t *testing.T) {
	cases := map[string]func() error{
		"after today":        func() error { _, err := NewAscent("r", Done, day("2026-10-24"), "", now); return err },
		"a day on a project": func() error { _, err := NewAscent("r", Project, day("2026-10-01"), "", now); return err },
		"a day on a todo":    func() error { _, err := NewAscent("r", Todo, day("2026-10-01"), "", now); return err },
		"unknown status":     func() error { _, err := NewAscent("r", Status("maybe"), nil, "", now); return err },
		"no route":           func() error { _, err := NewAscent("", Done, nil, "", now); return err },
		"long notes": func() error {
			_, err := NewAscent("r", Done, nil, strings.Repeat("é", MaxNotes+1), now)
			return err
		},
	}
	for name, f := range cases {
		if err := f(); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: %v, want ErrInvalid", name, err)
		}
	}
	if _, err := NewAscent("r", Done, nil, strings.Repeat("é", MaxNotes), now); err != nil {
		t.Errorf("MaxNotes characters refused: %v", err)
	}
}

func TestParseStatus(t *testing.T) {
	for _, s := range []string{"done", "project", "todo"} {
		if st, err := ParseStatus(s); err != nil || string(st) != s {
			t.Errorf("%q: %v", s, err)
		}
	}
	if _, err := ParseStatus("Done"); !errors.Is(err, ErrInvalid) {
		t.Errorf("case: %v", err)
	}
}

func TestTheDiaryOrder(t *testing.T) {
	at := func(h int) time.Time { return now.Add(time.Duration(-h) * time.Hour) }
	entries := []Ascent{
		Restore("todo", Todo, nil, "", at(0)),
		Restore("project-old", Project, nil, "", at(5)),
		Restore("project-new", Project, nil, "", at(1)),
		Restore("done-undated", Done, nil, "", at(0)),
		Restore("done-sept", Done, day("2026-09-01"), "", at(0)),
		Restore("done-oct", Done, day("2026-10-01"), "", at(9)),
	}
	slices.SortFunc(entries, Compare)
	var got []string
	for _, a := range entries {
		got = append(got, string(a.Route()))
	}
	want := []string{"done-oct", "done-sept", "done-undated", "project-new", "project-old", "todo"}
	if !slices.Equal(got, want) {
		t.Fatalf("order %v, want %v", got, want)
	}
}
