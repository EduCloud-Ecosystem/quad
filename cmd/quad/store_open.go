// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver

	"github.com/quad/quad/internal/store"
	"github.com/quad/quad/internal/store/memory"
	"github.com/quad/quad/internal/store/postgres"
	"github.com/quad/quad/internal/store/sqlite"
)

// openStore returns a ready Store and a human-readable description of the backing
// store (used by logStartupSummary). Selection logic, in priority order:
//
//  1. QUAD_STORE=memory|sqlite|postgres — explicit override
//  2. QUAD_DATABASE_URL set (and QUAD_STORE not explicitly "sqlite") → postgres
//  3. Otherwise: SQLite at QUAD_SQLITE_PATH (default: quad.db in the working directory)
func openStore(ctx context.Context) (store.Store, string, error) {
	switch kind := os.Getenv("QUAD_STORE"); kind {
	case "memory":
		log.Println("store: in-memory (QUAD_STORE=memory) — data will not survive restart")
		return memory.New(), "memory (ephemeral)", nil
	case "postgres":
		return openPostgres(ctx)
	case "sqlite":
		return openSQLite()
	case "":
		// auto-select based on environment
	default:
		return nil, "", fmt.Errorf("unknown QUAD_STORE=%q: valid values are memory, sqlite, postgres", kind)
	}

	// Auto: postgres if QUAD_DATABASE_URL is set, otherwise sqlite.
	if os.Getenv("QUAD_DATABASE_URL") != "" {
		return openPostgres(ctx)
	}
	return openSQLite()
}

func openSQLite() (store.Store, string, error) {
	path := os.Getenv("QUAD_SQLITE_PATH")
	if path == "" {
		path = "quad.db"
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	st, err := sqlite.Open(path)
	if err != nil {
		return nil, "", fmt.Errorf("open sqlite %s: %w", abs, err)
	}
	return st, fmt.Sprintf("sqlite %s", abs), nil
}

func openPostgres(ctx context.Context) (store.Store, string, error) {
	dsn := os.Getenv("QUAD_DATABASE_URL")
	if dsn == "" {
		return nil, "", fmt.Errorf("QUAD_DATABASE_URL is required for postgres store")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, "", err
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxIdleTime(5 * time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		return nil, "", fmt.Errorf("connect to postgres: %w", err)
	}

	st := postgres.New(db)
	if os.Getenv("QUAD_DB_AUTOMIGRATE") == "1" {
		if err := st.Migrate(ctx); err != nil {
			return nil, "", fmt.Errorf("apply migrations: %w", err)
		}
	}
	return st, "postgres", nil
}
