-- The crags and routes are a copy of cragcast's API (OpenStreetMap data, ODbL), refreshed by
-- every import. The diary — ascent — is the user's own and never written by an import: a route
-- that leaves OpenStreetMap is marked gone, never deleted, so its ascents stay.
CREATE TABLE crag (
    id            TEXT             PRIMARY KEY,          -- osm-node-123, cragcast's id
    name          TEXT             NOT NULL,
    municipality  TEXT,
    province_code TEXT,
    latitude      DOUBLE PRECISION NOT NULL,
    longitude     DOUBLE PRECISION NOT NULL,
    aspect        TEXT,
    elevation     INTEGER,
    imported_at   TIMESTAMPTZ      NOT NULL
);

CREATE TABLE route (
    id          TEXT             PRIMARY KEY,
    crag_id     TEXT             NOT NULL REFERENCES crag (id),
    name        TEXT,
    grade       TEXT,
    grade_rank  INTEGER,                                 -- grade.Rank: 6a = 36
    length_m    DOUBLE PRECISION,
    bolts       INTEGER,
    pitches     INTEGER,
    sector      TEXT,                                    -- the sector's name, as cragcast gives it
    imported_at TIMESTAMPTZ      NOT NULL,
    gone_at     TIMESTAMPTZ                              -- set when an import no longer lists it
);
CREATE INDEX route_crag ON route (crag_id, grade_rank);

CREATE TABLE ascent (
    route_id   TEXT        PRIMARY KEY REFERENCES route (id),
    status     TEXT        NOT NULL CHECK (status IN ('done', 'project', 'todo')),
    climbed_on DATE        CHECK (climbed_on IS NULL OR status = 'done'),
    notes      TEXT        NOT NULL DEFAULT '' CHECK (length(notes) <= 2000),
    updated_at TIMESTAMPTZ NOT NULL
);
