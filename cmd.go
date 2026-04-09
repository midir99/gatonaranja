package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Config contains the runtime configuration required to start the bot.
type Config struct {
	AuthorizedUsers        []int64
	TelegramBotToken       string
	MaxConcurrentDownloads int
	MaxQueuedDownloads     int
	DownloadTimeout        time.Duration
	YTDLPConfig            string
	PrintVersion           bool
}

// validateAuthorizedUsers parses a comma-separated list of Telegram user IDs,
// trims surrounding whitespace from each value, removes duplicates, and
// returns the resulting IDs as int64 values.
func validateAuthorizedUsers(authorizedUsers string) ([]int64, error) {
	authorizedUsers = strings.TrimSpace(authorizedUsers)
	authorizedUsersIntArray := []int64{}
	if authorizedUsers != "" {
		authorizedUsersArray := strings.SplitSeq(authorizedUsers, ",")
		for userID := range authorizedUsersArray {
			userIDInt, err := strconv.ParseInt(strings.TrimSpace(userID), 10, 0)
			if err != nil {
				return nil, fmt.Errorf("invalid authorized user ID %q: must be a valid Telegram user ID", userID)
			}
			if !slices.Contains(authorizedUsersIntArray, userIDInt) {
				authorizedUsersIntArray = append(authorizedUsersIntArray, userIDInt)
			}
		}
	}
	return authorizedUsersIntArray, nil
}

// validateTelegramBotToken trims the Telegram bot token and reports an error
// if the resulting value is empty.
func validateTelegramBotToken(telegramBotToken string) (string, error) {
	invalidErr := errors.New(
		"telegram bot token is required: use -telegram-bot-token or TELEGRAM_BOT_TOKEN",
	)

	telegramBotToken = strings.TrimSpace(telegramBotToken)
	if telegramBotToken == "" {
		return "", invalidErr
	}
	return telegramBotToken, nil
}

// validateMaxConcurrentDownloads parses the maximum number of concurrent
// downloads and validates that it is between 1 and 100.
func validateMaxConcurrentDownloads(maxConcurrentDownloads string) (int, error) {
	invalidErr := fmt.Errorf(
		"invalid max concurrent downloads %q: must be between 1 and 100",
		maxConcurrentDownloads,
	)

	maxConcurrentDownloads = strings.TrimSpace(maxConcurrentDownloads)
	maxConcurrentDownloadsInt, err := strconv.Atoi(maxConcurrentDownloads)
	if err != nil {
		return 0, invalidErr
	}
	if maxConcurrentDownloadsInt <= 0 || maxConcurrentDownloadsInt > 100 {
		return 0, invalidErr
	}
	return maxConcurrentDownloadsInt, nil
}

// validateMaxQueuedDownloads parses the maximum number of queued
// downloads and validates that it is between 1 and 100.
func validateMaxQueuedDownloads(maxQueuedDownloads string) (int, error) {
	invalidErr := fmt.Errorf(
		"invalid max queued downloads %q: must be between 1 and 100",
		maxQueuedDownloads,
	)

	maxQueuedDownloads = strings.TrimSpace(maxQueuedDownloads)
	maxQueuedDownloadsInt, err := strconv.Atoi(maxQueuedDownloads)
	if err != nil {
		return 0, invalidErr
	}
	if maxQueuedDownloadsInt <= 0 || maxQueuedDownloadsInt > 100 {
		return 0, invalidErr
	}
	return maxQueuedDownloadsInt, nil
}

// validateDownloadTimeout parses a download timeout and validates that it is
// between 1 second and 10 minutes.
func validateDownloadTimeout(downloadTimeout string) (time.Duration, error) {
	invalidErr := fmt.Errorf(
		"invalid download timeout %q: must be between 1s and 10m, for example 30s, 2m, or 5m",
		downloadTimeout,
	)

	downloadTimeout = strings.TrimSpace(downloadTimeout)
	downloadTimeoutDuration, err := time.ParseDuration(downloadTimeout)
	if err != nil {
		return 0, invalidErr
	}
	if downloadTimeoutDuration.Seconds() <= 0 || downloadTimeoutDuration.Minutes() > 10 {
		return 0, invalidErr
	}
	return downloadTimeoutDuration, nil
}

// validateYTDLPConfig validates an optional yt-dlp configuration file path,
// expands a leading "~", normalizes it to an absolute path, and ensures the
// final path points to a readable regular file.
func validateYTDLPConfig(ytdlpConfig string) (string, error) {
	ytdlpConfigTrimmed := strings.TrimSpace(ytdlpConfig)
	if ytdlpConfigTrimmed == "" {
		return "", nil
	}

	if ytdlpConfigTrimmed == "-" {
		return "", fmt.Errorf("invalid ytdlp config %q: stdin is not supported", ytdlpConfig)
	}

	if ytdlpConfigTrimmed == "~" || strings.HasPrefix(ytdlpConfigTrimmed, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("invalid ytdlp config %q: resolve home directory: %w", ytdlpConfig, err)
		}
		if ytdlpConfigTrimmed == "~" {
			ytdlpConfigTrimmed = homeDir
		} else {
			ytdlpConfigTrimmed = filepath.Join(homeDir, strings.TrimPrefix(ytdlpConfigTrimmed, "~/"))
		}
	}

	ytdlpConfigAbsPath, err := filepath.Abs(ytdlpConfigTrimmed)
	if err != nil {
		return "", fmt.Errorf("invalid ytdlp config %q: %w", ytdlpConfig, err)
	}

	ytdlpConfigFile, err := os.Open(ytdlpConfigAbsPath)
	if err != nil {
		return "", fmt.Errorf("invalid ytdlp config %q: %w", ytdlpConfig, err)
	}
	defer ytdlpConfigFile.Close()

	ytdlpConfigFileInfo, err := ytdlpConfigFile.Stat()
	if err != nil {
		return "", fmt.Errorf("invalid ytdlp config %q: %w", ytdlpConfig, err)
	}
	if !ytdlpConfigFileInfo.Mode().IsRegular() {
		return "", fmt.Errorf("invalid ytdlp config %q: must be a regular file", ytdlpConfig)
	}

	return ytdlpConfigAbsPath, nil
}

