// Package cragcast reads cragcast's static API v1 (https://github.com/Allan-Nava/cragcast#api):
// an index of the crags, then one file per crag with its sectors and single routes, all from
// OpenStreetMap (© OpenStreetMap contributors, ODbL 1.0). The types below are the API's own;
// adapter.go turns them into the domain's.
package cragcast

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultBase is the API as GitHub Pages serves it, rebuilt every hour.
const DefaultBase = "https://allan-nava.github.io/cragcast/api/v1/"

// Index is crags.json: the credits and every crag with the path of its own file.
type Index struct {
	Meta  Meta         `json:"meta"`
	Crags []IndexEntry `json:"crags"`
}

type Meta struct {
	Version     int    `json:"version"`
	GeneratedAt string `json:"generatedAt"`
	CragData    string `json:"cragData"`
}

type IndexEntry struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Municipality *string `json:"municipality"`
	ProvinceCode *string `json:"provinceCode"`
	Latitude     float64 `json:"latitude"`
	Longitude    float64 `json:"longitude"`
	Aspect       *string `json:"aspect"`
	Elevation    *int    `json:"elevation"`
	URL          string  `json:"url"`
}

// Crag is the part of crags/<id>.json cragbook keeps.
type Crag struct {
	ID      string   `json:"id"`
	Sectors []Sector `json:"sectors"`
	Routes  []Route  `json:"routes"`
}

type Sector struct {
	ID   string  `json:"id"`
	Name *string `json:"name"`
}

type Route struct {
	ID      string   `json:"id"`
	Name    *string  `json:"name"`
	Grade   *string  `json:"grade"`
	Length  *float64 `json:"length"`
	Bolts   *int     `json:"bolts"`
	Pitches *int     `json:"pitches"`
	Sector  *string  `json:"sector"`
}

// Client fetches the API. Every request carries the caller's context and the client's timeout,
// so a stalled server never holds an import open.
type Client struct {
	base *url.URL
	http *http.Client
}

func New(base string, timeout time.Duration) (*Client, error) {
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, fmt.Errorf("cragcast API base %q: not an http(s) URL", base)
	}
	return &Client{base: u, http: &http.Client{Timeout: timeout}}, nil
}

func (c *Client) Index(ctx context.Context) (Index, error) {
	var idx Index
	if err := c.get(ctx, "crags.json", &idx); err != nil {
		return Index{}, err
	}
	if idx.Meta.Version != 1 {
		return Index{}, fmt.Errorf("crags.json: API version %d, cragbook reads version 1", idx.Meta.Version)
	}
	return idx, nil
}

// Crag fetches one crag's file, by the path the index gives it (crags/<id>.json).
func (c *Client) Crag(ctx context.Context, path string) (Crag, error) {
	var doc struct {
		Crag Crag `json:"crag"`
	}
	if err := c.get(ctx, path, &doc); err != nil {
		return Crag{}, err
	}
	return doc.Crag, nil
}

func (c *Client) get(ctx context.Context, path string, into any) error {
	ref, err := url.Parse(path)
	if err != nil || ref.IsAbs() || strings.HasPrefix(path, "/") {
		return fmt.Errorf("%q: not a path inside the API", path)
	}
	u := c.base.ResolveReference(ref)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return fmt.Errorf("GET %s: %w", u, err)
	}
	req.Header.Set("User-Agent", "cragbook (github.com/Allan-Nava/cragbook)")
	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", u, err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 64<<10)) // drained to reuse the connection, at best
		return fmt.Errorf("GET %s: %s", u, res.Status)
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 16<<20)).Decode(into); err != nil {
		return fmt.Errorf("GET %s: decoding: %w", u, err)
	}
	return nil
}
