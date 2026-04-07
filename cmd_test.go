package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
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
			errors.New(`invalid authorized user ID " 1111 111111 ": must be a valid Telegram user ID`),
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

func TestValidateMaxConcurrentDownloads(t *testing.T) {
	testCases := []struct {
		testName               string
		maxConcurrentDownloads string
		want                   int
		err                    error
	}{
		{
			"1 concurrent download",
			"1",
			1,
			nil,
		},
		{
			"6 concurrent downloads",
			"6",
			6,
			nil,
		},
		{
			"100 concurrent downloads",
			"100",
			100,
			nil,
		},
		{
			"value with surrounding spaces",
			"   8   ",
			8,
			nil,
		},
		{
			"zero concurrent downloads",
			"0",
			0,
			errors.New(`invalid max concurrent downloads "0": must be between 1 and 100`),
		},
		{
			"negative concurrent downloads",
			"-5",
			0,
			errors.New(`invalid max concurrent downloads "-5": must be between 1 and 100`),
		},
		{
			"more than maximum concurrent downloads",
			"101",
			0,
			errors.New(`invalid max concurrent downloads "101": must be between 1 and 100`),
		},
		{
			"empty value",
			"",
			0,
			errors.New(`invalid max concurrent downloads "": must be between 1 and 100`),
		},
		{
			"only spaces",
			"   \t\n  ",
			0,
			errors.New("invalid max concurrent downloads \"   \\t\\n  \": must be between 1 and 100"),
		},
		{
			"non numeric value",
			"abc",
			0,
			errors.New(`invalid max concurrent downloads "abc": must be between 1 and 100`),
		},
		{
			"decimal value",
			"1.5",
			0,
			errors.New(`invalid max concurrent downloads "1.5": must be between 1 and 100`),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			maxConcurrentDownloads, err := validateMaxConcurrentDownloads(tc.maxConcurrentDownloads)
			if tc.err != nil && err == nil {
				t.Fatalf("got nil, want %q", tc.err.Error())
			}
			if tc.err != nil && err != nil && tc.err.Error() != err.Error() {
				t.Fatalf("got %q, want %q", err.Error(), tc.err.Error())
			}
			if maxConcurrentDownloads != tc.want {
				t.Fatalf("got %d, want %d", maxConcurrentDownloads, tc.want)
			}
		})
	}
}

func TestValidateMaxQueuedDownloads(t *testing.T) {
	testCases := []struct {
		testName           string
		maxQueuedDownloads string
		want               int
		err                error
	}{
		{
			"1 queued download",
			"1",
			1,
			nil,
		},
		{
			"6 queued downloads",
			"6",
			6,
			nil,
		},
		{
			"100 queued downloads",
			"100",
			100,
			nil,
		},
		{
			"value with surrounding spaces",
			"   8   ",
			8,
			nil,
		},
		{
			"zero queued downloads",
			"0",
			0,
			errors.New(`invalid max queued downloads "0": must be between 1 and 100`),
		},
		{
			"negative queued downloads",
			"-5",
			0,
			errors.New(`invalid max queued downloads "-5": must be between 1 and 100`),
		},
		{
			"more than maximum queued downloads",
			"101",
			0,
			errors.New(`invalid max queued downloads "101": must be between 1 and 100`),
		},
		{
			"empty value",
			"",
			0,
			errors.New(`invalid max queued downloads "": must be between 1 and 100`),
		},
		{
			"only spaces",
			"   \t\n  ",
			0,
			errors.New("invalid max queued downloads \"   \\t\\n  \": must be between 1 and 100"),
		},
		{
			"non numeric value",
			"abc",
			0,
			errors.New(`invalid max queued downloads "abc": must be between 1 and 100`),
		},
		{
			"decimal value",
			"1.5",
			0,
			errors.New(`invalid max queued downloads "1.5": must be between 1 and 100`),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			maxQueuedDownloads, err := validateMaxQueuedDownloads(tc.maxQueuedDownloads)
			if tc.err != nil && err == nil {
				t.Fatalf("got nil, want %q", tc.err.Error())
			}
			if tc.err != nil && err != nil && tc.err.Error() != err.Error() {
				t.Fatalf("got %q, want %q", err.Error(), tc.err.Error())
			}
			if maxQueuedDownloads != tc.want {
				t.Fatalf("got %d, want %d", maxQueuedDownloads, tc.want)
			}
		})
	}
}

