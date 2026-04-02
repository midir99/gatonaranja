package main

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

const (
	telegramUpdatePollTimeoutSeconds = 60
	telegramUpdateRetryDelay         = time.Second
)

var afterRetryDelay = time.After

type TelegramBotClient interface {
	ReceiveUpdates(ctx context.Context, offset int64, timeoutSeconds int) ([]TelegramAPIUpdate, error)
	SendText(ctx context.Context, chatID int64, replyToMessageID int64, text string) (*TelegramAPIMessage, error)
	SendVideo(ctx context.Context, chatID int64, replyToMessageID int64, videoPath string) (*TelegramAPIMessage, error)
	SendAudio(ctx context.Context, chatID int64, replyToMessageID int64, audioPath string) (*TelegramAPIMessage, error)
}

// RunTelegramBot receives Telegram updates using long polling and calls
// handleUpdate for each update in order.
//
// It keeps track of the update offset internally. After an update is received,
// the next request uses update_id + 1 so Telegram does not send that update
// again.
func RunTelegramBot(
	ctx context.Context,
	client TelegramBotClient,
	logger *slog.Logger,
	handleUpdate func(context.Context, TelegramAPIUpdate) error,
) error {
	if client == nil {
		return errors.New("telegram API client is required")
	}
	if handleUpdate == nil {
		return errors.New("update handler is required")
	}
	if logger == nil {
		return errors.New("logger is required")
	}

	var offset int64

	for {
		select {
		case <-ctx.Done():
			logger.Info("Stopping Telegram update loop")
			return nil
		default:
		}

		updates, err := client.ReceiveUpdates(ctx, offset, telegramUpdatePollTimeoutSeconds)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				logger.Info("Stopping Telegram update loop")
				return nil
			}

			logger.Warn(
				"Failed to receive Telegram updates",
				"offset", offset,
				"error", err,
			)

			select {
			case <-ctx.Done():
				logger.Info("Stopping Telegram update loop")
				return nil
			case <-afterRetryDelay(telegramUpdateRetryDelay):
			}

			continue
		}

		for _, update := range updates {
			offset = update.UpdateID + 1

			if err := handleUpdate(ctx, update); err != nil {
				logger.Warn(
					"Failed to handle Telegram update",
					"update_id", update.UpdateID,
					"error", err,
				)
			}
		}
	}
}
