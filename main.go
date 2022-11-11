package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// VideoStartEndPattern is a regex to match patterns like:
//	1:05-1:10
//	0:10-0:51
//	17:49-58:09
//	21:50-58:00
//	3:17:55-4:17:59
//	41:40-1:23:00
// I chose this pattern because at the time I wrote this code, in the following link:
// https://support.google.com/youtube/answer/71673
// YouTube indicated that the max video length was 12 hours. I've seen YouTube videos
// that last more than 99 hours, so if you want to match those you could try expanding
// my regex.
var VideoStartEndPattern = regexp.MustCompile(`([\d]{1,2}:)?[\d]{1,2}:[\d]{1,2}-([\d]{1,2}:)?[\d]{1,2}:[\d]{1,2}`)

const InvalidVideoSecond = -1

func Spot2Second(spot string) (int, error) {
	parts := strings.Split(spot, ":")
	partsLen := len(parts)
	if partsLen < 2 {
		return 0, fmt.Errorf("unable to parse spot %s", spot)
	}
	// parse seconds and validate they are less than 60
	seconds, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, fmt.Errorf("unable to parse spot %s", spot)
	}
	if seconds > 59 {
		return 0, fmt.Errorf("unable to parse spot %s", spot)
	}
	// parse minutes and validate they are less than 60
	minutes, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("unable to parse spot %s", spot)
	}
	if minutes > 59 {
		return 0, fmt.Errorf("unable to parse spot %s", spot)
	}
	// turn the spot into a second by adding seconds and minutes
	second := seconds + minutes*60
	// if spot contains hours, parse hours and validate they are less than 24
	if partsLen == 3 {
		hours, err := strconv.Atoi(parts[2])
		if err != nil {
			return 0, fmt.Errorf("unable to parse spot %s", spot)
		}
		if hours > 11 {
			return 0, fmt.Errorf("unable to parse spot %s", spot)
		}
		// add the hours to the second representing the spot
		second += hours * 60 * 60
	}
	return second, nil
}

func ParseStartEndSeconds(span string) (int, int, error) {
	if !VideoStartEndPattern.MatchString(span) {
		return 0, 0, fmt.Errorf("unable to parse video span %s", span)
	}
	parts := strings.Split(span, "-")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("unable to parse video span %s", span)
	}
	startSecond, err := Spot2Second(parts[0])
	if err != nil {
		return 0, 0, err
	}
	endSecond, err := Spot2Second(parts[1])
	if err != nil {
		return 0, 0, err
	}
	if startSecond >= endSecond {
		return 0, 0, fmt.Errorf("start spot is after or at the same that the end spot")
	}
	return startSecond, endSecond, nil
}

func LoadDownloadConfigFromMsg(msg string) (*url.URL, int, int, bool, error) {
	args := strings.Split(msg, " ")
	videoUrl, err := url.Parse(args[0])
	if err != nil {
		return &url.URL{}, InvalidVideoSecond, InvalidVideoSecond, false, fmt.Errorf("unable to parse the 1st argument (video URL)")
	}
	argsLen := len(args)
	if argsLen == 1 {
		return videoUrl, InvalidVideoSecond, InvalidVideoSecond, false, nil
	}
	// at this point, argsLen is greather than 1
	var (
		startSecond = InvalidVideoSecond
		endSecond   = InvalidVideoSecond
		audioOnly   = false
		secondArg   = strings.ToLower(args[1])
	)
	if secondArg == "audio" {
		return videoUrl, startSecond, endSecond, true, nil
	}
	startSecond, endSecond, err = ParseStartEndSeconds(secondArg)
	if err != nil {
		return &url.URL{}, InvalidVideoSecond, InvalidVideoSecond, false, fmt.Errorf("unable to parse the 2nd argument (video spots to make the cut or audio word)")
	}
	if argsLen == 2 {
		return videoUrl, startSecond, endSecond, audioOnly, nil
	}
	// at this point, argsLen is greather than 2
	if argsLen > 3 {
		return &url.URL{}, InvalidVideoSecond, InvalidVideoSecond, false, fmt.Errorf("more than 3 arguments were used")
	}
	thirdArg := args[2]
	if thirdArg != "audio" {
		return &url.URL{}, InvalidVideoSecond, InvalidVideoSecond, false, fmt.Errorf("unable to parse the 3rd argument: this argument can only be the audio word")
	}
	return videoUrl, startSecond, endSecond, true, nil
}

func CutVideo(videoFilename string, startSecond, endSecond int, audioOnly bool) (string, error) {
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return "", fmt.Errorf("unable to cut video: %s", err)
	}
	videoFilenameExt := filepath.Ext(videoFilename)
	finalVideoFilename := videoFilename[:len(videoFilename)-len(videoFilenameExt)] + "-cut"
	if audioOnly {
		finalVideoFilename = finalVideoFilename + ".mp3"
	} else {
		finalVideoFilename = finalVideoFilename + videoFilenameExt
	}
	cutCmd := exec.Command(
		ffmpegPath, "-ss",
		fmt.Sprint(startSecond),
		"-i",
		videoFilename,
		"-t",
		fmt.Sprint(endSecond-startSecond),
		finalVideoFilename,
	)
	videoFilename = finalVideoFilename
	if err := cutCmd.Run(); err != nil {
		return "", fmt.Errorf("unable to cut video: %s", err)
	}
	return finalVideoFilename, nil
}