func TestValidateDownloadTimeout(t *testing.T) {
	testCases := []struct {
		testName        string
		downloadTimeout string
		want            time.Duration
		err             error
	}{
		{
			"ten seconds",
			"10s",
			10 * time.Second,
			nil,
		},
		{
			"1 second timeout",
			"1s",
			time.Second,
			nil,
		},
		{
			"10 seconds timeout",
			"10s",
			10 * time.Second,
			nil,
		},
		{
			"5 minutes timeout",
			"5m",
			5 * time.Minute,
			nil,
		},
		{
			"10 minutes timeout",
			"10m",
			10 * time.Minute,
			nil,
		},
		{
			"timeout with surrounding spaces",
			"   30s   ",
			30 * time.Second,
			nil,
		},
		{
			"zero timeout",
			"0s",
			0,
			errors.New(`invalid download timeout "0s": must be between 1s and 10m, for example 30s, 2m, or 5m`),
		},
		{
			"negative timeout",
			"-5s",
			0,
			errors.New(`invalid download timeout "-5s": must be between 1s and 10m, for example 30s, 2m, or 5m`),
		},
		{
			"more than maximum timeout",
			"11m",
			0,
			errors.New(`invalid download timeout "11m": must be between 1s and 10m, for example 30s, 2m, or 5m`),
		},
		{
			"empty timeout",
			"",
			0,
			errors.New(`invalid download timeout "": must be between 1s and 10m, for example 30s, 2m, or 5m`),
		},
		{
			"only spaces timeout",
			"   \t\n  ",
			0,
			errors.New(`invalid download timeout "   \t\n  ": must be between 1s and 10m, for example 30s, 2m, or 5m`),
		},
		{
			"invalid unit",
			"5x",
			0,
			errors.New(`invalid download timeout "5x": must be between 1s and 10m, for example 30s, 2m, or 5m`),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			downloadTimeout, err := validateDownloadTimeout(tc.downloadTimeout)
			if tc.err != nil && err == nil {
				t.Fatalf("got nil, want %q", tc.err.Error())
			}
			if tc.err != nil && err != nil && tc.err.Error() != err.Error() {
				t.Fatalf("got %q, want %q", err.Error(), tc.err.Error())
			}
			if downloadTimeout != tc.want {
				t.Fatalf("got %d, want %d", downloadTimeout, tc.want)
			}
		})
	}
}

func TestValidateYTDLPConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "yt-dlp.conf")
	if err := os.WriteFile(configPath, []byte("--proxy http://127.0.0.1:8080\n"), 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v, want nil", err)
	}

	testCases := []struct {
		testName    string
		ytdlpConfig string
		want        string
		err         error
	}{
		{
			"empty config path",
			"",
			"",
			nil,
		},
		{
			"absolute config path",
			configPath,
			configPath,
			nil,
		},
		{
			"config path with surrounding spaces",
			"  " + configPath + "  ",
			configPath,
			nil,
		},
		{
			"stdin is rejected",
			"-",
			"",
			errors.New(`invalid ytdlp config "-": stdin is not supported`),
		},
		{
			"nonexistent config file",
			filepath.Join(tempDir, "missing.conf"),
			"",
			errors.New(
				`invalid ytdlp config "` + filepath.Join(tempDir, "missing.conf") + `": open ` +
					filepath.Join(tempDir, "missing.conf") + `: no such file or directory`,
			),
		},
		{
			"directory is rejected",
			tempDir,
			"",
			errors.New(`invalid ytdlp config "` + tempDir + `": must be a regular file`),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			got, err := validateYTDLPConfig(tc.ytdlpConfig)
			if tc.err != nil && err == nil {
				t.Fatalf("got nil, want %q", tc.err.Error())
			}
			if tc.err != nil && err != nil && tc.err.Error() != err.Error() {
				t.Fatalf("got %q, want %q", err.Error(), tc.err.Error())
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
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
			"uses flag value directly",
			"thisisavalidtelegrambottoken",
			"TELEGRAM_BOT_TOKEN", "",
			"thisisavalidtelegrambottoken",
		},
		{
			"falls back to env value",
			"",
			"TELEGRAM_BOT_TOKEN", "thisisavalidtelegrambottoken",
			"thisisavalidtelegrambottoken",
		},
		{
			"trims flag value",
			"\n   \t   \rthisisavalidtelegrambottoken\n\t    ",
			"TELEGRAM_BOT_TOKEN", "",
			"thisisavalidtelegrambottoken",
		},
		{
			"trims env value",
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
				AuthorizedUsers:        []int64{1111111111},
				TelegramBotToken:       "thisisavalidtelegrambottoken",
				MaxConcurrentDownloads: 5,
				MaxQueuedDownloads:     5,
				DownloadTimeout:        5 * time.Minute,
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
				AuthorizedUsers:        []int64{1111111111, 1111111112},
				TelegramBotToken:       "thisisavalidtelegrambottoken",
				MaxConcurrentDownloads: 5,
				MaxQueuedDownloads:     5,
				DownloadTimeout:        5 * time.Minute,
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
				AuthorizedUsers:        []int64{1111111111, 1111111112, 1111111113},
				TelegramBotToken:       "thisisavalidtelegrambottoken",
				MaxConcurrentDownloads: 5,
				MaxQueuedDownloads:     5,
				DownloadTimeout:        5 * time.Minute,
			},
			nil,
		},
		{
			"empty authorized users",
			[]string{"-authorized-users", "", "-telegram-bot-token", "thisisavalidtelegrambottoken"},
			Config{
				AuthorizedUsers:        []int64{},
				TelegramBotToken:       "thisisavalidtelegrambottoken",
				MaxConcurrentDownloads: 5,
				MaxQueuedDownloads:     5,
				DownloadTimeout:        5 * time.Minute,
			},
			nil,
		},
		{
			"no authorized users",
			[]string{"-telegram-bot-token", "thisisavalidtelegrambottoken"},
			Config{
				AuthorizedUsers:        []int64{},
				TelegramBotToken:       "thisisavalidtelegrambottoken",
				MaxConcurrentDownloads: 5,
				MaxQueuedDownloads:     5,
				DownloadTimeout:        5 * time.Minute,
			},
			nil,
		},
		{
			"custom max concurrent downloads",
			[]string{
				"-telegram-bot-token",
				"thisisavalidtelegrambottoken",
				"-max-concurrent-downloads",
				"8",
			},
			Config{
				AuthorizedUsers:        []int64{},
				TelegramBotToken:       "thisisavalidtelegrambottoken",
				MaxConcurrentDownloads: 8,
				MaxQueuedDownloads:     5,
				DownloadTimeout:        5 * time.Minute,
			},
			nil,
		},
		{
			"custom max queued downloads",
			[]string{
				"-telegram-bot-token",
				"thisisavalidtelegrambottoken",
				"-max-queued-downloads",
				"8",
			},
			Config{
				AuthorizedUsers:        []int64{},
				TelegramBotToken:       "thisisavalidtelegrambottoken",
				MaxConcurrentDownloads: 5,
				MaxQueuedDownloads:     8,
				DownloadTimeout:        5 * time.Minute,
			},
			nil,
		},
		{
			"custom download timeout",
			[]string{
				"-telegram-bot-token",
				"thisisavalidtelegrambottoken",
				"-download-timeout",
				"2m",
			},
			Config{
				AuthorizedUsers:        []int64{},
				TelegramBotToken:       "thisisavalidtelegrambottoken",
				MaxConcurrentDownloads: 5,
				MaxQueuedDownloads:     5,
				DownloadTimeout:        2 * time.Minute,
			},
			nil,
		},
		{
			"custom max concurrent downloads queue size and timeout",
			[]string{
				"-telegram-bot-token",
				"thisisavalidtelegrambottoken",
				"-max-concurrent-downloads",
				"9",
				"-max-queued-downloads",
				"7",
				"-download-timeout",
				"30s",
			},
			Config{
				AuthorizedUsers:        []int64{},
				TelegramBotToken:       "thisisavalidtelegrambottoken",
				MaxConcurrentDownloads: 9,
				MaxQueuedDownloads:     7,
				DownloadTimeout:        30 * time.Second,
			},
			nil,
		},
		{
			"minimum max concurrent downloads",
			[]string{
				"-telegram-bot-token",
				"thisisavalidtelegrambottoken",
				"-max-concurrent-downloads",
				"1",
			},
			Config{
				AuthorizedUsers:        []int64{},
				TelegramBotToken:       "thisisavalidtelegrambottoken",
				MaxConcurrentDownloads: 1,
				MaxQueuedDownloads:     5,
				DownloadTimeout:        5 * time.Minute,
			},
			nil,
		},
		{
			"maximum max concurrent downloads",
			[]string{
				"-telegram-bot-token",
				"thisisavalidtelegrambottoken",
				"-max-concurrent-downloads",
				"100",
			},
			Config{
				AuthorizedUsers:        []int64{},
				TelegramBotToken:       "thisisavalidtelegrambottoken",
				MaxConcurrentDownloads: 100,
				MaxQueuedDownloads:     5,
				DownloadTimeout:        5 * time.Minute,
			},
			nil,
		},
		{
			"minimum max queued downloads",
			[]string{
				"-telegram-bot-token",
				"thisisavalidtelegrambottoken",
				"-max-queued-downloads",
				"1",
			},
			Config{
				AuthorizedUsers:        []int64{},
				TelegramBotToken:       "thisisavalidtelegrambottoken",
				MaxConcurrentDownloads: 5,
				MaxQueuedDownloads:     1,
				DownloadTimeout:        5 * time.Minute,
			},
			nil,
		},
		{
			"maximum max queued downloads",
			[]string{
				"-telegram-bot-token",
				"thisisavalidtelegrambottoken",
				"-max-queued-downloads",
				"100",
			},
			Config{
				AuthorizedUsers:        []int64{},
				TelegramBotToken:       "thisisavalidtelegrambottoken",
				MaxConcurrentDownloads: 5,
				MaxQueuedDownloads:     100,
				DownloadTimeout:        5 * time.Minute,
			},
			nil,
		},
		{
			"minimum download timeout",
			[]string{
				"-telegram-bot-token",
				"thisisavalidtelegrambottoken",
				"-download-timeout",
				"1s",
			},
			Config{
				AuthorizedUsers:        []int64{},
				TelegramBotToken:       "thisisavalidtelegrambottoken",
				MaxConcurrentDownloads: 5,
				MaxQueuedDownloads:     5,
				DownloadTimeout:        1 * time.Second,
			},
			nil,
		},
		{
			"maximum download timeout",
			[]string{
				"-telegram-bot-token",
				"thisisavalidtelegrambottoken",
				"-download-timeout",
				"10m",
			},
			Config{
				AuthorizedUsers:        []int64{},
				TelegramBotToken:       "thisisavalidtelegrambottoken",
				MaxConcurrentDownloads: 5,
				MaxQueuedDownloads:     5,
				DownloadTimeout:        10 * time.Minute,
			},
			nil,
		},
		{
			"invalid authorized users",
			[]string{"-authorized-users", "teamoabuelita", "-telegram-bot-token", "thisisavalidtelegrambottoken"},
			Config{},
			errors.New(`invalid authorized user ID "teamoabuelita": must be a valid Telegram user ID`),
		},
		{
			"empty telegram bot token",
			[]string{"-authorized-users", "1111111111", "-telegram-bot-token", ""},
			Config{},
			errors.New("telegram bot token is required: use -telegram-bot-token or TELEGRAM_BOT_TOKEN"),
		},
		{
			"no telegram bot token",
			[]string{"-authorized-users", "1111111111"},
			Config{},
			errors.New("telegram bot token is required: use -telegram-bot-token or TELEGRAM_BOT_TOKEN"),
		},
		{
			"invalid max concurrent downloads",
			[]string{
				"-telegram-bot-token",
				"thisisavalidtelegrambottoken",
				"-max-concurrent-downloads",
				"101",
			},
			Config{},
			errors.New(`invalid max concurrent downloads "101": must be between 1 and 100`),
		},
		{
			"invalid max queued downloads",
			[]string{
				"-telegram-bot-token",
				"thisisavalidtelegrambottoken",
				"-max-queued-downloads",
				"101",
			},
			Config{},
			errors.New(`invalid max queued downloads "101": must be between 1 and 100`),
		},
		{
			"zero max concurrent downloads",
			[]string{
				"-telegram-bot-token",
				"thisisavalidtelegrambottoken",
				"-max-concurrent-downloads",
				"0",
			},
			Config{},
			errors.New(`invalid max concurrent downloads "0": must be between 1 and 100`),
		},
		{
			"zero max queued downloads",
			[]string{
				"-telegram-bot-token",
				"thisisavalidtelegrambottoken",
				"-max-queued-downloads",
				"0",
			},
			Config{},
			errors.New(`invalid max queued downloads "0": must be between 1 and 100`),
		},
		{
			"invalid download timeout",
			[]string{
				"-telegram-bot-token",
				"thisisavalidtelegrambottoken",
				"-download-timeout",
				"11m",
			},
			Config{},
			errors.New(`invalid download timeout "11m": must be between 1s and 10m, for example 30s, 2m, or 5m`),
		},
		{
			"zero download timeout",
			[]string{
				"-telegram-bot-token",
				"thisisavalidtelegrambottoken",
				"-download-timeout",
				"0s",
			},
			Config{},
			errors.New(`invalid download timeout "0s": must be between 1s and 10m, for example 30s, 2m, or 5m`),
		},
		{
			"no args",
			[]string{},
			Config{},
			errors.New("telegram bot token is required: use -telegram-bot-token or TELEGRAM_BOT_TOKEN"),
		},
		{
			"invalid args",
			[]string{"-does-not-exist"},
			Config{},
			errors.New("flag provided but not defined: -does-not-exist"),
		},
		{
			"invalid flag missing token value",
			[]string{"-telegram-bot-token"},
			Config{},
			errors.New("flag needs an argument: -telegram-bot-token"),
		},
		{
			"invalid flag missing max concurrent downloads value",
			[]string{"-telegram-bot-token", "thisisavalidtelegrambottoken", "-max-concurrent-downloads"},
			Config{},
			errors.New("flag needs an argument: -max-concurrent-downloads"),
		},
		{
			"invalid flag missing max queued downloads value",
			[]string{"-telegram-bot-token", "thisisavalidtelegrambottoken", "-max-queued-downloads"},
			Config{},
			errors.New("flag needs an argument: -max-queued-downloads"),
		},
		{
			"invalid flag missing download timeout value",
			[]string{"-telegram-bot-token", "thisisavalidtelegrambottoken", "-download-timeout"},
			Config{},
			errors.New("flag needs an argument: -download-timeout"),
		},
		{
			"version flag only",
			[]string{"-version"},
			Config{
				PrintVersion: true,
			},
			nil,
		},
		{
			"version flag with other flags",
			[]string{
				"-version",
				"-telegram-bot-token", "123:abc",
				"-authorized-users", "1,2",
				"-max-concurrent-downloads", "6",
				"-max-queued-downloads", "7",
				"-download-timeout", "2m",
			},
			Config{
				PrintVersion: true,
			},
			nil,
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
			if config.MaxConcurrentDownloads != tc.want.MaxConcurrentDownloads {
				t.Fatalf("got %d, want %d", config.MaxConcurrentDownloads, tc.want.MaxConcurrentDownloads)
			}
			if config.MaxQueuedDownloads != tc.want.MaxQueuedDownloads {
				t.Fatalf("got %d, want %d", config.MaxQueuedDownloads, tc.want.MaxQueuedDownloads)
			}
			if config.DownloadTimeout != tc.want.DownloadTimeout {
				t.Fatalf("got %q, want %q", config.DownloadTimeout, tc.want.DownloadTimeout)
			}
			if config.PrintVersion != tc.want.PrintVersion {
				t.Fatalf("got %t, want %t", config.PrintVersion, tc.want.PrintVersion)
			}
			if config.YTDLPConfig != tc.want.YTDLPConfig {
				t.Fatalf("got %q, want %q", config.YTDLPConfig, tc.want.YTDLPConfig)
			}
		})
	}
}

