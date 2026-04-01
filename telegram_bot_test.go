package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

type sendTextCall struct {
	chatID           int64
	replyToMessageID int64
	text             string
}

type sendMediaCall struct {
	chatID           int64
	replyToMessageID int64
	filePath         string
}

type fakeTelegramBotClient struct {
	receiveUpdatesFunc func(ctx context.Context, offset int64, timeoutSeconds int) ([]TelegramAPIUpdate, error)
	sendTextErr        error
	sendVideoErr       error
	sendAudioErr       error
	sendTextCalls      []sendTextCall
	sendVideoCalls     []sendMediaCall
	sendAudioCalls     []sendMediaCall
}

func (c *fakeTelegramBotClient) ReceiveUpdates(ctx context.Context, offset int64, timeoutSeconds int) ([]TelegramAPIUpdate, error) {
	if c.receiveUpdatesFunc != nil {
		return c.receiveUpdatesFunc(ctx, offset, timeoutSeconds)
	}
	return nil, nil
}

func (c *fakeTelegramBotClient) SendText(ctx context.Context, chatID int64, replyToMessageID int64, text string) (*TelegramAPIMessage, error) {
	c.sendTextCalls = append(c.sendTextCalls, sendTextCall{
		chatID:           chatID,
		replyToMessageID: replyToMessageID,
		text:             text,
	})
	if c.sendTextErr != nil {
		return nil, c.sendTextErr
	}
	return &TelegramAPIMessage{MessageID: 1, Chat: TelegramAPIChat{ID: chatID}, Text: text}, nil
}

func (c *fakeTelegramBotClient) SendVideo(ctx context.Context, chatID int64, replyToMessageID int64, videoPath string) (*TelegramAPIMessage, error) {
	c.sendVideoCalls = append(c.sendVideoCalls, sendMediaCall{
		chatID:           chatID,
		replyToMessageID: replyToMessageID,
		filePath:         videoPath,
	})
	if c.sendVideoErr != nil {
		return nil, c.sendVideoErr
	}
	return &TelegramAPIMessage{MessageID: 1, Chat: TelegramAPIChat{ID: chatID}}, nil
}

func (c *fakeTelegramBotClient) SendAudio(ctx context.Context, chatID int64, replyToMessageID int64, audioPath string) (*TelegramAPIMessage, error) {
	c.sendAudioCalls = append(c.sendAudioCalls, sendMediaCall{
		chatID:           chatID,
		replyToMessageID: replyToMessageID,
		filePath:         audioPath,
	})
	if c.sendAudioErr != nil {
		return nil, c.sendAudioErr
	}
	return &TelegramAPIMessage{MessageID: 1, Chat: TelegramAPIChat{ID: chatID}}, nil
}

type fakeMediaDownloader struct {
	filename    string
	err         error
	mediaKind   MediaKind
	gotTimeout  time.Duration
	downloadHit bool
}

func (d *fakeMediaDownloader) Download(ctx context.Context, timeout time.Duration) (string, error) {
	d.gotTimeout = timeout
	d.downloadHit = true
	if d.err != nil {
		return "", d.err
	}
	return d.filename, nil
}

func (d *fakeMediaDownloader) MediaKind() MediaKind {
	return d.mediaKind
}

func newTestLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

func newBufferLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewTextHandler(buf, nil))
}

