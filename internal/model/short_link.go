package model

type ShortLink struct {
	ID   string
	Link string
}

type IShortLinkRepository interface {
	GetByShort(short string) (ShortLink, error)
	Save(id string, link string) (ShortLink, error)
	Close() error
}

type ShortLinkRequest struct {
	URL string `json:"url"`
}

type ShortLinkResponse struct {
	Result string `json:"result"`
}
