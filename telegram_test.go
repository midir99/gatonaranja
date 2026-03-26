package main

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func TestLogTelegramSendError(t *testing.T) {
	var buf bytes.Buffer

	logger := slog.New(slog.NewTextHandler(&buf, nil))

	err := errors.New("something went wrong")
	userID := int64(12345)
	userName := "arthurmorgan"

	logTelegramSendError(logger, userName, userID, err)

	output := buf.String()

	// Assert message
	if !strings.Contains(output, "Failed to send Telegram message") {
		t.Fatalf("expected log message, got: %s", output)
	}

	// Assert fields
	if !strings.Contains(output, "user_id=12345") {
		t.Fatalf("expected user_id, got: %s", output)
	}

	if !strings.Contains(output, "user_name=arthurmorgan") {
		t.Fatalf("expected user_name, got: %s", output)
	}

	if !strings.Contains(output, "error=\"something went wrong\"") &&
		!strings.Contains(output, "error=something went wrong") {
		t.Fatalf("expected error field, got: %s", output)
	}
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
		// Assert fields
		if !strings.Contains(output, "user_id=12345") {
			t.Fatalf("expected user_id, got: %s", output)
		}

		if !strings.Contains(output, "user_name=arthurmorgan") {
			t.Fatalf("expected user_name, got: %s", output)
		}

		if !strings.Contains(output, "error=\"no se pudo\"") &&
			!strings.Contains(output, "error=no se pudo") {
			t.Fatalf("expected error field, got: %s", output)
		}
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

func TestHandleDownloadRequest(t *testing.T) {
	t.Run("success with video", func(t *testing.T) {
		sender := goodMessageSender{}
		var buf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&buf, nil))
		message := tgbotapi.Message{
			MessageID: 1,
			Chat:      &tgbotapi.Chat{ID: 1},
			From:      &tgbotapi.User{ID: 12345, UserName: "arthurmorgan"},
			Text:      "",
		}
		downloader := goodMediaDownloader{mediaKind: MediaVideo}

		handleDownloadRequest(sender, logger, &message, downloader)

		output := buf.String()

		// Assert message
		if !strings.Contains(output, "Completed request") {
			t.Fatalf("expected log message, got: %s", output)
		}

		// Assert fields
		if !strings.Contains(output, "user_id=12345") {
			t.Fatalf("expected user_id, got: %s", output)
		}

		if !strings.Contains(output, "user_name=arthurmorgan") {
			t.Fatalf("expected user_name, got: %s", output)
		}

		if !strings.Contains(output, "message_text=") {
			t.Fatalf("expected message_text, got: %s", output)
		}
	})
}
func TestHandleMessage(t *testing.T) {

}
func TestRunTelegramBot(t *testing.T) {

}
