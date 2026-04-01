package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"
)

const usageMessage = `Send only the YouTube link, optionally followed by:
- a time range written with no spaces around the dash
- the word audio at the end

Do not write the time range like "1:00 - 1:05".

Send a message exactly like one of these examples:

https://www.youtube.com/watch?v=AqjB8DGt85U

https://www.youtube.com/watch?v=AqjB8DGt85U 1:00-1:05

https://www.youtube.com/watch?v=AqjB8DGt85U audio

https://www.youtube.com/watch?v=AqjB8DGt85U 1:00-1:05 audio

You can also use start or end:
https://www.youtube.com/watch?v=AqjB8DGt85U start-0:10
https://www.youtube.com/watch?v=AqjB8DGt85U 0:10-end`

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

type DownloadRequestHandler struct {
	client          TelegramBotClient
	logger          *slog.Logger
	authorizedUsers []int64
	downloadTimeout time.Duration
	downloadSlots   chan struct{}
	downloadsWG     *sync.WaitGroup
}

func NewDownloadRequestHandler(
	client TelegramBotClient,
	logger *slog.Logger,
	authorizedUsers []int64,
	downloadTimeout time.Duration,
	downloadSlots chan struct{},
	downloadsWG *sync.WaitGroup,
) (*DownloadRequestHandler, error) {
	if client == nil {
		return nil, errors.New("telegram bot client is required")
	}
	if logger == nil {
		return nil, errors.New("logger is required")
	}
	if downloadSlots == nil {
		return nil, errors.New("download slots channel is required")
	}
	if downloadsWG == nil {
		return nil, errors.New("downloads wait group is required")
	}

	return &DownloadRequestHandler{
		client:          client,
		logger:          logger,
		authorizedUsers: authorizedUsers,
		downloadTimeout: downloadTimeout,
		downloadSlots:   downloadSlots,
		downloadsWG:     downloadsWG,
	}, nil
}

func (h *DownloadRequestHandler) HandleUpdate(ctx context.Context, update TelegramAPIUpdate) error {
	if update.Message == nil || update.Message.From == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		h.logger.Info(
			"Ignoring request because shutdown is in progress",
			"user_id", update.Message.From.ID,
			"user_name", update.Message.From.UserName,
		)
		return nil
	default:
	}
	// Check if user is authorized
	if !UserIsAuthorized(update.Message.From.ID, h.authorizedUsers) {
		h.logger.Warn(
			"Rejected unauthorized request",
			"user_id", update.Message.From.ID,
			"user_name", update.Message.From.UserName,
			"message_text", update.Message.Text,
		)
		sendReply(ctx, h.client, h.logger, update.Message, "You are not authorized to use this bot 😾")
		return nil
	}
	h.logger.Info(
		"Received request",
		"user_id", update.Message.From.ID,
		"user_name", update.Message.From.UserName,
		"message_text", update.Message.Text,
	)
	// Parse the download request message
	downloadRequest, err := ParseDownloadRequest(update.Message.Text)
	if err != nil {
		h.logger.Warn(
			"Failed to parse request",
			"user_id", update.Message.From.ID,
			"user_name", update.Message.From.UserName,
			"message_text", update.Message.Text,
			"error", err,
		) // #nosec G706
		switch {
		case errors.Is(err, ErrInvalidYouTubeURL):
			sendReply(ctx, h.client, h.logger, update.Message,
				"That does not look like a valid YouTube video URL 🤔\n\n"+usageMessage)
		case errors.Is(err, ErrInvalidTimestampRange):
			sendReply(ctx, h.client, h.logger, update.Message,
				"I could not understand the time range 🤔\n\n"+usageMessage)
		default:
			sendReply(ctx, h.client, h.logger, update.Message,
				"I could not understand your request 🤔\n\n"+usageMessage)
		}
		return nil
	}
	// Let the user know you are working on the download
	sendReply(ctx, h.client, h.logger, update.Message, "Wait a minute ⏳")
	dispatchDownloadRequest(
		ctx,
		h.client,
		h.logger,
		update.Message,
		h.downloadTimeout,
		h.downloadSlots,
		downloadRequest,
		h.downloadsWG,
	)
	return nil
}

var dispatchDownloadRequest = func(
	ctx context.Context,
	bot TelegramBotClient,
	logger *slog.Logger,
	message *TelegramAPIMessage,
	downloadTimeout time.Duration,
	downloadSlots chan struct{},
	mediaDownloader MediaDownloader,
	downloadsWG *sync.WaitGroup,
) {
	downloadsWG.Go(func() {
		select {
		case <-ctx.Done():
			logger.Info(
				"Ignoring queued download because shutdown is in progress",
				"user_id", message.From.ID,
				"user_name", message.From.UserName,
			)
			return
		case downloadSlots <- struct{}{}:
		}
		defer func() { <-downloadSlots }()

		handleDownloadRequest(ctx, bot, logger, message, mediaDownloader, downloadTimeout)
	})
}

var removeFile = os.Remove

func handleDownloadRequest(
	ctx context.Context,
	client TelegramBotClient,
	logger *slog.Logger,
	message *TelegramAPIMessage,
	mediaDownloader MediaDownloader,
	downloadTimeout time.Duration,
) {
	mediaFilename, err := mediaDownloader.Download(ctx, downloadTimeout)
	if err != nil {
		logger.Error(
			"Failed to download request",
			"user_id", message.From.ID,
			"user_name", message.From.UserName,
			"message_text", message.Text,
			"error", err,
		)
		sendReply(ctx, client, logger, message, "I could not download your request 😿")
		return
	}

	switch mediaDownloader.MediaKind() {
	case MediaAudio:
		_, err = client.SendAudio(ctx, message.Chat.ID, message.MessageID, mediaFilename)
	case MediaVideo:
		_, err = client.SendVideo(ctx, message.Chat.ID, message.MessageID, mediaFilename)
	default:
		err = fmt.Errorf("unsupported media kind %v", mediaDownloader.MediaKind())
	}
	if err == nil {
		logger.Info(
			"Completed request",
			"user_id", message.From.ID,
			"user_name", message.From.UserName,
			"message_text", message.Text,
		)
	} else {
		logger.Error(
			"Failed to send media",
			"user_id", message.From.ID,
			"user_name", message.From.UserName,
			"message_text", message.Text,
			"error", err,
		)
		if errors.Is(err, ErrTelegramMediaTooLarge) {
			sendReply(
				ctx,
				client,
				logger,
				message,
				"I downloaded it, but the file is too big for me to send on Telegram 😿",
			)
		} else {
			sendReply(ctx, client, logger, message, "I downloaded it, but I couldn't send it to you 🙀")
		}
	}
	err = removeFile(mediaFilename)
	if err != nil {
		logger.Warn(
			"Failed to remove downloaded file",
			"user_id", message.From.ID,
			"user_name", message.From.UserName,
			"file_name", mediaFilename,
			"error", err,
		)
	}
}

// logTelegramSendError logs a Telegram send failure for the given user.
func logTelegramSendError(logger *slog.Logger, userName string, userID int64, err error) {
	logger.Error(
		"Failed to send Telegram message",
		"user_id", userID,
		"user_name", userName,
		"error", err,
	) // #nosec G706
}

func sendReply(
	ctx context.Context,
	bot TelegramBotClient,
	logger *slog.Logger,
	message *TelegramAPIMessage,
	text string,
) {
	_, err := bot.SendText(ctx, message.Chat.ID, message.MessageID, text)
	if err != nil {
		logTelegramSendError(logger, message.From.UserName, message.From.ID, err)
	}
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
