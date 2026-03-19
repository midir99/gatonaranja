package main

import (
	"bytes"
	"fmt"
	"net/url"
	"os/exec"
	"strings"
)

// DownloadRequest describes the URL, timestamp range, and audio-only option
// for a media download request.
type DownloadRequest struct {
	startSecond int
	endSecond   int
	audioOnly   bool
	videoURL    string
}

// ParseDownloadRequest parses a download request string in the form URL,
// URL audio, URL TIMESTAMP_RANGE, or URL TIMESTAMP_RANGE audio, and returns
// the corresponding DownloadRequest.
func ParseDownloadRequest(downloadRequestString string) (DownloadRequest, error) {
	invalidDownloadRequestErrTemplate := `invalid download request %q: expected one of the following formats:

https://www.youtube.com/watch?v=J---aiyznGQ
https://www.youtube.com/watch?v=J---aiyznGQ audio
https://www.youtube.com/watch?v=J---aiyznGQ 00:10-00:20
https://www.youtube.com/watch?v=J---aiyznGQ 00:10-00:20 audio
https://www.youtube.com/watch?v=J---aiyznGQ start-00:20 audio
https://www.youtube.com/watch?v=J---aiyznGQ 00:10-end

details: %w`

	invalidDownloadRequestErr := fmt.Errorf(
		invalidDownloadRequestErrTemplate,
		downloadRequestString,
		"download request does not follow the format",
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
		// If its 1 argument we just received the video URL.
		// Example: https://www.youtube.com/watch?v=J---aiyznGQ
		downloadRequest.startSecond = StartSecond
		downloadRequest.endSecond = EndSecond
		return downloadRequest, nil
	case argsNumber == 2:
		// If its 2 arguments we received the video URL and a timestamp range or the audio flag.
		// Example: https://www.youtube.com/watch?v=J---aiyznGQ start-0:10
		// Example: https://www.youtube.com/watch?v=J---aiyznGQ audio
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
		// If its 3 arguments we received the video URL, a timestamp range and the audio flag.
		// Example: https://www.youtube.com/watch?v=J---aiyznGQ start-0:10 audio
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

// SecondsToYTDLDownloadSections converts start and end values expressed in seconds
// into a yt-dlp --download-sections time range in the form *START-END.
// When endSecond is EndSecond, the returned range uses inf as the end value.
func SecondsToYTDLDownloadSections(startSecond, endSecond int) (string, error) {
	switch {
	case startSecond < 0 && startSecond != StartSecond:
		return "", fmt.Errorf("invalid start second %d", startSecond)
	case endSecond < 0 && endSecond != EndSecond:
		return "", fmt.Errorf("invalid end second %d", endSecond)
	case endSecond != EndSecond && startSecond >= endSecond:
		return "", fmt.Errorf("start second must be lower than end second")
	}

	start := SecondsToTimestamp(startSecond)
	end := "inf"
	if endSecond != EndSecond {
		end = SecondsToTimestamp(endSecond)
	}

	return "*" + start + "-" + end, nil
}

// BuildYTDLPCommand builds the yt-dlp command for the download request,
// including optional section download and audio extraction flags, and
// returns it as a slice of arguments ready to be passed to exec.Command.
func (dr DownloadRequest) BuildYTDLPCommand() ([]string, error) {
	cmd := []string{"yt-dlp", "--no-simulate", "--print", "after_move:filepath"}

	if dr.startSecond != StartSecond || dr.endSecond != EndSecond {
		downloadSections, err := SecondsToYTDLDownloadSections(dr.startSecond, dr.endSecond)
		if err != nil {
			return nil, err
		}
		cmd = append(cmd, "--download-sections", downloadSections)
	}

	if dr.audioOnly {
		cmd = append(cmd, "--extract-audio", "--audio-format", "mp3")
	}
	// Use a Telegram-friendly fallback format selection strategy:
	// --format "18/best[ext=mp4]/best" prefers YouTube format 18 first, then the
	// best single-file MP4, and finally the best single-file format available.
	// --format-sort "+size,+br,+res,+fps" biases selection toward smaller files by
	// preferring lower filesize, bitrate, resolution, and frame rate.
	cmd = append(
		cmd,
		"--format",
		"18/best[ext=mp4]/best",
		"--format-sort",
		"+size,+br,+res,+fps",
		"--output",
		"%(title)s.%(ext)s",
		dr.videoURL,
	)
	return cmd, nil
}

// Download executes the yt-dlp command for the download request and returns
// the final output filepath reported by yt-dlp.
func (dr DownloadRequest) Download() (string, error) {
	cmdArgs, err := dr.BuildYTDLPCommand()
	if err != nil {
		return "", err
	}

	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("yt-dlp failed: %s: %s", err, stderr.String())
	}

	outputPath := strings.TrimSpace(stdout.String())
	if outputPath == "" {
		return "", fmt.Errorf("yt-dlp succeeded but did not print the output filepath")
	}

	return outputPath, nil
}
