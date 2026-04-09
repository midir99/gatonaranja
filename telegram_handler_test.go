package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type handlerSendTextCall struct {
	ctx              context.Context
	chatID           int64
	replyToMessageID int64
	text             string
}

type handlerSendMediaCall struct {
	ctx              context.Context
	chatID           int64
	replyToMessageID int64
	filePath         string
}

type handlerTestBotClient struct {
	sendTextErr    error
	sendVideoErr   error
	sendAudioErr   error
	sendTextCalls  []handlerSendTextCall
	sendVideoCalls []handlerSendMediaCall
	sendAudioCalls []handlerSendMediaCall
}

func (c *handlerTestBotClient) ReceiveUpdates(
	ctx context.Context,
	offset int64,
	timeoutSeconds int,
) ([]TelegramAPIUpdate, error) {
	panic("unexpected ReceiveUpdates call in telegram_handler test")
}

func (c *handlerTestBotClient) SendText(
	ctx context.Context,
	chatID int64,
	replyToMessageID int64,
	text string,
) (*TelegramAPIMessage, error) {
	c.sendTextCalls = append(c.sendTextCalls, handlerSendTextCall{
		ctx:              ctx,
		chatID:           chatID,
		replyToMessageID: replyToMessageID,
		text:             text,
	})
	if c.sendTextErr != nil {
		return nil, c.sendTextErr
	}
	return &TelegramAPIMessage{MessageID: 1, Chat: TelegramAPIChat{ID: chatID}, Text: text}, nil
}

func (c *handlerTestBotClient) SendVideo(
	ctx context.Context,
	chatID int64,
	replyToMessageID int64,
	videoPath string,
) (*TelegramAPIMessage, error) {
	c.sendVideoCalls = append(c.sendVideoCalls, handlerSendMediaCall{
		ctx:              ctx,
		chatID:           chatID,
		replyToMessageID: replyToMessageID,
		filePath:         videoPath,
	})
	if c.sendVideoErr != nil {
		return nil, c.sendVideoErr
	}
	return &TelegramAPIMessage{MessageID: 1, Chat: TelegramAPIChat{ID: chatID}}, nil
}

func (c *handlerTestBotClient) SendAudio(
	ctx context.Context,
	chatID int64,
	replyToMessageID int64,
	audioPath string,
) (*TelegramAPIMessage, error) {
	c.sendAudioCalls = append(c.sendAudioCalls, handlerSendMediaCall{
		ctx:              ctx,
		chatID:           chatID,
		replyToMessageID: replyToMessageID,
		filePath:         audioPath,
	})
	if c.sendAudioErr != nil {
		return nil, c.sendAudioErr
	}
	return &TelegramAPIMessage{MessageID: 1, Chat: TelegramAPIChat{ID: chatID}}, nil
}

type handlerTestMediaDownloader struct {
	filename            string
	err                 error
	mediaKind           MediaKind
	downloadCalled      bool
	downloadCtx         context.Context
	downloadHasDeadline bool
	downloadDeadline    time.Time
}

func (d *handlerTestMediaDownloader) Download(ctx context.Context) (string, error) {
	d.downloadCalled = true
	d.downloadCtx = ctx
	d.downloadDeadline, d.downloadHasDeadline = ctx.Deadline()
	if d.err != nil {
		return "", d.err
	}
	return d.filename, nil
}

func (d *handlerTestMediaDownloader) MediaKind() MediaKind {
	return d.mediaKind
}

func newHandlerTestLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

func newHandlerBufferLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewTextHandler(buf, nil))
}

func newHandlerTestMessage(text string) *TelegramAPIMessage {
	return &TelegramAPIMessage{
		MessageID: 99,
		Text:      text,
		Chat:      TelegramAPIChat{ID: 12345},
		From: &TelegramAPIUser{
			ID:       777,
			UserName: "arthur",
		},
	}
}

func waitForWaitGroup(t *testing.T, wg *sync.WaitGroup) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("wait group did not finish in time")
	}
}

func telegramHandlerHelperOutputPath() string {
	return filepath.Join(os.TempDir(), "gatonaranja-telegram-handler-helper-file.mp4")
}

type secondDoneCanceledContext struct {
	context.Context
	mu         sync.Mutex
	doneCalls  int
	openDone   chan struct{}
	closedDone chan struct{}
}

