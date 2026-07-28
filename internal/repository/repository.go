package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sync"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/korzhev/yp-shorter/internal/config"
	"github.com/korzhev/yp-shorter/internal/model"
)

type InMemoryStorage map[string]model.ShortLink

type ShortLinkStorage struct {
	sync.RWMutex
	M InMemoryStorage
	F *os.File
}

type ShortLinkDB struct {
	storage     ShortLinkStorage
	storageType config.StorageType
	DB          *sql.DB
}

func (s *ShortLinkDB) getByShortFromMemory(short string) (model.ShortLink, error) {
	s.storage.RLock()
	sl, ok := s.storage.M[short]
	defer s.storage.RUnlock()
	if !ok {
		return sl, fmt.Errorf("No short link with ID: %s", short)
	}

	return sl, nil
}

func (s *ShortLinkDB) getByShortFromDB(ctx context.Context, short string) (model.ShortLink, error) {
	row := s.DB.QueryRowContext(ctx, "SELECT short, link FROM short_links WHERE short = $1 LIMIT 1", short)
	sl := model.ShortLink{ID: short}
	err := row.Scan(&sl.ID, &sl.Link)
	return sl, err
}

func (s *ShortLinkDB) GetByShort(ctx context.Context, short string) (model.ShortLink, error) {
	var sl model.ShortLink
	var err error
	switch s.storageType {
	case config.Database:
		sl, err = s.getByShortFromDB(ctx, short)
	default:
		// File also as InMemory uses memory to get ShortLink
		sl, err = s.getByShortFromMemory(short)
	}

	return sl, err
}

func (s *ShortLinkDB) saveDatabase(ctx context.Context, short, link string) (model.ShortLink, error) {
	sl := model.ShortLink{ID: short, Link: link}
	_, err := s.DB.ExecContext(ctx, "INSERT INTO short_links (short, link) VALUES ($1, $2)", short, link)
	if err != nil {
		return sl, err
	}
	return sl, nil
}

func (s *ShortLinkDB) saveInMemory(id, link string) (model.ShortLink, error) {
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

func (s *ShortLinkDB) saveFile(id, link string) (model.ShortLink, error) {
	sl, err := s.saveInMemory(id, link)
	if err != nil {
		return sl, err
	}

	b, err := json.Marshal(s.storage.M)
	if err != nil {
		return sl, fmt.Errorf("Can't marshal data: %v with error: %s", s.storage.M, err.Error())
	}
	if err := s.storage.F.Truncate(0); err != nil {
		return sl, fmt.Errorf("Can't clean storage: %s, %s", s.storage.F.Name(), err.Error())
	}

	if _, err := s.storage.F.Seek(0, io.SeekStart); err != nil {
		return sl, fmt.Errorf("Can't set cursor:  %s, %s", s.storage.F.Name(), err.Error())
	}
	if _, err := s.storage.F.Write(b); err != nil {
		return sl, fmt.Errorf("Can't write to file:  %s, %s", s.storage.F.Name(), err.Error())
	}
	return sl, nil
}

func (s *ShortLinkDB) Save(ctx context.Context, id, link string) (model.ShortLink, error) {
	var sl model.ShortLink
	var err error
	switch s.storageType {
	case config.File:
		sl, err = s.saveFile(id, link)
	case config.Database:
		sl, err = s.saveDatabase(ctx, id, link)
	default:
		sl, err = s.saveInMemory(id, link)
	}

	return sl, err
}

func (s *ShortLinkDB) Close() error {
	switch s.storageType {
	case config.File:
		return s.storage.F.Close()
	case config.Database:
		return s.DB.Close()
	default:
		return nil
	}
}

func initFile(filePath string, m InMemoryStorage) *os.File {
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
	return file
}

func initDB(dsn string) *sql.DB {
	pg, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatal(err)
	}
	return pg
}

func NewShortLinkDB(filePath string, dsn string, st config.StorageType) *ShortLinkDB {
	var file *os.File
	var pg *sql.DB
	m := make(InMemoryStorage)

	switch st {
	case config.File:
		file = initFile(filePath, m)
	case config.Database:
		pg = initDB(dsn)
	}

	return &ShortLinkDB{
		storage: ShortLinkStorage{
			M: m,
			F: file,
		},
		DB:          pg,
		storageType: st,
	}
}
