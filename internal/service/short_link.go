package service

import (
	"context"
	"errors"
	"math/rand"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/korzhev/yp-shorter/internal/model"
	"github.com/korzhev/yp-shorter/internal/repository"
)

var source = rand.NewSource(time.Now().UnixNano())
var IDSourceRand = rand.New(source)

type IShortLinkService interface {
	GenerateID() string
	GetByShort(ctx context.Context, id string) (model.ShortLink, error)
	GetByLink(ctx context.Context, Link string) (model.ShortLink, error)
	Save(ctx context.Context, link string) (model.ShortLink, error)
	SaveBatch(ctx context.Context, batch []model.ShortLinkBatchItemRequest) ([]model.ShortLink, error)
}

type ShortLinkService struct {
	Charset     string
	IDLength    int
	ShortLinkDB model.IShortLinkRepository
}

func (s ShortLinkService) GenerateID() string {
	b := make([]byte, s.IDLength)
	for i := range b {
		b[i] = s.Charset[IDSourceRand.Intn(len(s.Charset))]
	}
	return string(b)
}

func (s ShortLinkService) GetByShort(ctx context.Context, short string) (model.ShortLink, error) {
	return s.ShortLinkDB.GetByShort(ctx, short)
}
func (s ShortLinkService) GetByLink(ctx context.Context, link string) (model.ShortLink, error) {
	return s.ShortLinkDB.GetByLink(ctx, link)
}

func (s ShortLinkService) Save(ctx context.Context, link string) (model.ShortLink, error) {
	short := s.GenerateID()

	sl, err := s.ShortLinkDB.Save(ctx, short, link)
	i := 0

	for i < 10 && err != nil && s.isDuplicateIDError(err) {
		// retry up to 10 times to save if id is not unique
		short = s.GenerateID()
		sl, err = s.ShortLinkDB.Save(ctx, short, link)
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

func (s ShortLinkService) SaveBatch(ctx context.Context, batch []model.ShortLinkBatchItemRequest) ([]model.ShortLink, error) {
	sls := make([]model.ShortLink, 0, len(batch))

	for _, l := range batch {
		sls = append(sls, model.ShortLink{ID: s.GenerateID(), Link: l.OriginalURL})
	}
	// as i didn't understand, that CorrelationID != ShortID
	res, err := s.ShortLinkDB.SaveBatch(ctx, sls)

	i := 0
	// there is retry to save ALL batch items if uniqe error is thrown
	// i desided not to over complicate repository, because GenerateID() is in service
	for i < 10 && err != nil && s.isDuplicateIDError(err) {
		// retry up to 10 times to save if id is not unique
		for j := range sls {
			sls[j].ID = s.GenerateID()
		}
		res, err = s.ShortLinkDB.SaveBatch(ctx, sls)
		i++
	}
	return res, err
}
