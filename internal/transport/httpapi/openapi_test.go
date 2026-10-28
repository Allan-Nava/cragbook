package httpapi_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/Allan-Nava/cragbook/internal/transport/httpapi"
)

// openapi.json held to the handlers: every endpoint the mux serves is in it and nothing else,
// every response example is what the handler answers to the request in its x-request, and every
// answer fits its schema, with no field the schema does not name.

func readDoc(t *testing.T) map[string]any {
	t.Helper()
	b, err := os.ReadFile("openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("openapi.json: %v", err)
	}
	return doc
}

var methods = []string{"get", "put", "post", "delete", "patch"}

func TestOpenAPIDescribesEveryEndpoint(t *testing.T) {
	var documented []string
	for path, item := range obj(readDoc(t)["paths"]) {
		for _, m := range methods {
			if obj(item)[m] != nil {
				documented = append(documented, strings.ToUpper(m)+" "+path)
			}
		}
	}
	served := httpapi.Patterns()
	slices.Sort(documented)
	slices.Sort(served)
	if !slices.Equal(documented, served) {
		t.Fatalf("openapi.json documents\n  %v\nthe mux serves\n  %v", documented, served)
	}
}

// The examples' world: setup's crags, with one route already climbed.
func world(t *testing.T) http.Handler {
	h := setup(t, nil)
	if code, v := do(t, h, "PUT", "/api/routes/osm-node-2/ascent", `{"status":"done","climbedOn":"2026-10-18","notes":"a vista"}`); code != 200 {
		t.Fatalf("the examples' world: %d %v", code, v)
	}
	return h
}

func TestOpenAPIExamplesAreWhatTheHandlersAnswer(t *testing.T) {
	doc := readDoc(t)
	bodies, used, replayed := map[string]any{}, map[string]bool{}, 0
	for path, item := range obj(doc["paths"]) {
		for _, m := range methods {
			op := obj(obj(item)[m])
			if op == nil {
				continue
			}
			where := strings.ToUpper(m) + " " + path
			in := obj(obj(obj(op["requestBody"])["content"])["application/json"])
			for name, ex := range obj(in["examples"]) {
				bodies[name] = obj(ex)["value"]
				if err := fits(doc, in["schema"], obj(ex)["value"], "body"); err != nil {
					t.Errorf("%s: request example %s: %v", where, name, err)
				}
			}
			for code, resp := range obj(op["responses"]) {
				out := obj(obj(obj(resp)["content"])["application/json"])
				for name, ex := range obj(out["examples"]) {
					ex, at := obj(ex), fmt.Sprintf("%s %s example %s", where, code, name)
					req := obj(ex["x-request"])
					target, _ := req["path"].(string)
					if target == "" {
						t.Errorf("%s: no x-request.path to replay", at)
						continue
					}
					var body []byte
					if b, ok := req["body"].(string); ok {
						v, ok := bodies[b]
						if !ok {
							t.Errorf("%s: no request example %q", at, b)
							continue
						}
						used[b] = true
						body, _ = json.Marshal(v)
					}
					rec := httptest.NewRecorder()
					world(t).ServeHTTP(rec, httptest.NewRequest(strings.ToUpper(m), target, bytes.NewReader(body)))
					var got any
					json.Unmarshal(rec.Body.Bytes(), &got)
					replayed++
					if strconv.Itoa(rec.Code) != code || !reflect.DeepEqual(got, ex["value"]) {
						t.Errorf("%s: the handler answers %d\n%s", at, rec.Code, rec.Body.Bytes())
						continue
					}
					if err := fits(doc, out["schema"], got, "response"); err != nil {
						t.Errorf("%s: %v", at, err)
					}
				}
			}
		}
	}
	for _, name := range slices.Sorted(maps.Keys(bodies)) {
		if !used[name] {
			t.Errorf("request example %s is replayed by no response example", name)
		}
	}
	if replayed == 0 {
		t.Fatal("no example replayed")
	}
}

