package main

import (
	"fmt"
	"log"
	"os"
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
		log.Fatalf("Startup failed while checking dependencies: %s", err)
	}

	// Parse the flags
	config, err := ParseConfig(os.Args[1:])
	if err != nil {
		log.Fatalf("Startup failed: invalid configuration: %s", err)
	}

	// Bootstrap the bot
	bot, err := tgbotapi.NewBotAPI(config.TelegramBotToken)
	if err != nil {
		log.Fatalf("Startup failed: unable to create Telegram bot: %s", err)
	}
	log.Printf("Telegram bot started as @%s", bot.Self.UserName)
	// Set up a semaphore for limiting the downloads
	downloadSlots := make(chan struct{}, 5)
	log.Printf("Starting Telegram update loop with %d download slots", cap(downloadSlots))
	RunTelegramBot(bot, config.AuthorizedUsers, downloadSlots)
}
