package model

import "context"

type ShortLink struct {
	ID   string 
	Link string 
}

type IShortLinkRepository interface {
	GetByShort(ctx context.Context, short string) (ShortLink, error)
	GetByLink(ctx context.Context, link string) (ShortLink, error)
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

type ShortLinkBatchItemRequest struct {
	CorrelationID string `json:"correlation_id"`
	OriginalURL string `json:"original_url"`
}

type ShortLinkBatchItemResponse struct {
	CorrelationID string `json:"correlation_id"`
	ShortURL string `json:"short_url"`
}
