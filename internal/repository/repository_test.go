package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/korzhev/yp-shorter/internal/config"
	"github.com/korzhev/yp-shorter/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewShortLinkDB(t *testing.T) {
	ctx := context.Background()

	t.Run("CreatesNewStorageFile", func(t *testing.T) {
		filePath := filepath.Join(t.TempDir(), "storage.json")

		db := NewShortLinkDB(filePath, "", config.File)
		require.NotNil(t, db)
		require.NotNil(t, db.storage.F)
		t.Cleanup(func() {
			require.NoError(t, db.Close())
		})

		assert.Empty(t, db.storage.M)
		assert.Equal(t, filePath, db.storage.F.Name())

		info, err := os.Stat(filePath)
		require.NoError(t, err)
		assert.False(t, info.IsDir())
	})

	t.Run("LoadsExistingStorageFile", func(t *testing.T) {
		filePath := filepath.Join(t.TempDir(), "storage.json")
		expected := InMemoryStorage{
			"first-id": {
				ID:   "first-id",
				Link: "https://example.com/first",
			},
			"second-id": {
				ID:   "second-id",
				Link: "https://example.com/second",
			},
		}

		data, err := json.Marshal(expected)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filePath, data, 0644))

		db := NewShortLinkDB(filePath, "", config.File)
		require.NotNil(t, db)
		require.NotNil(t, db.storage.F)
		t.Cleanup(func() {
			require.NoError(t, db.Close())
		})

		assert.Equal(t, expected, db.storage.M)
		assert.Equal(t, filePath, db.storage.F.Name())

		for id, expectedLink := range expected {
			actualLink, err := db.GetByShort(ctx, id)
			require.NoError(t, err)
			assert.Equal(t, expectedLink, actualLink)
		}
	})
}

func TestShortLinkDB_GetById(t *testing.T) {
	ctx := context.Background()
	db := &ShortLinkDB{storage: ShortLinkStorage{
		M: make(InMemoryStorage),
	}}

	id := "test-id"
	link := "https://example.com"
	sl := model.ShortLink{ID: id, Link: link}

	// Pre-populate storage
	db.storage.M[id] = sl

	t.Run("Success", func(t *testing.T) {
		result, err := db.GetByShort(ctx, id)
		assert.NoError(t, err)
		assert.Equal(t, sl, result)
	})

	t.Run("NotFound", func(t *testing.T) {
		id := "non-existent"
		result, err := db.GetByShort(ctx, id)
		assert.Error(t, err)
		assert.Equal(t, err.Error(), fmt.Sprintf("No short link with ID: %s", id))
		assert.Empty(t, result.ID)
	})
}

func TestShortLinkDB_GetByLink(t *testing.T) {
	ctx := context.Background()
	expected := model.ShortLink{ID: "test-id", Link: "https://example.com"}
	db := NewShortLinkDB("", "", config.InMemory)
	db.storage.M[expected.ID] = expected

	t.Run("finds link in memory", func(t *testing.T) {
		actual, err := db.GetByLink(ctx, expected.Link)

		require.NoError(t, err)
		assert.Equal(t, expected, actual)
	})

	t.Run("returns not found error", func(t *testing.T) {
		link := "https://example.com/missing"

		actual, err := db.GetByLink(ctx, link)

		require.Error(t, err)
		assert.EqualError(t, err, fmt.Sprintf("No link: %s", link))
		assert.Equal(t, model.ShortLink{}, actual)
	})
}

func TestShortLinkDB_GetByUserID(t *testing.T) {
	ctx := context.Background()
	userID := 42
	expected := []model.ShortLink{
		{ID: "first-id", Link: "https://example.com/first", UserID: userID},
		{ID: "second-id", Link: "https://example.com/second", UserID: userID},
	}
	db := NewShortLinkDB("", "", config.InMemory)
	for _, sl := range expected {
		db.storage.M[sl.ID] = sl
	}
	db.storage.M["another-user-id"] = model.ShortLink{
		ID:     "another-user-id",
		Link:   "https://example.com/another-user",
		UserID: userID + 1,
	}

	actual, err := db.GetByUserID(ctx, userID)

	require.NoError(t, err)
	assert.ElementsMatch(t, expected, actual)
}

func TestInMemoryDuplicateError(t *testing.T) {
	link := "https://example.com"

	err := NewInMemoryDuplicateError(link)

	var duplicateErr *InMemoryDuplicateError
	require.ErrorAs(t, err, &duplicateErr)
	assert.Equal(t, link, duplicateErr.Link)
	assert.EqualError(t, err, "Link: https://example.com is already saved")
}

