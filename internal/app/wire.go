//go:build wireinject

package app

import (
	"context"

	"github.com/google/wire"

	"github.com/Allan-Nava/cragbook/internal/application/catalogue"
	"github.com/Allan-Nava/cragbook/internal/application/importing"
	"github.com/Allan-Nava/cragbook/internal/application/journal"
	"github.com/Allan-Nava/cragbook/internal/domain/climbing"
	"github.com/Allan-Nava/cragbook/internal/domain/diary"
	"github.com/Allan-Nava/cragbook/internal/infrastructure/cragcast"
	"github.com/Allan-Nava/cragbook/internal/infrastructure/memory"
	"github.com/Allan-Nava/cragbook/internal/infrastructure/postgres"
	"github.com/Allan-Nava/cragbook/internal/transport/httpapi"
)

// Infrastructure: the adapters, and the domain and application ports each one implements.
var Infrastructure = wire.NewSet(
	NewPostgres,
	wire.Bind(new(climbing.Catalog), new(*postgres.Store)),
	wire.Bind(new(climbing.CatalogWriter), new(*postgres.Store)),
	wire.Bind(new(diary.Repository), new(*postgres.Store)),
	wire.Bind(new(catalogue.Summaries), new(*postgres.Store)),
	NewCragcast,
	wire.Bind(new(importing.Source), new(*cragcast.Client)),
)

// InMemory is Infrastructure with the memory adapter in PostgreSQL's place: the same ports
// bound to another type, and nothing else in the graph changes.
var InMemory = wire.NewSet(
	memory.New,
	wire.Bind(new(climbing.Catalog), new(*memory.Store)),
	wire.Bind(new(climbing.CatalogWriter), new(*memory.Store)),
	wire.Bind(new(diary.Repository), new(*memory.Store)),
	wire.Bind(new(catalogue.Summaries), new(*memory.Store)),
	NewCragcast,
	wire.Bind(new(importing.Source), new(*cragcast.Client)),
)

// Application: the use cases, which see only ports.
var Application = wire.NewSet(
	NewClock, NewImportConfig,
	catalogue.NewService,
	journal.NewService,
	importing.NewService,
)

// Transport: the HTTP API, which sees the use cases through its own small interfaces (Wire wants
// each binding in a set that provides its concrete type, so Application is part of this one).
var Transport = wire.NewSet(
	Application,
	httpapi.New,
	wire.Bind(new(httpapi.Catalogue), new(*catalogue.Service)),
	wire.Bind(new(httpapi.Journal), new(*journal.Service)),
	wire.Bind(new(httpapi.Importer), new(*importing.Service)),
	NewHTTPServer,
)

// Build makes the App from cfg; the cleanup closes the database.
func Build(ctx context.Context, cfg Config) (*App, func(), error) {
	wire.Build(Infrastructure, Transport, NewLogger, wire.Struct(new(App), "*"))
	return nil, nil, nil
}

// BuildInMemory makes the App on the memory adapter: CRAGBOOK_STORE=memory, to try cragbook
// without a database. The diary is lost when the process ends.
func BuildInMemory(ctx context.Context, cfg Config) (*App, func(), error) {
	wire.Build(InMemory, Transport, NewLogger, wire.Struct(new(App), "*"))
	return nil, nil, nil
}
