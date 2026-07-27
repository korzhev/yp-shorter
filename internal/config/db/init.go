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

	if err := m.Up(); err != nil {
		return err
	}

	return nil
}