func newSecondDoneCanceledContext() *secondDoneCanceledContext {
	closedDone := make(chan struct{})
	close(closedDone)

	return &secondDoneCanceledContext{
		Context:    context.Background(),
		openDone:   make(chan struct{}),
		closedDone: closedDone,
	}
}

func (c *secondDoneCanceledContext) Done() <-chan struct{} {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.doneCalls++
	if c.doneCalls == 1 {
		return c.openDone
	}
	return c.closedDone
}

func (c *secondDoneCanceledContext) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.doneCalls >= 2 {
		return context.Canceled
	}
	return nil
}

func TestNewDownloadRequestHandler(t *testing.T) {
	logger := newHandlerTestLogger()
	downloadQueue := make(chan DownloadJob, 1)
	var downloadsWG sync.WaitGroup
	client := &handlerTestBotClient{}

	testCases := []struct {
		name    string
		client  TelegramBotClient
		logger  *slog.Logger
		queue   chan DownloadJob
		wg      *sync.WaitGroup
		wantErr string
	}{
		{
			name:    "nil client",
			logger:  logger,
			queue:   downloadQueue,
			wg:      &downloadsWG,
			wantErr: "telegram bot client is required",
		},
		{
			name:    "nil logger",
			client:  client,
			queue:   downloadQueue,
			wg:      &downloadsWG,
			wantErr: "logger is required",
		},
		{
			name:    "nil download queue",
			client:  client,
			logger:  logger,
			wg:      &downloadsWG,
			wantErr: "download queue channel is required",
		},
		{
			name:    "nil wait group",
			client:  client,
			logger:  logger,
			queue:   downloadQueue,
			wantErr: "downloads wait group is required",
		},
		{
			name:   "happy path",
			client: client,
			logger: logger,
			queue:  downloadQueue,
			wg:     &downloadsWG,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			handler, err := NewDownloadRequestHandler(
				tc.client,
				tc.logger,
				[]int64{777},
				2*time.Minute,
				"",
				tc.queue,
				tc.wg,
			)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("NewDownloadRequestHandler() error = nil, want %q", tc.wantErr)
				}
				if got := err.Error(); got != tc.wantErr {
					t.Fatalf("NewDownloadRequestHandler() error = %q, want %q", got, tc.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("NewDownloadRequestHandler() error = %v, want nil", err)
			}
			if handler == nil {
				t.Fatal("NewDownloadRequestHandler() = nil, want non-nil")
			}
		})
	}
}

