package main

import (
	"errors"
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
			"test 1",
			"https://youtu.be/8v_kBIIGViY?si=l79VR-6K5Bo73Tt8",
			"https://youtu.be/8v_kBIIGViY?si=l79VR-6K5Bo73Tt8",
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
			errors.New(`invalid URL "ftp://youtu.be/dQw4w9WgXcQ": scheme must be http or https`),
		},
		{
			"invalid host",
			"https://vimeo.com/123456",
			"",
			errors.New(`invalid YouTube URL "https://vimeo.com/123456": host must be youtube.com, www.youtube.com, music.youtube.com, youtu.be or m.youtube.com`),
		},
		{
			"malformed url",
			"://youtube.com",
			"",
			errors.New(`invalid URL "://youtube.com"`),
		},
		{
			"empty url",
			"",
			"",
			errors.New(`invalid URL ""`),
		},
		{
			"subdomain not allowed",
			"https://gaming.youtube.com/watch?v=dQw4w9WgXcQ",
			"",
			errors.New(`invalid YouTube URL "https://gaming.youtube.com/watch?v=dQw4w9WgXcQ": host must be youtube.com, www.youtube.com, music.youtube.com, youtu.be or m.youtube.com`),
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
				startSecond: StartSecond,
				endSecond:   EndSecond,
				mediaKind:   MediaVideo,
				sourceURL:   "https://www.youtube.com/watch?v=8v_kBIIGViY",
			},
			nil,
		},
		{
			"audio download request",
			"https://www.youtube.com/watch?v=8v_kBIIGViY audio",
			DownloadRequest{
				startSecond: StartSecond,
				endSecond:   EndSecond,
				mediaKind:   MediaAudio,
				sourceURL:   "https://www.youtube.com/watch?v=8v_kBIIGViY",
			},
			nil,
		},
		{
			"video clip download request",
			"https://www.youtube.com/watch?v=8v_kBIIGViY 2:45-2:53",
			DownloadRequest{
				startSecond: 165,
				endSecond:   173,
				mediaKind:   MediaVideo,
				sourceURL:   "https://www.youtube.com/watch?v=8v_kBIIGViY",
			},
			nil,
		},
		{
			"audio clip download request",
			"https://www.youtube.com/watch?v=8v_kBIIGViY 2:45-2:53 audio",
			DownloadRequest{
				startSecond: 165,
				endSecond:   173,
				mediaKind:   MediaAudio,
				sourceURL:   "https://www.youtube.com/watch?v=8v_kBIIGViY",
			},
			nil,
		},
		{
			"empty download request",
			"",
			DownloadRequest{},
			errors.New(`invalid download request "": download request does not follow the format; expected URL [TIMESTAMP_RANGE] [audio]`),
		},
		{
			"invalid YouTube URL",
			"https://gaming.youtube.com/watch?v=dQw4w9WgXcQ",
			DownloadRequest{},
			errors.New(`invalid download request "https://gaming.youtube.com/watch?v=dQw4w9WgXcQ": invalid YouTube URL "https://gaming.youtube.com/watch?v=dQw4w9WgXcQ": host must be youtube.com, www.youtube.com, music.youtube.com, youtu.be or m.youtube.com; expected URL [TIMESTAMP_RANGE] [audio]`),
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
			if got.startSecond != tc.wantDownloadRequest.startSecond {
				t.Fatalf("got %d, want %d", got.startSecond, tc.wantDownloadRequest.startSecond)
			}
			if got.endSecond != tc.wantDownloadRequest.endSecond {
				t.Fatalf("got %d, want %d", got.endSecond, tc.wantDownloadRequest.endSecond)
			}
			if got.mediaKind != tc.wantDownloadRequest.mediaKind {
				t.Fatalf("got %d, want %d", got.mediaKind, tc.wantDownloadRequest.mediaKind)
			}
			if got.sourceURL != tc.wantDownloadRequest.sourceURL {
				t.Fatalf("got %q, want %q", got.sourceURL, tc.wantDownloadRequest.sourceURL)
			}
		})
	}
}
func TestSecondsToDownloadSections(t *testing.T) {

}
func TestMediaKind(t *testing.T) {

}
func TestBuildCommand(t *testing.T) {

}
func TestDownload(t *testing.T) {

}
