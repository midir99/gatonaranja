package main

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"
)

type receiveUpdatesCall struct {
	offset         int64
	timeoutSeconds int
}

type fakeTelegramBotClient struct {
	receiveUpdatesFunc  func(ctx context.Context, offset int64, timeoutSeconds int) ([]TelegramAPIUpdate, error)
	receiveUpdatesCalls []receiveUpdatesCall
}

func (c *fakeTelegramBotClient) ReceiveUpdates(
	ctx context.Context,
	offset int64,
	timeoutSeconds int,
) ([]TelegramAPIUpdate, error) {
	c.receiveUpdatesCalls = append(c.receiveUpdatesCalls, receiveUpdatesCall{
		offset:         offset,
		timeoutSeconds: timeoutSeconds,
	})
	if c.receiveUpdatesFunc != nil {
		return c.receiveUpdatesFunc(ctx, offset, timeoutSeconds)
	}
	return nil, nil
}

func (c *fakeTelegramBotClient) SendText(
	ctx context.Context,
	chatID int64,
	replyToMessageID int64,
	text string,
) (*TelegramAPIMessage, error) {
	panic("unexpected SendText call in RunTelegramBot test")
}

func (c *fakeTelegramBotClient) SendVideo(
	ctx context.Context,
	chatID int64,
	replyToMessageID int64,
	videoPath string,
) (*TelegramAPIMessage, error) {
	panic("unexpected SendVideo call in RunTelegramBot test")
}

func (c *fakeTelegramBotClient) SendAudio(
	ctx context.Context,
	chatID int64,
	replyToMessageID int64,
	audioPath string,
) (*TelegramAPIMessage, error) {
	panic("unexpected SendAudio call in RunTelegramBot test")
}

func newTelegramBotTestLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

func TestRunTelegramBotValidatesArguments(t *testing.T) {
	logger := newTelegramBotTestLogger()
	client := &fakeTelegramBotClient{}
	handler := func(context.Context, TelegramAPIUpdate) error { return nil }

	testCases := []struct {
		name    string
		ctx     context.Context
		client  TelegramBotClient
		logger  *slog.Logger
		handler func(context.Context, TelegramAPIUpdate) error
		wantErr string
	}{
		{
			name:    "nil client",
			ctx:     context.Background(),
			logger:  logger,
			handler: handler,
			wantErr: "telegram API client is required",
		},
		{
			name:    "nil logger",
			ctx:     context.Background(),
			client:  client,
			handler: handler,
			wantErr: "logger is required",
		},
		{
			name:    "nil handler",
			ctx:     context.Background(),
			client:  client,
			logger:  logger,
			wantErr: "update handler is required",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := RunTelegramBot(tc.ctx, tc.client, tc.logger, tc.handler)
			if err == nil {
				t.Fatalf("RunTelegramBot() error = nil, want %q", tc.wantErr)
			}
			if got := err.Error(); got != tc.wantErr {
				t.Fatalf("RunTelegramBot() error = %q, want %q", got, tc.wantErr)
			}
		})
	}
}

func TestRunTelegramBotProcessesUpdatesAndAdvancesOffset(t *testing.T) {
	client := &fakeTelegramBotClient{}
	client.receiveUpdatesFunc = func(
		ctx context.Context,
		offset int64,
		timeoutSeconds int,
	) ([]TelegramAPIUpdate, error) {
		switch len(client.receiveUpdatesCalls) {
		case 1:
			return []TelegramAPIUpdate{
				{UpdateID: 5},
				{UpdateID: 8},
			}, nil
		case 2:
			return nil, context.Canceled
		default:
			t.Fatalf("ReceiveUpdates() called too many times")
			return nil, nil
		}
	}

	var gotUpdateIDs []int64
	handler := func(ctx context.Context, update TelegramAPIUpdate) error {
		gotUpdateIDs = append(gotUpdateIDs, update.UpdateID)
		return nil
	}

	err := RunTelegramBot(context.Background(), client, newTelegramBotTestLogger(), handler)
	if err != nil {
		t.Fatalf("RunTelegramBot() error = %v, want nil", err)
	}

	if got, want := len(client.receiveUpdatesCalls), 2; got != want {
		t.Fatalf("len(receiveUpdatesCalls) = %d, want %d", got, want)
	}
	if got, want := client.receiveUpdatesCalls[0].offset, int64(0); got != want {
		t.Fatalf("first offset = %d, want %d", got, want)
	}
	if got, want := client.receiveUpdatesCalls[1].offset, int64(9); got != want {
		t.Fatalf("second offset = %d, want %d", got, want)
	}
	if got, want := client.receiveUpdatesCalls[0].timeoutSeconds, telegramUpdatePollTimeoutSeconds; got != want {
		t.Fatalf("first timeoutSeconds = %d, want %d", got, want)
	}
	if got, want := client.receiveUpdatesCalls[1].timeoutSeconds, telegramUpdatePollTimeoutSeconds; got != want {
		t.Fatalf("second timeoutSeconds = %d, want %d", got, want)
	}
	if len(gotUpdateIDs) != 2 || gotUpdateIDs[0] != 5 || gotUpdateIDs[1] != 8 {
		t.Fatalf("got update IDs %v, want [5 8]", gotUpdateIDs)
	}
}

