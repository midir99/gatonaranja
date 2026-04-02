package main

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

type DownloadJob struct {
	Message         *TelegramAPIMessage
	DownloadRequest DownloadRequest
}

type DownloadRequestHandlerWP struct {
	client          TelegramBotClient
	logger          *slog.Logger
	authorizedUsers []int64
	downloadTimeout time.Duration
	downloadQueue   chan DownloadJob
	downloadsWG     *sync.WaitGroup
}

func NewDownloadRequestHandlerWP(
	client TelegramBotClient,
	logger *slog.Logger,
	authorizedUsers []int64,
	downloadTimeout time.Duration,
	downloadQueue chan DownloadJob,
	downloadsWG *sync.WaitGroup,
) (*DownloadRequestHandlerWP, error) {
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

	return &DownloadRequestHandlerWP{
		client:          client,
		logger:          logger,
		authorizedUsers: authorizedUsers,
		downloadTimeout: downloadTimeout,
		downloadQueue:   downloadQueue,
		downloadsWG:     downloadsWG,
	}, nil
}

func (h *DownloadRequestHandlerWP) HandleUpdate(ctx context.Context, update TelegramAPIUpdate) error {
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
		handleDownloadRequest(jobBaseCtx, bot, logger, job.Message, job.DownloadRequest, downloadTimeout)
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
