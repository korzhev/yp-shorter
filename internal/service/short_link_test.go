package service

import (
	"errors"
	"testing"

	"github.com/korzhev/yp-shorter/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockShortLinkRepository is a mock implementation of model.IShortLinkRepository
type MockShortLinkRepository struct {
	mock.Mock
}

func (m *MockShortLinkRepository) GetById(id string) (model.ShortLink, error) {
	args := m.Called(id)
	return args.Get(0).(model.ShortLink), args.Error(1)
}

func (m *MockShortLinkRepository) Save(id string, link string) (model.ShortLink, error) {
	args := m.Called(id, link)
	return args.Get(0).(model.ShortLink), args.Error(1)
}

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
	mockRepo := new(MockShortLinkRepository)
	service := ShortLinkService{
		ShortLinkDB: mockRepo,
	}

	expectedID := "abc123"
	expectedLink := "https://example.com"
	expectedShorLink := model.ShortLink{
		ID:   expectedID,
		Link: expectedLink,
	}

	t.Run("Success", func(t *testing.T) {
		mockRepo.On("GetById", expectedID).Return(expectedShorLink, nil).Once()

		res, err := service.GetById(expectedID)

		assert.NoError(t, err)
		assert.Equal(t, expectedShorLink, res)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Error", func(t *testing.T) {
		expectedErr := errors.New("not found")
		mockRepo.On("GetById", "nonexistent").Return(model.ShortLink{}, expectedErr).Once()

		res, err := service.GetById("nonexistent")

		assert.Error(t, err)
		assert.Equal(t, expectedErr, err)
		assert.Equal(t, model.ShortLink{}, res)
		mockRepo.AssertExpectations(t)
	})
}

func TestSave(t *testing.T) {
	mockRepo := new(MockShortLinkRepository)
	idLength:= 3
	service := ShortLinkService{
		Charset:     "abc",
		IDLength:    idLength,
		ShortLinkDB: mockRepo,
	}

	testLink := "https://example.com"

	t.Run("Success on first attempt", func(t *testing.T) {
		mockRepo.On("Save", mock.MatchedBy(func(id string) bool { return len(id) == idLength }), testLink).
			Return(model.ShortLink{ID: "abc", Link: testLink}, nil).Once()

		res, err := service.Save(testLink)

		assert.NoError(t, err)
		assert.Equal(t, testLink, res.Link)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Success after retry", func(t *testing.T) {
		// First call returns error (ID collision simulation), second succeeds
		mockRepo.On("Save", mock.MatchedBy(func(id string) bool { return len(id) == idLength }), testLink).
			Return(model.ShortLink{}, errors.New("collision")).Once()
		mockRepo.On("Save", mock.MatchedBy(func(id string) bool { return len(id) == idLength }), testLink).
			Return(model.ShortLink{ID: "abc", Link: testLink}, nil).Once()

		res, err := service.Save(testLink)

		assert.NoError(t, err)
		assert.Equal(t, testLink, res.Link)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Fail after max retries", func(t *testing.T) {
		// Should try 1 (initial) + 10 retries = 11 attempts total
		mockRepo.On("Save", mock.MatchedBy(func(id string) bool { return len(id) == idLength }), testLink).
			Return(model.ShortLink{}, errors.New("persistent error")).Times(11)

		res, err := service.Save(testLink)

		assert.Error(t, err)
		assert.Equal(t, "persistent error", err.Error())
		assert.Equal(t, model.ShortLink{}, res)
		mockRepo.AssertExpectations(t)
	})
}
