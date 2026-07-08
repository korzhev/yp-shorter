package model

import (
	"sync"
)

type ShortLink struct {
	ID   string
	Link string
}

type ShortLinkStorage struct {
	sync.RWMutex
	M map[string]ShortLink
}

type IShortLinkRepository interface {
	GetById(id string) (ShortLink, error)
	Save(id string, link string) (ShortLink, error)
}
