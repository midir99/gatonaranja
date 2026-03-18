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

const InvalidVideoSecond = -1

type DownloadRequest struct {
	startSecond int
	endSecond   int
	audioOnly   bool
	videoURL    string
}

// TimestampRangePattern matches the structure of timestamp ranges in
// MM:SS-MM:SS or HH:MM:SS-HH:MM:SS format, such as:
//
//	1:05-1:10
//	0:10-0:51
//	17:49-58:09
//	21:50-58:00
//	3:17:55-4:17:59
//	41:40-1:23:00
//
// Detailed validation, such as checking numeric bounds, is performed
// separately when parsing the timestamps.
//
// The optional hour component is limited to two digits because, when this
// code was originally written, YouTube documentation indicated a maximum
// video length of 12 hours.
// https://support.google.com/youtube/answer/71673
var TimestampRangePattern = regexp.MustCompile(`([\d]{1,2}:)?[\d]{1,2}:[\d]{1,2}-([\d]{1,2}:)?[\d]{1,2}:[\d]{1,2}`)

func parseSeconds(seconds string) (int, error) {
	invalidValueErr := fmt.Errorf("invalid seconds value %s: must be between 0 and 59", seconds)
	secondsInt, err := strconv.Atoi(seconds)
	if err != nil {
		return 0, invalidValueErr
	}
	if secondsInt < 0 || secondsInt > 59 {
		return 0, invalidValueErr
	}
	return secondsInt, nil
}

func parseMinutes(minutes string) (int, error) {
	invalidValueErr := fmt.Errorf("invalid minutes value %s: must be between 0 and 59", minutes)
	minutesInt, err := strconv.Atoi(minutes)
	if err != nil {
		return 0, invalidValueErr
	}
	if minutesInt < 0 || minutesInt > 59 {
		return 0, invalidValueErr
	}
	return minutesInt, nil
}

func parseHours(hours string) (int, error) {
	invalidValueErr := fmt.Errorf("invalid hours value %s: must be between 0 and 11", hours)
	hoursInt, err := strconv.Atoi(hours)
	if err != nil {
		return 0, invalidValueErr
	}
	if hoursInt < 0 || hoursInt > 11 {
		return 0, invalidValueErr
	}
	return hoursInt, nil
}

// TimestampToSeconds converts a timestamp in MM:SS or HH:MM:SS format into
// its total number of seconds.
func TimestampToSeconds(timestamp string) (int, error) {
	parts := strings.Split(timestamp, ":")
	partsNumber := len(parts)
	totalSeconds := 0
	switch {
	case partsNumber == 2:
		// Parse minutes and seconds (MM:SS), for example:
		// The string "00:10" is converted into this array: ["00", "10"]
		totalSeconds, err := parseSeconds(parts[1])
		if err != nil {
			return 0, err
		}
		minutes, err := parseMinutes(parts[0])
		if err != nil {
			return 0, err
		}
		totalSeconds += minutes * 60
	case partsNumber == 3:
		// Parse hours, minutes and seconds (HH:MM:SS), for example:
		// The string "01:05:10" is converted into this array: ["01", "05", "10"]
		totalSeconds, err := parseSeconds(parts[2])
		if err != nil {
			return 0, err
		}
		minutes, err := parseMinutes(parts[1])
		if err != nil {
			return 0, err
		}
		totalSeconds += minutes * 60
		hours, err := parseHours(parts[0])
		if err != nil {
			return 0, err
		}
		totalSeconds += hours * 60 * 60
	default:
		return 0, fmt.Errorf("invalid timestamp %s: it must follow the format HH:MM:SS or MM:SS", timestamp)
	}
	return totalSeconds, nil
}

// TimestampRangeToSeconds parses a timestamp range in MM:SS-MM:SS or
// HH:MM:SS-HH:MM:SS format, returns the start and end values in seconds,
// and validates that the start time is before the end time.
func TimestampRangeToSeconds(timestampRange string) (int, int, error) {
	if !TimestampRangePattern.MatchString(timestampRange) {
		return 0, 0, fmt.Errorf("unable to parse video span %s", timestampRange)
	}
	parts := strings.Split(timestampRange, "-")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("unable to parse video span %s", timestampRange)
	}
	startSecond, err := TimestampToSeconds(parts[0])
	if err != nil {
		return 0, 0, err
	}
	endSecond, err := TimestampToSeconds(parts[1])
	if err != nil {
		return 0, 0, err
	}
	if startSecond >= endSecond {
		return 0, 0, fmt.Errorf("start spot is after or at the same that the end spot")
	}
	return startSecond, endSecond, nil
}

func ParseDownloadRequest(downloadRequestString string) (DownloadRequest, error) {

	invalidDownloadRequestErrTemplate := `invalid download request %q: expected one of the following formats:

https://www.youtube.com/watch?v=J---aiyznGQ
https://www.youtube.com/watch?v=J---aiyznGQ audio
https://www.youtube.com/watch?v=J---aiyznGQ 00:10-00:20
https://www.youtube.com/watch?v=J---aiyznGQ 00:10-00:20 audio

details: %w`

	invalidDownloadRequestErr := fmt.Errorf(
		invalidDownloadRequestErrTemplate,
		downloadRequestString,
		fmt.Errorf("download request does not follow the format"),
	)

	args := strings.Fields(downloadRequestString)
	argsNumber := len(args)

	if argsNumber == 0 {
		return DownloadRequest{}, invalidDownloadRequestErr
	}

	downloadRequest := DownloadRequest{}

	videoURL, err := url.Parse(args[0])
	if err != nil {
		return DownloadRequest{}, fmt.Errorf(invalidDownloadRequestErrTemplate, downloadRequestString, err)
	}
	downloadRequest.videoURL = videoURL.String()

	switch {
	case argsNumber == 1:
		return downloadRequest, nil
	case argsNumber == 2:
		if strings.ToLower(args[1]) == "audio" {
			downloadRequest.audioOnly = true
			return downloadRequest, nil
		}
		startSecond, endSecond, err := TimestampRangeToSeconds(args[1])
		if err != nil {
			return DownloadRequest{}, fmt.Errorf(invalidDownloadRequestErrTemplate, downloadRequestString, err)
		}
		downloadRequest.startSecond = startSecond
		downloadRequest.endSecond = endSecond
		return downloadRequest, nil
	case argsNumber == 3:
		startSecond, endSecond, err := TimestampRangeToSeconds(args[1])
		if err != nil {
			return DownloadRequest{}, fmt.Errorf(invalidDownloadRequestErrTemplate, downloadRequestString, err)
		}
		downloadRequest.startSecond = startSecond
		downloadRequest.endSecond = endSecond
		if strings.ToLower(args[2]) == "audio" {
			downloadRequest.audioOnly = true
			return downloadRequest, nil
		}
		return DownloadRequest{}, invalidDownloadRequestErr
	default:
		return DownloadRequest{}, invalidDownloadRequestErr
	}
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
		f, err := os.OpenFile(logFileEnv, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
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
			videoUrl, startSecond, endSecond, audioOnly, err := ParseDownloadRequest(update.Message.Text)
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