func newTestMessage(text string) *TelegramAPIMessage {
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

func TestNewDownloadRequestHandler(t *testing.T) {
	logger := newTestLogger()
	downloadSlots := make(chan struct{}, 1)
	var downloadsWG sync.WaitGroup
	client := &fakeTelegramBotClient{}

	testCases := []struct {
		name    string
		client  TelegramBotClient
		logger  *slog.Logger
		slots   chan struct{}
		wg      *sync.WaitGroup
		wantErr string
	}{
		{
			name:    "nil client",
			logger:  logger,
			slots:   downloadSlots,
			wg:      &downloadsWG,
			wantErr: "telegram bot client is required",
		},
		{
			name:    "nil logger",
			client:  client,
			slots:   downloadSlots,
			wg:      &downloadsWG,
			wantErr: "logger is required",
		},
		{
			name:    "nil download slots",
			client:  client,
			logger:  logger,
			wg:      &downloadsWG,
			wantErr: "download slots channel is required",
		},
		{
			name:    "nil wait group",
			client:  client,
			logger:  logger,
			slots:   downloadSlots,
			wantErr: "downloads wait group is required",
		},
		{
			name:   "happy path",
			client: client,
			logger: logger,
			slots:  downloadSlots,
			wg:     &downloadsWG,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			handler, err := NewDownloadRequestHandler(tc.client, tc.logger, []int64{777}, 2*time.Minute, tc.slots, tc.wg)
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
				t.Fatal("NewDownloadRequestHandler() handler = nil, want non-nil")
			}
		})
	}
}

func TestSendReply(t *testing.T) {
	client := &fakeTelegramBotClient{}
	message := newTestMessage("hello")

	sendReply(context.Background(), client, newTestLogger(), message, "hi there")

	if got, want := len(client.sendTextCalls), 1; got != want {
		t.Fatalf("len(sendTextCalls) = %d, want %d", got, want)
	}
	call := client.sendTextCalls[0]
	if got, want := call.chatID, int64(12345); got != want {
		t.Fatalf("sendReply chatID = %d, want %d", got, want)
	}
	if got, want := call.replyToMessageID, int64(99); got != want {
		t.Fatalf("sendReply replyToMessageID = %d, want %d", got, want)
	}
	if got, want := call.text, "hi there"; got != want {
		t.Fatalf("sendReply text = %q, want %q", got, want)
	}
}

func TestLogTelegramSendError(t *testing.T) {
	var buf bytes.Buffer
	logger := newBufferLogger(&buf)

	logTelegramSendError(logger, "arthur", 777, errors.New("boom"))

	output := buf.String()
	for _, want := range []string{
		"Failed to send Telegram message",
		"user_id=777",
		"user_name=arthur",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("log output = %q, want it to contain %q", output, want)
		}
	}
	if !strings.Contains(output, `error=boom`) && !strings.Contains(output, `error="boom"`) {
		t.Fatalf("log output = %q, want it to contain the error field", output)
	}
}

func TestSendReplyLogsErrorOnSendFailure(t *testing.T) {
	var buf bytes.Buffer
	logger := newBufferLogger(&buf)
	client := &fakeTelegramBotClient{
		sendTextErr: errors.New("send exploded"),
	}
	message := newTestMessage("hello")

	sendReply(context.Background(), client, logger, message, "hi there")

	if got, want := len(client.sendTextCalls), 1; got != want {
		t.Fatalf("len(sendTextCalls) = %d, want %d", got, want)
	}
	output := buf.String()
	if !strings.Contains(output, "Failed to send Telegram message") {
		t.Fatalf("log output = %q, want it to contain the send failure message", output)
	}
	if !strings.Contains(output, "user_id=777") {
		t.Fatalf("log output = %q, want it to contain the user id", output)
	}
}

func TestHandleDownloadRequestDownloadFailure(t *testing.T) {
	client := &fakeTelegramBotClient{}
	message := newTestMessage("request")
	downloader := &fakeMediaDownloader{
		err: errors.New("boom"),
	}

	handleDownloadRequest(context.Background(), client, newTestLogger(), message, downloader, 2*time.Minute)

	if !downloader.downloadHit {
		t.Fatal("Download() was not called")
	}
	if got, want := downloader.gotTimeout, 2*time.Minute; got != want {
		t.Fatalf("download timeout = %v, want %v", got, want)
	}
	if got, want := len(client.sendTextCalls), 1; got != want {
		t.Fatalf("len(sendTextCalls) = %d, want %d", got, want)
	}
	if got, want := client.sendTextCalls[0].text, "I could not download your request 😿"; got != want {
		t.Fatalf("fallback text = %q, want %q", got, want)
	}
}

