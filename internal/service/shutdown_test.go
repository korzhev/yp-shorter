package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/korzhev/yp-shorter/internal/logger"
	"github.com/korzhev/yp-shorter/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func TestSemaphoreWaitDrainsQueuedDeletions(t *testing.T) {
	originalLogger := logger.Log
	logger.Log = zap.NewNop().Sugar()
	t.Cleanup(func() { logger.Log = originalLogger })
	expectedErr := errors.New("deletion failed")
	semaphore := NewSemaphore(1)
	semaphore.Acquire()
	repo := mocks.NewMockIShortLinkRepository(gomock.NewController(t))
	ids := make([]string, 41)
	for i := range ids {
		ids[i] = fmt.Sprint(i)
	}
	for i := 0; i < len(ids); i += 20 {
		var batchErr error
		if i == 0 {
			batchErr = expectedErr
		}
		repo.EXPECT().DeleteBatch(gomock.Any(), 42, ids[i:min(i+20, len(ids))]).Return(batchErr)
	}
	s := ShortLinkService{ShortLinkDB: repo, DBSemaphore: semaphore}
	require.NoError(t, s.DeleteBatch(context.Background(), 42, ids))
	done := make(chan error, 1)
	go func() { done <- semaphore.Wait() }()
	select {
	case <-done:
		t.Error("Wait returned with queued deletions")
	case <-time.After(20 * time.Millisecond):
	}
	semaphore.Release()
	select {
	case err := <-done:
		require.ErrorIs(t, err, expectedErr)
	case <-time.After(5 * time.Second):
		t.Fatal("deletions did not finish")
	}
}

func TestAuditPublisherWaitDrainsEvents(t *testing.T) {
	originalLogger := logger.Log
	logger.Log = zap.NewNop().Sugar()
	t.Cleanup(func() { logger.Log = originalLogger })
	var publisher AuditPublisher
	expectedErr := errors.New("audit failed")
	failed := make(chan struct{})
	publisher.Register("failing", &auditRepositoryStub{save: func(_, _, _ string) error {
		close(failed)
		return expectedErr
	}})
	release := make(chan struct{})
	publisher.Register("slow", &auditRepositoryStub{save: func(_, _, _ string) error {
		<-release
		return nil
	}})
	publisher.Publish("shorten", 42, "https://example.com")
	<-failed
	done := make(chan error, 1)
	go func() { done <- publisher.Wait() }()
	select {
	case <-done:
		t.Error("Wait returned before audit delivery")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	select {
	case err := <-done:
		require.ErrorIs(t, err, expectedErr)
	case <-time.After(5 * time.Second):
		t.Fatal("audit did not finish")
	}
}
