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

const (
	// telegramSendGrace is the time budget reserved for sending Telegram replies
	// after a download request has been accepted or completed.
	telegramSendGrace = 30 * time.Second
)

// usageMessage explains the supported bot request formats shown to users when
// their request cannot be parsed.
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

// DownloadJob represents a parsed Telegram request that has been accepted and
// queued for background processing.
type DownloadJob struct {
	Message         *TelegramAPIMessage
	DownloadRequest DownloadRequest
	YTDLPConfig     string
}

// DownloadRequestHandler validates Telegram messages, turns accepted ones into
// download jobs, and enqueues them for worker processing.
type DownloadRequestHandler struct {
	client          TelegramBotClient
	logger          *slog.Logger
	authorizedUsers []int64
	downloadTimeout time.Duration
	ytdlpConfig     string
	downloadQueue   chan DownloadJob
	downloadsWG     *sync.WaitGroup
}

// NewDownloadRequestHandler constructs a download request handler with the
// dependencies needed to validate, acknowledge, and enqueue incoming requests.
func NewDownloadRequestHandler(
	client TelegramBotClient,
	logger *slog.Logger,
	authorizedUsers []int64,
	downloadTimeout time.Duration,
	ytdlpConfig string,
	downloadQueue chan DownloadJob,
	downloadsWG *sync.WaitGroup,
) (*DownloadRequestHandler, error) {
	if client == nil {
		return nil, errors.New("telegram bot client is required")
	}
	if logger == nil {
		return nil, errors.New("logger is required")
	}
	if downloadQueue == nil {
		return nil, errors.New("download queue channel is required")
	}
	if downloadsWG == nil {
		return nil, errors.New("downloads wait group is required")
	}

	return &DownloadRequestHandler{
		client:          client,
		logger:          logger,
		authorizedUsers: authorizedUsers,
		downloadTimeout: downloadTimeout,
		ytdlpConfig:     ytdlpConfig,
		downloadQueue:   downloadQueue,
		downloadsWG:     downloadsWG,
	}, nil
}

// HandleUpdate validates a Telegram update, replies to the user when needed,
// and enqueues accepted download requests for background processing.
func (h *DownloadRequestHandler) HandleUpdate(ctx context.Context, update TelegramAPIUpdate) error {
	// Check if update is a message
	if update.Message == nil || update.Message.From == nil {
		return nil
	}

	// Check if shutdown is in progress
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

	sendCtx, cancelSend := context.WithTimeout(context.WithoutCancel(ctx), telegramSendGrace)
	defer cancelSend()

	// Check if user is authorized
	if !UserIsAuthorized(update.Message.From.ID, h.authorizedUsers) {
		h.logger.Warn(
			"Rejected unauthorized request",
			"user_id", update.Message.From.ID,
			"user_name", update.Message.From.UserName,
			"message_text", update.Message.Text,
		)
		sendReply(sendCtx, h.client, h.logger, update.Message, "You are not authorized to use this bot 😾")
		return nil
	}

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
			sendReply(sendCtx, h.client, h.logger, update.Message,
				"That does not look like a valid YouTube video URL 🤔\n\n"+usageMessage)
		case errors.Is(err, ErrInvalidTimestampRange):
			sendReply(sendCtx, h.client, h.logger, update.Message,
				"I could not understand the time range 🤔\n\n"+usageMessage)
		default:
			sendReply(sendCtx, h.client, h.logger, update.Message,
				"I could not understand your request 🤔\n\n"+usageMessage)
		}
		return nil
	}

	// Create download job and try to enqueue
	downloadJob := DownloadJob{
		Message:         update.Message,
		DownloadRequest: downloadRequest,
		YTDLPConfig:     h.ytdlpConfig,
	}
	h.downloadsWG.Add(1)
	select {
	case <-ctx.Done(): // Job is rejected, shutdown is in progress
		h.logger.Warn(
			"Rejected request, shutdown is in progress",
			"user_id", update.Message.From.ID,
			"user_name", update.Message.From.UserName,
			"message_text", update.Message.Text,
		)
		sendReply(sendCtx, h.client, h.logger, update.Message, "Not now, it's my time to sleep 😌")
		h.downloadsWG.Done()
	case h.downloadQueue <- downloadJob: // Job is accepted
		h.logger.Info(
			"Received request",
			"user_id", update.Message.From.ID,
			"user_name", update.Message.From.UserName,
			"message_text", update.Message.Text,
		)
		sendReply(sendCtx, h.client, h.logger, update.Message, "Wait a minute (maybe more) ⏳")
	default: // Job is rejected, queue is full
		h.logger.Warn(
			"Rejected request, queue is full",
			"user_id", update.Message.From.ID,
			"user_name", update.Message.From.UserName,
			"message_text", update.Message.Text,
		)
		sendReply(sendCtx, h.client, h.logger, update.Message, "I'm too busy right now, please try again later 😵")
		h.downloadsWG.Done()
	}
	return nil
}

