package db

import (
	"context"
	"database/sql"
)

type IPG interface {
	PingContext(ctx context.Context) error
}

type ISQLRow interface {
	Scan(dest ...any) error
}

type IShortLinkDB interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}
