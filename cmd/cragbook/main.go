// cragbook: a climbing diary on top of cragcast's crags and routes.
//
//	cragbook serve    the API (and the built frontend, with CRAGBOOK_WEB_DIR) on CRAGBOOK_ADDR
//	cragbook import   copy cragcast's crags and routes into the database, then exit
//
// Configuration is by environment (see README.md); the layers are wired by internal/app.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Allan-Nava/cragbook/internal/app"
	"github.com/Allan-Nava/cragbook/internal/application/catalogue"
)

func main() {
	cmd := "serve"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, cmd, app.ConfigFromEnv()); err != nil {
		fmt.Fprintf(os.Stderr, "cragbook %s: %v\n", cmd, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cmd string, cfg app.Config) error {
	if cmd != "serve" && cmd != "import" {
		return fmt.Errorf("unknown command %q: serve or import", cmd)
	}
	build := app.Build
	switch cfg.Store {
	case "postgres":
	case "memory":
		build = app.BuildInMemory
	default:
		return fmt.Errorf("CRAGBOOK_STORE %q: postgres or memory", cfg.Store)
	}
	a, cleanup, err := build(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()
	if cmd == "import" {
		sum, err := a.Importer.Run(ctx)
		if err != nil {
			return err
		}
		fmt.Printf("%d crags, %d routes imported; %d failed\n", sum.Crags, sum.Routes, len(sum.Failed))
		for _, f := range sum.Failed {
			fmt.Println("  " + f)
		}
		return nil
	}
	// An empty database gets its first import in the background, so the page has crags.
	if crags, err := a.Catalogue.Crags(ctx, catalogue.Filter{}); err == nil && len(crags) == 0 {
		go func() {
			if _, err := a.Importer.Run(ctx); err != nil {
				a.Log.Error("first import", "err", err)
			}
		}()
	}
	errc := make(chan error, 1)
	go func() {
		a.Log.Info("listening", "addr", cfg.Addr)
		errc <- a.HTTP.ListenAndServe()
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	a.Log.Info("shutting down")
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := a.HTTP.Shutdown(shutdown); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
