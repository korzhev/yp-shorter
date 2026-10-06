package db

import (
	"context"
)

// IPG provides the database connectivity check used by the health handler.
type IPG interface {
	// PingContext checks connectivity within the lifetime of ctx.
	PingContext(ctx context.Context) error
}