func TestShortLinkDB_Save(t *testing.T) {
	ctx := context.Background()
	userID := 42
	file, err := os.OpenFile(
		filepath.Join(t.TempDir(), "storage.json"),
		os.O_CREATE|os.O_RDWR,
		0644,
	)
	assert.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, file.Close())
	})

	db := &ShortLinkDB{storage: ShortLinkStorage{
		M: make(InMemoryStorage),
		F: file,
	}, storageType: config.File}

	id := "new-id"
	link := "https://new-link.com"

	t.Run("Success", func(t *testing.T) {
		result, err := db.Save(ctx, id, link, userID)
		assert.NoError(t, err)
		assert.Equal(t, id, result.ID)
		assert.Equal(t, link, result.Link)
		assert.Equal(t, userID, result.UserID)

		// Verify it's actually in storage
		stored, ok := db.storage.M[id]
		assert.True(t, ok)
		assert.Equal(t, result, stored)

		_, err = file.Seek(0, io.SeekStart)
		assert.NoError(t, err)

		data, err := io.ReadAll(file)
		assert.NoError(t, err)

		var persisted InMemoryStorage
		assert.NoError(t, json.Unmarshal(data, &persisted))
		assert.Equal(t, db.storage.M, persisted)
	})

	t.Run("AlreadyExists", func(t *testing.T) {
		// Attempt to save the same ID again
		_, err := db.Save(ctx, id, "https://another-link.com", userID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "is already used")
	})
}

func TestShortLinkDB_SaveBatch(t *testing.T) {
	ctx := context.Background()
	userID := 42
	batch := []model.ShortLink{
		{ID: "first-id", Link: "https://example.com/first"},
		{ID: "second-id", Link: "https://example.com/second"},
	}
	expected := []model.ShortLink{
		{ID: "first-id", Link: "https://example.com/first", UserID: userID},
		{ID: "second-id", Link: "https://example.com/second", UserID: userID},
	}

	t.Run("saves batch in memory", func(t *testing.T) {
		db := NewShortLinkDB("", "", config.InMemory)

		actual, err := db.SaveBatch(ctx, userID, batch)

		require.NoError(t, err)
		assert.Equal(t, expected, actual)
		assert.Equal(t, InMemoryStorage{
			"first-id":  expected[0],
			"second-id": expected[1],
		}, db.storage.M)
	})

	t.Run("returns duplicate error", func(t *testing.T) {
		db := NewShortLinkDB("", "", config.InMemory)
		db.storage.M[batch[0].ID] = batch[0]

		actual, err := db.SaveBatch(ctx, userID, batch)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "ID: first-id is already used")
		assert.Empty(t, actual)
	})

	t.Run("does not partially save when a later item is duplicate", func(t *testing.T) {
		db := NewShortLinkDB("", "", config.InMemory)
		existing := model.ShortLink{
			ID:   batch[1].ID,
			Link: "https://example.com/already-stored",
		}
		db.storage.M[existing.ID] = existing

		actual, err := db.SaveBatch(ctx, userID, batch)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "ID: second-id is already used")
		assert.Empty(t, actual)
		assert.Equal(t, InMemoryStorage{existing.ID: existing}, db.storage.M)
		assert.NotContains(t, db.storage.M, batch[0].ID)
	})

	t.Run("saves batch to file", func(t *testing.T) {
		filePath := filepath.Join(t.TempDir(), "storage.json")
		db := NewShortLinkDB(filePath, "", config.File)
		t.Cleanup(func() {
			require.NoError(t, db.Close())
		})

		actual, err := db.SaveBatch(ctx, userID, batch)

		require.NoError(t, err)
		assert.Equal(t, expected, actual)

		data, err := os.ReadFile(filePath)
		require.NoError(t, err)
		var persisted InMemoryStorage
		require.NoError(t, json.Unmarshal(data, &persisted))
		assert.Equal(t, db.storage.M, persisted)
	})
}

