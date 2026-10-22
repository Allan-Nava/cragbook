package climbing

import (
	"regexp"
	"strconv"
	"strings"
)

var french = regexp.MustCompile(`^([1-9])([abc]?)(\+?)$`)

// Grade is a French climbing grade as OpenStreetMap gives it — 4, 5c, 6b+, 8a — a value object:
// two grades with the same text are the same grade.
type Grade struct {
	text string
	rank int
}

// ParseGrade reads a French grade, in any case and with spaces around it; ok is false for
// anything else (a UIAA "V+", an "6d").
func ParseGrade(s string) (g Grade, ok bool) {
	t := strings.ToLower(strings.TrimSpace(s))
	m := french.FindStringSubmatch(t)
	if m == nil {
		return Grade{}, false
	}
	n, _ := strconv.Atoi(m[1])
	letter, plus := 0, 0
	if m[2] != "" {
		letter = int(m[2][0] - 'a')
	}
	if m[3] != "" {
		plus = 1
	}
	return Grade{text: t, rank: n*6 + letter*2 + plus}, true
}

func (g Grade) String() string { return g.text }

// Rank orders grades as climbers read them: six steps a number, so 6a = 36, 6a+ = 37, 6b = 38 …
// 6c+ = 41, 7a = 42. A bare number counts as its "a" (6 = 6a).
func (g Grade) Rank() int { return g.rank }

// Level is the grade's number: how a crag's routes are grouped to be read at a glance.
type Level string

const (
	LevelEasy    Level = "4" // 4 and under
	Level5       Level = "5"
	Level6       Level = "6"
	Level7       Level = "7"
	LevelExtreme Level = "8" // 8 and over
	LevelNone    Level = ""  // no grade
)

func (g Grade) Level() Level {
	switch n := g.rank / 6; {
	case n <= 4:
		return LevelEasy
	case n >= 8:
		return LevelExtreme
	default:
		return Level(strconv.Itoa(n))
	}
}
