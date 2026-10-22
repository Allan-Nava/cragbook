package climbing

import (
	"slices"
	"testing"
)

func TestGradesRankAsClimbersReadThem(t *testing.T) {
	in := []string{"7a", "6a+", "5c", "6c+", "4", "6b", "8a", "6a", "6c"}
	want := []string{"4", "5c", "6a", "6a+", "6b", "6c", "6c+", "7a", "8a"}
	slices.SortFunc(in, func(a, b string) int {
		ga, _ := ParseGrade(a)
		gb, _ := ParseGrade(b)
		return ga.Rank() - gb.Rank()
	})
	if !slices.Equal(in, want) {
		t.Fatalf("sorted %v, want %v", in, want)
	}
	for s, want := range map[string]int{"6a": 36, "6c+": 41, "7a": 42, "6": 36} {
		if g, ok := ParseGrade(s); !ok || g.Rank() != want {
			t.Errorf("%q: rank %d, %v; want %d", s, g.Rank(), ok, want)
		}
	}
	if g, _ := ParseGrade(" 6B+ "); g.String() != "6b+" {
		t.Errorf("normalised to %q", g)
	}
}

func TestWhatIsNoFrenchGrade(t *testing.T) {
	for _, s := range []string{"", "V+", "6d", "10a", "6a++", "VI"} {
		if _, ok := ParseGrade(s); ok {
			t.Errorf("%q parsed", s)
		}
	}
}

func TestLevels(t *testing.T) {
	for s, want := range map[string]Level{"3b": LevelEasy, "4+": LevelEasy, "5a": Level5, "5c+": Level5, "6b": Level6,
		"7c+": Level7, "8a": LevelExtreme, "9a": LevelExtreme} {
		g, _ := ParseGrade(s)
		if got := (Route{Grade: &g}).Level(); got != want {
			t.Errorf("%q: level %q, want %q", s, got, want)
		}
	}
	if (Route{}).Level() != LevelNone {
		t.Error("an ungraded route has a level")
	}
}

func ptr[T any](v T) *T { return &v }

func TestRoutesSortEasiestFirstTheUngradedLast(t *testing.T) {
	g := func(s string) *Grade { v, _ := ParseGrade(s); return &v }
	routes := []Route{
		{ID: "e", Name: ptr("Senza grado")},
		{ID: "d", Name: ptr("zeta"), Grade: g("6a")},
		{ID: "c", Name: ptr("Alfa"), Grade: g("6a")},
		{ID: "b", Grade: g("6a")},
		{ID: "a", Name: ptr("Facile"), Grade: g("4")},
	}
	SortRoutes(routes)
	var ids []string
	for _, r := range routes {
		ids = append(ids, string(r.ID))
	}
	if !slices.Equal(ids, []string{"a", "c", "d", "b", "e"}) {
		t.Fatalf("order %v", ids)
	}
}

func TestOSMURL(t *testing.T) {
	for id, want := range map[string]string{
		"osm-node-2": "https://www.openstreetmap.org/node/2", "osm-relation-11740994": "https://www.openstreetmap.org/relation/11740994",
		"osm-area-3": "", "node-2": "", "osm-way-": "", "": "",
	} {
		if got := OSMURL(id); got != want {
			t.Errorf("OSMURL(%q) = %q, want %q", id, got, want)
		}
	}
}
