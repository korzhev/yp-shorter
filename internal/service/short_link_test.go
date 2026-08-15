package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/korzhev/yp-shorter/internal/model"
	"github.com/korzhev/yp-shorter/internal/repository"
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

func TestGetByLink(t *testing.T) {
	ctx := context.Background()
	link := "https://example.com"
	expected := model.ShortLink{ID: "abc123", Link: link}

	t.Run("Success", func(t *testing.T) {
		mockRepo := mocks.NewMockIShortLinkRepository(gomock.NewController(t))
		service := ShortLinkService{ShortLinkDB: mockRepo}
		mockRepo.EXPECT().GetByLink(ctx, link).Return(expected, nil)

		actual, err := service.GetByLink(ctx, link)

		assert.NoError(t, err)
		assert.Equal(t, expected, actual)
	})

	t.Run("Error", func(t *testing.T) {
		mockRepo := mocks.NewMockIShortLinkRepository(gomock.NewController(t))
		service := ShortLinkService{ShortLinkDB: mockRepo}
		expectedErr := errors.New("not found")
		mockRepo.EXPECT().GetByLink(ctx, link).Return(model.ShortLink{}, expectedErr)

		actual, err := service.GetByLink(ctx, link)

		assert.ErrorIs(t, err, expectedErr)
		assert.Equal(t, model.ShortLink{}, actual)
	})
}

func TestGetByUserID(t *testing.T) {
	ctx := context.Background()
	userID := 42
	expected := []model.ShortLink{
		{ID: "first-id", Link: "https://example.com/first", UserID: userID},
		{ID: "second-id", Link: "https://example.com/second", UserID: userID},
	}

	t.Run("Success", func(t *testing.T) {
		mockRepo := mocks.NewMockIShortLinkRepository(gomock.NewController(t))
		service := ShortLinkService{ShortLinkDB: mockRepo}
		mockRepo.EXPECT().GetByUserID(ctx, userID).Return(expected, nil)

		actual, err := service.GetByUserID(ctx, userID)

		assert.NoError(t, err)
		assert.Equal(t, expected, actual)
	})

	t.Run("Error", func(t *testing.T) {
		mockRepo := mocks.NewMockIShortLinkRepository(gomock.NewController(t))
		service := ShortLinkService{ShortLinkDB: mockRepo}
		expectedErr := errors.New("repository error")
		mockRepo.EXPECT().GetByUserID(ctx, userID).Return(nil, expectedErr)

		actual, err := service.GetByUserID(ctx, userID)

		assert.ErrorIs(t, err, expectedErr)
		assert.Nil(t, actual)
	})
}

func TestSave(t *testing.T) {
	ctx := context.Background()
	const charset = "abc"
	const idLength = 3
	const userID = 42
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
		mockRepo.EXPECT().Save(ctx, idMatcher, testLink, userID).
			Return(model.ShortLink{ID: "abc", Link: testLink, UserID: userID}, nil)

		res, err := service.Save(ctx, testLink, userID)

		assert.NoError(t, err)
		assert.Equal(t, testLink, res.Link)
		assert.Equal(t, userID, res.UserID)
	})

	t.Run("Success after retry", func(t *testing.T) {
		mockRepo := mocks.NewMockIShortLinkRepository(gomock.NewController(t))
		service := newService(mockRepo)
		gomock.InOrder(
			mockRepo.EXPECT().Save(ctx, idMatcher, testLink, userID).
				Return(model.ShortLink{}, repository.NewInMemoryDuplicateIDError("abc")),
			mockRepo.EXPECT().Save(ctx, idMatcher, testLink, userID).
				Return(model.ShortLink{ID: "abc", Link: testLink, UserID: userID}, nil),
		)

		res, err := service.Save(ctx, testLink, userID)

		assert.NoError(t, err)
		assert.Equal(t, testLink, res.Link)
		assert.Equal(t, userID, res.UserID)
	})

	t.Run("Fail after max retries", func(t *testing.T) {
		mockRepo := mocks.NewMockIShortLinkRepository(gomock.NewController(t))
		service := newService(mockRepo)
		expectedErr := repository.NewInMemoryDuplicateIDError("abc")
		mockRepo.EXPECT().Save(ctx, idMatcher, testLink, userID).
			Return(model.ShortLink{}, expectedErr).Times(11)

		res, err := service.Save(ctx, testLink, userID)

		assert.ErrorIs(t, err, expectedErr)
		assert.Equal(t, model.ShortLink{}, res)
	})
}

func TestSaveBatch(t *testing.T) {
	ctx := context.Background()
	const charset = "abc"
	const idLength = 3
	const userID = 42
	batch := []model.ShortLinkBatchItemRequest{
		{CorrelationID: "first-id", OriginalURL: "http://first"},
		{CorrelationID: "second-id", OriginalURL: "http://second"},
	}
	res := []model.ShortLink{
		{ID: "first", Link: "http://f", UserID: userID},
		{ID: "second", Link: "http://s", UserID: userID},
	}
	batchMatcher := gomock.Cond(func(links []model.ShortLink) bool {
		if len(links) != len(batch) {
			return false
		}

		for i, link := range links {
			if link.Link != batch[i].OriginalURL || len(link.ID) != idLength {
				return false
			}
			for _, char := range link.ID {
				if !strings.ContainsRune(charset, char) {
					return false
				}
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

	t.Run("Success", func(t *testing.T) {
		mockRepo := mocks.NewMockIShortLinkRepository(gomock.NewController(t))
		service := newService(mockRepo)
		mockRepo.EXPECT().SaveBatch(ctx, userID, batchMatcher).Return(res, nil)

		actual, err := service.SaveBatch(ctx, userID, batch)

		assert.NoError(t, err)
		assert.Equal(t, res, actual)
	})

	t.Run("Repository error", func(t *testing.T) {
		mockRepo := mocks.NewMockIShortLinkRepository(gomock.NewController(t))
		service := newService(mockRepo)
		expectedErr := errors.New("batch save failed")
		mockRepo.EXPECT().SaveBatch(ctx, userID, batchMatcher).Return(nil, expectedErr)

		actual, err := service.SaveBatch(ctx, userID, batch)

		assert.ErrorIs(t, err, expectedErr)
		assert.Nil(t, actual)
	})

	t.Run("Success after ID collision", func(t *testing.T) {
		mockRepo := mocks.NewMockIShortLinkRepository(gomock.NewController(t))
		service := newService(mockRepo)
		collisionErr := repository.NewInMemoryDuplicateIDError("abc")
		gomock.InOrder(
			mockRepo.EXPECT().SaveBatch(ctx, userID, batchMatcher).Return(nil, collisionErr),
			mockRepo.EXPECT().SaveBatch(ctx, userID, batchMatcher).Return(res, nil),
		)

		actual, err := service.SaveBatch(ctx, userID, batch)

		assert.NoError(t, err)
		assert.Equal(t, res, actual)
	})
}
