package main

import (
	"context"
	"log/slog"
	"os"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const usageMessage = `I couldn't understand your request, but I can help you download YouTube videos.

Try one of these examples:

Download a video
https://www.youtube.com/watch?v=AqjB8DGt85U

Download a video clip
https://www.youtube.com/watch?v=AqjB8DGt85U 1:00-1:05

Download audio only
https://www.youtube.com/watch?v=AqjB8DGt85U audio

Download an audio clip
https://www.youtube.com/watch?v=AqjB8DGt85U 1:00-1:05 audio

You can also use start or end:
https://www.youtube.com/watch?v=AqjB8DGt85U start-0:10
https://www.youtube.com/watch?v=AqjB8DGt85U 0:10-end`

type MessageSender interface {
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
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

// sendReply sends a text reply to the given Telegram message and logs any
// error returned by the Telegram API.
func sendReply(
	bot MessageSender,
	logger *slog.Logger,
	message *tgbotapi.Message,
	text string,
) {
	msg := tgbotapi.NewMessage(message.Chat.ID, text)
	msg.ReplyToMessageID = message.MessageID
	_, err := bot.Send(msg)
	if err != nil {
		logTelegramSendError(logger, message.From.UserName, message.From.ID, err)
	}
}

var removeFile = os.Remove

// handleDownloadRequest executes the download request and replies to the
// original Telegram message with the downloaded media or an error message.
func handleDownloadRequest(
	ctx context.Context,
	bot MessageSender,
	logger *slog.Logger,
	message *tgbotapi.Message,
	mediaDownloader MediaDownloader,
) {
	mediaFilename, err := mediaDownloader.Download(ctx)
	if err != nil {
		logger.Error(
			"Failed to download request",
			"user_id", message.From.ID,
			"user_name", message.From.UserName,
			"message_text", message.Text,
			"error", err,
		)
		sendReply(bot, logger, message, "I could not download your video :(")
		return
	}

	var mediaMsg tgbotapi.Chattable
	switch mediaDownloader.MediaKind() {
	case MediaAudio:
		audioMsg := tgbotapi.NewAudio(message.Chat.ID, tgbotapi.FilePath(mediaFilename))
		audioMsg.ReplyToMessageID = message.MessageID
		mediaMsg = audioMsg
	case MediaVideo:
		videoMsg := tgbotapi.NewVideo(message.Chat.ID, tgbotapi.FilePath(mediaFilename))
		videoMsg.ReplyToMessageID = message.MessageID
		mediaMsg = videoMsg
	}
	_, err = bot.Send(mediaMsg)
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
		sendReply(bot, logger, message, "I downloaded it, but I couldn't send it to you.")
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

var dispatchDownloadRequest = func(
	ctx context.Context,
	bot MessageSender,
	logger *slog.Logger,
	message *tgbotapi.Message,
	downloadSlots chan struct{},
	mediaDownloader MediaDownloader,
	downloadsWG *sync.WaitGroup,
) {
	downloadsWG.Add(1)
	go func() {
		defer downloadsWG.Done()

		downloadSlots <- struct{}{}
		defer func() { <-downloadSlots }()

		handleDownloadRequest(ctx, bot, logger, message, mediaDownloader)
	}()
}

// handleMessage validates the incoming Telegram message, checks user
// authorization, parses the download request, sends an acknowledgement,
// and dispatches the download work asynchronously.
func handleMessage(
	ctx context.Context,
	bot MessageSender,
	logger *slog.Logger,
	message *tgbotapi.Message,
	authorizedUsers []int64,
	downloadSlots chan struct{},
	downloadsWG *sync.WaitGroup,
) {
	if message == nil {
		return
	}
	select {
	case <-ctx.Done():
		logger.Info(
			"Ignoring request because shutdown is in progress",
			"user_id", message.From.ID,
			"user_name", message.From.UserName,
		)
		return
	default:
	}
	// Check if user is authorized
	if !UserIsAuthorized(message.From.ID, authorizedUsers) {
		logger.Warn(
			"Rejected unauthorized request",
			"user_id", message.From.ID,
			"user_name", message.From.UserName,
			"message_text", message.Text,
		) // #nosec G706
		sendReply(bot, logger, message, "You are not authorized to use this bot")
		return
	}
	logger.Info(
		"Received request",
		"user_id", message.From.ID,
		"user_name", message.From.UserName,
		"message_text", message.Text,
	) // #nosec G706

	downloadRequest, err := ParseDownloadRequest(message.Text)
	if err != nil {
		logger.Warn(
			"Failed to parse request",
			"user_id", message.From.ID,
			"user_name", message.From.UserName,
			"message_text", message.Text,
			"error", err,
		) // #nosec G706
		sendReply(bot, logger, message, usageMessage)
		return
	}

	// Let the user know you are working on the download
	sendReply(bot, logger, message, "Wait a minute...")
	dispatchDownloadRequest(ctx, bot, logger, message, downloadSlots, downloadRequest, downloadsWG)
}

var getUpdatesChan = func(bot *tgbotapi.BotAPI, u tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
	return bot.GetUpdatesChan(u)
}

// RunTelegramBot starts receiving Telegram updates and handles each incoming
// message using the provided authorization list and download concurrency limit.
func RunTelegramBot(ctx context.Context, bot *tgbotapi.BotAPI, logger *slog.Logger, authorizedUsers []int64, downloadSlots chan struct{}, downloadsWG *sync.WaitGroup) {
	// Start the infinite loop to receive messages
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := getUpdatesChan(bot, u)
	for {
		select {
		case <-ctx.Done():
			logger.Info("Stopping Telegram update loop")
			return
		case update, ok := <-updates:
			if !ok {
				return
			}
			handleMessage(ctx, bot, logger, update.Message, authorizedUsers, downloadSlots, downloadsWG)
		}
	}
}
