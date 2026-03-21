package main

import (
	"log"
	"os"

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

// logTelegramSendError logs a Telegram send failure for the given user.
func logTelegramSendError(userName string, userID int64, err error) {
	log.Printf(
		"[%s %d] Failed to send Telegram message: %s",
		userName,
		userID,
		err,
	)
}

// sendReply sends a text reply to the given Telegram message and logs any
// error returned by the Telegram API.
func sendReply(bot *tgbotapi.BotAPI, message *tgbotapi.Message, text string) {
	msg := tgbotapi.NewMessage(message.Chat.ID, text)
	msg.ReplyToMessageID = message.MessageID
	_, err := bot.Send(msg)
	if err != nil {
		logTelegramSendError(message.From.UserName, message.From.ID, err)
	}
}

// handleDownloadRequest executes the download request and replies to the
// original Telegram message with the downloaded media or an error message.
func handleDownloadRequest(bot *tgbotapi.BotAPI, message *tgbotapi.Message, downloadRequest DownloadRequest) {
	mediaFilename, err := downloadRequest.Download()
	if err != nil {
		log.Printf(
			"[%s %d] Failed to download request %q: %s",
			message.From.UserName,
			message.From.ID,
			message.Text,
			err,
		)
		sendReply(bot, message, "I could not download your video :(")
		return
	}

	var mediaMsg tgbotapi.Chattable
	if downloadRequest.audioOnly {
		audioMsg := tgbotapi.NewAudio(message.Chat.ID, tgbotapi.FilePath(mediaFilename))
		audioMsg.ReplyToMessageID = message.MessageID
		mediaMsg = audioMsg
	} else {
		videoMsg := tgbotapi.NewVideo(message.Chat.ID, tgbotapi.FilePath(mediaFilename))
		videoMsg.ReplyToMessageID = message.MessageID
		mediaMsg = videoMsg
	}
	_, err = bot.Send(mediaMsg)
	if err == nil {
		log.Printf("[%s %d] Completed request %q", message.From.UserName, message.From.ID, message.Text)
	} else {
		sendReply(bot, message, "I downloaded it, but I couldn't send it to you.")
		logTelegramSendError(message.From.UserName, message.From.ID, err)
	}
	if err := os.Remove(mediaFilename); err != nil {
		log.Printf(
			"[%s %d] Failed to remove downloaded file %q: %s",
			message.From.UserName,
			message.From.ID,
			mediaFilename,
			err,
		)
	}
}

// handleMessage validates the incoming Telegram message, checks user
// authorization, parses the download request, sends an acknowledgement,
// and dispatches the download work asynchronously.
func handleMessage(
	bot *tgbotapi.BotAPI,
	message *tgbotapi.Message,
	authorizedUsers []int64,
	downloadSlots chan struct{},
) {
	if message == nil {
		return
	}
	// Check if user is authorized
	if !UserIsAuthorized(message.From.ID, authorizedUsers) {
		log.Printf("[%s %d] Rejected unauthorized request: %q", message.From.UserName, message.From.ID, message.Text)
		sendReply(bot, message, "You are not authorized to use this bot")
		return
	}
	log.Printf("[%s %d] Received request: %q", message.From.UserName, message.From.ID, message.Text)

	// Let the user know you are working on the download
	downloadRequest, err := ParseDownloadRequest(message.Text)
	if err != nil {
		log.Printf(
			"[%s %d] Failed to parse request %q: %s",
			message.From.UserName,
			message.From.ID,
			message.Text,
			err,
		)
		sendReply(bot, message, usageMessage)
		return
	}

	sendReply(bot, message, "Wait a minute...")
	go func() {
		downloadSlots <- struct{}{}
		defer func() { <-downloadSlots }()

		handleDownloadRequest(bot, message, downloadRequest)
	}()
}

// RunTelegramBot starts receiving Telegram updates and handles each incoming
// message using the provided authorization list and download concurrency limit.
func RunTelegramBot(bot *tgbotapi.BotAPI, authorizedUsers []int64, downloadSlots chan struct{}) {
	// Start the infinite loop to receive messages
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)
	for update := range updates {
		handleMessage(bot, update.Message, authorizedUsers, downloadSlots)
	}
}
