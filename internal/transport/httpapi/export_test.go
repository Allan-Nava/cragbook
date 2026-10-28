package httpapi

// Patterns are the mux's patterns, for the test that holds openapi.json to them.
func Patterns() []string {
	var out []string
	for _, e := range (&Handler{}).endpoints() {
		out = append(out, e.pattern)
	}
	return out
}