func TestRunTelegramBotContinuesWhenHandlerReturnsError(t *testing.T) {
	client := &fakeTelegramBotClient{}
	client.receiveUpdatesFunc = func(
		ctx context.Context,
		offset int64,
		timeoutSeconds int,
	) ([]TelegramAPIUpdate, error) {
		switch len(client.receiveUpdatesCalls) {
		case 1:
			return []TelegramAPIUpdate{
				{UpdateID: 1},
				{UpdateID: 2},
			}, nil
		case 2:
			return nil, context.Canceled
		default:
			t.Fatalf("ReceiveUpdates() called too many times")
			return nil, nil
		}
	}

	var gotUpdateIDs []int64
	handler := func(ctx context.Context, update TelegramAPIUpdate) error {
		gotUpdateIDs = append(gotUpdateIDs, update.UpdateID)
		if update.UpdateID == 1 {
			return errors.New("boom")
		}
		return nil
	}

	err := RunTelegramBot(context.Background(), client, newTelegramBotTestLogger(), handler)
	if err != nil {
		t.Fatalf("RunTelegramBot() error = %v, want nil", err)
	}

	if len(gotUpdateIDs) != 2 || gotUpdateIDs[0] != 1 || gotUpdateIDs[1] != 2 {
		t.Fatalf("got update IDs %v, want [1 2]", gotUpdateIDs)
	}
}

func TestRunTelegramBotRetriesAfterReceiveError(t *testing.T) {
	originalAfterRetryDelay := afterRetryDelay
	defer func() { afterRetryDelay = originalAfterRetryDelay }()

	afterRetryDelay = func(time.Duration) <-chan time.Time {
		ch := make(chan time.Time, 1)
		ch <- time.Now()
		return ch
	}

	client := &fakeTelegramBotClient{}
	client.receiveUpdatesFunc = func(
		ctx context.Context,
		offset int64,
		timeoutSeconds int,
	) ([]TelegramAPIUpdate, error) {
		switch len(client.receiveUpdatesCalls) {
		case 1:
			return nil, errors.New("temporary failure")
		case 2:
			return nil, context.Canceled
		default:
			t.Fatalf("ReceiveUpdates() called too many times")
			return nil, nil
		}
	}

	err := RunTelegramBot(
		context.Background(),
		client,
		newTelegramBotTestLogger(),
		func(context.Context, TelegramAPIUpdate) error { return nil },
	)
	if err != nil {
		t.Fatalf("RunTelegramBot() error = %v, want nil", err)
	}

	if got, want := len(client.receiveUpdatesCalls), 2; got != want {
		t.Fatalf("len(receiveUpdatesCalls) = %d, want %d", got, want)
	}
	if got, want := client.receiveUpdatesCalls[0].offset, int64(0); got != want {
		t.Fatalf("first offset = %d, want %d", got, want)
	}
	if got, want := client.receiveUpdatesCalls[1].offset, int64(0); got != want {
		t.Fatalf("second offset = %d, want %d", got, want)
	}
}

func TestRunTelegramBotStopsImmediatelyWhenContextIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := &fakeTelegramBotClient{}
	client.receiveUpdatesFunc = func(
		ctx context.Context,
		offset int64,
		timeoutSeconds int,
	) ([]TelegramAPIUpdate, error) {
		t.Fatal("ReceiveUpdates() should not be called when context is already canceled")
		return nil, nil
	}

	err := RunTelegramBot(
		ctx,
		client,
		newTelegramBotTestLogger(),
		func(context.Context, TelegramAPIUpdate) error { return nil },
	)
	if err != nil {
		t.Fatalf("RunTelegramBot() error = %v, want nil", err)
	}

	if got, want := len(client.receiveUpdatesCalls), 0; got != want {
		t.Fatalf("len(receiveUpdatesCalls) = %d, want %d", got, want)
	}
}

func TestRunTelegramBotStopsWhenClientReturnsDeadlineExceeded(t *testing.T) {
	client := &fakeTelegramBotClient{}
	client.receiveUpdatesFunc = func(
		ctx context.Context,
		offset int64,
		timeoutSeconds int,
	) ([]TelegramAPIUpdate, error) {
		return nil, context.DeadlineExceeded
	}

	err := RunTelegramBot(
		context.Background(),
		client,
		newTelegramBotTestLogger(),
		func(context.Context, TelegramAPIUpdate) error { return nil },
	)
	if err != nil {
		t.Fatalf("RunTelegramBot() error = %v, want nil", err)
	}

	if got, want := len(client.receiveUpdatesCalls), 1; got != want {
		t.Fatalf("len(receiveUpdatesCalls) = %d, want %d", got, want)
	}
}

func TestRunTelegramBotStopsDuringRetryWaitWhenContextIsCanceled(t *testing.T) {
	originalAfterRetryDelay := afterRetryDelay
	defer func() { afterRetryDelay = originalAfterRetryDelay }()

	retryWaitStarted := make(chan struct{}, 1)
	blockRetry := make(chan time.Time)

	afterRetryDelay = func(time.Duration) <-chan time.Time {
		retryWaitStarted <- struct{}{}
		return blockRetry
	}

	client := &fakeTelegramBotClient{
		receiveUpdatesFunc: func(ctx context.Context, offset int64, timeoutSeconds int) ([]TelegramAPIUpdate, error) {
			return nil, errors.New("temporary failure")
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- RunTelegramBot(ctx, client, newTelegramBotTestLogger(), func(context.Context, TelegramAPIUpdate) error {
			return nil
		})
	}()

	<-retryWaitStarted
	cancel()

	err := <-done
	if err != nil {
		t.Fatalf("RunTelegramBot() error = %v, want nil", err)
	}
}
