// Package app wires cragbook's layers with Google Wire (https://github.com/google/wire): the
// providers below build the adapters, wire.go binds each port the use cases and the transport
// ask for to the adapter that implements it, and `go generate ./internal/app` writes
// wire_gen.go, the constructor main calls. A missing or ambiguous dependency fails generation,
// not a running server.
package app

//go:generate go tool wire

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/Allan-Nava/cragbook/internal/application/catalogue"
	"github.com/Allan-Nava/cragbook/internal/application/importing"
	"github.com/Allan-Nava/cragbook/internal/infrastructure/cragcast"
	"github.com/Allan-Nava/cragbook/internal/infrastructure/metrics"
	"github.com/Allan-Nava/cragbook/internal/infrastructure/postgres"
	"github.com/Allan-Nava/cragbook/internal/transport/httpapi"
)

// Config is read from the environment; see README.md.
type Config struct {
	DatabaseURL string
	Addr        string
	CragcastURL string
	WebDir      string
	Store       string // "postgres" (the default) or "memory"
	ImportAt    string // the daily import's time of day, HH:MM in TZ's zone, or "off"
	Version     string // the build's, from main
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func ConfigFromEnv() Config {
	return Config{
		DatabaseURL: env("CRAGBOOK_DATABASE_URL", "postgres://cragbook:cragbook@localhost:5432/cragbook?sslmode=disable"),
		Addr:        env("CRAGBOOK_ADDR", "localhost:8080"),
		CragcastURL: env("CRAGBOOK_CRAGCAST_URL", cragcast.DefaultBase),
		WebDir:      os.Getenv("CRAGBOOK_WEB_DIR"),
		Store:       env("CRAGBOOK_STORE", "postgres"),
		ImportAt:    env("CRAGBOOK_IMPORT_AT", "06:30"),
	}
}

// App is what main runs.
type App struct {
	Config    Config
	Catalogue *catalogue.Service
	Importer  *importing.Service
	HTTP      *http.Server
	Log       *slog.Logger
}

// NewVersion is the version /healthz reports.
func NewVersion(cfg Config) httpapi.Version { return httpapi.Version(cfg.Version) }

func NewLogger() *slog.Logger { return slog.New(slog.NewTextHandler(os.Stderr, nil)) }

// NewClock is the time every use case reads.
func NewClock() func() time.Time { return time.Now }

func NewImportConfig() importing.Config { return importing.DefaultConfig }

func NewCragcast(cfg Config) (*cragcast.Client, error) {
	return cragcast.New(cfg.CragcastURL, 20*time.Second)
}

// NewMetrics records the requests and the imports for /metrics, labelled with the version.
func NewMetrics(cfg Config) *metrics.Metrics { return metrics.New(cfg.Version) }

// NewPostgres opens the database and applies the migrations; the cleanup closes it.
func NewPostgres(ctx context.Context, cfg Config) (*postgres.Store, func(), error) {
	db, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, nil, err
	}
	if err := db.Migrate(ctx); err != nil {
		db.Close()
		return nil, nil, err
	}
	return db, db.Close, nil
}

// NewHTTPServer serves the API, /metrics, and the built frontend when Config.WebDir names it.
func NewHTTPServer(cfg Config, h *httpapi.Handler, m *metrics.Metrics) *http.Server {
	var web http.Handler
	if cfg.WebDir != "" {
		web = http.FileServer(http.Dir(cfg.WebDir))
	}
	return &http.Server{
		Addr:              cfg.Addr,
		Handler:           h.Routes(web, m.Handler()),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      6 * time.Minute, // an import answers when it is done
		IdleTimeout:       time.Minute,
	}
}