func TestHandleDownloadRequestMediaTooLarge(t *testing.T) {
	client := &fakeTelegramBotClient{
		sendVideoErr: ErrTelegramMediaTooLarge,
	}
	message := newTestMessage("request")
	downloader := &fakeMediaDownloader{
		filename:  "clip.mp4",
		mediaKind: MediaVideo,
	}

	productionRemoveFile := removeFile
	defer func() { removeFile = productionRemoveFile }()

	var removedPath string
	removeFile = func(path string) error {
		removedPath = path
		return nil
	}

	handleDownloadRequest(context.Background(), client, newTestLogger(), message, downloader, time.Minute)

	if got, want := len(client.sendVideoCalls), 1; got != want {
		t.Fatalf("len(sendVideoCalls) = %d, want %d", got, want)
	}
	if got, want := client.sendVideoCalls[0].filePath, "clip.mp4"; got != want {
		t.Fatalf("video file path = %q, want %q", got, want)
	}
	if got, want := len(client.sendTextCalls), 1; got != want {
		t.Fatalf("len(sendTextCalls) = %d, want %d", got, want)
	}
	if got, want := client.sendTextCalls[0].text, "I downloaded it, but the file is too big for me to send on Telegram 😿"; got != want {
		t.Fatalf("fallback text = %q, want %q", got, want)
	}
	if got, want := removedPath, "clip.mp4"; got != want {
		t.Fatalf("removed path = %q, want %q", got, want)
	}
}

func TestHandleDownloadRequestAudioSuccess(t *testing.T) {
	client := &fakeTelegramBotClient{}
	message := newTestMessage("request")
	downloader := &fakeMediaDownloader{
		filename:  "clip.mp3",
		mediaKind: MediaAudio,
	}

	productionRemoveFile := removeFile
	defer func() { removeFile = productionRemoveFile }()

	var removedPath string
	removeFile = func(path string) error {
		removedPath = path
		return nil
	}

	handleDownloadRequest(context.Background(), client, newTestLogger(), message, downloader, 30*time.Second)

	if got, want := len(client.sendAudioCalls), 1; got != want {
		t.Fatalf("len(sendAudioCalls) = %d, want %d", got, want)
	}
	if got, want := client.sendAudioCalls[0].filePath, "clip.mp3"; got != want {
		t.Fatalf("audio file path = %q, want %q", got, want)
	}
	if got, want := len(client.sendTextCalls), 0; got != want {
		t.Fatalf("len(sendTextCalls) = %d, want %d", got, want)
	}
	if got, want := removedPath, "clip.mp3"; got != want {
		t.Fatalf("removed path = %q, want %q", got, want)
	}
}

func TestHandleDownloadRequestGenericSendFailure(t *testing.T) {
	client := &fakeTelegramBotClient{
		sendVideoErr: errors.New("send exploded"),
	}
	message := newTestMessage("request")
	downloader := &fakeMediaDownloader{
		filename:  "clip.mp4",
		mediaKind: MediaVideo,
	}

	productionRemoveFile := removeFile
	defer func() { removeFile = productionRemoveFile }()

	removeFile = func(path string) error { return nil }

	handleDownloadRequest(context.Background(), client, newTestLogger(), message, downloader, time.Minute)

	if got, want := len(client.sendVideoCalls), 1; got != want {
		t.Fatalf("len(sendVideoCalls) = %d, want %d", got, want)
	}
	if got, want := len(client.sendTextCalls), 1; got != want {
		t.Fatalf("len(sendTextCalls) = %d, want %d", got, want)
	}
	if got, want := client.sendTextCalls[0].text, "I downloaded it, but I couldn't send it to you 🙀"; got != want {
		t.Fatalf("fallback text = %q, want %q", got, want)
	}
}

