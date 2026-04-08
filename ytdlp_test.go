package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateYouTubeURL(t *testing.T) {
	testCases := []struct {
		testName string
		rawURL   string
		want     string
		err      error
	}{
		{
			"youtu.be url",
			"https://youtu.be/8v_kBIIGViY?si=l79VR-6K5Bo73Tt8",
			"https://youtu.be/8v_kBIIGViY?si=l79VR-6K5Bo73Tt8",
			nil,
		},
		{
			"youtu.be url strips playlist parameters",
			"https://youtu.be/8v_kBIIGViY?list=PL123456&index=4",
			"https://youtu.be/8v_kBIIGViY",
			nil,
		},
		{
			"music youtube",
			"https://music.youtube.com/watch?v=Tsz8x47jhz8",
			"https://music.youtube.com/watch?v=Tsz8x47jhz8",
			nil,
		},
		{
			"www youtube",
			"https://www.youtube.com/watch?v=dQw4w9WgXcQ",
			"https://www.youtube.com/watch?v=dQw4w9WgXcQ",
			nil,
		},
		{
			"mobile youtube",
			"https://m.youtube.com/watch?v=dQw4w9WgXcQ",
			"https://m.youtube.com/watch?v=dQw4w9WgXcQ",
			nil,
		},
		{
			"plain youtube.com",
			"https://youtube.com/watch?v=dQw4w9WgXcQ",
			"https://youtube.com/watch?v=dQw4w9WgXcQ",
			nil,
		},
		{
			"shorts url",
			"https://www.youtube.com/shorts/dQw4w9WgXcQ",
			"https://www.youtube.com/shorts/dQw4w9WgXcQ",
			nil,
		},
		{
			"shorts url strips playlist parameters",
			"https://www.youtube.com/shorts/dQw4w9WgXcQ?list=PL123456&index=7",
			"https://www.youtube.com/shorts/dQw4w9WgXcQ",
			nil,
		},
		{
			"shorts url keeps non playlist query parameters",
			"https://www.youtube.com/shorts/dQw4w9WgXcQ?feature=share&list=PL123456",
			"https://www.youtube.com/shorts/dQw4w9WgXcQ?feature=share",
			nil,
		},
		{
			"watch url strips playlist parameters",
			"https://music.youtube.com/watch?v=5X-Mrc2l1d0&list=RDAMVM5X-Mrc2l1d0&index=7",
			"https://music.youtube.com/watch?v=5X-Mrc2l1d0",
			nil,
		},
		{
			"watch url keeps non playlist query parameters",
			"https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=43s&list=PL123456",
			"https://www.youtube.com/watch?t=43s&v=dQw4w9WgXcQ",
			nil,
		},
		{
			"http scheme",
			"http://youtu.be/dQw4w9WgXcQ",
			"http://youtu.be/dQw4w9WgXcQ",
			nil,
		},
		{
			"uppercase host",
			"https://YouTube.com/watch?v=dQw4w9WgXcQ",
			"https://YouTube.com/watch?v=dQw4w9WgXcQ",
			nil,
		},
		{
			"leading/trailing spaces",
			"   https://youtu.be/dQw4w9WgXcQ   ",
			"https://youtu.be/dQw4w9WgXcQ",
			nil,
		},
		{
			"invalid scheme",
			"ftp://youtu.be/dQw4w9WgXcQ",
			"",
			errors.New("invalid YouTube URL: \"ftp://youtu.be/dQw4w9WgXcQ\": scheme must be http or https"),
		},
		{
			"invalid host",
			"https://vimeo.com/123456",
			"",
			errors.New(
				"invalid YouTube URL: \"https://vimeo.com/123456\": host must be youtube.com, www.youtube.com, music.youtube.com, youtu.be or m.youtube.com",
			),
		},
		{
			"malformed url",
			"://youtube.com",
			"",
			errors.New("invalid YouTube URL: \"://youtube.com\""),
		},
		{
			"empty url",
			"",
			"",
			errors.New("invalid YouTube URL: \"\""),
		},
		{
			"subdomain not allowed",
			"https://gaming.youtube.com/watch?v=dQw4w9WgXcQ",
			"",
			errors.New(
				"invalid YouTube URL: \"https://gaming.youtube.com/watch?v=dQw4w9WgXcQ\": host must be youtube.com, www.youtube.com, music.youtube.com, youtu.be or m.youtube.com",
			),
		},
		{
			"invalid youtu.be path without video id",
			"https://youtu.be/",
			"",
			errors.New("invalid YouTube URL: \"https://youtu.be/\": youtu.be path must be \"/<VIDEO_ID>\""),
		},
		{
			"invalid youtu.be nested path",
			"https://youtu.be/dQw4w9WgXcQ/extra",
			"",
			errors.New(
				"invalid YouTube URL: \"https://youtu.be/dQw4w9WgXcQ/extra\": youtu.be path must be \"/<VIDEO_ID>\"",
			),
		},
		{
			"invalid path on youtube host",
			"https://www.youtube.com/channel/UC38IQsAvIsxxjztdMZQtwHA",
			"",
			errors.New(
				"invalid YouTube URL: \"https://www.youtube.com/channel/UC38IQsAvIsxxjztdMZQtwHA\": path must be \"/watch\" or \"/shorts/<VIDEO_ID>\"",
			),
		},
		{
			"invalid shorts path without video id",
			"https://www.youtube.com/shorts/",
			"",
			errors.New(
				"invalid YouTube URL: \"https://www.youtube.com/shorts/\": path must be \"/watch\" or \"/shorts/<VIDEO_ID>\"",
			),
		},
		{
			"invalid shorts nested path",
			"https://www.youtube.com/shorts/dQw4w9WgXcQ/extra",
			"",
			errors.New(
				"invalid YouTube URL: \"https://www.youtube.com/shorts/dQw4w9WgXcQ/extra\": path must be \"/watch\" or \"/shorts/<VIDEO_ID>\"",
			),
		},
		{
			"missing v query parameter",
			"https://www.youtube.com/watch?list=PL123456",
			"",
			errors.New(
				"invalid YouTube URL: \"https://www.youtube.com/watch?list=PL123456\": \"v\" query parameter is missing",
			),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			got, err := validateYouTubeURL(tc.rawURL)
			if tc.err != nil && err == nil {
				t.Fatalf("got nil, want %q", tc.err.Error())
			}
			if tc.err != nil && err != nil && tc.err.Error() != err.Error() {
				t.Fatalf("got %q, want %q", err.Error(), tc.err.Error())
			}
			if tc.err != nil && !errors.Is(err, ErrInvalidYouTubeURL) {
				t.Fatalf("errors.Is(%v, ErrInvalidYouTubeURL) = false, want true", err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestIsValidYouTubeVideoIDPath(t *testing.T) {
	testCases := []struct {
		testName string
		prefix   string
		path     string
		want     bool
	}{
		{
			"youtu.be path with video id",
			"/",
			"/dQw4w9WgXcQ",
			true,
		},
		{
			"shorts path with video id",
			"/shorts/",
			"/shorts/dQw4w9WgXcQ",
			true,
		},
		{
			"empty youtu.be path",
			"/",
			"/",
			false,
		},
		{
			"empty shorts path",
			"/shorts/",
			"/shorts/",
			false,
		},
		{
			"nested youtu.be path",
			"/",
			"/foo/bar",
			false,
		},
		{
			"nested shorts path",
			"/shorts/",
			"/shorts/dQw4w9WgXcQ/extra",
			false,
		},
		{
			"wrong prefix",
			"/shorts/",
			"/watch",
			false,
		},
		{
			"path without leading slash for youtu.be prefix",
			"/",
			"dQw4w9WgXcQ",
			false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			got := isValidYouTubeVideoIDPath(tc.prefix, tc.path)
			if got != tc.want {
				t.Fatalf("got %t, want %t", got, tc.want)
			}
		})
	}
}

func TestSanitizeYouTubeURL(t *testing.T) {
	testCases := []struct {
		testName string
		rawURL   string
		want     string
	}{
		{
			"watch url without playlist parameters",
			"https://www.youtube.com/watch?v=dQw4w9WgXcQ",
			"https://www.youtube.com/watch?v=dQw4w9WgXcQ",
		},
		{
			"watch url removes list and index",
			"https://www.youtube.com/watch?v=dQw4w9WgXcQ&list=PL123456&index=3",
			"https://www.youtube.com/watch?v=dQw4w9WgXcQ",
		},
		{
			"watch url keeps unrelated query parameters",
			"https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=43s&list=PL123456",
			"https://www.youtube.com/watch?t=43s&v=dQw4w9WgXcQ",
		},
		{
			"shorts url removes playlist parameters",
			"https://www.youtube.com/shorts/dQw4w9WgXcQ?list=PL123456&index=7",
			"https://www.youtube.com/shorts/dQw4w9WgXcQ",
		},
		{
			"shorts url keeps unrelated query parameters",
			"https://www.youtube.com/shorts/dQw4w9WgXcQ?feature=share&list=PL123456",
			"https://www.youtube.com/shorts/dQw4w9WgXcQ?feature=share",
		},
		{
			"youtu.be url removes playlist parameters",
			"https://youtu.be/dQw4w9WgXcQ?list=PL123456&index=7",
			"https://youtu.be/dQw4w9WgXcQ",
		},
		{
			"youtu.be url keeps unrelated query parameters",
			"https://youtu.be/dQw4w9WgXcQ?si=abc123&list=PL123456",
			"https://youtu.be/dQw4w9WgXcQ?si=abc123",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			parsedURL, err := url.Parse(tc.rawURL)
			if err != nil {
				t.Fatalf("url.Parse() error = %v, want nil", err)
			}

			got := sanitizeYouTubeURL(parsedURL)
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParseDownloadRequest(t *testing.T) {
	testCases := []struct {
		testName              string
		downloadRequestString string
		wantDownloadRequest   DownloadRequest
		err                   error
	}{
		{
			"video download request",
			"https://www.youtube.com/watch?v=8v_kBIIGViY",
			DownloadRequest{
				StartSecond: StartSecond,
				EndSecond:   EndSecond,
				MediaKind:   MediaVideo,
				SourceURL:   "https://www.youtube.com/watch?v=8v_kBIIGViY",
			},
			nil,
		},
		{
			"audio download request",
			"https://www.youtube.com/watch?v=8v_kBIIGViY audio",
			DownloadRequest{
				StartSecond: StartSecond,
				EndSecond:   EndSecond,
				MediaKind:   MediaAudio,
				SourceURL:   "https://www.youtube.com/watch?v=8v_kBIIGViY",
			},
			nil,
		},
		{
			"video clip download request",
			"https://www.youtube.com/watch?v=8v_kBIIGViY 2:45-2:53",
			DownloadRequest{
				StartSecond: 165,
				EndSecond:   173,
				MediaKind:   MediaVideo,
				SourceURL:   "https://www.youtube.com/watch?v=8v_kBIIGViY",
			},
			nil,
		},
		{
			"audio clip download request",
			"https://www.youtube.com/watch?v=8v_kBIIGViY 2:45-2:53 audio",
			DownloadRequest{
				StartSecond: 165,
				EndSecond:   173,
				MediaKind:   MediaAudio,
				SourceURL:   "https://www.youtube.com/watch?v=8v_kBIIGViY",
			},
			nil,
		},
		{
			"playlist parameters are stripped from video url",
			"https://music.youtube.com/watch?v=5X-Mrc2l1d0&list=RDAMVM5X-Mrc2l1d0 2:45-2:53 audio",
			DownloadRequest{
				StartSecond: 165,
				EndSecond:   173,
				MediaKind:   MediaAudio,
				SourceURL:   "https://music.youtube.com/watch?v=5X-Mrc2l1d0",
			},
			nil,
		},
		{
			"youtu.be short link download request",
			"https://youtu.be/8v_kBIIGViY?list=PL123456&index=4 audio",
			DownloadRequest{
				StartSecond: StartSecond,
				EndSecond:   EndSecond,
				MediaKind:   MediaAudio,
				SourceURL:   "https://youtu.be/8v_kBIIGViY",
			},
			nil,
		},
		{
			"empty download request",
			"",
			DownloadRequest{},
			errors.New(
				"invalid download request: \"\": expected URL [TIMESTAMP_RANGE] [audio]",
			),
		},
		{
			"invalid YouTube URL",
			"https://gaming.youtube.com/watch?v=dQw4w9WgXcQ",
			DownloadRequest{},
			errors.New(
				"invalid download request: \"https://gaming.youtube.com/watch?v=dQw4w9WgXcQ\": invalid YouTube URL: \"https://gaming.youtube.com/watch?v=dQw4w9WgXcQ\": host must be youtube.com, www.youtube.com, music.youtube.com, youtu.be or m.youtube.com",
			),
		},
		{
			"missing v query parameter",
			"https://www.youtube.com/watch?list=PL123456",
			DownloadRequest{},
			errors.New(
				"invalid download request: \"https://www.youtube.com/watch?list=PL123456\": invalid YouTube URL: \"https://www.youtube.com/watch?list=PL123456\": \"v\" query parameter is missing",
			),
		},
		{
			"invalid timestamp",
			"https://www.youtube.com/watch?v=8v_kBIIGViY invalidtimestamp",
			DownloadRequest{},
			errors.New(
				"invalid download request: \"https://www.youtube.com/watch?v=8v_kBIIGViY invalidtimestamp\": invalid timestamp range: \"invalidtimestamp\": expected START-END where each value is MM:SS, HH:MM:SS, start, or end",
			),
		},
		{
			"invalid timestamp 2",
			"https://www.youtube.com/watch?v=8v_kBIIGViY invalidtimestamp audio",
			DownloadRequest{},
			errors.New(
				"invalid download request: \"https://www.youtube.com/watch?v=8v_kBIIGViY invalidtimestamp audio\": invalid timestamp range: \"invalidtimestamp\": expected START-END where each value is MM:SS, HH:MM:SS, start, or end",
			),
		},
		{
			"invalid audio",
			"https://www.youtube.com/watch?v=8v_kBIIGViY 2:45-2:53 invalidaudio",
			DownloadRequest{},
			errors.New(
				"invalid download request: \"https://www.youtube.com/watch?v=8v_kBIIGViY 2:45-2:53 invalidaudio\": expected URL [TIMESTAMP_RANGE] [audio]",
			),
		},
		{
			"invalid download request",
			"https://www.youtube.com/watch?v=8v_kBIIGViY 2:45-2:53 audio anotherinvalidparameter",
			DownloadRequest{},
			errors.New(
				"invalid download request: \"https://www.youtube.com/watch?v=8v_kBIIGViY 2:45-2:53 audio anotherinvalidparameter\": expected URL [TIMESTAMP_RANGE] [audio]",
			),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			got, err := ParseDownloadRequest(tc.downloadRequestString)
			if tc.err != nil && err == nil {
				t.Fatalf("got nil, want %q", tc.err.Error())
			}
			if tc.err != nil && err != nil && tc.err.Error() != err.Error() {
				t.Fatalf("got %q, want %q", err.Error(), tc.err.Error())
			}
			if tc.err != nil && !errors.Is(err, ErrInvalidDownloadRequest) {
				t.Fatalf("errors.Is(%v, ErrInvalidDownloadRequest) = false, want true", err)
			}
			if got.StartSecond != tc.wantDownloadRequest.StartSecond {
				t.Fatalf("got %d, want %d", got.StartSecond, tc.wantDownloadRequest.StartSecond)
			}
			if got.EndSecond != tc.wantDownloadRequest.EndSecond {
				t.Fatalf("got %d, want %d", got.EndSecond, tc.wantDownloadRequest.EndSecond)
			}
			if got.MediaKind != tc.wantDownloadRequest.MediaKind {
				t.Fatalf("got %d, want %d", got.MediaKind, tc.wantDownloadRequest.MediaKind)
			}
			if got.SourceURL != tc.wantDownloadRequest.SourceURL {
				t.Fatalf("got %q, want %q", got.SourceURL, tc.wantDownloadRequest.SourceURL)
			}
		})
	}
}
func TestSecondsToDownloadSections(t *testing.T) {
	testCases := []struct {
		testName             string
		startSecond          int
		endSecond            int
		wantDownloadSections string
		err                  error
	}{
		{
			"seconds range",
			0,
			5,
			"*00:00-00:05",
			nil,
		},
		{
			"minutes range",
			10,
			75,
			"*00:10-01:15",
			nil,
		},
		{
			"hours included",
			3600,
			3665,
			"*01:00:00-01:01:05",
			nil,
		},
		{
			"start zero to one hour",
			0,
			3600,
			"*00:00-01:00:00",
			nil,
		},
		{
			"same start and end (invalid)",
			10,
			10,
			"",
			errors.New("start second must be lower than end second"),
		},
		{
			"start greater than end (invalid)",
			20,
			10,
			"",
			errors.New("start second must be lower than end second"),
		},
		{
			"negative start (invalid)",
			-5,
			10,
			"",
			fmt.Errorf("invalid start second %d", -5),
		},
		{
			"negative end (invalid)",
			5,
			-10,
			"",
			fmt.Errorf("invalid end second %d", -10),
		},
		{
			"end is infinity",
			5,
			EndSecond,
			"*00:05-inf",
			nil,
		},
		{
			"start is sentinel StartSecond",
			StartSecond,
			10,
			"*" + SecondsToTimestamp(StartSecond) + "-00:10",
			nil,
		},
		{
			"both sentinels",
			StartSecond,
			EndSecond,
			"*" + SecondsToTimestamp(StartSecond) + "-inf",
			nil,
		},
		{
			"large values",
			7325,
			10800,
			"*02:02:05-03:00:00",
			nil,
		},
		{
			"start zero end infinity",
			0,
			EndSecond,
			"*00:00-inf",
			nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			got, err := SecondsToDownloadSections(tc.startSecond, tc.endSecond)
			if tc.err != nil && err == nil {
				t.Fatalf("got nil, want %q", tc.err.Error())
			}
			if tc.err != nil && err != nil && tc.err.Error() != err.Error() {
				t.Fatalf("got %q, want %q", err.Error(), tc.err.Error())
			}
			if got != tc.wantDownloadSections {
				t.Fatalf("got %q, want %q", got, tc.wantDownloadSections)
			}
		})
	}
}
func TestYTDLPDownloaderMediaKind(t *testing.T) {
	testCases := []struct {
		testName      string
		downloader    YTDLPDownloader
		wantMediaKind MediaKind
	}{
		{
			"media video",
			NewYTDLPDownloader(DownloadRequest{MediaKind: MediaVideo}, YTDLPOptions{}),
			MediaVideo,
		},
		{
			"media audio",
			NewYTDLPDownloader(DownloadRequest{MediaKind: MediaAudio}, YTDLPOptions{}),
			MediaAudio,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			got := tc.downloader.MediaKind()
			if got != tc.wantMediaKind {
				t.Fatalf("got %d, want %d", got, tc.wantMediaKind)
			}
		})
	}
}

func compareStringArray(t *testing.T, got, want []string) {
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

func ytdlpHelperOutputPath() string {
	return filepath.Join(os.TempDir(), "gatonaranja-ytdlp-helper-file.mp4")
}

func TestYTDLPDownloaderBuildCommand(t *testing.T) {
	testCases := []struct {
		testName    string
		downloader  YTDLPDownloader
		wantCommand []string
		err         error
	}{
		{
			"video without sections",
			NewYTDLPDownloader(DownloadRequest{
				StartSecond: StartSecond,
				EndSecond:   EndSecond,
				SourceURL:   "https://www.youtube.com/watch?v=8v_kBIIGViY",
				MediaKind:   MediaVideo,
			}, YTDLPOptions{}),
			[]string{
				"yt-dlp",
				"--no-simulate",
				"--no-playlist",
				"--print", "after_move:filepath",
				"--ignore-config",
				"--format", "18/best[ext=mp4]/best",
				"--format-sort", "+size,+br,+res,+fps",
				"--output", "%(title)s.%(ext)s",
				"https://www.youtube.com/watch?v=8v_kBIIGViY",
			},
			nil,
		},
		{
			"video with download sections",
			NewYTDLPDownloader(DownloadRequest{
				StartSecond: StartSecond,
				EndSecond:   5,
				SourceURL:   "https://www.youtube.com/watch?v=8v_kBIIGViY",
				MediaKind:   MediaVideo,
			}, YTDLPOptions{}),
			[]string{
				"yt-dlp",
				"--no-simulate",
				"--no-playlist",
				"--print", "after_move:filepath",
				"--ignore-config",
				"--download-sections", "*00:00-00:05",
				"--format", "18/best[ext=mp4]/best",
				"--format-sort", "+size,+br,+res,+fps",
				"--output", "%(title)s.%(ext)s",
				"https://www.youtube.com/watch?v=8v_kBIIGViY",
			},
			nil,
		},
		{
			"audio with download sections",
			NewYTDLPDownloader(DownloadRequest{
				StartSecond: StartSecond,
				EndSecond:   5,
				SourceURL:   "https://www.youtube.com/watch?v=8v_kBIIGViY",
				MediaKind:   MediaAudio,
			}, YTDLPOptions{}),
			[]string{
				"yt-dlp",
				"--no-simulate",
				"--no-playlist",
				"--print", "after_move:filepath",
				"--ignore-config",
				"--download-sections", "*00:00-00:05",
				"--extract-audio",
				"--audio-format", "mp3",
				"--format", "18/best[ext=mp4]/best",
				"--format-sort", "+size,+br,+res,+fps",
				"--output", "%(title)s.%(ext)s",
				"https://www.youtube.com/watch?v=8v_kBIIGViY",
			},
			nil,
		},
		{
			"video with explicit ytdlp config",
			NewYTDLPDownloader(DownloadRequest{
				StartSecond: StartSecond,
				EndSecond:   EndSecond,
				SourceURL:   "https://www.youtube.com/watch?v=8v_kBIIGViY",
				MediaKind:   MediaVideo,
			}, YTDLPOptions{ConfigPath: "/home/arthur/.config/gatonaranja/yt-dlp.conf"}),
			[]string{
				"yt-dlp",
				"--no-simulate",
				"--no-playlist",
				"--print", "after_move:filepath",
				"--ignore-config",
				"--config-locations", "/home/arthur/.config/gatonaranja/yt-dlp.conf",
				"--format", "18/best[ext=mp4]/best",
				"--format-sort", "+size,+br,+res,+fps",
				"--output", "%(title)s.%(ext)s",
				"https://www.youtube.com/watch?v=8v_kBIIGViY",
			},
			nil,
		},
		{
			"invalid seconds",
			NewYTDLPDownloader(DownloadRequest{
				StartSecond: 20,
				EndSecond:   10,
				SourceURL:   "https://www.youtube.com/watch?v=8v_kBIIGViY",
				MediaKind:   MediaVideo,
			}, YTDLPOptions{}),
			nil,
			errors.New("start second must be lower than end second"),
		},
		{
			"explicit range without sentinel start",
			NewYTDLPDownloader(DownloadRequest{
				StartSecond: 10,
				EndSecond:   20,
				SourceURL:   "https://www.youtube.com/watch?v=8v_kBIIGViY",
				MediaKind:   MediaVideo,
			}, YTDLPOptions{}),
			[]string{
				"yt-dlp",
				"--no-simulate",
				"--no-playlist",
				"--print", "after_move:filepath",
				"--ignore-config",
				"--download-sections", "*00:10-00:20",
				"--format", "18/best[ext=mp4]/best",
				"--format-sort", "+size,+br,+res,+fps",
				"--output", "%(title)s.%(ext)s",
				"https://www.youtube.com/watch?v=8v_kBIIGViY",
			},
			nil,
		},
		{
			"hours range",
			NewYTDLPDownloader(DownloadRequest{
				StartSecond: 3600,
				EndSecond:   3665,
				SourceURL:   "https://www.youtube.com/watch?v=8v_kBIIGViY",
				MediaKind:   MediaVideo,
			}, YTDLPOptions{}),
			[]string{
				"yt-dlp",
				"--no-simulate",
				"--no-playlist",
				"--print", "after_move:filepath",
				"--ignore-config",
				"--download-sections", "*01:00:00-01:01:05",
				"--format", "18/best[ext=mp4]/best",
				"--format-sort", "+size,+br,+res,+fps",
				"--output", "%(title)s.%(ext)s",
				"https://www.youtube.com/watch?v=8v_kBIIGViY",
			},
			nil,
		},
		{
			"start zero end infinity",
			NewYTDLPDownloader(DownloadRequest{
				StartSecond: 0,
				EndSecond:   EndSecond,
				SourceURL:   "https://www.youtube.com/watch?v=8v_kBIIGViY",
				MediaKind:   MediaVideo,
			}, YTDLPOptions{}),
			[]string{
				"yt-dlp",
				"--no-simulate",
				"--no-playlist",
				"--print", "after_move:filepath",
				"--ignore-config",
				"--format", "18/best[ext=mp4]/best",
				"--format-sort", "+size,+br,+res,+fps",
				"--output", "%(title)s.%(ext)s",
				"https://www.youtube.com/watch?v=8v_kBIIGViY",
			},
			nil,
		},
		{
			"audio without sections",
			NewYTDLPDownloader(DownloadRequest{
				StartSecond: StartSecond,
				EndSecond:   EndSecond,
				SourceURL:   "https://www.youtube.com/watch?v=8v_kBIIGViY",
				MediaKind:   MediaAudio,
			}, YTDLPOptions{}),
			[]string{
				"yt-dlp",
				"--no-simulate",
				"--no-playlist",
				"--print", "after_move:filepath",
				"--ignore-config",
				"--extract-audio",
				"--audio-format", "mp3",
				"--format", "18/best[ext=mp4]/best",
				"--format-sort", "+size,+br,+res,+fps",
				"--output", "%(title)s.%(ext)s",
				"https://www.youtube.com/watch?v=8v_kBIIGViY",
			},
			nil,
		},
		{
			"audio with infinity end",
			NewYTDLPDownloader(DownloadRequest{
				StartSecond: 30,
				EndSecond:   EndSecond,
				SourceURL:   "https://www.youtube.com/watch?v=8v_kBIIGViY",
				MediaKind:   MediaAudio,
			}, YTDLPOptions{}),
			[]string{
				"yt-dlp",
				"--no-simulate",
				"--no-playlist",
				"--print", "after_move:filepath",
				"--ignore-config",
				"--download-sections", "*00:30-inf",
				"--extract-audio",
				"--audio-format", "mp3",
				"--format", "18/best[ext=mp4]/best",
				"--format-sort", "+size,+br,+res,+fps",
				"--output", "%(title)s.%(ext)s",
				"https://www.youtube.com/watch?v=8v_kBIIGViY",
			},
			nil,
		},
		{
			"negative start",
			NewYTDLPDownloader(DownloadRequest{
				StartSecond: -5,
				EndSecond:   10,
				SourceURL:   "https://www.youtube.com/watch?v=8v_kBIIGViY",
				MediaKind:   MediaVideo,
			}, YTDLPOptions{}),
			nil,
			fmt.Errorf("invalid start second %d", -5),
		},
		{
			"negative end",
			NewYTDLPDownloader(DownloadRequest{
				StartSecond: 5,
				EndSecond:   -10,
				SourceURL:   "https://www.youtube.com/watch?v=8v_kBIIGViY",
				MediaKind:   MediaVideo,
			}, YTDLPOptions{}),
			nil,
			fmt.Errorf("invalid end second %d", -10),
		},
		{
			"equal start and end",
			NewYTDLPDownloader(DownloadRequest{
				StartSecond: 10,
				EndSecond:   10,
				SourceURL:   "https://www.youtube.com/watch?v=8v_kBIIGViY",
				MediaKind:   MediaVideo,
			}, YTDLPOptions{}),
			nil,
			errors.New("start second must be lower than end second"),
		},
		{
			"no sections when both sentinels",
			NewYTDLPDownloader(DownloadRequest{
				StartSecond: StartSecond,
				EndSecond:   EndSecond,
				SourceURL:   "https://www.youtube.com/watch?v=8v_kBIIGViY",
				MediaKind:   MediaAudio,
			}, YTDLPOptions{}),
			[]string{
				"yt-dlp",
				"--no-simulate",
				"--no-playlist",
				"--print", "after_move:filepath",
				"--ignore-config",
				"--extract-audio",
				"--audio-format", "mp3",
				"--format", "18/best[ext=mp4]/best",
				"--format-sort", "+size,+br,+res,+fps",
				"--output", "%(title)s.%(ext)s",
				"https://www.youtube.com/watch?v=8v_kBIIGViY",
			},
			nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			got, err := tc.downloader.BuildCommand()
			if tc.err != nil && err == nil {
				t.Fatalf("got nil, want %q", tc.err.Error())
			}
			if tc.err != nil && err != nil && tc.err.Error() != err.Error() {
				t.Fatalf("got %q, want %q", err.Error(), tc.err.Error())
			}
			compareStringArray(t, got, tc.wantCommand)
		})
	}
}

func TestCommandContext(t *testing.T) {
	t.Run("basic command with args", func(t *testing.T) {
		ctx := context.TODO()
		name := "echo"
		args := []string{"hello", "world"}

		cmd := commandContext(ctx, name, args...)

		want := append([]string{name}, args...)
		compareStringArray(t, cmd.Args, want)
	})

	t.Run("no args", func(t *testing.T) {
		ctx := context.TODO()
		name := "echo"

		cmd := commandContext(ctx, name)

		want := []string{name}
		compareStringArray(t, cmd.Args, want)
	})

	t.Run("empty args slice", func(t *testing.T) {
		ctx := context.TODO()
		name := "echo"
		var args []string

		cmd := commandContext(ctx, name, args...)

		want := []string{name}
		compareStringArray(t, cmd.Args, want)
	})

	t.Run("preserves argument order", func(t *testing.T) {
		ctx := context.TODO()
		name := "cmd"
		args := []string{"a", "b", "c"}

		cmd := commandContext(ctx, name, args...)

		want := []string{"cmd", "a", "b", "c"}
		compareStringArray(t, cmd.Args, want)
	})

	t.Run("context is set", func(t *testing.T) {
		ctx := context.TODO()
		name := "echo"

		cmd := commandContext(ctx, name)

		if cmd == nil {
			t.Fatal("expected command, got nil")
		}

		// We can't directly compare contexts, but we can ensure it's not nil
		if cmd.Process != nil {
			t.Fatal("process should not be started yet")
		}
	})
}

func TestHelperProcess(_ *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	args := os.Args
	separatorIndex := -1
	for i, arg := range args {
		if arg == "--" {
			separatorIndex = i
			break
		}
	}
	if separatorIndex == -1 || separatorIndex+1 >= len(args) {
		fmt.Fprint(os.Stderr, "missing helper arguments")
		os.Exit(2)
	}

	helperArgs := args[separatorIndex+1:]
	mode := helperArgs[0]

	switch mode {
	case "success":
		if err := os.WriteFile(ytdlpHelperOutputPath(), []byte("video"), 0o600); err != nil {
			fmt.Fprint(os.Stderr, err.Error())
			os.Exit(2)
		}
		fmt.Fprint(os.Stdout, ytdlpHelperOutputPath())
		os.Exit(0)
	case "empty-success":
		os.Exit(0)
	case "success-with-spaces":
		if err := os.WriteFile(ytdlpHelperOutputPath(), []byte("video"), 0o600); err != nil {
			fmt.Fprint(os.Stderr, err.Error())
			os.Exit(2)
		}
		fmt.Fprintf(os.Stdout, "\n\n \t \r %s   \t\t\n\t", ytdlpHelperOutputPath())
		os.Exit(0)
	case "stderr-and-fail":
		fmt.Fprint(os.Stderr, "error")
		os.Exit(1)
	case "fail-without-stderr":
		os.Exit(1)
	case "multiline-stdout":
		if err := os.WriteFile(ytdlpHelperOutputPath(), []byte("video"), 0o600); err != nil {
			fmt.Fprint(os.Stderr, err.Error())
			os.Exit(2)
		}
		fmt.Fprintf(os.Stdout, "%s\nextra\n", ytdlpHelperOutputPath())
		os.Exit(0)
	case "directory-output":
		outputDirPath := filepath.Join(os.TempDir(), "gatonaranja-ytdlp-helper-dir")
		if err := os.MkdirAll(outputDirPath, 0o755); err != nil {
			fmt.Fprint(os.Stderr, err.Error())
			os.Exit(2)
		}
		fmt.Fprint(os.Stdout, outputDirPath)
		os.Exit(0)
	default:
		fmt.Fprint(os.Stderr, "unknown helper mode")
		os.Exit(2)
	}
}

func helperCommand(ctx context.Context, mode string, args ...string) *exec.Cmd {
	commandArgs := []string{
		"-test.run=TestHelperProcess",
		"--",
		mode,
	}
	commandArgs = append(commandArgs, args...)

	cmd := exec.CommandContext(ctx, os.Args[0], commandArgs...)
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	return cmd
}

func TestYTDLPDownloaderDownload(t *testing.T) {
	testCases := []struct {
		testName        string
		downloader      YTDLPDownloader
		funcCommand     func(ctx context.Context, name string, args ...string) *exec.Cmd
		wantFilepath    string
		wantErr         bool
		wantErrContains []string
	}{
		{
			"successful download returns filepath",
			NewYTDLPDownloader(DownloadRequest{
				StartSecond: StartSecond,
				EndSecond:   EndSecond,
				SourceURL:   "https://www.youtube.com/watch?v=IFbXnS1odNs",
				MediaKind:   MediaVideo,
			}, YTDLPOptions{}),
			func(ctx context.Context, _ string, args ...string) *exec.Cmd {
				return helperCommand(ctx, "success", args...)
			},
			ytdlpHelperOutputPath(),
			false,
			[]string{},
		},
		{
			"build command error is returned",
			NewYTDLPDownloader(DownloadRequest{
				StartSecond: 10,
				EndSecond:   10,
				SourceURL:   "https://www.youtube.com/watch?v=IFbXnS1odNs",
				MediaKind:   MediaVideo,
			}, YTDLPOptions{}),
			func(ctx context.Context, _ string, args ...string) *exec.Cmd {
				return helperCommand(ctx, "success", args...)
			},
			"",
			true,
			[]string{"start second must be lower than end second"},
		},
		{
			"empty stdout returns filepath error",
			NewYTDLPDownloader(DownloadRequest{
				StartSecond: StartSecond,
				EndSecond:   EndSecond,
				SourceURL:   "https://www.youtube.com/watch?v=IFbXnS1odNs",
				MediaKind:   MediaVideo,
			}, YTDLPOptions{}),
			func(ctx context.Context, _ string, args ...string) *exec.Cmd {
				return helperCommand(ctx, "empty-success", args...)
			},
			"",
			true,
			[]string{"yt-dlp succeeded but did not print the output filepath"},
		},
		{
			"trims spaces from stdout",
			NewYTDLPDownloader(DownloadRequest{
				StartSecond: StartSecond,
				EndSecond:   EndSecond,
				SourceURL:   "https://www.youtube.com/watch?v=IFbXnS1odNs",
				MediaKind:   MediaVideo,
			}, YTDLPOptions{}),
			func(ctx context.Context, _ string, args ...string) *exec.Cmd {
				return helperCommand(ctx, "success-with-spaces", args...)
			},
			ytdlpHelperOutputPath(),
			false,
			[]string{},
		},
		{
			"command fails with stderr output",
			NewYTDLPDownloader(DownloadRequest{
				StartSecond: StartSecond,
				EndSecond:   EndSecond,
				SourceURL:   "https://www.youtube.com/watch?v=IFbXnS1odNs",
				MediaKind:   MediaVideo,
			}, YTDLPOptions{}),
			func(ctx context.Context, _ string, args ...string) *exec.Cmd {
				return helperCommand(ctx, "stderr-and-fail", args...)
			},
			"",
			true,
			[]string{"yt-dlp failed", "error"},
		},
		{
			"command fails without stderr",
			NewYTDLPDownloader(DownloadRequest{
				StartSecond: StartSecond,
				EndSecond:   EndSecond,
				SourceURL:   "https://www.youtube.com/watch?v=IFbXnS1odNs",
				MediaKind:   MediaVideo,
			}, YTDLPOptions{}),
			func(ctx context.Context, _ string, args ...string) *exec.Cmd {
				return helperCommand(ctx, "fail-without-stderr", args...)
			},
			"",
			true,
			[]string{"yt-dlp failed"},
		},
		{
			"multi-line stdout fails file validation",
			NewYTDLPDownloader(DownloadRequest{
				StartSecond: StartSecond,
				EndSecond:   EndSecond,
				SourceURL:   "https://www.youtube.com/watch?v=IFbXnS1odNs",
				MediaKind:   MediaVideo,
			}, YTDLPOptions{}),
			func(ctx context.Context, _ string, args ...string) *exec.Cmd {
				return helperCommand(ctx, "multiline-stdout", args...)
			},
			"",
			true,
			[]string{"yt-dlp printed output filepath", "not accessible"},
		},
		{
			"printed output path is a directory",
			NewYTDLPDownloader(DownloadRequest{
				StartSecond: StartSecond,
				EndSecond:   EndSecond,
				SourceURL:   "https://www.youtube.com/watch?v=IFbXnS1odNs",
				MediaKind:   MediaVideo,
			}, YTDLPOptions{}),
			func(ctx context.Context, _ string, args ...string) *exec.Cmd {
				return helperCommand(ctx, "directory-output", args...)
			},
			"",
			true,
			[]string{"yt-dlp printed output filepath", "not a regular file"},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			productionCommandContext := commandContext
			commandContext = tc.funcCommand
			defer func() {
				commandContext = productionCommandContext
			}()
			got, err := tc.downloader.Download(context.Background())
			if !tc.wantErr && err != nil {
				t.Fatalf("got error %q, want nil", err.Error())
			}
			if tc.wantErr {
				if err == nil {
					t.Fatalf("got nil error, want error")
				} else {
					for _, errString := range tc.wantErrContains {
						if !strings.Contains(err.Error(), errString) {
							t.Fatalf("error %q does not contain %q", err.Error(), errString)
						}
					}
				}
			}
			if got != tc.wantFilepath {
				t.Fatalf("got %q, want %q", got, tc.wantFilepath)
			}
		})
	}

	t.Run("canceled context returns command error", func(t *testing.T) {
		productionCommandContext := commandContext
		commandContext = func(ctx context.Context, _ string, args ...string) *exec.Cmd {
			return helperCommand(ctx, "success", args...)
		}
		defer func() { commandContext = productionCommandContext }()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		got, err := NewYTDLPDownloader(DownloadRequest{
			StartSecond: StartSecond,
			EndSecond:   EndSecond,
			SourceURL:   "https://www.youtube.com/watch?v=IFbXnS1odNs",
			MediaKind:   MediaVideo,
		}, YTDLPOptions{}).Download(ctx)

		if err == nil {
			t.Fatal("got nil error, want error")
		}
		if got != "" {
			t.Fatalf("got %q, want %q", got, "")
		}
		if !strings.Contains(err.Error(), "yt-dlp failed") {
			t.Fatalf("got error %q, want it to contain %q", err.Error(), "yt-dlp failed")
		}
	})
}
