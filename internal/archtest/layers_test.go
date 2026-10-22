// Package archtest keeps the layers' dependencies pointing inwards: domain ← application ←
// infrastructure and transport ← app. A wrong import fails here, before review has to see it.
package archtest

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

const module = "github.com/Allan-Nava/cragbook/internal/"

// What each layer may import from inside the module; anything else from it is a violation.
var allowed = map[string][]string{
	"domain":         {"domain"},
	"application":    {"domain", "application"},
	"infrastructure": {"domain", "application", "infrastructure"},
	"transport":      {"domain", "application"},
	"app":            {"domain", "application", "infrastructure", "transport"},
}

func TestLayersDependInwards(t *testing.T) {
	root := filepath.Join("..")
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		layer := strings.Split(filepath.ToSlash(rel), "/")[0]
		ok, known := allowed[layer]
		if !known {
			return nil
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range f.Imports {
			p := strings.Trim(imp.Path.Value, `"`)
			if !strings.HasPrefix(p, module) {
				continue
			}
			target := strings.Split(strings.TrimPrefix(p, module), "/")[0]
			if !contains(ok, target) {
				t.Errorf("%s (%s) imports %s (%s)", rel, layer, p, target)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestTheDomainImportsNoInfrastructureLibrary(t *testing.T) {
	err := filepath.WalkDir(filepath.Join("..", "domain"), func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range f.Imports {
			if p := strings.Trim(imp.Path.Value, `"`); strings.Contains(p, ".") && !strings.HasPrefix(p, "github.com/Allan-Nava/cragbook/") {
				t.Errorf("%s imports %s: the domain uses the standard library only", path, p)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