func BuildYtdlpCmd(videoUrl string, audioOnly bool) (string, string, []string, error) {
	ytdlpPath, err := exec.LookPath("yt-dlp")
	if err != nil {
		return "", "", nil, fmt.Errorf("yt-dlp is not installed: %s", err)
	}
	ytdlpArgs := []string{}
	if audioOnly {
		ytdlpArgs = append(ytdlpArgs, "-x", "--audio-format", "mp3")
	}
	ytdlpArgs = append(ytdlpArgs, "-f", "18", videoUrl)
	f, err := os.CreateTemp("", "gatonaranja.*.mp4")
	if err != nil {
		return "", "", nil, fmt.Errorf("unable to create temp file to save the downloaded video: %s", err)
	}
	outputFilename := f.Name()
	f.Close()
	err = os.Remove(outputFilename)
	if err != nil {
		return "", "", nil, fmt.Errorf("unable to remove temp file to save the downloaded video: %s", err)
	}
	if audioOnly {
		outputFilename = outputFilename[:len(outputFilename)-1] + "3"
	}
	ytdlpArgs = append(ytdlpArgs, "-o", outputFilename)
	return ytdlpPath, outputFilename, ytdlpArgs, nil
}

func DownloadVideo(videoUrl string, startSecond, endSecond int, audioOnly bool) (string, error) {
	ytdlpPath, videoFilename, ytdlpArgs, err := BuildYtdlpCmd(videoUrl, audioOnly)
	if err != nil {
		return "", fmt.Errorf("unable to download video %s: %s", videoUrl, err)
	}
	downloadCmd := exec.Command(ytdlpPath, ytdlpArgs...)
	if err := downloadCmd.Run(); err != nil {
		return "", fmt.Errorf("unable to download video %s: %s", videoUrl, err)
	}
	if startSecond != InvalidVideoSecond && endSecond != InvalidVideoSecond {
		videoFilename, err = CutVideo(videoFilename, startSecond, endSecond, audioOnly)
		if err != nil {
			return "", fmt.Errorf("unable to download video %s: %s", videoUrl, err)
		}
	}
	return videoFilename, nil
}

func UserIsAuthorized(userId int64, authorizedUserIds []int64) bool {
	if len(authorizedUserIds) == 0 {
		return true
	}
	for _, allowedUserId := range authorizedUserIds {
		if userId == allowedUserId {
			return true
		}
	}
	return false
}

func LoadAuthorizedUserIds(authorizedUsersEnv string) ([]int64, error) {
	authorizedUsersEnvContent := strings.TrimSpace(os.Getenv(authorizedUsersEnv))
	if authorizedUsersEnvContent == "" {
		return []int64{}, nil
	}
	authorizedUserIds := strings.Split(authorizedUsersEnvContent, ",")
	ids := []int64{}
	for _, authorizedUserId := range authorizedUserIds {
		id, err := strconv.ParseInt(authorizedUserId, 10, 0)
		if err != nil {
			return []int64{}, fmt.Errorf("unable to parse %s into an int64: %s", authorizedUserId, err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

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
		f, err := os.OpenFile("LOGFILE", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
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
	// Start the infinite loop to receive messages
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)
	for update := range updates {
		if update.Message != nil {
			// Check if user is authorized
			if !UserIsAuthorized(update.Message.From.ID, authorizedUserIds) {
				log.Printf("[%s %d] Non-Authorized user sent: %s", update.Message.From.UserName, update.Message.From.ID, update.Message.Text)
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "You are NOT AUTHORIZED to use me! 😠")
				bot.Send(msg)
				continue
			} else {
				log.Printf("[%s %d] Authorized user sent: %s", update.Message.From.UserName, update.Message.From.ID, update.Message.Text)
			}
			// Let the user know you are working on the download
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Ok, just wait a second...")
			msg.ReplyToMessageID = update.Message.MessageID
			bot.Send(msg)
			videoUrl, startSecond, endSecond, audioOnly, err := LoadDownloadConfigFromMsg(update.Message.Text)
			if err != nil {
				log.Printf("[%s %d] Unable to complete request %s: %s", update.Message.From.UserName, update.Message.From.ID, update.Message.Text, err)
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "I'm sorry I was not able to download your video ☹")
				msg.ReplyToMessageID = update.Message.MessageID
				bot.Send(msg)
				continue
			}
			videoFilename, err := DownloadVideo(videoUrl.String(), startSecond, endSecond, audioOnly)
			if err != nil {
				log.Printf("[%s %d] Unable to complete request %s: %s", update.Message.From.UserName, update.Message.From.ID, update.Message.Text, err)
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "I'm sorry I was not able to download your video ☹")
				msg.ReplyToMessageID = update.Message.MessageID
				bot.Send(msg)
				continue
			}
			if audioOnly {
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
}
