package repository

import (
	"fmt"

	"github.com/korzhev/yp-shorter/internal/model"
)

type ShortLinkDB struct {
	storage model.ShortLinkStorage
}

func (s *ShortLinkDB) GetById(id string) (model.ShortLink, error) {
	s.storage.RLock()
	sl, ok := s.storage.M[id]
	defer s.storage.RUnlock()
	if !ok {
		return sl, fmt.Errorf("No short link with ID: %s", id)
	}

	return sl, nil
}

func (s *ShortLinkDB) Save(id, link string) (model.ShortLink, error) {
	s.storage.Lock()
	sl := model.ShortLink{ID: id, Link: link}
	defer s.storage.Unlock()
	// check that id is not used
	_, ok := s.storage.M[id]
	// ok means id is already used
	if ok {
		// Unique index error, like DB
		return sl, fmt.Errorf("ID: %s is already used", id)
	}
	s.storage.M[id] = sl
	return sl, nil
}

func NewShortLinkDB() *ShortLinkDB{
	return &ShortLinkDB{
		storage: model.ShortLinkStorage{
			M: make(map[string]model.ShortLink),
		},
	}
}
