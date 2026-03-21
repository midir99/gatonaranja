package main

import (
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

func validateAuthorizedUsers(authorizedUsers string) ([]int64, error) {
	// Process the authorized-users value
	authorizedUsers = strings.TrimSpace(authorizedUsers)
	if authorizedUsers == "" {
		authorizedUsers = os.Getenv("AUTHORIZED_USERS")
		authorizedUsers = strings.TrimSpace(authorizedUsers)
	}
	authorizedUsersIntArray := []int64{}
	if authorizedUsers != "" {
		authorizedUsersArray := strings.Split(authorizedUsers, ",")
		for _, userID := range authorizedUsersArray {
			userIDInt, err := strconv.ParseInt(strings.TrimSpace(userID), 10, 0)
			if err != nil {
				return nil, fmt.Errorf("invalid authorized user ID %q: must be a valid Telegram user ID", userID)
			}
			authorizedUsersIntArray = append(authorizedUsersIntArray, userIDInt)
		}
	}
	return authorizedUsersIntArray, nil
}

func validateTelegramBotToken(telegramBotToken string) (string, error) {
	// Process the telegram-bot-token value
	telegramBotToken = strings.TrimSpace(telegramBotToken)
	if telegramBotToken == "" {
		telegramBotToken = os.Getenv("TELEGRAM_BOT_TOKEN")
		telegramBotToken = strings.TrimSpace(telegramBotToken)
		if telegramBotToken == "" {
			return "", fmt.Errorf("telegram bot token is required: use -telegram-bot-token or TELEGRAM_BOT_TOKEN")
		}
	}
	return telegramBotToken, nil
}

func fallbackToEnv(variable string, envVariableName string) string {

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

	authorizedUsersArray, err := validateAuthorizedUsers(authorizedUsers)
	if err != nil {
		return Config{}, err
	}
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