func TestDownloadRequestHandlerHandleUpdateIgnoresNilMessageOrSender(t *testing.T) {
	client := &handlerTestBotClient{}
	handler, err := NewDownloadRequestHandler(
		client,
		newHandlerTestLogger(),
		[]int64{777},
		time.Minute,
		"",
		make(chan DownloadJob, 1),
		&sync.WaitGroup{},
	)
	if err != nil {
		t.Fatalf("NewDownloadRequestHandler() error = %v, want nil", err)
	}

	testCases := []struct {
		name   string
		update TelegramAPIUpdate
	}{
		{
			name:   "nil message",
			update: TelegramAPIUpdate{UpdateID: 1},
		},
		{
			name: "nil sender",
			update: TelegramAPIUpdate{
				UpdateID: 2,
				Message: &TelegramAPIMessage{
					MessageID: 99,
					Chat:      TelegramAPIChat{ID: 12345},
					Text:      "https://www.youtube.com/watch?v=AqjB8DGt85U",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if err := handler.HandleUpdate(context.Background(), tc.update); err != nil {
				t.Fatalf("HandleUpdate() error = %v, want nil", err)
			}
			if got, want := len(client.sendTextCalls), 0; got != want {
				t.Fatalf("len(sendTextCalls) = %d, want %d", got, want)
			}
		})
	}
}

func TestDownloadRequestHandlerHandleUpdateIgnoresRequestWhenShutdownInProgress(t *testing.T) {
	var buf bytes.Buffer
	logger := newHandlerBufferLogger(&buf)
	client := &handlerTestBotClient{}
	handler, err := NewDownloadRequestHandler(
		client,
		logger,
		[]int64{777},
		time.Minute,
		"",
		make(chan DownloadJob, 1),
		&sync.WaitGroup{},
	)
	if err != nil {
		t.Fatalf("NewDownloadRequestHandler() error = %v, want nil", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = handler.HandleUpdate(ctx, TelegramAPIUpdate{
		UpdateID: 1,
		Message:  newHandlerTestMessage("https://www.youtube.com/watch?v=AqjB8DGt85U"),
	})
	if err != nil {
		t.Fatalf("HandleUpdate() error = %v, want nil", err)
	}
	if got, want := len(client.sendTextCalls), 0; got != want {
		t.Fatalf("len(sendTextCalls) = %d, want %d", got, want)
	}
	if !strings.Contains(buf.String(), "Ignoring request because shutdown is in progress") {
		t.Fatalf("log output = %q, want shutdown message", buf.String())
	}
}

func TestDownloadRequestHandlerHandleUpdateRejectsUnauthorizedUser(t *testing.T) {
	client := &handlerTestBotClient{}
	handler, err := NewDownloadRequestHandler(
		client,
		newHandlerTestLogger(),
		[]int64{999},
		time.Minute,
		"",
		make(chan DownloadJob, 1),
		&sync.WaitGroup{},
	)
	if err != nil {
		t.Fatalf("NewDownloadRequestHandler() error = %v, want nil", err)
	}

	err = handler.HandleUpdate(context.Background(), TelegramAPIUpdate{
		UpdateID: 1,
		Message:  newHandlerTestMessage("https://www.youtube.com/watch?v=AqjB8DGt85U"),
	})
	if err != nil {
		t.Fatalf("HandleUpdate() error = %v, want nil", err)
	}
	if got, want := len(client.sendTextCalls), 1; got != want {
		t.Fatalf("len(sendTextCalls) = %d, want %d", got, want)
	}
	if got, want := client.sendTextCalls[0].text, "You are not authorized to use this bot 😾"; got != want {
		t.Fatalf("reply text = %q, want %q", got, want)
	}
}

func TestDownloadRequestHandlerHandleUpdateParseFailures(t *testing.T) {
	testCases := []struct {
		name      string
		text      string
		wantReply string
	}{
		{
			name:      "invalid YouTube URL",
			text:      "download a video clip",
			wantReply: "That does not look like a valid YouTube video URL 🤔\n\n" + usageMessage,
		},
		{
			name:      "invalid timestamp range",
			text:      "https://www.youtube.com/watch?v=IFbXnS1odNs invalid",
			wantReply: "I could not understand the time range 🤔\n\n" + usageMessage,
		},
		{
			name:      "generic parse failure",
			text:      "https://www.youtube.com/watch?v=IFbXnS1odNs 0:10-end invalid",
			wantReply: "I could not understand your request 🤔\n\n" + usageMessage,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := &handlerTestBotClient{}
			handler, err := NewDownloadRequestHandler(
				client,
				newHandlerTestLogger(),
				[]int64{777},
				time.Minute,
				"",
				make(chan DownloadJob, 1),
				&sync.WaitGroup{},
			)
			if err != nil {
				t.Fatalf("NewDownloadRequestHandler() error = %v, want nil", err)
			}

			err = handler.HandleUpdate(context.Background(), TelegramAPIUpdate{
				UpdateID: 1,
				Message:  newHandlerTestMessage(tc.text),
			})
			if err != nil {
				t.Fatalf("HandleUpdate() error = %v, want nil", err)
			}
			if got, want := len(client.sendTextCalls), 1; got != want {
				t.Fatalf("len(sendTextCalls) = %d, want %d", got, want)
			}
			if got := client.sendTextCalls[0].text; got != tc.wantReply {
				t.Fatalf("reply text = %q, want %q", got, tc.wantReply)
			}
		})
	}
}

func TestDownloadRequestHandlerHandleUpdateEnqueuesJobAndSendsAck(t *testing.T) {
	client := &handlerTestBotClient{}
	downloadQueue := make(chan DownloadJob, 1)
	var downloadsWG sync.WaitGroup
	handler, err := NewDownloadRequestHandler(
		client,
		newHandlerTestLogger(),
		[]int64{777},
		2*time.Minute,
		"/home/arthur/.config/gatonaranja/yt-dlp.conf",
		downloadQueue,
		&downloadsWG,
	)
	if err != nil {
		t.Fatalf("NewDownloadRequestHandler() error = %v, want nil", err)
	}

	message := newHandlerTestMessage("https://www.youtube.com/watch?v=AqjB8DGt85U 1:00-1:05 audio")
	err = handler.HandleUpdate(context.Background(), TelegramAPIUpdate{
		UpdateID: 1,
		Message:  message,
	})
	if err != nil {
		t.Fatalf("HandleUpdate() error = %v, want nil", err)
	}

	if got, want := len(client.sendTextCalls), 1; got != want {
		t.Fatalf("len(sendTextCalls) = %d, want %d", got, want)
	}
	if got, want := client.sendTextCalls[0].text, "Wait a minute (maybe more) ⏳"; got != want {
		t.Fatalf("reply text = %q, want %q", got, want)
	}

	select {
	case job := <-downloadQueue:
		if job.Message != message {
			t.Fatal("queued job message does not match original message")
		}
		if got, want := job.YTDLPConfig, "/home/arthur/.config/gatonaranja/yt-dlp.conf"; got != want {
			t.Fatalf("queued YTDLP config path = %q, want %q", got, want)
		}
		if got, want := job.DownloadRequest.MediaKind, MediaAudio; got != want {
			t.Fatalf("queued media kind = %v, want %v", got, want)
		}
		if got, want := job.DownloadRequest.StartSecond, 60; got != want {
			t.Fatalf("queued startSecond = %d, want %d", got, want)
		}
		if got, want := job.DownloadRequest.EndSecond, 65; got != want {
			t.Fatalf("queued endSecond = %d, want %d", got, want)
		}
		if got, want := job.DownloadRequest.SourceURL, "https://www.youtube.com/watch?v=AqjB8DGt85U"; got != want {
			t.Fatalf("queued sourceURL = %q, want %q", got, want)
		}
		downloadsWG.Done()
	default:
		t.Fatal("expected download job to be queued")
	}
	waitForWaitGroup(t, &downloadsWG)
}

func TestDownloadRequestHandlerHandleUpdateRejectsWhenQueueIsFull(t *testing.T) {
	client := &handlerTestBotClient{}
	downloadQueue := make(chan DownloadJob, 1)
	downloadQueue <- DownloadJob{}
	var downloadsWG sync.WaitGroup
	handler, err := NewDownloadRequestHandler(
		client,
		newHandlerTestLogger(),
		[]int64{777},
		time.Minute,
		"",
		downloadQueue,
		&downloadsWG,
	)
	if err != nil {
		t.Fatalf("NewDownloadRequestHandler() error = %v, want nil", err)
	}

	err = handler.HandleUpdate(context.Background(), TelegramAPIUpdate{
		UpdateID: 1,
		Message:  newHandlerTestMessage("https://www.youtube.com/watch?v=AqjB8DGt85U"),
	})
	if err != nil {
		t.Fatalf("HandleUpdate() error = %v, want nil", err)
	}

	if got, want := len(client.sendTextCalls), 1; got != want {
		t.Fatalf("len(sendTextCalls) = %d, want %d", got, want)
	}
	if got, want := client.sendTextCalls[0].text, "I'm too busy right now, please try again later 😵"; got != want {
		t.Fatalf("reply text = %q, want %q", got, want)
	}
	waitForWaitGroup(t, &downloadsWG)
}

func TestDownloadRequestHandlerHandleUpdateRejectsWhenShutdownStartsBeforeEnqueue(t *testing.T) {
	client := &handlerTestBotClient{}
	downloadQueue := make(chan DownloadJob)
	var downloadsWG sync.WaitGroup
	handler, err := NewDownloadRequestHandler(
		client,
		newHandlerTestLogger(),
		[]int64{777},
		time.Minute,
		"",
		downloadQueue,
		&downloadsWG,
	)
	if err != nil {
		t.Fatalf("NewDownloadRequestHandler() error = %v, want nil", err)
	}

	ctx := newSecondDoneCanceledContext()
	err = handler.HandleUpdate(ctx, TelegramAPIUpdate{
		UpdateID: 1,
		Message:  newHandlerTestMessage("https://www.youtube.com/watch?v=AqjB8DGt85U"),
	})
	if err != nil {
		t.Fatalf("HandleUpdate() error = %v, want nil", err)
	}

	if got, want := len(client.sendTextCalls), 1; got != want {
		t.Fatalf("len(sendTextCalls) = %d, want %d", got, want)
	}
	if got, want := client.sendTextCalls[0].text, "Not now, it's my time to sleep 😌"; got != want {
		t.Fatalf("reply text = %q, want %q", got, want)
	}
	waitForWaitGroup(t, &downloadsWG)
}

func TestHandleDownloadRequestDownloadFailure(t *testing.T) {
	client := &handlerTestBotClient{}
	downloader := &handlerTestMediaDownloader{
		err:       errors.New("download failed"),
		mediaKind: MediaVideo,
	}

	handleDownloadRequest(
		context.Background(),
		client,
		newHandlerTestLogger(),
		newHandlerTestMessage("https://www.youtube.com/watch?v=AqjB8DGt85U"),
		downloader,
		2*time.Minute,
	)

	if !downloader.downloadCalled {
		t.Fatal("Download() was not called")
	}
	if got, want := len(client.sendTextCalls), 1; got != want {
		t.Fatalf("len(sendTextCalls) = %d, want %d", got, want)
	}
	if got, want := client.sendTextCalls[0].text, "I could not download your request 😿"; got != want {
		t.Fatalf("reply text = %q, want %q", got, want)
	}
}

func TestHandleDownloadRequestAudioSuccessAndCleanup(t *testing.T) {
	client := &handlerTestBotClient{}
	downloader := &handlerTestMediaDownloader{
		filename:  "clip.mp3",
		mediaKind: MediaAudio,
	}

	productionRemoveFile := removeFile
	defer func() { removeFile = productionRemoveFile }()

	var removedFile string
	removeFile = func(name string) error {
		removedFile = name
		return nil
	}

	handleDownloadRequest(
		context.Background(),
		client,
		newHandlerTestLogger(),
		newHandlerTestMessage("https://www.youtube.com/watch?v=AqjB8DGt85U audio"),
		downloader,
		2*time.Minute,
	)

	if got, want := len(client.sendAudioCalls), 1; got != want {
		t.Fatalf("len(sendAudioCalls) = %d, want %d", got, want)
	}
	if got, want := client.sendAudioCalls[0].filePath, "clip.mp3"; got != want {
		t.Fatalf("audio filepath = %q, want %q", got, want)
	}
	if got, want := len(client.sendTextCalls), 0; got != want {
		t.Fatalf("len(sendTextCalls) = %d, want %d", got, want)
	}
	if removedFile != "clip.mp3" {
		t.Fatalf("removed file = %q, want %q", removedFile, "clip.mp3")
	}
	if !downloader.downloadHasDeadline {
		t.Fatal("download context has no deadline, want deadline")
	}
	sendDeadline, ok := client.sendAudioCalls[0].ctx.Deadline()
	if !ok {
		t.Fatal("send context has no deadline, want deadline")
	}
	if time.Until(sendDeadline) <= 0 {
		t.Fatal("send context deadline already expired")
	}
}

func TestHandleDownloadRequestSendFailureRepliesToUser(t *testing.T) {
	testCases := []struct {
		name      string
		sendErr   error
		wantReply string
	}{
		{
			name:      "generic send failure",
			sendErr:   errors.New("send failed"),
			wantReply: "I downloaded it, but I couldn't send it to you 🙀",
		},
		{
			name:      "telegram media too large",
			sendErr:   ErrTelegramMediaTooLarge,
			wantReply: "I downloaded it, but the file is too big for me to send on Telegram 😿",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := &handlerTestBotClient{sendVideoErr: tc.sendErr}
			downloader := &handlerTestMediaDownloader{
				filename:  "clip.mp4",
				mediaKind: MediaVideo,
			}

			productionRemoveFile := removeFile
			defer func() { removeFile = productionRemoveFile }()
			removeFile = func(string) error { return nil }

			handleDownloadRequest(
				context.Background(),
				client,
				newHandlerTestLogger(),
				newHandlerTestMessage("https://www.youtube.com/watch?v=AqjB8DGt85U"),
				downloader,
				2*time.Minute,
			)

			if got, want := len(client.sendVideoCalls), 1; got != want {
				t.Fatalf("len(sendVideoCalls) = %d, want %d", got, want)
			}
			if got, want := len(client.sendTextCalls), 1; got != want {
				t.Fatalf("len(sendTextCalls) = %d, want %d", got, want)
			}
			if got := client.sendTextCalls[0].text; got != tc.wantReply {
				t.Fatalf("reply text = %q, want %q", got, tc.wantReply)
			}
		})
	}
}

func TestHandleDownloadRequestUnsupportedMediaKind(t *testing.T) {
	client := &handlerTestBotClient{}
	downloader := &handlerTestMediaDownloader{
		filename:  "clip.bin",
		mediaKind: MediaKind(99),
	}

	productionRemoveFile := removeFile
	defer func() { removeFile = productionRemoveFile }()
	removeFile = func(string) error { return nil }

	handleDownloadRequest(
		context.Background(),
		client,
		newHandlerTestLogger(),
		newHandlerTestMessage("https://www.youtube.com/watch?v=AqjB8DGt85U"),
		downloader,
		2*time.Minute,
	)

	if got, want := len(client.sendVideoCalls), 0; got != want {
		t.Fatalf("len(sendVideoCalls) = %d, want %d", got, want)
	}
	if got, want := len(client.sendAudioCalls), 0; got != want {
		t.Fatalf("len(sendAudioCalls) = %d, want %d", got, want)
	}
	if got, want := len(client.sendTextCalls), 1; got != want {
		t.Fatalf("len(sendTextCalls) = %d, want %d", got, want)
	}
	if got, want := client.sendTextCalls[0].text, "I downloaded it, but I couldn't send it to you 🙀"; got != want {
		t.Fatalf("reply text = %q, want %q", got, want)
	}
}

func TestHandleDownloadRequestLogsCleanupFailure(t *testing.T) {
	var buf bytes.Buffer
	client := &handlerTestBotClient{}
	downloader := &handlerTestMediaDownloader{
		filename:  "clip.mp4",
		mediaKind: MediaVideo,
	}

	productionRemoveFile := removeFile
	defer func() { removeFile = productionRemoveFile }()
	removeFile = func(string) error { return errors.New("remove failed") }

	handleDownloadRequest(
		context.Background(),
		client,
		newHandlerBufferLogger(&buf),
		newHandlerTestMessage("https://www.youtube.com/watch?v=AqjB8DGt85U"),
		downloader,
		2*time.Minute,
	)

	if !strings.Contains(buf.String(), "Failed to remove downloaded file") {
		t.Fatalf("log output = %q, want cleanup warning", buf.String())
	}
}

func TestSendReply(t *testing.T) {
	client := &handlerTestBotClient{}
	message := newHandlerTestMessage("hello")

	sendReply(context.Background(), client, newHandlerTestLogger(), message, "Wait a minute")

	if got, want := len(client.sendTextCalls), 1; got != want {
		t.Fatalf("len(sendTextCalls) = %d, want %d", got, want)
	}
	if got, want := client.sendTextCalls[0].chatID, message.Chat.ID; got != want {
		t.Fatalf("chatID = %d, want %d", got, want)
	}
	if got, want := client.sendTextCalls[0].replyToMessageID, message.MessageID; got != want {
		t.Fatalf("replyToMessageID = %d, want %d", got, want)
	}
	if got, want := client.sendTextCalls[0].text, "Wait a minute"; got != want {
		t.Fatalf("text = %q, want %q", got, want)
	}
}

func TestSendReplyLogsErrorOnFailure(t *testing.T) {
	var buf bytes.Buffer
	client := &handlerTestBotClient{sendTextErr: errors.New("boom")}
	message := newHandlerTestMessage("hello")

	sendReply(context.Background(), client, newHandlerBufferLogger(&buf), message, "Wait a minute")

	if !strings.Contains(buf.String(), "Failed to send Telegram message") {
		t.Fatalf("log output = %q, want send error log", buf.String())
	}
}

func TestLogTelegramSendError(t *testing.T) {
	var buf bytes.Buffer

	logTelegramSendError(newHandlerBufferLogger(&buf), "arthur", 777, errors.New("boom"))

	logOutput := buf.String()
	if !strings.Contains(logOutput, "Failed to send Telegram message") {
		t.Fatalf("log output = %q, want message", logOutput)
	}
	if !strings.Contains(logOutput, "user_id=777") {
		t.Fatalf("log output = %q, want user_id", logOutput)
	}
	if !strings.Contains(logOutput, "user_name=arthur") {
		t.Fatalf("log output = %q, want user_name", logOutput)
	}
}

func TestTelegramHandlerHelperProcess(_ *testing.T) {
	if os.Getenv("GO_WANT_TELEGRAM_HANDLER_HELPER_PROCESS") != "1" {
		return
	}

	args := os.Args
	separatorIndex := -1
	for i, arg := range args {
		if arg == "--" {
			separatorIndex = i
			break
		}
	}
	if separatorIndex == -1 || separatorIndex+1 >= len(args) {
		fmt.Fprint(os.Stderr, "missing helper arguments")
		os.Exit(2)
	}

	helperArgs := args[separatorIndex+1:]
	mode := helperArgs[0]

	switch mode {
	case "success":
		if err := os.WriteFile(telegramHandlerHelperOutputPath(), []byte("video"), 0o600); err != nil {
			fmt.Fprint(os.Stderr, err.Error())
			os.Exit(2)
		}
		fmt.Fprint(os.Stdout, telegramHandlerHelperOutputPath())
		os.Exit(0)
	default:
		fmt.Fprint(os.Stderr, "unknown helper mode")
		os.Exit(2)
	}
}

func telegramHandlerHelperCommand(ctx context.Context, mode string, args ...string) *exec.Cmd {
	commandArgs := []string{
		"-test.run=TestTelegramHandlerHelperProcess",
		"--",
		mode,
	}
	commandArgs = append(commandArgs, args...)

	cmd := exec.CommandContext(ctx, os.Args[0], commandArgs...)
	cmd.Env = append(os.Environ(), "GO_WANT_TELEGRAM_HANDLER_HELPER_PROCESS=1")
	return cmd
}

func TestDownloadWorkerProcessesQueuedJob(t *testing.T) {
	client := &handlerTestBotClient{}
	var downloadsWG sync.WaitGroup
	downloadsWG.Add(1)
	jobs := make(chan DownloadJob, 1)
	jobs <- DownloadJob{
		Message: newHandlerTestMessage("https://www.youtube.com/watch?v=IFbXnS1odNs"),
		DownloadRequest: DownloadRequest{
			StartSecond: StartSecond,
			EndSecond:   EndSecond,
			SourceURL:   "https://www.youtube.com/watch?v=IFbXnS1odNs",
			MediaKind:   MediaVideo,
		},
		YTDLPConfig: "/home/arthur/.config/gatonaranja/yt-dlp.conf",
	}
	close(jobs)

	productionCommandContext := commandContext
	productionRemoveFile := removeFile
	defer func() {
		commandContext = productionCommandContext
		removeFile = productionRemoveFile
	}()

	var gotArgs []string
	commandContext = func(ctx context.Context, _ string, args ...string) *exec.Cmd {
		gotArgs = append([]string(nil), args...)
		return telegramHandlerHelperCommand(ctx, "success", args...)
	}
	removeFile = func(string) error { return nil }

	downloadWorker(
		context.Background(),
		newHandlerTestLogger(),
		1,
		client,
		time.Minute,
		jobs,
		&downloadsWG,
	)

	waitForWaitGroup(t, &downloadsWG)
	if got, want := len(client.sendVideoCalls), 1; got != want {
		t.Fatalf("len(sendVideoCalls) = %d, want %d", got, want)
	}
	if got, want := client.sendVideoCalls[0].filePath, telegramHandlerHelperOutputPath(); got != want {
		t.Fatalf("video filepath = %q, want %q", got, want)
	}
	if !strings.Contains(strings.Join(gotArgs, "\x00"), "--config-locations") {
		t.Fatalf("worker yt-dlp args = %v, want --config-locations", gotArgs)
	}
	if !strings.Contains(strings.Join(gotArgs, "\x00"), "/home/arthur/.config/gatonaranja/yt-dlp.conf") {
		t.Fatalf("worker yt-dlp args = %v, want configured yt-dlp config path", gotArgs)
	}
}