func TestHandleDownloadRequestUnsupportedMediaKind(t *testing.T) {
	client := &fakeTelegramBotClient{}
	message := newTestMessage("request")
	downloader := &fakeMediaDownloader{
		filename:  "clip.bin",
		mediaKind: MediaKind(99),
	}

	productionRemoveFile := removeFile
	defer func() { removeFile = productionRemoveFile }()

	var removedPath string
	removeFile = func(path string) error {
		removedPath = path
		return nil
	}

	handleDownloadRequest(context.Background(), client, newTestLogger(), message, downloader, time.Minute)

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
		t.Fatalf("fallback text = %q, want %q", got, want)
	}
	if got, want := removedPath, "clip.bin"; got != want {
		t.Fatalf("removed path = %q, want %q", got, want)
	}
}

func TestHandleDownloadRequestCleanupFailure(t *testing.T) {
	var buf bytes.Buffer
	logger := newBufferLogger(&buf)
	client := &fakeTelegramBotClient{}
	message := newTestMessage("request")
	downloader := &fakeMediaDownloader{
		filename:  "clip.mp4",
		mediaKind: MediaVideo,
	}

	productionRemoveFile := removeFile
	defer func() { removeFile = productionRemoveFile }()

	removeFile = func(path string) error {
		return errors.New("remove exploded")
	}

	handleDownloadRequest(context.Background(), client, logger, message, downloader, time.Minute)

	if got, want := len(client.sendVideoCalls), 1; got != want {
		t.Fatalf("len(sendVideoCalls) = %d, want %d", got, want)
	}
	if !strings.Contains(buf.String(), "Failed to remove downloaded file") {
		t.Fatalf("log output = %q, want it to contain the cleanup failure", buf.String())
	}
}

func TestDownloadRequestHandlerHandleUpdateUnauthorizedUser(t *testing.T) {
	client := &fakeTelegramBotClient{}
	logger := newTestLogger()
	downloadSlots := make(chan struct{}, 1)
	var downloadsWG sync.WaitGroup

	handler, err := NewDownloadRequestHandler(client, logger, []int64{999}, time.Minute, downloadSlots, &downloadsWG)
	if err != nil {
		t.Fatalf("NewDownloadRequestHandler() error = %v, want nil", err)
	}

	update := TelegramAPIUpdate{
		UpdateID: 1,
		Message:  newTestMessage("https://www.youtube.com/watch?v=AqjB8DGt85U"),
	}

	if err := handler.HandleUpdate(context.Background(), update); err != nil {
		t.Fatalf("HandleUpdate() error = %v, want nil", err)
	}

	if got, want := len(client.sendTextCalls), 1; got != want {
		t.Fatalf("len(sendTextCalls) = %d, want %d", got, want)
	}
	if got, want := client.sendTextCalls[0].text, "You are not authorized to use this bot 😾"; got != want {
		t.Fatalf("reply text = %q, want %q", got, want)
	}
}

func TestDownloadRequestHandlerHandleUpdateIgnoresNilMessage(t *testing.T) {
	client := &fakeTelegramBotClient{}
	handler, err := NewDownloadRequestHandler(client, newTestLogger(), []int64{777}, time.Minute, make(chan struct{}, 1), &sync.WaitGroup{})
	if err != nil {
		t.Fatalf("NewDownloadRequestHandler() error = %v, want nil", err)
	}

	err = handler.HandleUpdate(context.Background(), TelegramAPIUpdate{UpdateID: 1})
	if err != nil {
		t.Fatalf("HandleUpdate() error = %v, want nil", err)
	}
	if got, want := len(client.sendTextCalls), 0; got != want {
		t.Fatalf("len(sendTextCalls) = %d, want %d", got, want)
	}
}

func TestDownloadRequestHandlerHandleUpdateIgnoresNilSender(t *testing.T) {
	client := &fakeTelegramBotClient{}
	handler, err := NewDownloadRequestHandler(client, newTestLogger(), []int64{777}, time.Minute, make(chan struct{}, 1), &sync.WaitGroup{})
	if err != nil {
		t.Fatalf("NewDownloadRequestHandler() error = %v, want nil", err)
	}

	err = handler.HandleUpdate(context.Background(), TelegramAPIUpdate{
		UpdateID: 1,
		Message: &TelegramAPIMessage{
			MessageID: 99,
			Chat:      TelegramAPIChat{ID: 12345},
			Text:      "https://www.youtube.com/watch?v=AqjB8DGt85U",
		},
	})
	if err != nil {
		t.Fatalf("HandleUpdate() error = %v, want nil", err)
	}
	if got, want := len(client.sendTextCalls), 0; got != want {
		t.Fatalf("len(sendTextCalls) = %d, want %d", got, want)
	}
}

