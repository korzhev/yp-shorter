package service

import (
	"math/rand"
	"time"

	"github.com/korzhev/yp-shorter/internal/model"
)

var source = rand.NewSource(time.Now().UnixNano())
var IDSourceRand = rand.New(source)

type IShortLinkService interface {
	GenerateID() string
	GetById(id string) (model.ShortLink, error)
	Save(link string) (model.ShortLink, error)
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

func (s ShortLinkService) GetById(id string) (model.ShortLink, error) {
	return s.ShortLinkDB.GetById(id)
}

func (s ShortLinkService) Save(link string) (model.ShortLink, error) {
	var id = s.GenerateID()

	var sl, err = s.ShortLinkDB.Save(id, link)
	i := 0

	for i < 10 && err != nil {
		// retry up to 10 times to save if id is not unique
		id = s.GenerateID()
		sl, err = s.ShortLinkDB.Save(id, link)
		i++
	}
	return sl, err
}
