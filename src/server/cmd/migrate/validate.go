package main

import (
	"context"
	"database/sql"
	"oblivious/server/internal/migration"
)

type sqlDBAdapter struct {
	db *sql.DB
}

func (a *sqlDBAdapter) QueryRow(ctx context.Context, query string, args ...any) migration.Row {
	return a.db.QueryRowContext(ctx, query, args...)
}

func (a *sqlDBAdapter) Query(ctx context.Context, query string, args ...any) (migration.Rows, error) {
	return a.db.QueryContext(ctx, query, args...)
}

func wrapDB(db *sql.DB) migration.DB {
	return &sqlDBAdapter{db: db}
}
