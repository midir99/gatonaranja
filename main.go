package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// Version identifies the application version printed by -version. It defaults to "dev" and can be overridden at build time with -ldflags.
var Version = "dev"

// newLogger creates the application logger used by the bot.
func newLogger() *slog.Logger {
	return slog.New(
		slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}),
	)
}

// run validates the startup configuration, initializes the bot, and runs the
// Telegram update loop until shutdown. It may also return early for non-bot
// startup flows such as printing the application version.
func run(logger *slog.Logger) error {
	// Parse the flags
	config, err := ParseConfig(os.Args[1:])
	if err != nil {
		return fmt.Errorf("startup failed: invalid configuration: %w", err)
	}

	if config.PrintVersion {
		fmt.Printf("gatonaranja %s\n", Version)
		return nil
	}

	// Check system has required dependencies
	err = ValidateRequiredDependencies()
	if err != nil {
		return fmt.Errorf("startup failed while checking dependencies: %w", err)
	}

	// Set up graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Bootstrap the bot
	bot, err := NewTelegramAPIClient(config.TelegramBotToken, nil)
	if err != nil {
		return fmt.Errorf("startup failed: unable to create Telegram bot: %w", err)
	}

	getMeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	botUser, err := bot.GetMe(getMeCtx)
	if err != nil {
		return fmt.Errorf("startup failed: GetMe failed: %w", err)
	}
	logger.Info("Telegram bot started", "bot_user_id", botUser.ID, "bot_user_name", botUser.UserName) // #nosec G706

	var downloadsWG sync.WaitGroup

	// Set up a semaphore for limiting the downloads
	downloadSlots := make(chan struct{}, config.MaxConcurrentDownloads)
	logger.Info(
		"Starting Telegram update loop",
		"max_concurrent_downloads",
		cap(downloadSlots),
		"download_timeout_seconds",
		config.DownloadTimeout.Seconds(),
	)

	downloadRequestHandler, err := NewDownloadRequestHandler(
		bot,
		logger,
		config.AuthorizedUsers,
		config.DownloadTimeout,
		downloadSlots,
		&downloadsWG,
	)
	if err != nil {
		return fmt.Errorf("startup failed: unable to create download request handler: %w", err)
	}

	if err := RunTelegramBot(ctx, bot, logger, downloadRequestHandler.HandleUpdate); err != nil {
		return fmt.Errorf("telegram update loop failed: %w", err)
	}

	logger.Info("Waiting for active downloads to finish")
	downloadsWG.Wait()
	logger.Info("Shutdown complete")
	return nil
}

// main creates the application logger, runs the program, and exits with the
// appropriate status code.
func main() {
	logger := newLogger()

	err := run(logger)
	if err == nil {
		return
	}
	if errors.Is(err, flag.ErrHelp) {
		os.Exit(0)
	}
	logger.Error("Application failed", "error", err)
	os.Exit(1)
}