func TestOpenAPIIsServedWithTheBuildsVersion(t *testing.T) {
	code, v := do(t, setup(t, nil), "GET", "/api/openapi.json", "")
	if code != 200 || obj(v)["openapi"] != "3.1.0" || obj(obj(v)["info"])["version"] != version {
		t.Fatalf("%d %v", code, obj(v)["info"])
	}
	if len(obj(obj(v)["paths"])) != len(obj(readDoc(t)["paths"])) {
		t.Fatalf("served without its paths")
	}
}

// fits checks v against the part of JSON Schema the document uses: $ref, allOf, oneOf, type,
// enum, properties, required, items, maxLength. An object's fields must all be named by its
// schema (through allOf), so a field the handlers add and the document misses fails here.
func fits(doc map[string]any, schema, v any, at string) error {
	s := resolve(doc, schema)
	for _, sub := range list(s["allOf"]) {
		if err := fitsShape(doc, sub, v, at); err != nil {
			return err
		}
	}
	if err := fitsShape(doc, s, v, at); err != nil {
		return err
	}
	if fields, ok := v.(map[string]any); ok && len(properties(doc, s)) > 0 {
		named := properties(doc, s)
		for k := range fields {
			if _, ok := named[k]; !ok {
				return fmt.Errorf("%s.%s: not in the schema", at, k)
			}
		}
	}
	return nil
}

func fitsShape(doc map[string]any, schema, v any, at string) error {
	s := resolve(doc, schema)
	if alts := list(s["oneOf"]); alts != nil {
		matched := 0
		for _, alt := range alts {
			if fits(doc, alt, v, at) == nil {
				matched++
			}
		}
		if matched != 1 {
			return fmt.Errorf("%s: %v fits %d of oneOf, not 1", at, v, matched)
		}
	}
	if t, ok := s["type"]; ok {
		types := list(t)
		if types == nil {
			types = []any{t}
		}
		if !slices.ContainsFunc(types, func(t any) bool { return isType(v, t.(string)) }) {
			return fmt.Errorf("%s: %v is not %v", at, v, t)
		}
	}
	if enum := list(s["enum"]); enum != nil && !slices.Contains(enum, v) {
		return fmt.Errorf("%s: %v is not one of %v", at, v, enum)
	}
	if max, ok := s["maxLength"].(float64); ok {
		if str, _ := v.(string); len([]rune(str)) > int(max) {
			return fmt.Errorf("%s: longer than %v", at, max)
		}
	}
	if fields, ok := v.(map[string]any); ok {
		for _, r := range list(s["required"]) {
			if _, ok := fields[r.(string)]; !ok {
				return fmt.Errorf("%s.%s: required, missing", at, r)
			}
		}
		for k, p := range obj(s["properties"]) {
			if f, ok := fields[k]; ok {
				if err := fits(doc, p, f, at+"."+k); err != nil {
					return err
				}
			}
		}
	}
	if items, ok := v.([]any); ok && s["items"] != nil {
		for i, item := range items {
			if err := fits(doc, s["items"], item, fmt.Sprintf("%s[%d]", at, i)); err != nil {
				return err
			}
		}
	}
	return nil
}

func resolve(doc map[string]any, schema any) map[string]any {
	s := obj(schema)
	for s["$ref"] != nil {
		var at any = doc
		for _, part := range strings.Split(strings.TrimPrefix(s["$ref"].(string), "#/"), "/") {
			at = obj(at)[part]
		}
		s = obj(at)
	}
	return s
}

// properties are the fields a schema names, its allOf's included.
func properties(doc map[string]any, schema any) map[string]any {
	s := resolve(doc, schema)
	out := maps.Clone(obj(s["properties"]))
	if out == nil {
		out = map[string]any{}
	}
	for _, sub := range list(s["allOf"]) {
		maps.Copy(out, properties(doc, sub))
	}
	return out
}

func isType(v any, t string) bool {
	switch t {
	case "null":
		return v == nil
	case "string":
		_, ok := v.(string)
		return ok
	case "boolean":
		_, ok := v.(bool)
		return ok
	case "number":
		_, ok := v.(float64)
		return ok
	case "integer":
		n, ok := v.(float64)
		return ok && n == float64(int64(n))
	case "array":
		_, ok := v.([]any)
		return ok
	case "object":
		_, ok := v.(map[string]any)
		return ok
	}
	return false
}
