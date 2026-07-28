package service

import (
	"context"
	"math/rand"
	"time"

	"github.com/korzhev/yp-shorter/internal/model"
)

var source = rand.NewSource(time.Now().UnixNano())
var IDSourceRand = rand.New(source)

type IShortLinkService interface {
	GenerateID() string
	GetByShort(ctx context.Context, id string) (model.ShortLink, error)
	Save(ctx context.Context, link string) (model.ShortLink, error)
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

func (s ShortLinkService) Save(ctx context.Context, link string) (model.ShortLink, error) {
	var short = s.GenerateID()

	var sl, err = s.ShortLinkDB.Save(ctx, short, link)
	i := 0

	for i < 10 && err != nil {
		// retry up to 10 times to save if id is not unique
		short = s.GenerateID()
		sl, err = s.ShortLinkDB.Save(ctx, short, link)
		i++
	}
	return sl, err
}
