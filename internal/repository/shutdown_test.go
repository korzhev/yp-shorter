package repository

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/korzhev/yp-shorter/internal/config"
	"github.com/korzhev/yp-shorter/internal/model"
	"github.com/stretchr/testify/require"
)

func TestClosePersistsUnsavedLinks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "storage.json")
	db := NewShortLinkDB(path, "", config.File)
	link := model.ShortLink{ID: "pending", Link: "https://example.com", UserID: 42}
	db.storage.M[link.ID] = link
	require.NoError(t, db.Close())
	reopened := NewShortLinkDB(path, "", config.File)
	defer func() { require.NoError(t, reopened.Close()) }()
	saved, err := reopened.GetByShort(context.Background(), link.ID)
	require.NoError(t, err)
	require.Equal(t, link, saved)
}
