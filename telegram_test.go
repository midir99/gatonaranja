package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func assertFieldContains(t *testing.T, got, want, field string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Fatalf("%s: expected log output to contain %q, got %q", field, want, got)
	}
}

func assertErrorFieldContains(t *testing.T, output, errMessage string) {
	t.Helper()

	quoted := fmt.Sprintf(`error=%q`, errMessage)
	unquoted := "error=" + errMessage

	if !strings.Contains(output, quoted) && !strings.Contains(output, unquoted) {
		t.Fatalf(
			"error field: expected log output to contain %q or %q, got %q",
			quoted,
			unquoted,
			output,
		)
	}
}

func TestLogTelegramSendError(t *testing.T) {
	var buf bytes.Buffer

	logger := slog.New(slog.NewTextHandler(&buf, nil))

	err := errors.New("something went wrong")
	userID := int64(12345)
	userName := "arthurmorgan"

	logTelegramSendError(logger, userName, userID, err)

	output := buf.String()

	assertFieldContains(t, output, "Failed to send Telegram message", "log message")
	assertFieldContains(t, output, "user_id=12345", "user_id")
	assertFieldContains(t, output, "user_name=arthurmorgan", "user_name")
	assertErrorFieldContains(t, output, "something went wrong")
}

type goodMessageSender struct{}

func (ms goodMessageSender) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	return tgbotapi.Message{}, nil
}

type badMessageSender struct{}

func (ms badMessageSender) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	return tgbotapi.Message{}, errors.New("no se pudo")
}

func TestSendReply(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		sender := goodMessageSender{}
		var buf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&buf, nil))
		message := tgbotapi.Message{
			MessageID: 1,
			Chat:      &tgbotapi.Chat{ID: 1},
			From:      &tgbotapi.User{ID: 12345, UserName: "arthurmorgan"},
		}

		sendReply(sender, logger, &message, "ya cállate pinche Chalino")

		output := buf.String()
		if output != "" {
			t.Fatalf("got %q, want %q", output, "")
		}
	})
	t.Run("failure", func(t *testing.T) {
		sender := badMessageSender{}
		var buf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&buf, nil))
		message := tgbotapi.Message{
			MessageID: 1,
			Chat:      &tgbotapi.Chat{ID: 1},
			From:      &tgbotapi.User{ID: 12345, UserName: "arthurmorgan"},
		}

		sendReply(sender, logger, &message, "ya cállate pinche Chalino")

		output := buf.String()

		assertFieldContains(t, output, "Failed to send Telegram message", "log message")
		assertFieldContains(t, output, "user_id=12345", "user_id")
		assertFieldContains(t, output, "user_name=arthurmorgan", "user_name")
		assertErrorFieldContains(t, output, "no se pudo")
	})
}

type goodMediaDownloader struct {
	mediaKind MediaKind
}

func (md goodMediaDownloader) Download() (string, error) {
	return "funny-video.mp4", nil
}

func (md goodMediaDownloader) MediaKind() MediaKind {
	return md.mediaKind
}

type badMediaDownloader struct {
	mediaKind MediaKind
}

func (md badMediaDownloader) Download() (string, error) {
	return "", errors.New("no se pudo")
}

func (md badMediaDownloader) MediaKind() MediaKind {
	return md.mediaKind
}

func goodRemoveFile(name string) error {
	return nil
}

func badRemoveFile(name string) error {
	return fmt.Errorf("impossible to delete %s", name)
}

