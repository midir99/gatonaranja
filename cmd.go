package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config contains the runtime configuration required to start the bot.
type Config struct {
	AuthorizedUsers  []int64
	TelegramBotToken string
}

// validateAuthorizedUsers parses a comma-separated list of Telegram user IDs
// and returns them as int64 values.
func validateAuthorizedUsers(authorizedUsers string) ([]int64, error) {
	authorizedUsersIntArray := []int64{}
	if authorizedUsers != "" {
		authorizedUsersArray := strings.SplitSeq(authorizedUsers, ",")
		for userID := range authorizedUsersArray {
			userIDInt, err := strconv.ParseInt(strings.TrimSpace(userID), 10, 0)
			if err != nil {
				return nil, fmt.Errorf("invalid authorized user ID %q: must be a valid Telegram user ID", userID)
			}
			authorizedUsersIntArray = append(authorizedUsersIntArray, userIDInt)
		}
	}
	return authorizedUsersIntArray, nil
}

// validateTelegramBotToken validates that the Telegram bot token is not empty.
func validateTelegramBotToken(telegramBotToken string) (string, error) {
	// Process the telegram-bot-token value
	if telegramBotToken == "" {
		return "", errors.New("telegram bot token is required: use -telegram-bot-token or TELEGRAM_BOT_TOKEN")
	}
	return telegramBotToken, nil
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

// ParseConfig parses command-line flags, falls back to environment variables
// when needed, and returns the runtime configuration for the bot.
func ParseConfig(args []string) (Config, error) {
	// Use a FlagSet for easier unit testing
	fs := flag.NewFlagSet("gatonaranja", flag.ContinueOnError)

	// Parse the flags
	var authorizedUsers string
	var telegramBotToken string

	fs.StringVar(
		&authorizedUsers,
		"authorized-users",
		"",
		"A comma-separated list of Telegram user IDs allowed to use the bot (defaults to AUTHORIZED_USERS). If no IDs are specified, everyone can use the bot",
	)
	fs.StringVar(
		&telegramBotToken,
		"telegram-bot-token",
		"",
		"The Telegram bot token used to authenticate this bot (defaults to TELEGRAM_BOT_TOKEN)",
	)
	if err := fs.Parse(args); err != nil {
		return Config{}, err
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

	// Prepare the configuration
	config := Config{
		AuthorizedUsers:  authorizedUsersArray,
		TelegramBotToken: telegramBotToken,
	}
	return config, nil
}
