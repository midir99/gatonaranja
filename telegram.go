package main

import (
	"fmt"
	"log"
	"os"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func handleDownloadRequest(bot *tgbotapi.BotAPI, message *tgbotapi.Message, downloadRequest DownloadRequest) {
	mediaFilename, err := downloadRequest.Download()
	if err != nil {
		log.Printf("[%s %d] Unable to complete request %s: %s", message.From.UserName, message.From.ID, message.Text, err)
		msg := tgbotapi.NewMessage(
			message.Chat.ID,
			fmt.Sprintf("I could not download your video :(\n%s", err),
		)
		msg.ReplyToMessageID = message.MessageID
		bot.Send(msg)
		return
	}

	if downloadRequest.audioOnly {
		audioMsg := tgbotapi.NewAudio(message.Chat.ID, tgbotapi.FilePath(mediaFilename))
		audioMsg.ReplyToMessageID = message.MessageID
		bot.Send(audioMsg)
	} else {
		videoMsg := tgbotapi.NewVideo(message.Chat.ID, tgbotapi.FilePath(mediaFilename))
		videoMsg.ReplyToMessageID = message.MessageID
		bot.Send(videoMsg)
	}
	log.Printf("[%s %d] Request %s completed", message.From.UserName, message.From.ID, message.Text)
	if err := os.Remove(mediaFilename); err != nil {
		log.Printf("[%s %d] Unable to erase file %s", message.From.UserName, message.From.ID, mediaFilename)
	}
}

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
		log.Printf("[%s %d] Unauthorized user sent: %s", message.From.UserName, message.From.ID, message.Text)
		msg := tgbotapi.NewMessage(message.Chat.ID, "Your user is not authorized")
		bot.Send(msg)
		return
	}
	log.Printf("[%s %d] Authorized user sent: %s", message.From.UserName, message.From.ID, message.Text)

	// Let the user know you are working on the download
	downloadRequest, err := ParseDownloadRequest(message.Text)
	if err != nil {
		log.Printf("[%s %d] Unable to complete request %s: %s", message.From.UserName, message.From.ID, message.Text, err)
		msg := tgbotapi.NewMessage(
			message.Chat.ID,
			fmt.Sprintf("I could not download your video :(\n%s", err),
		)
		msg.ReplyToMessageID = message.MessageID
		bot.Send(msg)
		return
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, "Wait a minute...")
	msg.ReplyToMessageID = message.MessageID
	bot.Send(msg)

	go func() {
		downloadSlots <- struct{}{}
		defer func() { <-downloadSlots }()

		handleDownloadRequest(bot, message, downloadRequest)
	}()
}

func RunTelegramBot(bot *tgbotapi.BotAPI, authorizedUsers []int64, downloadSlots chan struct{}) error {
	// Start the infinite loop to receive messages
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)
	for update := range updates {
		handleMessage(bot, update.Message, authorizedUsers, downloadSlots)
	}
}
