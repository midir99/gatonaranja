package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"strings"
)

var (
	// ErrInvalidYouTubeURL reports that a YouTube URL is malformed or uses an
	// unsupported host, path, or query shape.
	ErrInvalidYouTubeURL = errors.New("invalid YouTube URL")
	// ErrInvalidDownloadRequest reports that a download request does not follow the
	// expected input format.
	ErrInvalidDownloadRequest = errors.New("invalid download request")
)

// MediaKind identifies the type of media to download.
type MediaKind int

const (
	MediaAudio MediaKind = iota
	MediaVideo
)

// MediaDownloader describes a media download request that can download itself
// using a context and report the kind of media it produces.
type MediaDownloader interface {
	Download(ctx context.Context) (string, error)
	MediaKind() MediaKind
}

// DownloadRequest describes the source URL, timestamp range, and media kind
// for a media download request.
type DownloadRequest struct {
	startSecond int
	endSecond   int
	mediaKind   MediaKind
	sourceURL   string
}

// validateYouTubeURL parses rawURL, validates that it is a supported YouTube
// video URL, and returns a sanitized version of the URL.
func validateYouTubeURL(rawURL string) (string, error) {
	rawURLNoSpaces := strings.TrimSpace(rawURL)
	if rawURLNoSpaces == "" {
		return "", fmt.Errorf("%w: %q", ErrInvalidYouTubeURL, rawURL)
	}

	parsedURL, err := url.Parse(rawURLNoSpaces)
	if err != nil {
		return "", fmt.Errorf("%w: %q", ErrInvalidYouTubeURL, rawURL)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return "", fmt.Errorf("%w: %q: scheme must be http or https", ErrInvalidYouTubeURL, rawURL)
	}

	host := strings.ToLower(parsedURL.Host)
	switch host {
	case "youtu.be":
		if !isValidYouTubeVideoIDPath("/", parsedURL.Path) {
			return "", fmt.Errorf(`%w: %q: youtu.be path must be "/<VIDEO_ID>"`, ErrInvalidYouTubeURL, rawURL)
		}
		return sanitizeYouTubeURL(parsedURL), nil
	case "youtube.com", "www.youtube.com", "music.youtube.com", "m.youtube.com":
		if parsedURL.Path == "/watch" {
			query := parsedURL.Query()
			if query.Get("v") == "" {
				return "", fmt.Errorf(`%w: %q: "v" query parameter is missing`, ErrInvalidYouTubeURL, rawURL)
			}
			return sanitizeYouTubeURL(parsedURL), nil
		} else if isValidYouTubeVideoIDPath("/shorts/", parsedURL.Path) {
			return sanitizeYouTubeURL(parsedURL), nil
		} else {
			return "", fmt.Errorf(`%w: %q: path must be "/watch" or "/shorts/<VIDEO_ID>"`, ErrInvalidYouTubeURL, rawURL)
		}
	default:
		return "", fmt.Errorf(
			"%w: %q: host must be youtube.com, www.youtube.com, music.youtube.com, youtu.be or m.youtube.com",
			ErrInvalidYouTubeURL,
			rawURL,
		)
	}
}

// isValidYouTubeVideoIDPath reports whether path starts with prefix and the
// remaining suffix is exactly one non-empty path segment that can be treated as
// a YouTube video ID.
func isValidYouTubeVideoIDPath(prefix, path string) bool {
	if !strings.HasPrefix(path, prefix) {
		return false
	}

	videoID := strings.TrimPrefix(path, prefix)
	if videoID == "" {
		return false
	}
	if strings.Contains(videoID, "/") {
		return false
	}

	return true
}

// sanitizeYouTubeURL removes playlist-related query parameters from a parsed
// YouTube video URL while preserving the actual video reference.
func sanitizeYouTubeURL(parsedURL *url.URL) string {
	query := parsedURL.Query()
	query.Del("list")
	query.Del("index")
	parsedURL.RawQuery = query.Encode()

	return parsedURL.String()
}

