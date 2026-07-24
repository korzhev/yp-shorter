package db

import "context"

type IPG interface {
	PingContext(ctx context.Context) error
}