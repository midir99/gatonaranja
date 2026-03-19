package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func CheckSystemHasRequiredDependencies() error {
	dependencies := []string{
		"ffmpeg",
		"yt-dlp",
	}
	for _, dep := range dependencies {
		_, err := exec.LookPath(dep)
		if err != nil {
			return fmt.Errorf("dependency %s is not installed in the system: %s", dep, err)
		}
	}
	return nil
}

func main() {
	// Set up logging
	logFileEnv := strings.TrimSpace(os.Getenv("LOGFILE"))
	if logFileEnv != "" {
		f, err := os.OpenFile(logFileEnv, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			log.Fatalf("Unable to start since can not open the file pointed by LOGFILE (environment variable) %s: %s", logFileEnv, err)
		}
		defer f.Close()
		log.SetOutput(f)
	}
	// Check system has required dependencies
	err := CheckSystemHasRequiredDependencies()
	if err != nil {
		log.Fatalf("Unable to start since system has missing dependencies: %s", err)
	}
	// Load authorized users
	authorizedUserIds, err := LoadAuthorizedUserIds("AUTHORIZED_USERS")
	if err != nil {
		log.Fatalf("Unable to start since can not load user ids from AUTHORIZED_USERS (environment variable): %s", err)
	}
	if len(authorizedUserIds) == 0 {
		log.Print("You did not specified AUTHORIZED_USERS so everyone is able to use this bot")
	}
	// Bootstrap the bot
	token := os.Getenv("TOKEN")
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Fatalf("Unable to start since can not create Telegram bot: %s", err)
	}
	log.Printf("Authorized on account %s", bot.Self.UserName)
	// Set up a semaphore for limiting the downloads
	downloadSlots := make(chan struct{}, 5)
	// Start the infinite loop to receive messages
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)
	for update := range updates {
		if update.Message == nil {
			continue
		}
		// Check if user is authorized
		if !UserIsAuthorized(update.Message.From.ID, authorizedUserIds) {
			log.Printf("[%s %d] Unauthorized user sent: %s", update.Message.From.UserName, update.Message.From.ID, update.Message.Text)
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "You are NOT AUTHORIZED to use me! 😠")
			bot.Send(msg)
			continue
		}
		log.Printf("[%s %d] Authorized user sent: %s", update.Message.From.UserName, update.Message.From.ID, update.Message.Text)

		// Let the user know you are working on the download
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Ok, just wait a second...")
		msg.ReplyToMessageID = update.Message.MessageID
		bot.Send(msg)
		downloadRequest, err := ParseDownloadRequest(update.Message.Text)
		if err != nil {
			log.Printf("[%s %d] Unable to complete request %s: %s", update.Message.From.UserName, update.Message.From.ID, update.Message.Text, err)
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "I'm sorry I was not able to download your video ☹")
			msg.ReplyToMessageID = update.Message.MessageID
			bot.Send(msg)
			continue
		}
		videoFilename, err := downloadRequest.Download()
		if err != nil {
			log.Printf("[%s %d] Unable to complete request %s: %s", update.Message.From.UserName, update.Message.From.ID, update.Message.Text, err)
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "I'm sorry I was not able to download your video ☹")
			msg.ReplyToMessageID = update.Message.MessageID
			bot.Send(msg)
			continue
		}
		if downloadRequest.audioOnly {
			audioMsg := tgbotapi.NewAudio(update.Message.Chat.ID, tgbotapi.FilePath(videoFilename))
			msg.ReplyToMessageID = update.Message.MessageID
			bot.Send(audioMsg)
		} else {
			videoMsg := tgbotapi.NewVideo(update.Message.Chat.ID, tgbotapi.FilePath(videoFilename))
			msg.ReplyToMessageID = update.Message.MessageID
			bot.Send(videoMsg)
		}
		log.Printf("[%s %d] Request %s completed", update.Message.From.UserName, update.Message.From.ID, update.Message.Text)
		if err := os.Remove(videoFilename); err != nil {
			log.Printf("[%s %d] Unable to erase file %s", update.Message.From.UserName, update.Message.From.ID, videoFilename)
		}
	}
}