func TestHandleDownloadRequest(t *testing.T) {
	t.Run("success with video", func(t *testing.T) {
		sender := goodMessageSender{}
		var buf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&buf, nil))
		message := tgbotapi.Message{
			MessageID: 1,
			Chat:      &tgbotapi.Chat{ID: 1},
			From:      &tgbotapi.User{ID: 12345, UserName: "arthurmorgan"},
			Text:      "https://www.youtube.com/watch?v=AqjB8DGt85U 1:00-1:05",
		}
		downloader := goodMediaDownloader{mediaKind: MediaVideo}
		productionRemoveFile := removeFile
		removeFile = goodRemoveFile
		defer func() {
			removeFile = productionRemoveFile
		}()

		handleDownloadRequest(sender, logger, &message, downloader)

		output := buf.String()

		assertFieldContains(t, output, "Completed request", "log message")
		assertFieldContains(t, output, "user_id=12345", "user_id")
		assertFieldContains(t, output, "user_name=arthurmorgan", "user_name")
		assertFieldContains(t, output, `message_text="https://www.youtube.com/watch?v=AqjB8DGt85U 1:00-1:05"`, "message_text")
	})

	t.Run("success with audio", func(t *testing.T) {
		sender := goodMessageSender{}
		var buf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&buf, nil))
		message := tgbotapi.Message{
			MessageID: 1,
			Chat:      &tgbotapi.Chat{ID: 1},
			From:      &tgbotapi.User{ID: 12345, UserName: "arthurmorgan"},
			Text:      "https://www.youtube.com/watch?v=AqjB8DGt85U 1:00-1:05 audio",
		}
		downloader := goodMediaDownloader{mediaKind: MediaAudio}
		productionRemoveFile := removeFile
		removeFile = goodRemoveFile
		defer func() {
			removeFile = productionRemoveFile
		}()

		handleDownloadRequest(sender, logger, &message, downloader)

		output := buf.String()

		assertFieldContains(t, output, "Completed request", "log message")
		assertFieldContains(t, output, "user_id=12345", "user_id")
		assertFieldContains(t, output, "user_name=arthurmorgan", "user_name")
		assertFieldContains(t, output, `message_text="https://www.youtube.com/watch?v=AqjB8DGt85U 1:00-1:05 audio"`, "message_text")
	})

	t.Run("download failure", func(t *testing.T) {
		sender := goodMessageSender{}
		var buf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&buf, nil))
		message := tgbotapi.Message{
			MessageID: 1,
			Chat:      &tgbotapi.Chat{ID: 1},
			From:      &tgbotapi.User{ID: 12345, UserName: "arthurmorgan"},
			Text:      "https://www.youtube.com/watch?v=AqjB8DGt85U 1:00-1:05",
		}
		downloader := badMediaDownloader{mediaKind: MediaVideo}
		productionRemoveFile := removeFile
		removeFile = goodRemoveFile
		defer func() {
			removeFile = productionRemoveFile
		}()

		handleDownloadRequest(sender, logger, &message, downloader)

		output := buf.String()

		assertFieldContains(t, output, "Failed to download request", "log message")
		assertFieldContains(t, output, "user_id=12345", "user_id")
		assertFieldContains(t, output, "user_name=arthurmorgan", "user_name")
		assertFieldContains(t, output, `message_text="https://www.youtube.com/watch?v=AqjB8DGt85U 1:00-1:05"`, "message_text")
		assertErrorFieldContains(t, output, "no se pudo")
	})

	t.Run("send failure", func(t *testing.T) {
		sender := badMessageSender{}
		var buf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&buf, nil))
		message := tgbotapi.Message{
			MessageID: 1,
			Chat:      &tgbotapi.Chat{ID: 1},
			From:      &tgbotapi.User{ID: 12345, UserName: "arthurmorgan"},
			Text:      "https://www.youtube.com/watch?v=AqjB8DGt85U 1:00-1:05",
		}
		downloader := goodMediaDownloader{mediaKind: MediaVideo}
		productionRemoveFile := removeFile
		removeFile = goodRemoveFile
		defer func() {
			removeFile = productionRemoveFile
		}()

		handleDownloadRequest(sender, logger, &message, downloader)

		output := buf.String()

		assertFieldContains(t, output, "Failed to send media", "log message")
		assertFieldContains(t, output, "user_id=12345", "user_id")
		assertFieldContains(t, output, "user_name=arthurmorgan", "user_name")
		assertFieldContains(t, output, `message_text="https://www.youtube.com/watch?v=AqjB8DGt85U 1:00-1:05"`, "message_text")
		assertErrorFieldContains(t, output, "no se pudo")
	})

	t.Run("cleanup failure", func(t *testing.T) {
		sender := goodMessageSender{}
		var buf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&buf, nil))
		message := tgbotapi.Message{
			MessageID: 1,
			Chat:      &tgbotapi.Chat{ID: 1},
			From:      &tgbotapi.User{ID: 12345, UserName: "arthurmorgan"},
			Text:      "https://www.youtube.com/watch?v=AqjB8DGt85U 1:00-1:05",
		}
		downloader := goodMediaDownloader{mediaKind: MediaVideo}
		productionRemoveFile := removeFile
		removeFile = badRemoveFile
		defer func() {
			removeFile = productionRemoveFile
		}()

		handleDownloadRequest(sender, logger, &message, downloader)

		output := buf.String()

		assertFieldContains(t, output, "Completed request", "log message")
		assertFieldContains(t, output, "user_id=12345", "user_id")
		assertFieldContains(t, output, "user_name=arthurmorgan", "user_name")
		assertFieldContains(t, output, `message_text="https://www.youtube.com/watch?v=AqjB8DGt85U 1:00-1:05"`, "message_text")

		assertFieldContains(t, output, "Failed to remove downloaded file", "log message")
		assertFieldContains(t, output, "user_id=12345", "user_id")
		assertFieldContains(t, output, "user_name=arthurmorgan", "user_name")
		assertFieldContains(t, output, "file_name=funny-video.mp4", "file_name")
		assertErrorFieldContains(t, output, "impossible to delete funny-video.mp4")
	})
}

