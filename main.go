package main

import (
	"log/slog"
	"os"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// newLogger creates the application logger used by the bot.
func newLogger() *slog.Logger {
	return slog.New(
		slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}),
	)
}

// main initializes the bot and starts the Telegram update loop.
func main() {
	var logger = newLogger()

	// Check system has required dependencies
	err := ValidateRequiredDependencies()
	if err != nil {
		logger.Error("Startup failed while checking dependencies", "error", err)
		os.Exit(1)
	}

	// Parse the flags
	config, err := ParseConfig(os.Args[1:])
	if err != nil {
		logger.Error("Startup failed: invalid configuration", "error", err)
		os.Exit(1)
	}

	// Bootstrap the bot
	bot, err := tgbotapi.NewBotAPI(config.TelegramBotToken)
	if err != nil {
		logger.Error("Startup failed: unable to create Telegram bot", "error", err)
		os.Exit(1)
	}
	logger.Info("Telegram bot started", "bot_user_id", bot.Self.ID, "bot_user_name", bot.Self.UserName) // #nosec G706
	// Set up a semaphore for limiting the downloads
	downloadSlots := make(chan struct{}, 5)
	logger.Info("Starting Telegram update loop", "download_slots", cap(downloadSlots))
	RunTelegramBot(bot, logger, config.AuthorizedUsers, downloadSlots)
}
