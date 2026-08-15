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

type InMemoryDuplicateIDError struct {
	ID string
}

func (e *InMemoryDuplicateIDError) Error() string {
	return fmt.Sprintf("ID: %s is already used", e.ID)
}

func NewInMemoryDuplicateIDError(id string) error {
	return &InMemoryDuplicateIDError{
		ID: id,
	}
}

type InMemoryDuplicateError struct {
	Link string
}

func (e *InMemoryDuplicateError) Error() string {
	return fmt.Sprintf("Link: %s is already saved", e.Link)
}

func NewInMemoryDuplicateError(link string) error {
	return &InMemoryDuplicateError{
		Link: link,
	}
}

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

func (s *ShortLinkDB) getByLinkFromMemory(link string) (model.ShortLink, error) {
	s.storage.RLock()
	defer s.storage.RUnlock()

	for _, v := range s.storage.M {
		if v.Link == link {
			return v, nil
		}
	}

	return model.ShortLink{}, fmt.Errorf("No link: %s", link)
}

func (s *ShortLinkDB) getByUserIDFromMemory(userID int) ([]model.ShortLink, error) {
	res := make([]model.ShortLink, 0)
	s.storage.RLock()
	defer s.storage.RUnlock()

	for _, v := range s.storage.M {
		if v.UserID == userID {
			res = append(res, v)
		}
	}

	return res, nil
}

func (s *ShortLinkDB) getByShortFromDB(ctx context.Context, short string) (model.ShortLink, error) {
	row := s.DB.QueryRowContext(ctx, "SELECT short, link FROM short_links WHERE short = $1 LIMIT 1", short)
	sl := model.ShortLink{ID: short}
	err := row.Scan(&sl.ID, &sl.Link)
	return sl, err
}

func (s *ShortLinkDB) getByLinkFromDB(ctx context.Context, link string) (model.ShortLink, error) {
	row := s.DB.QueryRowContext(ctx, "SELECT short, link FROM short_links WHERE link = $1 LIMIT 1", link)
	sl := model.ShortLink{}
	err := row.Scan(&sl.ID, &sl.Link)
	return sl, err
}

