package model

import (
	"os"
	"sync"
)

type ShortLink struct {
	ID   string
	Link string
}

type ShortLinkStorage struct {
	sync.RWMutex
	M map[string]ShortLink
	F *os.File
}

type IShortLinkRepository interface {
	GetById(id string) (ShortLink, error)
	Save(id string, link string) (ShortLink, error)
}

type ShortLinkRequest struct {
	URL string `json:"url"`
}

type ShortLinkResponse struct {
	Result string `json:"result"`
}