// ParseDownloadRequest parses a download request string in the form URL,
// URL audio, URL TIMESTAMP_RANGE, or URL TIMESTAMP_RANGE audio, and returns
// the corresponding DownloadRequest.
func ParseDownloadRequest(downloadRequestString string) (DownloadRequest, error) {
	args := strings.Fields(downloadRequestString)
	argsNumber := len(args)

	if argsNumber == 0 {
		return DownloadRequest{}, fmt.Errorf(
			"%w: %q: expected URL [TIMESTAMP_RANGE] [audio]", ErrInvalidDownloadRequest, downloadRequestString,
		)
	}

	downloadRequest := DownloadRequest{
		startSecond: StartSecond,
		endSecond:   EndSecond,
		mediaKind:   MediaVideo,
	}

	videoURL, err := validateYouTubeURL(args[0])
	if err != nil {
		return DownloadRequest{}, fmt.Errorf("%w: %q: %w", ErrInvalidDownloadRequest, downloadRequestString, err)
	}
	downloadRequest.sourceURL = videoURL

	switch argsNumber {
	case 1:
		// If its 1 argument we just received the video URL.
		// Example: https://www.youtube.com/watch?v=J---aiyznGQ
		return downloadRequest, nil
	case 2:
		// If its 2 arguments we received the video URL and a timestamp range or the audio flag.
		// Example: https://www.youtube.com/watch?v=J---aiyznGQ start-0:10
		// Example: https://www.youtube.com/watch?v=J---aiyznGQ audio
		if strings.ToLower(args[1]) == "audio" {
			downloadRequest.mediaKind = MediaAudio
			return downloadRequest, nil
		}
		startSecond, endSecond, err2 := TimestampRangeToSeconds(args[1])
		if err2 != nil {
			return DownloadRequest{}, fmt.Errorf("%w: %q: %w", ErrInvalidDownloadRequest, downloadRequestString, err2)
		}
		downloadRequest.startSecond = startSecond
		downloadRequest.endSecond = endSecond
		return downloadRequest, nil
	case 3:
		// If its 3 arguments we received the video URL, a timestamp range and the audio flag.
		// Example: https://www.youtube.com/watch?v=J---aiyznGQ start-0:10 audio
		startSecond, endSecond, err2 := TimestampRangeToSeconds(args[1])
		if err2 != nil {
			return DownloadRequest{}, fmt.Errorf("%w: %q: %w", ErrInvalidDownloadRequest, downloadRequestString, err2)
		}
		downloadRequest.startSecond = startSecond
		downloadRequest.endSecond = endSecond
		if strings.ToLower(args[2]) == "audio" {
			downloadRequest.mediaKind = MediaAudio
			return downloadRequest, nil
		}
		return DownloadRequest{}, fmt.Errorf(
			"%w: %q: expected URL [TIMESTAMP_RANGE] [audio]", ErrInvalidDownloadRequest, downloadRequestString,
		)
	default:
		return DownloadRequest{}, fmt.Errorf(
			"%w: %q: expected URL [TIMESTAMP_RANGE] [audio]", ErrInvalidDownloadRequest, downloadRequestString,
		)
	}
}

// SecondsToDownloadSections converts start and end values expressed in seconds
// into a yt-dlp --download-sections time range in the form *START-END.
// When endSecond is EndSecond, the returned range uses inf as the end value.
func SecondsToDownloadSections(startSecond, endSecond int) (string, error) {
	switch {
	case startSecond < 0 && startSecond != StartSecond:
		return "", fmt.Errorf("invalid start second %d", startSecond)
	case endSecond < 0 && endSecond != EndSecond:
		return "", fmt.Errorf("invalid end second %d", endSecond)
	case endSecond != EndSecond && startSecond >= endSecond:
		return "", errors.New("start second must be lower than end second")
	}

	start := SecondsToTimestamp(startSecond)
	end := "inf"
	if endSecond != EndSecond {
		end = SecondsToTimestamp(endSecond)
	}

	return "*" + start + "-" + end, nil
}

// MediaKind reports the kind of media produced by the download request.
func (dr DownloadRequest) MediaKind() MediaKind {
	return dr.mediaKind
}

// BuildCommand builds the yt-dlp command for the download request,
// including optional section download and audio extraction flags, and
// returns it as a slice of arguments ready to be passed to "[exec.Command]".
func (dr DownloadRequest) BuildCommand() ([]string, error) {
	cmd := []string{"yt-dlp", "--no-simulate", "--no-playlist", "--print", "after_move:filepath"}

	if dr.startSecond != StartSecond || dr.endSecond != EndSecond {
		downloadSections, err := SecondsToDownloadSections(dr.startSecond, dr.endSecond)
		if err != nil {
			return nil, err
		}
		cmd = append(cmd, "--download-sections", downloadSections)
	}

	if dr.MediaKind() == MediaAudio {
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
		dr.sourceURL,
	)
	return cmd, nil
}

var commandContext = exec.CommandContext

// Download executes the yt-dlp command for the download request using the
// provided context, and returns the final output filepath reported by yt-dlp.
func (dr DownloadRequest) Download(ctx context.Context) (string, error) {
	cmdArgs, err := dr.BuildCommand()
	if err != nil {
		return "", err
	}

	cmd := commandContext(ctx, cmdArgs[0], cmdArgs[1:]...)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		return "", fmt.Errorf("yt-dlp failed: %w: %s", err, stderr.String())
	}

	outputPath := strings.TrimSpace(stdout.String())
	if outputPath == "" {
		return "", errors.New("yt-dlp succeeded but did not print the output filepath")
	}

	return outputPath, nil
}