func TestParseConfigWithYTDLPConfigFlag(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "yt-dlp.conf")
	if err := os.WriteFile(configPath, []byte("--cookies cookies.txt\n"), 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v, want nil", err)
	}

	config, err := ParseConfig([]string{
		"-telegram-bot-token", "thisisavalidtelegrambottoken",
		"-ytdlp-config", configPath,
	})
	if err != nil {
		t.Fatalf("ParseConfig() error = %v, want nil", err)
	}

	if got, want := config.YTDLPConfig, configPath; got != want {
		t.Fatalf("config.YTDLPConfig = %q, want %q", got, want)
	}
}

func TestParseConfigWithYTDLPConfigEnv(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "yt-dlp.conf")
	if err := os.WriteFile(configPath, []byte("--proxy socks5://127.0.0.1:9050\n"), 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v, want nil", err)
	}

	t.Setenv("YTDLP_CONFIG", configPath)

	config, err := ParseConfig([]string{
		"-telegram-bot-token", "thisisavalidtelegrambottoken",
	})
	if err != nil {
		t.Fatalf("ParseConfig() error = %v, want nil", err)
	}

	if got, want := config.YTDLPConfig, configPath; got != want {
		t.Fatalf("config.YTDLPConfig = %q, want %q", got, want)
	}
}