func TestShortLinkDB_SaveBatchDatabase(t *testing.T) {
	ctx := context.Background()
	userID := 42
	batch := []model.ShortLink{
		{ID: "first-id", Link: "https://example.com/first"},
		{ID: "second-id", Link: "https://example.com/second"},
	}
	expected := []model.ShortLink{
		{ID: "first-id", Link: "https://example.com/first", UserID: userID},
		{ID: "second-id", Link: "https://example.com/second", UserID: userID},
	}
	insertQuery := `INSERT INTO short_links \(short, link, user_id\) VALUES \(\$1, \$2, \$3\)`

	newDB := func(t *testing.T) (*ShortLinkDB, sqlmock.Sqlmock) {
		t.Helper()
		sqlDB, mock, err := sqlmock.New()
		require.NoError(t, err)
		t.Cleanup(func() {
			mock.ExpectClose()
			require.NoError(t, sqlDB.Close())
			require.NoError(t, mock.ExpectationsWereMet())
		})
		return &ShortLinkDB{DB: sqlDB, storageType: config.Database}, mock
	}

	t.Run("commits batch", func(t *testing.T) {
		db, mock := newDB(t)
		mock.ExpectBegin()
		for _, item := range batch {
			mock.ExpectExec(insertQuery).
				WithArgs(item.ID, item.Link, userID).
				WillReturnResult(sqlmock.NewResult(1, 1))
		}
		mock.ExpectCommit()

		actual, err := db.SaveBatch(ctx, userID, batch)

		require.NoError(t, err)
		assert.Equal(t, expected, actual)
	})

	t.Run("returns begin error", func(t *testing.T) {
		db, mock := newDB(t)
		expectedErr := errors.New("begin failed")
		mock.ExpectBegin().WillReturnError(expectedErr)

		actual, err := db.SaveBatch(ctx, userID, batch)

		assert.ErrorIs(t, err, expectedErr)
		assert.Empty(t, actual)
	})

	t.Run("rolls back on insert error", func(t *testing.T) {
		db, mock := newDB(t)
		expectedErr := errors.New("insert failed")
		mock.ExpectBegin()
		mock.ExpectExec(insertQuery).
			WithArgs(batch[0].ID, batch[0].Link, userID).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec(insertQuery).
			WithArgs(batch[1].ID, batch[1].Link, userID).
			WillReturnError(expectedErr)
		mock.ExpectRollback()

		actual, err := db.SaveBatch(ctx, userID, batch)

		assert.ErrorIs(t, err, expectedErr)
		assert.Equal(t, expected[:1], actual)
	})

	t.Run("returns commit error", func(t *testing.T) {
		db, mock := newDB(t)
		expectedErr := errors.New("commit failed")
		mock.ExpectBegin()
		for _, item := range batch {
			mock.ExpectExec(insertQuery).
				WithArgs(item.ID, item.Link, userID).
				WillReturnResult(sqlmock.NewResult(1, 1))
		}
		mock.ExpectCommit().WillReturnError(expectedErr)

		actual, err := db.SaveBatch(ctx, userID, batch)

		assert.ErrorIs(t, err, expectedErr)
		assert.Equal(t, expected, actual)
	})
}