// downloadWorker drains accepted download jobs from the queue and processes
// them until the queue is closed.
func downloadWorker(
	ctx context.Context,
	logger *slog.Logger,
	workerID int,
	bot TelegramBotClient,
	downloadTimeout time.Duration,
	jobs <-chan DownloadJob,
	wg *sync.WaitGroup,
) {
	for job := range jobs {
		logger.Info(
			"Worker processing download",
			"worker_id", workerID,
			"user_id", job.Message.From.ID,
			"user_name", job.Message.From.UserName,
			"message_text", job.Message.Text,
		)
		jobBaseCtx := context.WithoutCancel(ctx)
		mediaDownloader := NewYTDLPDownloader(job.DownloadRequest, job.YTDLPConfig)
		handleDownloadRequest(jobBaseCtx, bot, logger, job.Message, mediaDownloader, downloadTimeout)
		logger.Info(
			"Worker finished download",
			"worker_id", workerID,
			"user_id", job.Message.From.ID,
			"user_name", job.Message.From.UserName,
			"message_text", job.Message.Text,
		)
		wg.Done()
	}
}

// removeFile is a test seam for deleting downloaded files after processing.
var removeFile = os.Remove

// handleDownloadRequest performs the download, sends the resulting media or a
// fallback reply, and removes the downloaded file when possible.
func handleDownloadRequest(
	ctx context.Context,
	client TelegramBotClient,
	logger *slog.Logger,
	message *TelegramAPIMessage,
	mediaDownloader MediaDownloader,
	downloadTimeout time.Duration,
) {
	downloadCtx, cancelDownload := context.WithTimeout(ctx, downloadTimeout)
	defer cancelDownload()
	mediaFilename, err := mediaDownloader.Download(downloadCtx)

	sendCtx, cancelSend := context.WithTimeout(ctx, telegramSendGrace)
	defer cancelSend()

	if err != nil {
		logger.Error(
			"Failed to download request",
			"user_id", message.From.ID,
			"user_name", message.From.UserName,
			"message_text", message.Text,
			"error", err,
		)
		sendReply(sendCtx, client, logger, message, "I could not download your request 😿")
		return
	}

	switch mediaDownloader.MediaKind() {
	case MediaAudio:
		_, err = client.SendAudio(sendCtx, message.Chat.ID, message.MessageID, mediaFilename)
	case MediaVideo:
		_, err = client.SendVideo(sendCtx, message.Chat.ID, message.MessageID, mediaFilename)
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
				sendCtx,
				client,
				logger,
				message,
				"I downloaded it, but the file is too big for me to send on Telegram 😿",
			)
		} else {
			sendReply(sendCtx, client, logger, message, "I downloaded it, but I couldn't send it to you 🙀")
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

// sendReply sends a text reply to the given Telegram message and logs send
// failures.
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

// logTelegramSendError logs a Telegram send failure for the given user.
func logTelegramSendError(logger *slog.Logger, userName string, userID int64, err error) {
	logger.Error(
		"Failed to send Telegram message",
		"user_id", userID,
		"user_name", userName,
		"error", err,
	) // #nosec G706
}
