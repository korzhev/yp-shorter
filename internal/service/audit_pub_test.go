package service

import (
	"errors"
	"testing"
	"time"

	"github.com/korzhev/yp-shorter/internal/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

type auditRepositoryStub struct {
	save func(action, userID, url string) error
}

func (r *auditRepositoryStub) Save(action, userID, url string) error {
	return r.save(action, userID, url)
}

type auditPublishCall struct {
	action string
	userID string
	url    string
}

func recordingAuditRepository(calls chan<- auditPublishCall) *auditRepositoryStub {
	return &auditRepositoryStub{save: func(action, userID, url string) error {
		calls <- auditPublishCall{action: action, userID: userID, url: url}
		return nil
	}}
}

func receiveAuditPublishCall(t *testing.T, calls <-chan auditPublishCall) auditPublishCall {
	t.Helper()
	select {
	case call := <-calls:
		return call
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for audit Save")
		return auditPublishCall{}
	}
}

func TestAuditPublisherRegisterInitializesSubscribers(t *testing.T) {
	var publisher AuditPublisher
	repo := &auditRepositoryStub{}

	publisher.Register("file", repo)

	require.Len(t, publisher.subs, 1)
	assert.Same(t, repo, publisher.subs["file"])
}

func TestAuditPublisherRegisterPreservesExistingSubscribers(t *testing.T) {
	var publisher AuditPublisher
	fileRepo := &auditRepositoryStub{}
	httpRepo := &auditRepositoryStub{}

	publisher.Register("file", fileRepo)
	publisher.Register("http", httpRepo)

	require.Len(t, publisher.subs, 2)
	assert.Same(t, fileRepo, publisher.subs["file"])
	assert.Same(t, httpRepo, publisher.subs["http"])
}

func TestAuditPublisherRegisterReplacesSubscriber(t *testing.T) {
	var publisher AuditPublisher
	original := &auditRepositoryStub{}
	replacement := &auditRepositoryStub{}
	publisher.Register("file", original)

	publisher.Register("file", replacement)

	require.Len(t, publisher.subs, 1)
	assert.Same(t, replacement, publisher.subs["file"])
}

func TestAuditPublisherPublishWithoutSubscribers(t *testing.T) {
	var publisher AuditPublisher

	assert.NotPanics(t, func() {
		publisher.Publish("shorten", 42, "https://example.com")
	})
	assert.Nil(t, publisher.subs)
}

func TestAuditPublisherPublishPassesArguments(t *testing.T) {
	var publisher AuditPublisher
	calls := make(chan auditPublishCall, 1)
	publisher.Register("file", recordingAuditRepository(calls))

	publisher.Publish("shorten", 42, "https://example.com")

	assert.Equal(t, auditPublishCall{
		action: "shorten", userID: "42", url: "https://example.com",
	}, receiveAuditPublishCall(t, calls))
}

func TestAuditPublisherPublishNotifiesAllSubscribers(t *testing.T) {
	var publisher AuditPublisher
	fileCalls := make(chan auditPublishCall, 1)
	httpCalls := make(chan auditPublishCall, 1)
	publisher.Register("file", recordingAuditRepository(fileCalls))
	publisher.Register("http", recordingAuditRepository(httpCalls))

	publisher.Publish("follow", 123, "https://example.com/path")

	expected := auditPublishCall{
		action: "follow", userID: "123", url: "https://example.com/path",
	}
	assert.Equal(t, expected, receiveAuditPublishCall(t, fileCalls))
	assert.Equal(t, expected, receiveAuditPublishCall(t, httpCalls))
}

func TestAuditPublisherPublishWithZeroUserIDAndEmptyFields(t *testing.T) {
	var publisher AuditPublisher
	calls := make(chan auditPublishCall, 1)
	publisher.Register("file", recordingAuditRepository(calls))

	publisher.Publish("", 0, "")

	assert.Equal(t, auditPublishCall{userID: "0"}, receiveAuditPublishCall(t, calls))
}

func TestAuditPublisherPublishWithNegativeUserID(t *testing.T) {
	var publisher AuditPublisher
	calls := make(chan auditPublishCall, 1)
	publisher.Register("file", recordingAuditRepository(calls))

	publisher.Publish("follow", -42, "https://example.com")

	assert.Equal(t, auditPublishCall{
		action: "follow", userID: "-42", url: "https://example.com",
	}, receiveAuditPublishCall(t, calls))
}

func TestAuditPublisherPublishDoesNotWaitForSubscribers(t *testing.T) {
	var publisher AuditPublisher
	started := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan struct{})
	returned := make(chan struct{})
	fastCalls := make(chan auditPublishCall, 1)
	t.Cleanup(func() {
		close(release)
		select {
		case <-finished:
		case <-time.After(5 * time.Second):
			t.Error("timed out waiting for blocked subscriber to finish")
		}
	})
	publisher.Register("slow", &auditRepositoryStub{save: func(_, _, _ string) error {
		close(started)
		<-release
		close(finished)
		return nil
	}})
	publisher.Register("fast", recordingAuditRepository(fastCalls))

	go func() {
		publisher.Publish("shorten", 42, "https://example.com")
		close(returned)
	}()

	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("subscriber did not start")
	}
	select {
	case <-returned:
	case <-time.After(5 * time.Second):
		t.Fatal("Publish waited for a blocked subscriber")
	}
	assert.Equal(t, auditPublishCall{
		action: "shorten", userID: "42", url: "https://example.com",
	}, receiveAuditPublishCall(t, fastCalls))
}

func TestAuditPublisherPublishLogsErrorAndNotifiesOtherSubscribers(t *testing.T) {
	core, logs := observer.New(zapcore.ErrorLevel)
	originalLogger := logger.Log
	logger.Log = zap.New(core).Sugar()
	t.Cleanup(func() {
		logger.Log = originalLogger
	})

	var publisher AuditPublisher
	failedCalls := make(chan auditPublishCall, 1)
	successfulCalls := make(chan auditPublishCall, 1)
	expectedErr := errors.New("audit save failed")
	publisher.Register("failing", &auditRepositoryStub{save: func(action, userID, url string) error {
		failedCalls <- auditPublishCall{action: action, userID: userID, url: url}
		return expectedErr
	}})
	publisher.Register("successful", recordingAuditRepository(successfulCalls))

	publisher.Publish("shorten", 42, "https://example.com")

	expected := auditPublishCall{
		action: "shorten", userID: "42", url: "https://example.com",
	}
	assert.Equal(t, expected, receiveAuditPublishCall(t, failedCalls))
	assert.Equal(t, expected, receiveAuditPublishCall(t, successfulCalls))
	require.Eventually(t, func() bool {
		return logs.Len() == 1
	}, 5*time.Second, time.Millisecond)
	entry := logs.All()[0]
	assert.Equal(t, zapcore.ErrorLevel, entry.Level)
	assert.Contains(t, entry.Message, "Audit Error")
	assert.Contains(t, entry.Message, expectedErr.Error())
	assert.Contains(t, entry.Message, "shorten")
	assert.Contains(t, entry.Message, "42")
	assert.Contains(t, entry.Message, "https://example.com")
}