func (s *ShortLinkDB) getByUserIDFromDB(ctx context.Context, userID int) ([]model.ShortLink, error) {
	rows, err := s.DB.QueryContext(ctx, "SELECT short, link, user_id FROM short_links WHERE user_id = $1 LIMIT 1", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make([]model.ShortLink, 0)
	for rows.Next() {
		var sl model.ShortLink
		err = rows.Scan(&sl.ID, &sl.Link, &sl.UserID)
		if err != nil {
			return nil, err
		}

		res = append(res, sl)
	}

	err = rows.Err()
	if err != nil {
		return nil, err
	}

	return res, err
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

func (s *ShortLinkDB) GetByLink(ctx context.Context, link string) (model.ShortLink, error) {
	var sl model.ShortLink
	var err error
	switch s.storageType {
	case config.Database:
		sl, err = s.getByLinkFromDB(ctx, link)
	default:
		// File also as InMemory uses memory to get ShortLink
		sl, err = s.getByLinkFromMemory(link)
	}

	return sl, err
}

func (s *ShortLinkDB) GetByUserID(ctx context.Context, userID int) ([]model.ShortLink, error) {
	res := make([]model.ShortLink, 0)
	var err error
	switch s.storageType {
	case config.Database:
		res, err = s.getByUserIDFromDB(ctx, userID)
	default:
		// File also as InMemory uses memory to get ShortLink
		res, err = s.getByUserIDFromMemory(userID)
	}

	return res, err
}

func (s *ShortLinkDB) saveDatabase(ctx context.Context, short, link string, userID int) (model.ShortLink, error) {
	sl := model.ShortLink{ID: short, Link: link, UserID: userID}
	_, err := s.DB.ExecContext(ctx, "INSERT INTO short_links (short, link, user_id) VALUES ($1, $2, $3)", short, link, userID)
	if err != nil {
		return sl, err
	}
	return sl, nil
}

func (s *ShortLinkDB) saveBatchDatabase(ctx context.Context, batch []model.ShortLink, userID int) ([]model.ShortLink, error) {
	res := make([]model.ShortLink, 0, len(batch))
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return res, err
	}
	for _, item := range batch {
		sl := model.ShortLink{ID: item.ID, Link: item.Link, UserID: userID}
		_, err := tx.ExecContext(ctx, "INSERT INTO short_links (short, link, user_id) VALUES ($1, $2, $3)", item.ID, item.Link, userID)
		if err != nil {
			if rerr := tx.Rollback(); rerr != nil {
				return nil, rerr
			}
			return res, err
		}
		res = append(res, sl)
	}

	err = tx.Commit()
	return res, err
}

func (s *ShortLinkDB) saveInMemory(id, link string, userID int) (model.ShortLink, error) {
	sl := model.ShortLink{ID: id, Link: link, UserID: userID}
	// check that id is not used
	_, ok := s.storage.M[id]
	// ok means id is already used
	if ok {
		// Unique index error, like DB
		return sl, NewInMemoryDuplicateIDError(id)
	}
	for _, v := range s.storage.M {
		if v.Link == link {
			return sl, NewInMemoryDuplicateError(link)
		}
	}
	s.storage.M[id] = sl
	return sl, nil
}

func (s *ShortLinkDB) saveFile() error {
	b, err := json.Marshal(s.storage.M)
	if err != nil {
		return fmt.Errorf("Can't marshal data: %v with error: %s", s.storage.M, err.Error())
	}
	if err := s.storage.F.Truncate(0); err != nil {
		return fmt.Errorf("Can't clean storage: %s, %s", s.storage.F.Name(), err.Error())
	}

	if _, err := s.storage.F.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("Can't set cursor:  %s, %s", s.storage.F.Name(), err.Error())
	}
	if _, err := s.storage.F.Write(b); err != nil {
		return fmt.Errorf("Can't write to file:  %s, %s", s.storage.F.Name(), err.Error())
	}
	return nil
}

func (s *ShortLinkDB) Save(ctx context.Context, id, link string, userID int) (model.ShortLink, error) {
	var sl model.ShortLink
	var err error
	if s.storageType != config.Database {
		s.storage.Lock()
		defer s.storage.Unlock()
	}
	switch s.storageType {
	case config.File:
		sl, err = s.saveInMemory(id, link, userID)
		if err != nil {
			return sl, err
		}
		err = s.saveFile()
	case config.Database:
		sl, err = s.saveDatabase(ctx, id, link, userID)
	default:
		sl, err = s.saveInMemory(id, link, userID)
	}

	return sl, err
}

func (s *ShortLinkDB) saveBatchInMemory(batch []model.ShortLink, userID int) ([]model.ShortLink, error) {
	res := make([]model.ShortLink, 0, len(batch))

	for _, item := range batch {
		// check that id is not used
		_, ok := s.storage.M[item.ID]
		// ok means id is already used
		if ok {
			// Unique index error, like DB
			return res, NewInMemoryDuplicateIDError(item.ID)
		}
		for _, v := range s.storage.M {
			if v.Link == item.Link {
				return res, NewInMemoryDuplicateError(item.Link)
			}
		}
	}

	for _, item := range batch {
		sl := model.ShortLink{ID: item.ID, Link: item.Link, UserID: userID}
		s.storage.M[item.ID] = sl
		res = append(res, sl)
	}
	return res, nil
}

func (s *ShortLinkDB) SaveBatch(ctx context.Context, userID int, batch []model.ShortLink) ([]model.ShortLink, error) {
	res := make([]model.ShortLink, 0, len(batch))
	var err error
	if s.storageType != config.Database {
		s.storage.Lock()
		defer s.storage.Unlock()
	}

	switch s.storageType {
	case config.File:
		res, err = s.saveBatchInMemory(batch, userID)
		if err != nil {
			return res, err
		}
		err = s.saveFile()
	case config.Database:
		res, err = s.saveBatchDatabase(ctx, batch, userID)
	default:
		res, err = s.saveBatchInMemory(batch, userID)
	}

	return res, err
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
