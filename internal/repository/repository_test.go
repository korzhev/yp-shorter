package repository

import (
	"fmt"
	"testing"

	"github.com/korzhev/yp-shorter/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestShortLinkDB_GetById(t *testing.T) {
	db := &ShortLinkDB{storage: model.ShortLinkStorage{
		M: make(map[string]model.ShortLink),
	}}

	id := "test-id"
	link := "https://example.com"
	sl := model.ShortLink{ID: id, Link: link}

	// Pre-populate storage
	db.storage.M[id] = sl

	t.Run("Success", func(t *testing.T) {
		result, err := db.GetById(id)
		assert.NoError(t, err)
		assert.Equal(t, sl, result)
	})

	t.Run("NotFound", func(t *testing.T) {
		id := "non-existent"
		result, err := db.GetById(id)
		assert.Error(t, err)
		assert.Equal(t, err.Error(), fmt.Sprintf("No short link with ID: %s", id))
		assert.Empty(t, result.ID)
	})
}

func TestShortLinkDB_Save(t *testing.T) {
	db := &ShortLinkDB{storage: model.ShortLinkStorage{
		M: make(map[string]model.ShortLink),
	}}

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
	})

	t.Run("AlreadyExists", func(t *testing.T) {
		// Attempt to save the same ID again
		_, err := db.Save(id, "https://another-link.com")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "is already used")
	})
}
