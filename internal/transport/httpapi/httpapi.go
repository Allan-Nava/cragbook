// Package httpapi is cragbook's HTTP API over the use cases: JSON in and out, errors as
// {"error": "..."}. It holds no rule of its own — it decodes, calls a use case, and maps the
// domain's errors to statuses in one place (status below).
package httpapi

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/Allan-Nava/cragbook/internal/application/catalogue"
	"github.com/Allan-Nava/cragbook/internal/application/importing"
	"github.com/Allan-Nava/cragbook/internal/application/journal"
	"github.com/Allan-Nava/cragbook/internal/domain/climbing"
	"github.com/Allan-Nava/cragbook/internal/domain/diary"
)

// The use cases, as this package needs them: one small interface each.
type (
	Catalogue interface {
		Crags(ctx context.Context, f catalogue.Filter) ([]catalogue.CragSummary, error)
		Crag(ctx context.Context, id climbing.CragID) (catalogue.CragDetail, error)
	}
	Journal interface {
		Record(ctx context.Context, cmd journal.Record) (climbing.Route, diary.Ascent, error)
		Clear(ctx context.Context, id climbing.RouteID) error
		Entries(ctx context.Context) ([]journal.Entry, error)
	}
	Importer interface {
		Run(ctx context.Context) (importing.Summary, error)
	}
)

// Version is the build's version, set at link time (dev for a local build).
type Version string

type Handler struct {
	version   Version
	catalogue Catalogue
	journal   Journal
	importer  Importer
	log       *slog.Logger
	openAPI   json.RawMessage
}

// openAPIDoc describes every endpoint below; a test holds it to what the handlers answer.
//
//go:embed openapi.json
var openAPIDoc []byte

func New(v Version, c Catalogue, j Journal, i Importer, log *slog.Logger) *Handler {
	return &Handler{version: v, catalogue: c, journal: j, importer: i, log: log, openAPI: withVersion(openAPIDoc, v)}
}

// withVersion is the document with info.version set to the build's.
func withVersion(doc []byte, v Version) json.RawMessage {
	var d map[string]any
	if err := json.Unmarshal(doc, &d); err != nil {
		panic("httpapi: openapi.json: " + err.Error())
	}
	d["info"].(map[string]any)["version"] = string(v)
	out, _ := json.Marshal(d)
	return out
}

type endpoint struct {
	pattern string
	handle  http.HandlerFunc
}

// endpoints are what the mux serves, apart from the page.
func (h *Handler) endpoints() []endpoint {
	return []endpoint{
		{"GET /healthz", h.healthz},
		{"GET /api/openapi.json", h.openAPIJSON},
		{"GET /api/crags", h.crags},
		{"GET /api/crags/{id}", h.crag},
		{"PUT /api/routes/{id}/ascent", h.record},
		{"DELETE /api/routes/{id}/ascent", h.clear},
		{"GET /api/diary", h.diary},
		{"POST /api/import", h.importNow},
	}
}

// Routes serves the API under /api/, /healthz, and, when web is not nil, everything else.
func (h *Handler) Routes(web http.Handler) http.Handler {
	mux := http.NewServeMux()
	for _, e := range h.endpoints() {
		mux.HandleFunc(e.pattern, e.handle)
	}
	if web != nil {
		mux.Handle("/", web)
	}
	return mux
}

// healthz answers while the process serves, with the version it runs.
func (h *Handler) healthz(w http.ResponseWriter, r *http.Request) {
	reply(w, http.StatusOK, healthDTO{Status: "ok", Version: string(h.version)})
}

func (h *Handler) openAPIJSON(w http.ResponseWriter, r *http.Request) {
	reply(w, http.StatusOK, h.openAPI)
}

func (h *Handler) crags(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	list, err := h.catalogue.Crags(r.Context(), catalogue.Filter{Query: q.Get("q"), WithRoutes: q.Get("withRoutes") == "true"})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]cragDTO, 0, len(list))
	for _, c := range list {
		out = append(out, toCrag(c))
	}
	reply(w, http.StatusOK, out)
}

func (h *Handler) crag(w http.ResponseWriter, r *http.Request) {
	d, err := h.catalogue.Crag(r.Context(), climbing.CragID(r.PathValue("id")))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	reply(w, http.StatusOK, toDetail(d))
}

func (h *Handler) record(w http.ResponseWriter, r *http.Request) {
	var in ascentIn
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		reply(w, http.StatusBadRequest, errorDTO{"body: " + err.Error()})
		return
	}
	cmd := journal.Record{Route: climbing.RouteID(r.PathValue("id")), Status: in.Status, Notes: in.Notes}
	if in.ClimbedOn != nil && *in.ClimbedOn != "" {
		day, err := time.Parse(time.DateOnly, *in.ClimbedOn)
		if err != nil {
			reply(w, http.StatusBadRequest, errorDTO{"climbedOn: a date as YYYY-MM-DD"})
			return
		}
		cmd.ClimbedOn = &day
	}
	route, a, err := h.journal.Record(r.Context(), cmd)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	reply(w, http.StatusOK, toRoute(route, &a))
}

func (h *Handler) clear(w http.ResponseWriter, r *http.Request) {
	if err := h.journal.Clear(r.Context(), climbing.RouteID(r.PathValue("id"))); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) diary(w http.ResponseWriter, r *http.Request) {
	entries, err := h.journal.Entries(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]diaryEntryDTO, 0, len(entries))
	for _, e := range entries {
		a := e.Ascent
		out = append(out, diaryEntryDTO{routeDTO: toRoute(e.Route, &a), CragID: string(e.Crag.ID), CragName: e.Crag.Name})
	}
	reply(w, http.StatusOK, out)
}

// importNow runs an import while the request waits.
func (h *Handler) importNow(w http.ResponseWriter, r *http.Request) {
	sum, err := h.importer.Run(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	reply(w, http.StatusOK, summaryDTO{Crags: sum.Crags, Routes: sum.Routes, Failed: sum.Failed})
}

// status maps an error to the status it answers with, and whether its text may be shown.
func status(err error) (int, bool) {
	switch {
	case errors.Is(err, climbing.ErrCragNotFound), errors.Is(err, climbing.ErrRouteNotFound):
		return http.StatusNotFound, true
	case errors.Is(err, diary.ErrInvalid), errors.Is(err, catalogue.ErrInvalidQuery):
		return http.StatusBadRequest, true
	case errors.Is(err, importing.ErrAlreadyRunning):
		return http.StatusConflict, true
	case errors.Is(err, importing.ErrSource):
		return http.StatusBadGateway, true
	}
	return http.StatusInternalServerError, false
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	code, public := status(err)
	if !public {
		h.log.Error("request", "method", r.Method, "path", r.URL.Path, "err", err)
		reply(w, code, errorDTO{"internal error"})
		return
	}
	reply(w, code, errorDTO{err.Error()})
}

func reply(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
