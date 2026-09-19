package service

import (
	"context"
	"errors"
	"math/rand/v2"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/korzhev/yp-shorter/internal/logger"
	"github.com/korzhev/yp-shorter/internal/model"
	"github.com/korzhev/yp-shorter/internal/repository"
)

type IShortLinkService interface {
	GenerateID() string
	GetByShort(ctx context.Context, id string) (model.ShortLink, error)
	GetByLink(ctx context.Context, Link string) (model.ShortLink, error)
	GetByUserID(ctx context.Context, userID int) ([]model.ShortLink, error)
	Save(ctx context.Context, link string, userID int) (model.ShortLink, error)
	SaveBatch(ctx context.Context, userID int, batch []model.ShortLinkBatchItemRequest) ([]model.ShortLink, error)
	DeleteBatch(ctx context.Context, userID int, batch []string) error
}

// Semaphore for DB
type Semaphore struct {
	semaCh chan struct{}
}

// maxReq - maximum parallel request for db
func NewSemaphore(maxReq int) *Semaphore {
	return &Semaphore{
		semaCh: make(chan struct{}, maxReq),
	}
}

// when goroutine starts send semaChto channel
func (s *Semaphore) Acquire() {
	s.semaCh <- struct{}{}
}

// when goroutine ends remove semaCh from channel
func (s *Semaphore) Release() {
	<-s.semaCh
}

type ShortLinkService struct {
	Charset     string
	IDLength    int
	ShortLinkDB model.IShortLinkRepository
	DBSemaphore *Semaphore
}

func (s ShortLinkService) GenerateID() string {
	b := make([]byte, s.IDLength)
	for i := range b {
		b[i] = s.Charset[rand.IntN(len(s.Charset))]
	}
	return string(b)
}

func (s ShortLinkService) GetByShort(ctx context.Context, short string) (model.ShortLink, error) {
	return s.ShortLinkDB.GetByShort(ctx, short)
}
func (s ShortLinkService) GetByLink(ctx context.Context, link string) (model.ShortLink, error) {
	return s.ShortLinkDB.GetByLink(ctx, link)
}
func (s ShortLinkService) GetByUserID(ctx context.Context, userID int) ([]model.ShortLink, error) {
	return s.ShortLinkDB.GetByUserID(ctx, userID)
}

func (s ShortLinkService) Save(ctx context.Context, link string, userID int) (model.ShortLink, error) {
	short := s.GenerateID()

	sl, err := s.ShortLinkDB.Save(ctx, short, link, userID)
	i := 0

	for i < 10 && err != nil && s.isDuplicateIDError(err) {
		// retry up to 10 times to save if id is not unique
		short = s.GenerateID()
		sl, err = s.ShortLinkDB.Save(ctx, short, link, userID)
		i++
	}
	return sl, err
}

func (s ShortLinkService) isDuplicateIDError(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		// uniq index error
		if pgErr.Code == "23505" && pgErr.ConstraintName == "short_links_short_uidx" {
			return true
		}
	}
	var dError *repository.InMemoryDuplicateIDError
	if errors.As(err, &dError) {
		if dError.ID != "" {
			return true
		}
	}

	return false
}

func (s ShortLinkService) SaveBatch(ctx context.Context, userID int, batch []model.ShortLinkBatchItemRequest) ([]model.ShortLink, error) {
	sls := make([]model.ShortLink, 0, len(batch))

	for _, l := range batch {
		sls = append(sls, model.ShortLink{ID: s.GenerateID(), Link: l.OriginalURL})
	}
	// as i didn't understand, that CorrelationID != ShortID
	res, err := s.ShortLinkDB.SaveBatch(ctx, userID, sls)

	i := 0
	// there is retry to save ALL batch items if uniqe error is thrown
	// i desided not to over complicate repository, because GenerateID() is in service
	for i < 10 && err != nil && s.isDuplicateIDError(err) {
		// retry up to 10 times to save if id is not unique
		for j := range sls {
			sls[j].ID = s.GenerateID()
		}
		res, err = s.ShortLinkDB.SaveBatch(ctx, userID, sls)
		i++
	}
	return res, err
}

func (s ShortLinkService) DeleteBatch(ctx context.Context, userID int, shortIDs []string) error {
	// max number of ids in one batch sql request
	batchSize := 20
	l := len(shortIDs)
	// number of goruties
	w := l / batchSize
	if l%batchSize != 0 {
		w++
	}
	// run some gorutines
	for i := 0; i < w; i++ {
		go func() {
			s.DBSemaphore.Acquire()
			defer s.DBSemaphore.Release()
			j := i * batchSize
			k := min(j+batchSize, l)
			// task says that there is no need to notify
			err := s.ShortLinkDB.DeleteBatch(context.Background(), userID, shortIDs[j:k])
			if err != nil {
				logger.Log.Errorw("Cannot decode request JSON body", "error", err)
			}
		}() // no need to wait for gorutines
	}
	return nil
}
