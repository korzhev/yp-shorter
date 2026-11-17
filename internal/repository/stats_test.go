package repository

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/korzhev/yp-shorter/internal/config"
	"github.com/korzhev/yp-shorter/internal/model"
	"github.com/stretchr/testify/require"
)

func TestGetStatsMemoryAndFile(t *testing.T) {
	for _, storageType := range []config.StorageType{config.InMemory, config.File} {
		t.Run(map[config.StorageType]string{config.InMemory: "memory", config.File: "file"}[storageType], func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "links.json")
			db := NewShortLinkDB(path, "", storageType)
			t.Cleanup(func() { require.NoError(t, db.Close()) })
			ctx := context.Background()
			stats, err := db.GetStats(ctx)
			require.NoError(t, err)
			require.Equal(t, model.Stats{}, stats)
			for i, id := range []string{"one", "two", "three"} {
				_, err := db.Save(ctx, id, "https://example.com/"+id, i/2+1)
				require.NoError(t, err)
			}
			require.NoError(t, db.DeleteBatch(ctx, 2, []string{"three"}))
			stats, err = db.GetStats(ctx)
			require.NoError(t, err)
			require.Equal(t, model.Stats{URLs: 3, Users: 2}, stats)
			if storageType == config.File {
				require.NoError(t, db.Close())
				db = NewShortLinkDB(path, "", storageType)
				stats, err = db.GetStats(ctx)
				require.NoError(t, err)
				require.Equal(t, model.Stats{URLs: 3, Users: 2}, stats)
			}
		})
	}
}

func TestGetStatsDatabase(t *testing.T) {
	pg, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = pg.Close() }()
	db := &ShortLinkDB{DB: pg, storageType: config.Database}
	query := `SELECT COUNT\(\*\), COUNT\(DISTINCT user_id\) FROM short_links`
	for _, want := range []model.Stats{{}, {URLs: 3, Users: 2}} {
		mock.ExpectQuery(query).WillReturnRows(sqlmock.NewRows([]string{"urls", "users"}).AddRow(want.URLs, want.Users))
		stats, err := db.GetStats(context.Background())
		require.NoError(t, err)
		require.Equal(t, want, stats)
	}
	wantErr := errors.New("database unavailable")
	mock.ExpectQuery(query).WillReturnError(wantErr)
	_, err = db.GetStats(context.Background())
	require.ErrorIs(t, err, wantErr)
	require.NoError(t, mock.ExpectationsWereMet())
}
