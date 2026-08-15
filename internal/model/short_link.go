package model

import "context"

type ShortLink struct {
	ID     string
	Link   string
	UserID int
}

type IShortLinkRepository interface {
	GetByShort(ctx context.Context, short string) (ShortLink, error)
	GetByLink(ctx context.Context, link string) (ShortLink, error)
	GetByUserID(ctx context.Context, userID int) ([]ShortLink, error)
	Save(ctx context.Context, id string, link string, userID int) (ShortLink, error)
	SaveBatch(ctx context.Context, userID int, batch []ShortLink) ([]ShortLink, error)
	Close() error
}

type ShortLinkRequest struct {
	URL string `json:"url"`
}

type ShortLinkResponse struct {
	Result string `json:"result"`
}

type UserShortLinkResponse struct {
	ShortURL string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}

type ShortLinkBatchItemRequest struct {
	CorrelationID string `json:"correlation_id"`
	OriginalURL   string `json:"original_url"`
}

type ShortLinkBatchItemResponse struct {
	CorrelationID string `json:"correlation_id"`
	ShortURL      string `json:"short_url"`
}
