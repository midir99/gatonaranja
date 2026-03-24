package main

import (
	"errors"
	"testing"
)

func compareInt64Array(t *testing.T, got, want []int64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestValidateAuthorizedUsers(t *testing.T) {
	testCases := []struct {
		testName        string
		authorizedUsers string
		want            []int64
		err             error
	}{
		{
			"empty list",
			"",
			[]int64{},
			nil,
		},
		{
			"non-empty list",
			"1111111111,   1111111112,1111111113",
			[]int64{1111111111, 1111111112, 1111111113},
			nil,
		},
		{
			"duplicate user IDs",
			"1111111111,1111111111,1111111113",
			[]int64{1111111111, 1111111113},
			nil,
		},
		{
			"user IDs with spaces",
			" 1111111111 ,1111111112,   1111111113	\n,1111111114",
			[]int64{1111111111, 1111111112, 1111111113, 1111111114},
			nil,
		},
		{
			"invalid values 1",
			"rachelcorrierip,esperanza,123kj432",
			nil,
			errors.New(`invalid authorized user ID "rachelcorrierip": must be a valid Telegram user ID`),
		},
		{
			"invalid values 2",
			"1111111111, 1111 111111 ,1111111113",
			nil,
			errors.New(`invalid authorized user ID "1111 111111": must be a valid Telegram user ID`),
		},
		{
			"only spaces",
			"   \t\n  ",
			[]int64{},
			nil,
		},
		{
			"single user id with surrounding spaces",
			"   1111111111   ",
			[]int64{1111111111},
			nil,
		},
		{
			"duplicate user IDs with spaces",
			"1111111111, 1111111111 , 1111111112",
			[]int64{1111111111, 1111111112},
			nil,
		},
		{
			"empty element between commas",
			"1111111111,,1111111112",
			nil,
			errors.New(`invalid authorized user ID "": must be a valid Telegram user ID`),
		},
		{
			"trailing comma",
			"1111111111,",
			nil,
			errors.New(`invalid authorized user ID "": must be a valid Telegram user ID`),
		},
		{
			"leading comma",
			",1111111111",
			nil,
			errors.New(`invalid authorized user ID "": must be a valid Telegram user ID`),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			authorizedUsersArray, err := validateAuthorizedUsers(tc.authorizedUsers)
			if tc.err != nil && err == nil {
				t.Fatalf("got nil, want %q", tc.err.Error())
			}
			if tc.err != nil && err != nil && tc.err.Error() != err.Error() {
				t.Fatalf("got %q, want %q", err.Error(), tc.err.Error())
			}
			compareInt64Array(t, authorizedUsersArray, tc.want)
		})
	}
}
func TestValidateTelegramBotToken(t *testing.T) {
	testCases := []struct {
		testName         string
		telegramBotToken string
		want             string
		err              error
	}{
		{
			"token is valid",
			"thisisavalidtelegrambottoken",
			"thisisavalidtelegrambottoken",
			nil,
		},
		{
			"token is an empty string",
			"",
			"",
			errors.New("telegram bot token is required: use -telegram-bot-token or TELEGRAM_BOT_TOKEN"),
		},
		{
			"token contains spaces",
			"\n   \t   \rthisisavalidtelegrambottoken\n\t    ",
			"thisisavalidtelegrambottoken",
			nil,
		},
		{
			"token is only spaces",
			"   \n\t  ",
			"",
			errors.New("telegram bot token is required: use -telegram-bot-token or TELEGRAM_BOT_TOKEN"),
		},
		{
			"token with surrounding spaces",
			"   thisisavalidtelegrambottoken   ",
			"thisisavalidtelegrambottoken",
			nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			telegramBotToken, err := validateTelegramBotToken(tc.telegramBotToken)
			if tc.err != nil && err == nil {
				t.Fatalf("got nil, want %q", tc.err.Error())
			}
			if tc.err != nil && err != nil && tc.err.Error() != err.Error() {
				t.Fatalf("got %q, want %q", err.Error(), tc.err.Error())
			}
			if telegramBotToken != tc.want {
				t.Fatalf("got %q, want %q", telegramBotToken, tc.want)
			}
		})
	}
}
func TestFlagOrEnv(t *testing.T) {
	testCases := []struct {
		testName          string
		variableFlagValue string
		variableEnvName   string
		variableEnvValue  string
		want              string
	}{
		{
			"test 1",
			"thisisavalidtelegrambottoken",
			"TELEGRAM_BOT_TOKEN", "",
			"thisisavalidtelegrambottoken",
		},
		{
			"test 2",
			"",
			"TELEGRAM_BOT_TOKEN", "thisisavalidtelegrambottoken",
			"thisisavalidtelegrambottoken",
		},
		{
			"test 3",
			"\n   \t   \rthisisavalidtelegrambottoken\n\t    ",
			"TELEGRAM_BOT_TOKEN", "",
			"thisisavalidtelegrambottoken",
		},
		{
			"test 4",
			"",
			"TELEGRAM_BOT_TOKEN", "\n   \t   \rthisisavalidtelegrambottoken\n\t    ",
			"thisisavalidtelegrambottoken",
		},
		{
			"flag wins over env",
			"flag-value",
			"TELEGRAM_BOT_TOKEN",
			"env-value",
			"flag-value",
		},
		{
			"both empty",
			"",
			"TELEGRAM_BOT_TOKEN",
			"",
			"",
		},
		{
			"flag has only spaces and env is used",
			"   \n\t ",
			"TELEGRAM_BOT_TOKEN",
			"env-value",
			"env-value",
		},
		{
			"env has only spaces",
			"",
			"TELEGRAM_BOT_TOKEN",
			"   \n\t ",
			"",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			if tc.variableEnvValue != "" {
				t.Setenv(tc.variableEnvName, tc.variableEnvValue)
			}
			got := flagOrEnv(tc.variableFlagValue, tc.variableEnvName)
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
func TestParseConfig(t *testing.T) {
	testCases := []struct {
		testName string
		args     []string
		want     Config
		err      error
	}{
		{
			"happy path",
			[]string{"-authorized-users", "1111111111", "-telegram-bot-token", "thisisavalidtelegrambottoken"},
			Config{
				[]int64{1111111111},
				"thisisavalidtelegrambottoken",
			},
			nil,
		},
		{
			"multiple authorized users",
			[]string{
				"-authorized-users",
				"1111111111,1111111112",
				"-telegram-bot-token",
				"thisisavalidtelegrambottoken",
			},
			Config{
				[]int64{1111111111, 1111111112},
				"thisisavalidtelegrambottoken",
			},
			nil,
		},
		{
			"duplicate authorized users",
			[]string{
				"-authorized-users",
				"1111111111,1111111111,1111111112,1111111113",
				"-telegram-bot-token",
				"thisisavalidtelegrambottoken",
			},
			Config{
				[]int64{1111111111, 1111111112, 1111111113},
				"thisisavalidtelegrambottoken",
			},
			nil,
		},
		{
			"empty authorized users",
			[]string{"-authorized-users", "", "-telegram-bot-token", "thisisavalidtelegrambottoken"},
			Config{
				[]int64{},
				"thisisavalidtelegrambottoken",
			},
			nil,
		},
		{
			"no authorized users",
			[]string{"-telegram-bot-token", "thisisavalidtelegrambottoken"},
			Config{
				[]int64{},
				"thisisavalidtelegrambottoken",
			},
			nil,
		},
		{
			"invalid authorized users",
			[]string{"-authorized-users", "teamoabuelita", "-telegram-bot-token", "thisisavalidtelegrambottoken"},
			Config{
				[]int64{},
				"",
			},
			errors.New(`invalid authorized user ID "teamoabuelita": must be a valid Telegram user ID`),
		},
		{
			"empty telegram bot token",
			[]string{"-authorized-users", "1111111111", "-telegram-bot-token", ""},
			Config{
				[]int64{},
				"",
			},
			errors.New("telegram bot token is required: use -telegram-bot-token or TELEGRAM_BOT_TOKEN"),
		},
		{
			"no telegram bot token",
			[]string{"-authorized-users", "1111111111"},
			Config{
				[]int64{},
				"",
			},
			errors.New("telegram bot token is required: use -telegram-bot-token or TELEGRAM_BOT_TOKEN"),
		},
		{
			"no args",
			[]string{},
			Config{
				[]int64{},
				"",
			},
			errors.New("telegram bot token is required: use -telegram-bot-token or TELEGRAM_BOT_TOKEN"),
		},
		{
			"invalid args",
			[]string{"-does-not-exist"},
			Config{
				[]int64{},
				"",
			},
			errors.New("flag provided but not defined: -does-not-exist"),
		},
		{
			"invalid flag missing value",
			[]string{"-telegram-bot-token"},
			Config{
				[]int64{},
				"",
			},
			errors.New("flag needs an argument: -telegram-bot-token"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			config, err := ParseConfig(tc.args)
			if tc.err != nil && err == nil {
				t.Fatalf("got nil, want %q", tc.err.Error())
			}
			if tc.err != nil && err != nil && tc.err.Error() != err.Error() {
				t.Fatalf("got %q, want %q", err.Error(), tc.err.Error())
			}
			compareInt64Array(t, config.AuthorizedUsers, tc.want.AuthorizedUsers)
			if config.TelegramBotToken != tc.want.TelegramBotToken {
				t.Fatalf("got %q, want %q", config.TelegramBotToken, tc.want.TelegramBotToken)
			}
		})
	}
}
