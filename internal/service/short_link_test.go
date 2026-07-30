package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/korzhev/yp-shorter/internal/model"
	"github.com/korzhev/yp-shorter/mocks"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestGenerateID(t *testing.T) {
	charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	idLength := 8
	service := ShortLinkService{
		Charset:  charset,
		IDLength: idLength,
	}

	id := service.GenerateID()

	assert.Len(t, id, idLength)
	for _, char := range id {
		assert.Contains(t, charset, string(char))
	}
}

func TestGetById(t *testing.T) {
	ctx := context.Background()
	expectedID := "abc123"
	expectedLink := "https://example.com"
	expectedShortLink := model.ShortLink{
		ID:   expectedID,
		Link: expectedLink,
	}

	t.Run("Success", func(t *testing.T) {
		mockRepo := mocks.NewMockIShortLinkRepository(gomock.NewController(t))
		service := ShortLinkService{ShortLinkDB: mockRepo}
		mockRepo.EXPECT().GetByShort(ctx, expectedID).Return(expectedShortLink, nil)

		res, err := service.GetByShort(ctx, expectedID)

		assert.NoError(t, err)
		assert.Equal(t, expectedShortLink, res)
	})

	t.Run("Error", func(t *testing.T) {
		mockRepo := mocks.NewMockIShortLinkRepository(gomock.NewController(t))
		service := ShortLinkService{ShortLinkDB: mockRepo}
		expectedErr := errors.New("not found")
		mockRepo.EXPECT().GetByShort(ctx, "nonexistent").Return(model.ShortLink{}, expectedErr)

		res, err := service.GetByShort(ctx, "nonexistent")

		assert.Error(t, err)
		assert.Equal(t, expectedErr, err)
		assert.Equal(t, model.ShortLink{}, res)
	})
}

func TestSave(t *testing.T) {
	ctx := context.Background()
	const charset = "abc"
	const idLength = 3
	testLink := "https://example.com"
	idMatcher := gomock.Cond(func(id string) bool {
		if len(id) != idLength {
			return false
		}
		for _, char := range id {
			if !strings.ContainsRune(charset, char) {
				return false
			}
		}
		return true
	})
	newService := func(mockRepo model.IShortLinkRepository) ShortLinkService {
		return ShortLinkService{
			Charset:     charset,
			IDLength:    idLength,
			ShortLinkDB: mockRepo,
		}
	}

	t.Run("Success on first attempt", func(t *testing.T) {
		mockRepo := mocks.NewMockIShortLinkRepository(gomock.NewController(t))
		service := newService(mockRepo)
		mockRepo.EXPECT().Save(ctx, idMatcher, testLink).
			Return(model.ShortLink{ID: "abc", Link: testLink}, nil)

		res, err := service.Save(ctx, testLink)

		assert.NoError(t, err)
		assert.Equal(t, testLink, res.Link)
	})

	t.Run("Success after retry", func(t *testing.T) {
		mockRepo := mocks.NewMockIShortLinkRepository(gomock.NewController(t))
		service := newService(mockRepo)
		gomock.InOrder(
			mockRepo.EXPECT().Save(ctx, idMatcher, testLink).
				Return(model.ShortLink{}, errors.New("collision")),
			mockRepo.EXPECT().Save(ctx, idMatcher, testLink).
				Return(model.ShortLink{ID: "abc", Link: testLink}, nil),
		)

		res, err := service.Save(ctx, testLink)

		assert.NoError(t, err)
		assert.Equal(t, testLink, res.Link)
	})

	t.Run("Fail after max retries", func(t *testing.T) {
		mockRepo := mocks.NewMockIShortLinkRepository(gomock.NewController(t))
		service := newService(mockRepo)
		expectedErr := errors.New("persistent error")
		mockRepo.EXPECT().Save(ctx, idMatcher, testLink).
			Return(model.ShortLink{}, expectedErr).Times(11)

		res, err := service.Save(ctx, testLink)

		assert.ErrorIs(t, err, expectedErr)
		assert.Equal(t, model.ShortLink{}, res)
	})
}

func TestSaveBatch(t *testing.T) {
	ctx := context.Background()
	batch := []model.ShortLink{
		{ID: "first-id", Link: "/first"},
		{ID: "second-id", Link: "/second"},
	}

	t.Run("Success", func(t *testing.T) {
		mockRepo := mocks.NewMockIShortLinkRepository(gomock.NewController(t))
		service := ShortLinkService{ShortLinkDB: mockRepo}
		mockRepo.EXPECT().SaveBatch(ctx, batch).Return(batch, nil)

		actual, err := service.SaveBatch(ctx, batch)

		assert.NoError(t, err)
		assert.Equal(t, batch, actual)
	})

	t.Run("Repository error", func(t *testing.T) {
		mockRepo := mocks.NewMockIShortLinkRepository(gomock.NewController(t))
		service := ShortLinkService{ShortLinkDB: mockRepo}
		expectedErr := errors.New("batch save failed")
		mockRepo.EXPECT().SaveBatch(ctx, batch).Return(nil, expectedErr)

		actual, err := service.SaveBatch(ctx, batch)

		assert.ErrorIs(t, err, expectedErr)
		assert.Nil(t, actual)
	})
}
