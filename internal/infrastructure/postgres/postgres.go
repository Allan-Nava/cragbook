// Package postgres keeps the catalogue and the diary in PostgreSQL. The schema is in
// migrations/, applied in name order and once each by Migrate.
package postgres

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Allan-Nava/cragbook/internal/application/catalogue"
	"github.com/Allan-Nava/cragbook/internal/domain/climbing"
	"github.com/Allan-Nava/cragbook/internal/domain/diary"
)

//go:embed migrations/*.sql
var migrations embed.FS

type Store struct {
	pool *pgxpool.Pool
}

var (
	_ climbing.Catalog       = (*Store)(nil)
	_ climbing.CatalogWriter = (*Store)(nil)
	_ diary.Repository       = (*Store)(nil)
	_ catalogue.Summaries    = (*Store)(nil)
)

func Open(ctx context.Context, url string) (*Store, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("database: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() { s.pool.Close() }

// Migrate applies the migrations not yet recorded in schema_migration, in one transaction; an
// advisory lock keeps two starting processes from applying one twice.
func (s *Store) Migrate(ctx context.Context) (err error) {
	names, err := fs.Glob(migrations, "migrations/*.sql")
	if err != nil {
		return err
	}
	slices.Sort(names)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer rollback(ctx, tx, &err)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(20261023)`); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migration (name TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL)`); err != nil {
		return err
	}
	for _, name := range names {
		var done bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migration WHERE name = $1)`, name).Scan(&done); err != nil {
			return err
		}
		if done {
			continue
		}
		sql, err := migrations.ReadFile(name)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("migration %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migration (name, applied_at) VALUES ($1, now())`, name); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) SaveCrag(ctx context.Context, c climbing.Crag, routes []climbing.Route, at time.Time) (err error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer rollback(ctx, tx, &err)
	p := c.Place
	_, err = tx.Exec(ctx, `
		INSERT INTO crag (id, name, municipality, province_code, latitude, longitude, aspect, elevation, imported_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, municipality = EXCLUDED.municipality,
			province_code = EXCLUDED.province_code, latitude = EXCLUDED.latitude, longitude = EXCLUDED.longitude,
			aspect = EXCLUDED.aspect, elevation = EXCLUDED.elevation, imported_at = EXCLUDED.imported_at`,
		c.ID, c.Name, p.Municipality, p.ProvinceCode, p.Latitude, p.Longitude, p.Aspect, p.Elevation, at)
	if err != nil {
		return fmt.Errorf("crag %s: %w", c.ID, err)
	}
	ids := make([]string, 0, len(routes))
	for _, r := range routes {
		ids = append(ids, string(r.ID))
		var grade *string
		var rank *int
		if r.Grade != nil {
			g, n := r.Grade.String(), r.Grade.Rank()
			grade, rank = &g, &n
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO route (id, crag_id, name, grade, grade_rank, length_m, bolts, pitches, sector, imported_at, gone_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NULL)
			ON CONFLICT (id) DO UPDATE SET crag_id = EXCLUDED.crag_id, name = EXCLUDED.name, grade = EXCLUDED.grade,
				grade_rank = EXCLUDED.grade_rank, length_m = EXCLUDED.length_m, bolts = EXCLUDED.bolts,
				pitches = EXCLUDED.pitches, sector = EXCLUDED.sector, imported_at = EXCLUDED.imported_at, gone_at = NULL`,
			r.ID, c.ID, r.Name, grade, rank, r.Length, r.Bolts, r.Pitches, r.Sector, at)
		if err != nil {
			return fmt.Errorf("route %s: %w", r.ID, err)
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE route SET gone_at = $3 WHERE crag_id = $1 AND gone_at IS NULL AND NOT (id = ANY($2))`,
		c.ID, ids, at); err != nil {
		return fmt.Errorf("crag %s: marking gone routes: %w", c.ID, err)
	}
	return tx.Commit(ctx)
}

// rollback ends a transaction not committed, adding a failed rollback to *err; after Commit it
// does nothing.
func rollback(ctx context.Context, tx pgx.Tx, err *error) {
	if e := tx.Rollback(ctx); e != nil && !errors.Is(e, pgx.ErrTxClosed) {
		*err = errors.Join(*err, fmt.Errorf("rollback: %w", e))
	}
}

const cragSelect = `SELECT id, name, municipality, province_code, latitude, longitude, aspect, elevation FROM crag`

func scanCrag(row pgx.Row, extra ...any) (climbing.Crag, error) {
	var c climbing.Crag
	p := &c.Place
	err := row.Scan(append([]any{&c.ID, &c.Name, &p.Municipality, &p.ProvinceCode, &p.Latitude, &p.Longitude, &p.Aspect, &p.Elevation}, extra...)...)
	return c, err
}

func (s *Store) Crag(ctx context.Context, id climbing.CragID) (climbing.Crag, error) {
	c, err := scanCrag(s.pool.QueryRow(ctx, cragSelect+` WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return climbing.Crag{}, climbing.ErrCragNotFound
	}
	return c, err
}

func (s *Store) CragsByID(ctx context.Context, ids []climbing.CragID) ([]climbing.Crag, error) {
	rows, err := s.pool.Query(ctx, cragSelect+` WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (climbing.Crag, error) { return scanCrag(r) })
}

const routeSelect = `SELECT id, crag_id, name, grade, length_m, bolts, pitches, sector, gone_at IS NOT NULL FROM route`

func scanRoute(row pgx.Row) (climbing.Route, error) {
	var r climbing.Route
	var grade *string
	err := row.Scan(&r.ID, &r.CragID, &r.Name, &grade, &r.Length, &r.Bolts, &r.Pitches, &r.Sector, &r.Gone)
	if err == nil && grade != nil {
		if g, ok := climbing.ParseGrade(*grade); ok {
			r.Grade = &g
		}
	}
	return r, err
}

func collectRoutes(rows pgx.Rows, err error) ([]climbing.Route, error) {
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (climbing.Route, error) { return scanRoute(r) })
}

func (s *Store) Routes(ctx context.Context, crag climbing.CragID) ([]climbing.Route, error) {
	return collectRoutes(s.pool.Query(ctx, routeSelect+` WHERE crag_id = $1`, crag))
}

func (s *Store) Route(ctx context.Context, id climbing.RouteID) (climbing.Route, error) {
	r, err := scanRoute(s.pool.QueryRow(ctx, routeSelect+` WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return climbing.Route{}, climbing.ErrRouteNotFound
	}
	return r, err
}

func (s *Store) RoutesByID(ctx context.Context, ids []climbing.RouteID) ([]climbing.Route, error) {
	return collectRoutes(s.pool.Query(ctx, routeSelect+` WHERE id = ANY($1)`, ids))
}

// Save writes the entry, replacing the route's previous one; ErrRouteNotFound for a route the
// catalogue does not have.
func (s *Store) Save(ctx context.Context, a diary.Ascent) error {
	tag, err := s.pool.Exec(ctx, `
		INSERT INTO ascent (route_id, status, climbed_on, notes, updated_at)
		SELECT id, $2, $3, $4, $5 FROM route WHERE id = $1
		ON CONFLICT (route_id) DO UPDATE SET status = EXCLUDED.status, climbed_on = EXCLUDED.climbed_on,
			notes = EXCLUDED.notes, updated_at = EXCLUDED.updated_at`,
		a.Route(), a.Status(), a.ClimbedOn(), a.Notes(), a.UpdatedAt())
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return climbing.ErrRouteNotFound
	}
	return nil
}

func (s *Store) Delete(ctx context.Context, route climbing.RouteID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM ascent WHERE route_id = $1`, route)
	return err
}

func collectAscents(rows pgx.Rows, err error) ([]diary.Ascent, error) {
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (diary.Ascent, error) {
		var route climbing.RouteID
		var status diary.Status
		var day *time.Time
		var notes string
		var updated time.Time
		err := r.Scan(&route, &status, &day, &notes, &updated)
		return diary.Restore(route, status, day, notes, updated), err
	})
}

const ascentSelect = `SELECT route_id, status, climbed_on, notes, updated_at FROM ascent`

func (s *Store) ForRoutes(ctx context.Context, routes []climbing.RouteID) (map[climbing.RouteID]diary.Ascent, error) {
	list, err := collectAscents(s.pool.Query(ctx, ascentSelect+` WHERE route_id = ANY($1)`, routes))
	if err != nil {
		return nil, err
	}
	out := make(map[climbing.RouteID]diary.Ascent, len(list))
	for _, a := range list {
		out[a.Route()] = a
	}
	return out, nil
}

func (s *Store) All(ctx context.Context) ([]diary.Ascent, error) {
	return collectAscents(s.pool.Query(ctx, ascentSelect))
}

// CragSummaries counts in the database: one query for the whole list.
func (s *Store) CragSummaries(ctx context.Context, f catalogue.Filter) ([]catalogue.CragSummary, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT c.id, c.name, c.municipality, c.province_code, c.latitude, c.longitude, c.aspect, c.elevation,
			count(r.id) FILTER (WHERE r.gone_at IS NULL),
			count(a.route_id) FILTER (WHERE r.gone_at IS NULL AND a.status = 'done')
		FROM crag c LEFT JOIN route r ON r.crag_id = c.id LEFT JOIN ascent a ON a.route_id = r.id
		WHERE $1 = '' OR c.name ILIKE '%' || $1 || '%' OR c.municipality ILIKE '%' || $1 || '%'
		GROUP BY c.id
		HAVING NOT $2 OR count(r.id) FILTER (WHERE r.gone_at IS NULL) > 0
		ORDER BY c.name, c.id`, likeEscape(f.Query), f.WithRoutes)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (catalogue.CragSummary, error) {
		var sum catalogue.CragSummary
		c, err := scanCrag(r, &sum.Routes, &sum.Done)
		sum.Crag = c
		return sum, err
	})
}

// likeEscape makes q match literally inside ILIKE, where % and _ are wildcards.
func likeEscape(q string) string {
	var b strings.Builder
	for _, r := range q {
		if r == '%' || r == '_' || r == '\\' {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}
