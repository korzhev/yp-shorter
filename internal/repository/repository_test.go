package repository

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/korzhev/yp-shorter/internal/config"
	"github.com/korzhev/yp-shorter/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewShortLinkDB(t *testing.T) {
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
			actualLink, err := db.GetByShort(id)
			require.NoError(t, err)
			assert.Equal(t, expectedLink, actualLink)
		}
	})
}

func TestShortLinkDB_GetById(t *testing.T) {
	db := &ShortLinkDB{storage: ShortLinkStorage{
		M: make(InMemoryStorage),
	}}

	id := "test-id"
	link := "https://example.com"
	sl := model.ShortLink{ID: id, Link: link}

	// Pre-populate storage
	db.storage.M[id] = sl

	t.Run("Success", func(t *testing.T) {
		result, err := db.GetByShort(id)
		assert.NoError(t, err)
		assert.Equal(t, sl, result)
	})

	t.Run("NotFound", func(t *testing.T) {
		id := "non-existent"
		result, err := db.GetByShort(id)
		assert.Error(t, err)
		assert.Equal(t, err.Error(), fmt.Sprintf("No short link with ID: %s", id))
		assert.Empty(t, result.ID)
	})
}

func TestShortLinkDB_Save(t *testing.T) {
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
		result, err := db.Save(id, link)
		assert.NoError(t, err)
		assert.Equal(t, id, result.ID)
		assert.Equal(t, link, result.Link)

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
		_, err := db.Save(id, "https://another-link.com")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "is already used")
	})
}
