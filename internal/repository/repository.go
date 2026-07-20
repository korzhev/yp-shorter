package repository

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"

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
	b, err := json.Marshal(s.storage.M)
	if err != nil {
		return sl, fmt.Errorf("Can't marshal data: %v wit error: %s", s.storage.M, err.Error())
	}
	if err := s.storage.F.Truncate(0); err != nil {
		return sl, fmt.Errorf("Can't clean storage: %s, %s", s.storage.F.Name(), err.Error())
	}

	if _, err := s.storage.F.Seek(0, io.SeekStart); err != nil {
		return sl, fmt.Errorf("Can't set cursor:  %s, %s", s.storage.F.Name(), err.Error())
	}
	s.storage.F.Write(b)
	return sl, nil
}

func (s *ShortLinkDB) GetFile() *os.File {
	return s.storage.F
}

func NewShortLinkDB(filePath string) *ShortLinkDB {

	m := make(map[string]model.ShortLink)
	// not sure about O_SYNC
	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		log.Fatal(err)
	}
	data, err := io.ReadAll(file)
	if err != nil {
		log.Fatal(err)
	}

	// check if just created
	if len(data) != 0 {
		if err := json.Unmarshal(data, &m); err != nil {
			log.Fatal(err)
		}
	}

	return &ShortLinkDB{
		storage: model.ShortLinkStorage{
			M: m,
			F: file,
		},
	}
}
