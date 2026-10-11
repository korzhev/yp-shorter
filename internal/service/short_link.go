package service

import (
	"context"
	"errors"
	"math/rand/v2"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/korzhev/yp-shorter/internal/logger"
	"github.com/korzhev/yp-shorter/internal/model"
	"github.com/korzhev/yp-shorter/internal/repository"
)

// IShortLinkService defines ID generation and short link operations.
type IShortLinkService interface {
	// GenerateID creates a candidate short ID without checking uniqueness.
	GenerateID() string
	// GetByShort retrieves a link by its short ID.
	GetByShort(ctx context.Context, id string) (model.ShortLink, error)
	// GetByLink retrieves a non-deleted link by its original URL.
	GetByLink(ctx context.Context, Link string) (model.ShortLink, error)
	// GetByUserID returns non-deleted links owned by userID.
	GetByUserID(ctx context.Context, userID int) ([]model.ShortLink, error)
	// Save generates a short ID and stores the URL for userID.
	Save(ctx context.Context, link string, userID int) (model.ShortLink, error)
	// SaveBatch generates IDs and stores a batch of URLs for userID.
	SaveBatch(ctx context.Context, userID int, batch []model.ShortLinkBatchItemRequest) ([]model.ShortLink, error)
	// DeleteBatch schedules deletion of the supplied short IDs owned by userID.
	DeleteBatch(ctx context.Context, userID int, batch []string) error
}

// Semaphore limits concurrent operations using a buffered channel.
// Construct it with NewSemaphore; its zero value is not usable.
type Semaphore struct {
	semaCh chan struct{}
}

// NewSemaphore creates a semaphore allowing maxReq concurrent operations.
// maxReq must be positive for Acquire to make progress without a matching Release.
func NewSemaphore(maxReq int) *Semaphore {
	return &Semaphore{
		semaCh: make(chan struct{}, maxReq),
	}
}

// Acquire reserves a slot, blocking until one becomes available.
func (s *Semaphore) Acquire() {
	s.semaCh <- struct{}{}
}

// Release frees a previously acquired slot. It blocks if no slot is held.
func (s *Semaphore) Release() {
	<-s.semaCh
}

// ShortLinkService generates short IDs and coordinates repository operations.
type ShortLinkService struct {
	// Charset contains the bytes available for ID generation.
	Charset string
	// IDLength is the generated short ID length in bytes.
	IDLength int
	// ShortLinkDB provides persistent or in-memory link storage.
	ShortLinkDB model.IShortLinkRepository
	// DBSemaphore limits concurrent background deletion operations.
	DBSemaphore *Semaphore
}

// GenerateID returns a random IDLength-byte ID drawn from Charset.
// It does not guarantee uniqueness. IDLength must be nonnegative, and Charset
// must be nonempty when IDLength is positive.
func (s ShortLinkService) GenerateID() string {
	b := make([]byte, s.IDLength)
	for i := range b {
		b[i] = s.Charset[rand.IntN(len(s.Charset))]
	}
	return string(b)
}

// GetByShort delegates lookup by short ID to the repository.
func (s ShortLinkService) GetByShort(ctx context.Context, short string) (model.ShortLink, error) {
	return s.ShortLinkDB.GetByShort(ctx, short)
}

// GetByLink retrieves a non-deleted link by its original URL from the repository.
func (s ShortLinkService) GetByLink(ctx context.Context, link string) (model.ShortLink, error) {
	return s.ShortLinkDB.GetByLink(ctx, link)
}

// GetByUserID retrieves non-deleted links owned by userID from the repository.
func (s ShortLinkService) GetByUserID(ctx context.Context, userID int) ([]model.ShortLink, error) {
	return s.ShortLinkDB.GetByUserID(ctx, userID)
}

// Save generates an ID and stores link for userID. On a recognized ID collision,
// it retries with a new ID up to ten times after the initial attempt.
func (s ShortLinkService) Save(ctx context.Context, link string, userID int) (model.ShortLink, error) {
	short := s.GenerateID()

	sl, err := s.ShortLinkDB.Save(ctx, short, link, userID)
	i := 0

	for i < 10 && err != nil && s.isDuplicateIDError(err) {
		// retry up to 10 times to save if id is not unique
		short = s.GenerateID()
		sl, err = s.ShortLinkDB.Save(ctx, short, link, userID)
		i++
	}
	return sl, err
}

func (s ShortLinkService) isDuplicateIDError(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		// uniq index error
		if pgErr.Code == "23505" && pgErr.ConstraintName == "short_links_short_uidx" {
			return true
		}
	}
	var dError *repository.InMemoryDuplicateIDError
	if errors.As(err, &dError) {
		if dError.ID != "" {
			return true
		}
	}

	return false
}

// SaveBatch generates IDs and stores the batch's original URLs for userID in
// input order. Correlation IDs are not stored. On a recognized ID collision,
// it regenerates every ID and retries the whole batch up to ten times.
func (s ShortLinkService) SaveBatch(ctx context.Context, userID int, batch []model.ShortLinkBatchItemRequest) ([]model.ShortLink, error) {
	sls := make([]model.ShortLink, 0, len(batch))

	for _, l := range batch {
		sls = append(sls, model.ShortLink{ID: s.GenerateID(), Link: l.OriginalURL})
	}
	// as i didn't understand, that CorrelationID != ShortID
	res, err := s.ShortLinkDB.SaveBatch(ctx, userID, sls)

	i := 0
	// there is retry to save ALL batch items if uniqe error is thrown
	// i desided not to over complicate repository, because GenerateID() is in service
	for i < 10 && err != nil && s.isDuplicateIDError(err) {
		// retry up to 10 times to save if id is not unique
		for j := range sls {
			sls[j].ID = s.GenerateID()
		}
		res, err = s.ShortLinkDB.SaveBatch(ctx, userID, sls)
		i++
	}
	return res, err
}

// DeleteBatch schedules soft deletion of userID's links in groups of up to 20.
// Background operations use DBSemaphore and are independent of ctx cancellation.
// It returns nil immediately without waiting; deletion errors are logged.
func (s ShortLinkService) DeleteBatch(ctx context.Context, userID int, shortIDs []string) error {
	// max number of ids in one batch sql request
	batchSize := 20
	l := len(shortIDs)
	// number of goruties
	w := l / batchSize
	if l%batchSize != 0 {
		w++
	}
	// run some gorutines
	for i := 0; i < w; i++ {
		go func() {
			s.DBSemaphore.Acquire()
			defer s.DBSemaphore.Release()
			j := i * batchSize
			k := min(j+batchSize, l)
			// task says that there is no need to notify
			err := s.ShortLinkDB.DeleteBatch(context.Background(), userID, shortIDs[j:k])
			if err != nil {
				logger.Log.Errorw("Cannot decode request JSON body", "error", err)
			}
		}() // no need to wait for gorutines
	}
	return nil
}
