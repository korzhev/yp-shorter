package model

import "context"

type ShortLink struct {
	ID   string `json:"correlation_id"`
	Link string `json:"original_url"`
}

type IShortLinkRepository interface {
	GetByShort(ctx context.Context, short string) (ShortLink, error)
	Save(ctx context.Context, id string, link string) (ShortLink, error)
	SaveBatch(ctx context.Context, batch []ShortLink) ([]ShortLink, error)
	Close() error
}

type ShortLinkRequest struct {
	URL string `json:"url"`
}

type ShortLinkResponse struct {
	Result string `json:"result"`
}

type ShortLinkBatchItemResponse struct {
	CorrelationID string `json:"correlation_id"`
	ShortURL string `json:"short_url"`
}