func TestShortLinkDB_Database(t *testing.T) {
	ctx := context.Background()

	t.Run("gets a link", func(t *testing.T) {
		sqlDB, mock, err := sqlmock.New()
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, sqlDB.Close())
			require.NoError(t, mock.ExpectationsWereMet())
		})
		// ExpectQuery uses strings as regexp, so `$`,`(`,`)` should be quoted
		mock.ExpectQuery(`SELECT short, link FROM short_links WHERE short = \$1 LIMIT 1`).
			WithArgs("abcde").
			WillReturnRows(sqlmock.NewRows([]string{"short", "link"}).
				AddRow("abcde", "https://example.com"))
		mock.ExpectClose()

		db := &ShortLinkDB{DB: sqlDB, storageType: config.Database}
		actual, err := db.GetByShort(ctx, "abcde")

		require.NoError(t, err)
		assert.Equal(t, model.ShortLink{ID: "abcde", Link: "https://example.com"}, actual)
	})

	t.Run("returns query error", func(t *testing.T) {
		sqlDB, mock, err := sqlmock.New()
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, sqlDB.Close())
			require.NoError(t, mock.ExpectationsWereMet())
		})
		expectedErr := errors.New("query failed")

		mock.ExpectQuery(`SELECT short, link FROM short_links WHERE short = \$1 LIMIT 1`).
			WithArgs("abcde").
			WillReturnError(expectedErr)
		mock.ExpectClose()

		db := &ShortLinkDB{DB: sqlDB, storageType: config.Database}
		actual, err := db.GetByShort(ctx, "abcde")

		assert.ErrorIs(t, err, expectedErr)
		assert.Equal(t, "abcde", actual.ID)
	})

	t.Run("gets by original link", func(t *testing.T) {
		sqlDB, mock, err := sqlmock.New()
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, sqlDB.Close())
			require.NoError(t, mock.ExpectationsWereMet())
		})
		link := "https://example.com"
		mock.ExpectQuery(`SELECT short, link FROM short_links WHERE link = \$1 LIMIT 1`).
			WithArgs(link).
			WillReturnRows(sqlmock.NewRows([]string{"short", "link"}).AddRow("abcde", link))
		mock.ExpectClose()

		db := &ShortLinkDB{DB: sqlDB, storageType: config.Database}
		actual, err := db.GetByLink(ctx, link)

		require.NoError(t, err)
		assert.Equal(t, model.ShortLink{ID: "abcde", Link: link}, actual)
	})

	t.Run("returns get by original link error", func(t *testing.T) {
		sqlDB, mock, err := sqlmock.New()
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, sqlDB.Close())
			require.NoError(t, mock.ExpectationsWereMet())
		})
		link := "https://example.com"
		expectedErr := errors.New("query failed")
		mock.ExpectQuery(`SELECT short, link FROM short_links WHERE link = \$1 LIMIT 1`).
			WithArgs(link).
			WillReturnError(expectedErr)
		mock.ExpectClose()

		db := &ShortLinkDB{DB: sqlDB, storageType: config.Database}
		actual, err := db.GetByLink(ctx, link)

		assert.ErrorIs(t, err, expectedErr)
		assert.Equal(t, model.ShortLink{}, actual)
	})

	t.Run("gets links by user ID", func(t *testing.T) {
		sqlDB, mock, err := sqlmock.New()
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, sqlDB.Close())
			require.NoError(t, mock.ExpectationsWereMet())
		})
		userID := 42
		expected := []model.ShortLink{
			{ID: "first-id", Link: "https://example.com/first", UserID: userID},
			{ID: "second-id", Link: "https://example.com/second", UserID: userID},
		}
		mock.ExpectQuery(`SELECT short, link, user_id FROM short_links WHERE user_id = \$1 LIMIT 1`).
			WithArgs(userID).
			WillReturnRows(sqlmock.NewRows([]string{"short", "link", "user_id"}).
				AddRow(expected[0].ID, expected[0].Link, expected[0].UserID).
				AddRow(expected[1].ID, expected[1].Link, expected[1].UserID))
		mock.ExpectClose()

		db := &ShortLinkDB{DB: sqlDB, storageType: config.Database}
		actual, err := db.GetByUserID(ctx, userID)

		require.NoError(t, err)
		assert.Equal(t, expected, actual)
	})

	t.Run("saves a link", func(t *testing.T) {
		sqlDB, mock, err := sqlmock.New()
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, sqlDB.Close())
			require.NoError(t, mock.ExpectationsWereMet())
		})

		userID := 42
		mock.ExpectExec(`INSERT INTO short_links \(short, link, user_id\) VALUES \(\$1, \$2, \$3\)`).
			WithArgs("abcde", "https://example.com", userID).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectClose()

		db := &ShortLinkDB{DB: sqlDB, storageType: config.Database}
		actual, err := db.Save(ctx, "abcde", "https://example.com", userID)

		require.NoError(t, err)
		assert.Equal(t, model.ShortLink{ID: "abcde", Link: "https://example.com", UserID: userID}, actual)
	})

	t.Run("returns insert error", func(t *testing.T) {
		sqlDB, mock, err := sqlmock.New()
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, sqlDB.Close())
			require.NoError(t, mock.ExpectationsWereMet())
		})
		expectedErr := errors.New("insert failed")
		userID := 42

		mock.ExpectExec(`INSERT INTO short_links \(short, link, user_id\) VALUES \(\$1, \$2, \$3\)`).
			WithArgs("abcde", "https://example.com", userID).
			WillReturnError(expectedErr)
		mock.ExpectClose()

		db := &ShortLinkDB{DB: sqlDB, storageType: config.Database}
		actual, err := db.Save(ctx, "abcde", "https://example.com", userID)

		assert.ErrorIs(t, err, expectedErr)
		assert.Equal(t, model.ShortLink{ID: "abcde", Link: "https://example.com", UserID: userID}, actual)
	})
}

func TestShortLinkDB_Close(t *testing.T) {
	t.Run("in-memory storage", func(t *testing.T) {
		db := NewShortLinkDB("", "", config.InMemory)
		require.NoError(t, db.Close())
	})

	t.Run("database storage", func(t *testing.T) {
		sqlDB, mock, err := sqlmock.New()
		require.NoError(t, err)
		mock.ExpectClose()

		db := &ShortLinkDB{DB: sqlDB, storageType: config.Database}
		require.NoError(t, db.Close())
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestShortLinkDB_SaveFileErrors(t *testing.T) {
	ctx := context.Background()
	userID := 42
	file, err := os.CreateTemp(t.TempDir(), "closed-storage-*.json")
	require.NoError(t, err)
	require.NoError(t, file.Close())

	db := &ShortLinkDB{
		storage:     ShortLinkStorage{M: make(InMemoryStorage), F: file},
		storageType: config.File,
	}

	actual, err := db.Save(ctx, "abcde", "https://example.com", userID)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Can't clean storage")
	assert.Equal(t, model.ShortLink{ID: "abcde", Link: "https://example.com", UserID: userID}, actual)
}

func TestNewShortLinkDB_Database(t *testing.T) {
	// real connection will be set only with first query, so dsn doesn't matter here
	db := NewShortLinkDB("", "postgres://user:pass@localhost/test", config.Database)
	require.NotNil(t, db)
	require.NotNil(t, db.DB)
	require.NoError(t, db.Close())
}