// flagOrEnv returns the trimmed flag value when it is not empty; otherwise it
// returns the trimmed value of the given environment variable.
func flagOrEnv(variableValue, variableEnvName string) string {
	variableValue = strings.TrimSpace(variableValue)
	if variableValue != "" {
		return variableValue
	}
	variableValue = os.Getenv(variableEnvName)
	variableValue = strings.TrimSpace(variableValue)
	return variableValue
}

// defaultTo returns the default value when the given value is empty after
// trimming surrounding whitespace; otherwise it returns the trimmed value.
func defaultTo(variableValue, variableDefaultValue string) string {
	variableValue = strings.TrimSpace(variableValue)
	if variableValue == "" {
		return variableDefaultValue
	}
	return variableValue
}

// ParseConfig parses command-line flags, falls back to environment variables
// when needed, and returns the runtime configuration for the bot. When the
// -version flag is provided, it returns a Config with PrintVersion set without
// validating the rest of the runtime configuration.
func ParseConfig(args []string) (Config, error) {
	// Use a FlagSet for easier unit testing
	fs := flag.NewFlagSet("gatonaranja", flag.ContinueOnError)

	// Parse the flags
	var (
		authorizedUsers        string
		telegramBotToken       string
		maxConcurrentDownloads string
		maxQueuedDownloads     string
		downloadTimeout        string
		ytdlpConfig            string
		printVersion           bool
	)

	fs.StringVar(
		&authorizedUsers,
		"authorized-users",
		"",
		"A comma-separated list of Telegram user IDs allowed to use the bot, if no IDs are specified, everyone can use the bot (can also be set with AUTHORIZED_USERS)",
	)
	fs.StringVar(
		&telegramBotToken,
		"telegram-bot-token",
		"",
		"The Telegram bot token used to authenticate this bot (can also be set with TELEGRAM_BOT_TOKEN)",
	)
	fs.StringVar(
		&maxConcurrentDownloads,
		"max-concurrent-downloads",
		"",
		"Maximum number of downloads that can run at the same time (default: 5; can also be set with MAX_CONCURRENT_DOWNLOADS)",
	)
	fs.StringVar(
		&maxQueuedDownloads,
		"max-queued-downloads",
		"",
		"Maximum number of accepted download requests waiting in the queue (default: 5; can also be set with MAX_QUEUED_DOWNLOADS)",
	)
	fs.StringVar(
		&downloadTimeout,
		"download-timeout",
		"",
		"Maximum time allowed for a single download before it is canceled, for example: 30s, 2m, or 5m (default: 5m; can also be set with DOWNLOAD_TIMEOUT)",
	)
	fs.StringVar(
		&ytdlpConfig,
		"ytdlp-config",
		"",
		"A yt-dlp configuration file with extra options such as proxy or cookies (can also be set with YTDLP_CONFIG)",
	)
	fs.BoolVar(
		&printVersion,
		"version",
		false,
		"Print the application version and exit",
	)

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	if printVersion {
		return Config{PrintVersion: true}, nil
	}

	authorizedUsers = flagOrEnv(authorizedUsers, "AUTHORIZED_USERS")
	authorizedUsersArray, err := validateAuthorizedUsers(authorizedUsers)
	if err != nil {
		return Config{}, err
	}

	telegramBotToken = flagOrEnv(telegramBotToken, "TELEGRAM_BOT_TOKEN")
	telegramBotToken, err = validateTelegramBotToken(telegramBotToken)
	if err != nil {
		return Config{}, err
	}

	maxConcurrentDownloads = flagOrEnv(maxConcurrentDownloads, "MAX_CONCURRENT_DOWNLOADS")
	maxConcurrentDownloads = defaultTo(maxConcurrentDownloads, "5")
	maxConcurrentDownloadsInt, err := validateMaxConcurrentDownloads(maxConcurrentDownloads)
	if err != nil {
		return Config{}, err
	}

	maxQueuedDownloads = flagOrEnv(maxQueuedDownloads, "MAX_QUEUED_DOWNLOADS")
	maxQueuedDownloads = defaultTo(maxQueuedDownloads, "5")
	maxQueuedDownloadsInt, err := validateMaxQueuedDownloads(maxQueuedDownloads)
	if err != nil {
		return Config{}, err
	}

	downloadTimeout = flagOrEnv(downloadTimeout, "DOWNLOAD_TIMEOUT")
	downloadTimeout = defaultTo(downloadTimeout, "5m")
	downloadTimeoutDuration, err := validateDownloadTimeout(downloadTimeout)
	if err != nil {
		return Config{}, err
	}

	ytdlpConfig = flagOrEnv(ytdlpConfig, "YTDLP_CONFIG")
	ytdlpConfig = defaultTo(ytdlpConfig, "")
	ytdlpConfig, err = validateYTDLPConfig(ytdlpConfig)
	if err != nil {
		return Config{}, err
	}

	// Prepare the configuration
	config := Config{
		AuthorizedUsers:        authorizedUsersArray,
		TelegramBotToken:       telegramBotToken,
		MaxConcurrentDownloads: maxConcurrentDownloadsInt,
		MaxQueuedDownloads:     maxQueuedDownloadsInt,
		DownloadTimeout:        downloadTimeoutDuration,
		YTDLPConfig:            ytdlpConfig,
	}
	return config, nil
}
