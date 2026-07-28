package model

import "context"

type ShortLink struct {
	ID   string
	Link string
}

type IShortLinkRepository interface {
	GetByShort(ctx context.Context, short string) (ShortLink, error)
	Save(ctx context.Context, id string, link string) (ShortLink, error)
	Close() error
}

type ShortLinkRequest struct {
	URL string `json:"url"`
}

type ShortLinkResponse struct {
	Result string `json:"result"`
}