var goodDispatchDownloadRequest = func(
	bot MessageSender,
	logger *slog.Logger,
	message *tgbotapi.Message,
	downloadSlots chan struct{},
	mediaDownloader MediaDownloader,
) {
}

func TestHandleMessage(t *testing.T) {
	t.Run("authorized user", func(t *testing.T) {
		sender := goodMessageSender{}
		var buf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&buf, nil))
		message := tgbotapi.Message{
			MessageID: 1,
			Chat:      &tgbotapi.Chat{ID: 1},
			From:      &tgbotapi.User{ID: 12345, UserName: "arthurmorgan"},
			Text:      "https://www.youtube.com/watch?v=AqjB8DGt85U 1:00-1:05",
		}
		authorizedUsers := []int64{1, 2, 3, 12345}
		downloadSlots := make(chan struct{}, 1)
		productionRunDownloadRequest := dispatchDownloadRequest
		dispatchDownloadRequest = goodDispatchDownloadRequest
		defer func() {
			dispatchDownloadRequest = productionRunDownloadRequest
		}()

		handleMessage(sender, logger, &message, authorizedUsers, downloadSlots)

		output := buf.String()

		assertFieldContains(t, output, "Received request", "log message")
		assertFieldContains(t, output, "user_id=12345", "user_id")
		assertFieldContains(t, output, "user_name=arthurmorgan", "user_name")
		assertFieldContains(t, output, `message_text="https://www.youtube.com/watch?v=AqjB8DGt85U 1:00-1:05"`, "message_text")
	})
	t.Run("nil message", func(t *testing.T) {
		sender := goodMessageSender{}
		var buf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&buf, nil))
		var nilMessage *tgbotapi.Message
		authorizedUsers := []int64{1, 2, 3}
		downloadSlots := make(chan struct{}, 1)
		productionRunDownloadRequest := dispatchDownloadRequest
		dispatchDownloadRequest = goodDispatchDownloadRequest
		defer func() {
			dispatchDownloadRequest = productionRunDownloadRequest
		}()

		handleMessage(sender, logger, nilMessage, authorizedUsers, downloadSlots)
	})
	t.Run("unauthorized user", func(t *testing.T) {
		sender := goodMessageSender{}
		var buf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&buf, nil))
		message := tgbotapi.Message{
			MessageID: 1,
			Chat:      &tgbotapi.Chat{ID: 1},
			From:      &tgbotapi.User{ID: 12345, UserName: "arthurmorgan"},
			Text:      "https://www.youtube.com/watch?v=AqjB8DGt85U 1:00-1:05",
		}
		authorizedUsers := []int64{1, 2, 3}
		downloadSlots := make(chan struct{}, 1)
		productionRunDownloadRequest := dispatchDownloadRequest
		dispatchDownloadRequest = goodDispatchDownloadRequest
		defer func() {
			dispatchDownloadRequest = productionRunDownloadRequest
		}()

		handleMessage(sender, logger, &message, authorizedUsers, downloadSlots)

		output := buf.String()

		assertFieldContains(t, output, "Rejected unauthorized request", "log message")
		assertFieldContains(t, output, "user_id=12345", "user_id")
		assertFieldContains(t, output, "user_name=arthurmorgan", "user_name")
		assertFieldContains(t, output, `message_text="https://www.youtube.com/watch?v=AqjB8DGt85U 1:00-1:05"`, "message_text")
	})
	t.Run("parse failure", func(t *testing.T) {
		sender := goodMessageSender{}
		var buf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&buf, nil))
		message := tgbotapi.Message{
			MessageID: 1,
			Chat:      &tgbotapi.Chat{ID: 1},
			From:      &tgbotapi.User{ID: 12345, UserName: "arthurmorgan"},
			Text:      "hello there",
		}
		authorizedUsers := []int64{1, 2, 3, 12345}
		downloadSlots := make(chan struct{}, 1)
		productionRunDownloadRequest := dispatchDownloadRequest
		dispatchDownloadRequest = goodDispatchDownloadRequest
		defer func() {
			dispatchDownloadRequest = productionRunDownloadRequest
		}()

		handleMessage(sender, logger, &message, authorizedUsers, downloadSlots)

		output := buf.String()

		assertFieldContains(t, output, "Received request", "log message")
		assertFieldContains(t, output, "user_id=12345", "user_id")
		assertFieldContains(t, output, "user_name=arthurmorgan", "user_name")
		assertFieldContains(t, output, `message_text="hello there"`, "message_text")

		assertFieldContains(t, output, "Failed to parse request", "log message")
		assertFieldContains(t, output, "user_id=12345", "user_id")
		assertFieldContains(t, output, "user_name=arthurmorgan", "user_name")
		assertFieldContains(t, output, `message_text="hello there"`, "message_text")
	})
	t.Run("download request is dispatched", func(t *testing.T) {
		sender := goodMessageSender{}
		var buf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&buf, nil))
		message := tgbotapi.Message{
			MessageID: 1,
			Chat:      &tgbotapi.Chat{ID: 1},
			From:      &tgbotapi.User{ID: 12345, UserName: "arthurmorgan"},
			Text:      "https://www.youtube.com/watch?v=AqjB8DGt85U 1:00-1:05 audio",
		}
		authorizedUsers := []int64{1, 2, 3, 12345}
		downloadSlots := make(chan struct{}, 1)

		var called bool
		var gotDownloader MediaDownloader

		productionDispatchDownloadRequest := dispatchDownloadRequest
		dispatchDownloadRequest = func(
			bot MessageSender,
			logger *slog.Logger,
			message *tgbotapi.Message,
			downloadSlots chan struct{},
			mediaDownloader MediaDownloader,
		) {
			called = true
			gotDownloader = mediaDownloader
		}

		defer func() {
			dispatchDownloadRequest = productionDispatchDownloadRequest
		}()

		handleMessage(sender, logger, &message, authorizedUsers, downloadSlots)
		if !called {
			t.Fatal("expected download request to be dispatched")
		}

		if gotDownloader.MediaKind() != MediaAudio {
			t.Fatalf("got media kind %v, want %v", gotDownloader.MediaKind(), MediaAudio)
		}
	})
}
func TestRunTelegramBot_ConsumesUpdatesChannel(t *testing.T) {
	t.Run("consumes updates channel", func(t *testing.T) {
		bot := &tgbotapi.BotAPI{}
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		authorizedUsers := []int64{12345}
		downloadSlots := make(chan struct{}, 1)

		updates := make(chan tgbotapi.Update)

		oldGetUpdatesChan := getUpdatesChan
		defer func() { getUpdatesChan = oldGetUpdatesChan }()

		var gotConfig tgbotapi.UpdateConfig
		getUpdatesChan = func(_ *tgbotapi.BotAPI, u tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
			gotConfig = u
			close(updates)
			return updates
		}

		RunTelegramBot(bot, logger, authorizedUsers, downloadSlots)

		if gotConfig.Timeout != 60 {
			t.Fatalf("got timeout %d, want 60", gotConfig.Timeout)
		}
	})
}
