package db

import (
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/korzhev/yp-shorter/migrations"
)

func InitSchema(dsn string) error {
	source, err := iofs.New(migrations.Files, ".")
	if err != nil {
		return err
	}

	m, err := migrate.NewWithSourceInstance(
		"iofs",
		source,
		dsn,
	)
	if err != nil {
		return err
	}

	defer m.Close()
	// migrate.ErrNoChange - fires if there isn't any new migration to run
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}

	return nil
}
