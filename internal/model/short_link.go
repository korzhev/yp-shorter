package model

import "context"

// ShortLink associates a short ID with an original URL and its owner.
type ShortLink struct {
	// ID is the short identifier, without the base URL.
	ID string
	// Link is the original URL.
	Link string
	// UserID identifies the link's owner.
	UserID int
	// Deleted indicates that the link has been soft-deleted.
	Deleted bool
}

// IShortLinkRepository defines storage operations for short links.
type IShortLinkRepository interface {
	// GetByShort looks up a link by its short ID.
	GetByShort(ctx context.Context, short string) (ShortLink, error)
	// GetByLink looks up a non-deleted link by its original URL.
	GetByLink(ctx context.Context, link string) (ShortLink, error)
	// GetByUserID returns non-deleted links owned by userID.
	GetByUserID(ctx context.Context, userID int) ([]ShortLink, error)
	// Save stores a link with the supplied short ID and owner.
	Save(ctx context.Context, id string, link string, userID int) (ShortLink, error)
	// SaveBatch stores links with preassigned short IDs for userID.
	SaveBatch(ctx context.Context, userID int, batch []ShortLink) ([]ShortLink, error)
	// DeleteBatch soft-deletes the supplied short IDs owned by userID.
	DeleteBatch(ctx context.Context, userID int, batch []string) error
	// Close releases storage resources.
	Close() error
}

// ShortLinkRequest is the JSON request to shorten a single URL.
type ShortLinkRequest struct {
	// URL is the original URL to shorten.
	URL string `json:"url"`
}

// ShortLinkResponse is the JSON response to a single URL shortening request.
type ShortLinkResponse struct {
	// Result is the complete shortened URL.
	Result string `json:"result"`
}

// UserShortLinkResponse represents one link in a user's link listing.
type UserShortLinkResponse struct {
	// ShortURL is the complete shortened URL.
	ShortURL string `json:"short_url"`
	// OriginalURL is the original destination URL.
	OriginalURL string `json:"original_url"`
}

// ShortLinkBatchItemRequest represents one item in a batch shortening request.
type ShortLinkBatchItemRequest struct {
	// CorrelationID is the client-provided identifier echoed in the response.
	CorrelationID string `json:"correlation_id"`
	// OriginalURL is the URL to shorten.
	OriginalURL string `json:"original_url"`
}

// ShortLinkBatchItemResponse represents one result of a batch shortening request.
type ShortLinkBatchItemResponse struct {
	// CorrelationID matches the corresponding request item.
	CorrelationID string `json:"correlation_id"`
	// ShortURL is the complete shortened URL.
	ShortURL string `json:"short_url"`
}
