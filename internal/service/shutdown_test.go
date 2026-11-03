package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/korzhev/yp-shorter/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestSemaphoreWaitDrainsQueuedDeletions(t *testing.T) {
	semaphore := NewSemaphore(1)
	semaphore.Acquire()
	repo := mocks.NewMockIShortLinkRepository(gomock.NewController(t))
	ids := make([]string, 41)
	for i := range ids {
		ids[i] = fmt.Sprint(i)
	}
	for i := 0; i < len(ids); i += 20 {
		repo.EXPECT().DeleteBatch(gomock.Any(), 42, ids[i:min(i+20, len(ids))]).Return(nil)
	}
	s := ShortLinkService{ShortLinkDB: repo, DBSemaphore: semaphore}
	require.NoError(t, s.DeleteBatch(context.Background(), 42, ids))
	done := make(chan struct{})
	go func() { semaphore.Wait(); close(done) }()
	select {
	case <-done:
		t.Error("Wait returned with queued deletions")
	case <-time.After(20 * time.Millisecond):
	}
	semaphore.Release()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("deletions did not finish")
	}
}

func TestAuditPublisherWaitDrainsEvents(t *testing.T) {
	var publisher AuditPublisher
	release := make(chan struct{})
	publisher.Register("slow", &auditRepositoryStub{save: func(_, _, _ string) error {
		<-release
		return nil
	}})
	publisher.Publish("shorten", 42, "https://example.com")
	done := make(chan struct{})
	go func() { publisher.Wait(); close(done) }()
	select {
	case <-done:
		t.Error("Wait returned before audit delivery")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("audit did not finish")
	}
}
