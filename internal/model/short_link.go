package model

import "sync"

type ShorLink struct {
	ID   string
	Link string
}

type ShortLinkStorage struct {
	sync.RWMutex
	M map[string]ShorLink
}

type IShortLinkRepository interface {
	GetById(id string) (ShorLink, error)
	Save(id string, link string) (ShorLink, error)
}
