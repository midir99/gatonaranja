package main

import (
	"fmt"
	"log"
	"os/exec"

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
	// Check system has required dependencies
	err := CheckSystemHasRequiredDependencies()
	if err != nil {
		log.Fatalf("Unable to start since system has missing dependencies: %s", err)
	}

	// Parse the flags
	config, err := ParseConfig()
	if err != nil {
		log.Fatalf("Configuration error: %s", err)
	}

	// Bootstrap the bot
	bot, err := tgbotapi.NewBotAPI(config.TelegramBotToken)
	if err != nil {
		log.Fatalf("Unable to start since can not create Telegram bot: %s", err)
	}
	log.Printf("Authorized on account %s", bot.Self.UserName)
	// Set up a semaphore for limiting the downloads
	downloadSlots := make(chan struct{}, 5)
	RunTelegramBot(bot, config.AuthorizedUsers, downloadSlots)
}