func TestDownloadRequestHandlerHandleUpdateIgnoresRequestWhenShutdownInProgress(t *testing.T) {
	var buf bytes.Buffer
	logger := newBufferLogger(&buf)
	client := &fakeTelegramBotClient{}
	handler, err := NewDownloadRequestHandler(client, logger, []int64{777}, time.Minute, make(chan struct{}, 1), &sync.WaitGroup{})
	if err != nil {
		t.Fatalf("NewDownloadRequestHandler() error = %v, want nil", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = handler.HandleUpdate(ctx, TelegramAPIUpdate{
		UpdateID: 1,
		Message:  newTestMessage("https://www.youtube.com/watch?v=AqjB8DGt85U"),
	})
	if err != nil {
		t.Fatalf("HandleUpdate() error = %v, want nil", err)
	}
	if got, want := len(client.sendTextCalls), 0; got != want {
		t.Fatalf("len(sendTextCalls) = %d, want %d", got, want)
	}
	if !strings.Contains(buf.String(), "Ignoring request because shutdown is in progress") {
		t.Fatalf("log output = %q, want it to contain the shutdown message", buf.String())
	}
}

func TestDownloadRequestHandlerHandleUpdateParseFailureYouTubeURL(t *testing.T) {
	client := &fakeTelegramBotClient{}
	logger := newTestLogger()
	downloadSlots := make(chan struct{}, 1)
	var downloadsWG sync.WaitGroup

	handler, err := NewDownloadRequestHandler(client, logger, []int64{777}, time.Minute, downloadSlots, &downloadsWG)
	if err != nil {
		t.Fatalf("NewDownloadRequestHandler() error = %v, want nil", err)
	}

	update := TelegramAPIUpdate{
		UpdateID: 1,
		Message:  newTestMessage("download a video clip"),
	}

	if err := handler.HandleUpdate(context.Background(), update); err != nil {
		t.Fatalf("HandleUpdate() error = %v, want nil", err)
	}

	if got, want := len(client.sendTextCalls), 1; got != want {
		t.Fatalf("len(sendTextCalls) = %d, want %d", got, want)
	}
	if got, want := client.sendTextCalls[0].text, "That does not look like a valid YouTube video URL 🤔\n\nSend only the YouTube link, optionally followed by:\n- a time range written with no spaces around the dash\n- the word audio at the end\n\nDo not write the time range like \"1:00 - 1:05\".\n\nSend a message exactly like one of these examples:\n\nhttps://www.youtube.com/watch?v=AqjB8DGt85U\n\nhttps://www.youtube.com/watch?v=AqjB8DGt85U 1:00-1:05\n\nhttps://www.youtube.com/watch?v=AqjB8DGt85U audio\n\nhttps://www.youtube.com/watch?v=AqjB8DGt85U 1:00-1:05 audio\n\nYou can also use start or end:\nhttps://www.youtube.com/watch?v=AqjB8DGt85U start-0:10\nhttps://www.youtube.com/watch?v=AqjB8DGt85U 0:10-end"; got != want {
		t.Fatalf("reply text = %q, want usageMessage", got)
	}
}

func TestDownloadRequestHandlerHandleUpdateParseFailureTimestampRange(t *testing.T) {
	client := &fakeTelegramBotClient{}
	logger := newTestLogger()
	downloadSlots := make(chan struct{}, 1)
	var downloadsWG sync.WaitGroup

	handler, err := NewDownloadRequestHandler(client, logger, []int64{777}, time.Minute, downloadSlots, &downloadsWG)
	if err != nil {
		t.Fatalf("NewDownloadRequestHandler() error = %v, want nil", err)
	}

	update := TelegramAPIUpdate{
		UpdateID: 1,
		Message:  newTestMessage("https://www.youtube.com/watch?v=IFbXnS1odNs invalid"),
	}

	if err := handler.HandleUpdate(context.Background(), update); err != nil {
		t.Fatalf("HandleUpdate() error = %v, want nil", err)
	}

	if got, want := len(client.sendTextCalls), 1; got != want {
		t.Fatalf("len(sendTextCalls) = %d, want %d", got, want)
	}
	if got, want := client.sendTextCalls[0].text, "I could not understand the time range 🤔\n\nSend only the YouTube link, optionally followed by:\n- a time range written with no spaces around the dash\n- the word audio at the end\n\nDo not write the time range like \"1:00 - 1:05\".\n\nSend a message exactly like one of these examples:\n\nhttps://www.youtube.com/watch?v=AqjB8DGt85U\n\nhttps://www.youtube.com/watch?v=AqjB8DGt85U 1:00-1:05\n\nhttps://www.youtube.com/watch?v=AqjB8DGt85U audio\n\nhttps://www.youtube.com/watch?v=AqjB8DGt85U 1:00-1:05 audio\n\nYou can also use start or end:\nhttps://www.youtube.com/watch?v=AqjB8DGt85U start-0:10\nhttps://www.youtube.com/watch?v=AqjB8DGt85U 0:10-end"; got != want {
		t.Fatalf("reply text = %q, want usageMessage", got)
	}
}

func TestDownloadRequestHandlerHandleUpdateParseFailure(t *testing.T) {
	client := &fakeTelegramBotClient{}
	logger := newTestLogger()
	downloadSlots := make(chan struct{}, 1)
	var downloadsWG sync.WaitGroup

	handler, err := NewDownloadRequestHandler(client, logger, []int64{777}, time.Minute, downloadSlots, &downloadsWG)
	if err != nil {
		t.Fatalf("NewDownloadRequestHandler() error = %v, want nil", err)
	}

	update := TelegramAPIUpdate{
		UpdateID: 1,
		Message:  newTestMessage("https://www.youtube.com/watch?v=IFbXnS1odNs 0:10-end invalid"),
	}

	if err := handler.HandleUpdate(context.Background(), update); err != nil {
		t.Fatalf("HandleUpdate() error = %v, want nil", err)
	}

	if got, want := len(client.sendTextCalls), 1; got != want {
		t.Fatalf("len(sendTextCalls) = %d, want %d", got, want)
	}
	if got, want := client.sendTextCalls[0].text, "I could not understand your request 🤔\n\nSend only the YouTube link, optionally followed by:\n- a time range written with no spaces around the dash\n- the word audio at the end\n\nDo not write the time range like \"1:00 - 1:05\".\n\nSend a message exactly like one of these examples:\n\nhttps://www.youtube.com/watch?v=AqjB8DGt85U\n\nhttps://www.youtube.com/watch?v=AqjB8DGt85U 1:00-1:05\n\nhttps://www.youtube.com/watch?v=AqjB8DGt85U audio\n\nhttps://www.youtube.com/watch?v=AqjB8DGt85U 1:00-1:05 audio\n\nYou can also use start or end:\nhttps://www.youtube.com/watch?v=AqjB8DGt85U start-0:10\nhttps://www.youtube.com/watch?v=AqjB8DGt85U 0:10-end"; got != want {
		t.Fatalf("reply text = %q, want usageMessage", got)
	}
}

func TestDownloadRequestHandlerHandleUpdateDispatchesDownloadRequest(t *testing.T) {
	client := &fakeTelegramBotClient{}
	logger := newTestLogger()
	downloadSlots := make(chan struct{}, 1)
	var downloadsWG sync.WaitGroup

	handler, err := NewDownloadRequestHandler(client, logger, []int64{777}, 2*time.Minute, downloadSlots, &downloadsWG)
	if err != nil {
		t.Fatalf("NewDownloadRequestHandler() error = %v, want nil", err)
	}

	productionDispatchDownloadRequest := dispatchDownloadRequest
	defer func() { dispatchDownloadRequest = productionDispatchDownloadRequest }()

	var (
		dispatchCalled   bool
		gotTimeout       time.Duration
		gotMessage       *TelegramAPIMessage
		gotMediaKind     MediaKind
		gotDownloadSlots chan struct{}
	)

	dispatchDownloadRequest = func(
		ctx context.Context,
		bot TelegramBotClient,
		logger *slog.Logger,
		message *TelegramAPIMessage,
		downloadTimeout time.Duration,
		downloadSlots chan struct{},
		mediaDownloader MediaDownloader,
		downloadsWG *sync.WaitGroup,
	) {
		dispatchCalled = true
		gotTimeout = downloadTimeout
		gotMessage = message
		gotMediaKind = mediaDownloader.MediaKind()
		gotDownloadSlots = downloadSlots
	}

	update := TelegramAPIUpdate{
		UpdateID: 1,
		Message:  newTestMessage("https://www.youtube.com/watch?v=AqjB8DGt85U 1:00-1:05 audio"),
	}

	if err := handler.HandleUpdate(context.Background(), update); err != nil {
		t.Fatalf("HandleUpdate() error = %v, want nil", err)
	}

	if got, want := len(client.sendTextCalls), 1; got != want {
		t.Fatalf("len(sendTextCalls) = %d, want %d", got, want)
	}
	if got, want := client.sendTextCalls[0].text, "Wait a minute ⏳"; got != want {
		t.Fatalf("reply text = %q, want %q", got, want)
	}
	if !dispatchCalled {
		t.Fatal("dispatchDownloadRequest() was not called")
	}
	if got, want := gotTimeout, 2*time.Minute; got != want {
		t.Fatalf("dispatch timeout = %v, want %v", got, want)
	}
	if gotMessage == nil {
		t.Fatal("dispatched message = nil, want non-nil")
	}
	if got, want := gotMediaKind, MediaAudio; got != want {
		t.Fatalf("dispatched media kind = %v, want %v", got, want)
	}
	if gotDownloadSlots != downloadSlots {
		t.Fatal("dispatch received the wrong download slots channel")
	}
}

func TestRunTelegramBotValidatesArguments(t *testing.T) {
	logger := newTestLogger()
	client := &fakeTelegramBotClient{}
	handler := func(ctx context.Context, update TelegramAPIUpdate) error { return nil }

	testCases := []struct {
		name    string
		client  TelegramBotClient
		logger  *slog.Logger
		handler func(context.Context, TelegramAPIUpdate) error
		wantErr string
	}{
		{
			name:    "nil client",
			logger:  logger,
			handler: handler,
			wantErr: "telegram API client is required",
		},
		{
			name:    "nil handler",
			client:  client,
			logger:  logger,
			wantErr: "update handler is required",
		},
		{
			name:    "nil logger",
			client:  client,
			handler: handler,
			wantErr: "logger is required",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := RunTelegramBot(context.Background(), tc.client, tc.logger, tc.handler)
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := &fakeTelegramBotClient{}
	logger := newTestLogger()

	var offsets []int64
	client.receiveUpdatesFunc = func(ctx context.Context, offset int64, timeoutSeconds int) ([]TelegramAPIUpdate, error) {
		offsets = append(offsets, offset)
		switch len(offsets) {
		case 1:
			if got, want := timeoutSeconds, telegramUpdatePollTimeoutSeconds; got != want {
				t.Fatalf("timeoutSeconds = %d, want %d", got, want)
			}
			return []TelegramAPIUpdate{{UpdateID: 7}}, nil
		case 2:
			cancel()
			return nil, context.Canceled
		default:
			t.Fatalf("ReceiveUpdates() called too many times: %d", len(offsets))
			return nil, nil
		}
	}

	var handledUpdates []int64
	handler := func(ctx context.Context, update TelegramAPIUpdate) error {
		handledUpdates = append(handledUpdates, update.UpdateID)
		return nil
	}

	if err := RunTelegramBot(ctx, client, logger, handler); err != nil {
		t.Fatalf("RunTelegramBot() error = %v, want nil", err)
	}

	if got, want := len(handledUpdates), 1; got != want {
		t.Fatalf("len(handledUpdates) = %d, want %d", got, want)
	}
	if got, want := handledUpdates[0], int64(7); got != want {
		t.Fatalf("handled update ID = %d, want %d", got, want)
	}
	if got, want := len(offsets), 2; got != want {
		t.Fatalf("len(offsets) = %d, want %d", got, want)
	}
	if got, want := offsets[0], int64(0); got != want {
		t.Fatalf("first offset = %d, want %d", got, want)
	}
	if got, want := offsets[1], int64(8); got != want {
		t.Fatalf("second offset = %d, want %d", got, want)
	}
}

func TestRunTelegramBotContinuesWhenHandlerReturnsError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := &fakeTelegramBotClient{}
	logger := newTestLogger()

	productionAfterRetryDelay := afterRetryDelay
	defer func() { afterRetryDelay = productionAfterRetryDelay }()
	afterRetryDelay = time.After

	var receiveCalls int
	client.receiveUpdatesFunc = func(ctx context.Context, offset int64, timeoutSeconds int) ([]TelegramAPIUpdate, error) {
		receiveCalls++
		switch receiveCalls {
		case 1:
			return []TelegramAPIUpdate{{UpdateID: 7}}, nil
		case 2:
			cancel()
			return nil, context.Canceled
		default:
			t.Fatalf("ReceiveUpdates() called too many times: %d", receiveCalls)
			return nil, nil
		}
	}

	var handled int
	handler := func(ctx context.Context, update TelegramAPIUpdate) error {
		handled++
		return errors.New("boom")
	}

	if err := RunTelegramBot(ctx, client, logger, handler); err != nil {
		t.Fatalf("RunTelegramBot() error = %v, want nil", err)
	}
	if got, want := handled, 1; got != want {
		t.Fatalf("handled updates = %d, want %d", got, want)
	}
	if got, want := receiveCalls, 2; got != want {
		t.Fatalf("receive calls = %d, want %d", got, want)
	}
}

func TestRunTelegramBotStopsImmediatelyWhenContextIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := &fakeTelegramBotClient{
		receiveUpdatesFunc: func(ctx context.Context, offset int64, timeoutSeconds int) ([]TelegramAPIUpdate, error) {
			t.Fatal("ReceiveUpdates() should not be called when context is already canceled")
			return nil, nil
		},
	}

	if err := RunTelegramBot(ctx, client, newTestLogger(), func(context.Context, TelegramAPIUpdate) error { return nil }); err != nil {
		t.Fatalf("RunTelegramBot() error = %v, want nil", err)
	}
}

func TestRunTelegramBotRetriesAfterReceiveError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := &fakeTelegramBotClient{}
	logger := newTestLogger()

	productionAfterRetryDelay := afterRetryDelay
	defer func() { afterRetryDelay = productionAfterRetryDelay }()

	retryDelayHit := false
	afterRetryDelay = func(d time.Duration) <-chan time.Time {
		retryDelayHit = true
		ch := make(chan time.Time, 1)
		ch <- time.Now()
		return ch
	}

	var receiveCalls int
	client.receiveUpdatesFunc = func(ctx context.Context, offset int64, timeoutSeconds int) ([]TelegramAPIUpdate, error) {
		receiveCalls++
		switch receiveCalls {
		case 1:
			return nil, errors.New("temporary failure")
		case 2:
			cancel()
			return nil, context.Canceled
		default:
			t.Fatalf("ReceiveUpdates() called too many times: %d", receiveCalls)
			return nil, nil
		}
	}

	err := RunTelegramBot(ctx, client, logger, func(context.Context, TelegramAPIUpdate) error { return nil })
	if err != nil {
		t.Fatalf("RunTelegramBot() error = %v, want nil", err)
	}
	if !retryDelayHit {
		t.Fatal("afterRetryDelay() was not called")
	}
	if got, want := receiveCalls, 2; got != want {
		t.Fatalf("receive calls = %d, want %d", got, want)
	}
}
