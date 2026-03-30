package main

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newTelegramAPIClientForTest(t *testing.T, fn roundTripFunc) *TelegramAPIClient {
	t.Helper()

	client, err := NewTelegramAPIClient("test-token", &http.Client{
		Transport: fn,
	})
	if err != nil {
		t.Fatalf("NewTelegramAPIClient() error = %v, want nil", err)
	}
	return client
}

func newHTTPResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestNewTelegramAPIClient(t *testing.T) {
	_, err := NewTelegramAPIClient("   ", nil)
	if err == nil {
		t.Fatal("NewTelegramAPIClient() error = nil, want non-nil")
	}
	if got, want := err.Error(), "telegram bot token is required"; got != want {
		t.Fatalf("NewTelegramAPIClient() error = %q, want %q", got, want)
	}
}

func TestTelegramAPIClientReceiveUpdates(t *testing.T) {
	client := newTelegramAPIClientForTest(t, func(req *http.Request) (*http.Response, error) {
		if got, want := req.Method, http.MethodGet; got != want {
			t.Fatalf("request method = %q, want %q", got, want)
		}
		if got, want := req.URL.Path, "/bottest-token/getUpdates"; got != want {
			t.Fatalf("request path = %q, want %q", got, want)
		}
		if got, want := req.URL.Query().Get("offset"), "42"; got != want {
			t.Fatalf("query offset = %q, want %q", got, want)
		}
		if got, want := req.URL.Query().Get("timeout"), "60"; got != want {
			t.Fatalf("query timeout = %q, want %q", got, want)
		}

		return newHTTPResponse(http.StatusOK, `{
			"ok": true,
			"result": [
				{
					"update_id": 1001,
					"message": {
						"message_id": 2002,
						"text": "hello",
						"chat": {"id": 3003},
						"from": {"id": 4004, "username": "arthur"}
					}
				}
			]
		}`), nil
	})

	updates, err := client.ReceiveUpdates(context.Background(), 42, 60)
	if err != nil {
		t.Fatalf("ReceiveUpdates() error = %v, want nil", err)
	}
	if got, want := len(updates), 1; got != want {
		t.Fatalf("len(updates) = %d, want %d", got, want)
	}
	if got, want := updates[0].UpdateID, int64(1001); got != want {
		t.Fatalf("updates[0].UpdateID = %d, want %d", got, want)
	}
	if updates[0].Message == nil {
		t.Fatal("updates[0].Message = nil, want non-nil")
	}
	if got, want := updates[0].Message.Text, "hello"; got != want {
		t.Fatalf("updates[0].Message.Text = %q, want %q", got, want)
	}
	if got, want := updates[0].Message.Chat.ID, int64(3003); got != want {
		t.Fatalf("updates[0].Message.Chat.ID = %d, want %d", got, want)
	}
	if updates[0].Message.From == nil {
		t.Fatal("updates[0].Message.From = nil, want non-nil")
	}
	if got, want := updates[0].Message.From.UserName, "arthur"; got != want {
		t.Fatalf("updates[0].Message.From.UserName = %q, want %q", got, want)
	}
}

func TestTelegramAPIClientGetMe(t *testing.T) {
	client := newTelegramAPIClientForTest(t, func(req *http.Request) (*http.Response, error) {
		if got, want := req.Method, http.MethodGet; got != want {
			t.Fatalf("request method = %q, want %q", got, want)
		}
		if got, want := req.URL.Path, "/bottest-token/getMe"; got != want {
			t.Fatalf("request path = %q, want %q", got, want)
		}
		if got := req.URL.RawQuery; got != "" {
			t.Fatalf("request raw query = %q, want empty string", got)
		}

		return newHTTPResponse(http.StatusOK, `{
			"ok": true,
			"result": {
				"id": 123456,
				"username": "gatonaranja_bot"
			}
		}`), nil
	})

	user, err := client.GetMe(context.Background())
	if err != nil {
		t.Fatalf("GetMe() error = %v, want nil", err)
	}
	if user == nil {
		t.Fatal("GetMe() user = nil, want non-nil")
	}
	if got, want := user.ID, int64(123456); got != want {
		t.Fatalf("user.ID = %d, want %d", got, want)
	}
	if got, want := user.UserName, "gatonaranja_bot"; got != want {
		t.Fatalf("user.UserName = %q, want %q", got, want)
	}
}

func TestTelegramAPIClientGetMeReturnsTelegramAPIError(t *testing.T) {
	client := newTelegramAPIClientForTest(t, func(req *http.Request) (*http.Response, error) {
		return newHTTPResponse(http.StatusOK, `{
			"ok": false,
			"error_code": 401,
			"description": "Unauthorized"
		}`), nil
	})

	_, err := client.GetMe(context.Background())
	if err == nil {
		t.Fatal("GetMe() error = nil, want non-nil")
	}
	if got, want := err.Error(), "telegram API error 401: Unauthorized"; got != want {
		t.Fatalf("GetMe() error = %q, want %q", got, want)
	}
}

func TestTelegramAPIClientSendText(t *testing.T) {
	client := newTelegramAPIClientForTest(t, func(req *http.Request) (*http.Response, error) {
		if got, want := req.Method, http.MethodPost; got != want {
			t.Fatalf("request method = %q, want %q", got, want)
		}
		if got, want := req.URL.Path, "/bottest-token/sendMessage"; got != want {
			t.Fatalf("request path = %q, want %q", got, want)
		}
		if got := req.Header.Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
			t.Fatalf("Content-Type = %q, want application/json", got)
		}

		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("reading request body: %v", err)
		}
		bodyString := string(body)
		for _, want := range []string{
			`"chat_id":12345`,
			`"reply_to_message_id":67890`,
			`"text":"hello there"`,
		} {
			if !strings.Contains(bodyString, want) {
				t.Fatalf("request body = %q, want substring %q", bodyString, want)
			}
		}

		return newHTTPResponse(http.StatusOK, `{
			"ok": true,
			"result": {
				"message_id": 999,
				"text": "hello there",
				"chat": {"id": 12345}
			}
		}`), nil
	})

	message, err := client.SendText(context.Background(), 12345, 67890, "hello there")
	if err != nil {
		t.Fatalf("SendText() error = %v, want nil", err)
	}
	if message == nil {
		t.Fatal("SendText() message = nil, want non-nil")
	}
	if got, want := message.MessageID, int64(999); got != want {
		t.Fatalf("message.MessageID = %d, want %d", got, want)
	}
	if got, want := message.Chat.ID, int64(12345); got != want {
		t.Fatalf("message.Chat.ID = %d, want %d", got, want)
	}
}

func TestTelegramAPIClientSendVideo(t *testing.T) {
	videoPath := filepath.Join(t.TempDir(), "clip.mp4")
	if err := os.WriteFile(videoPath, []byte("video-bytes"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v, want nil", err)
	}

	client := newTelegramAPIClientForTest(t, func(req *http.Request) (*http.Response, error) {
		if got, want := req.Method, http.MethodPost; got != want {
			t.Fatalf("request method = %q, want %q", got, want)
		}
		if got, want := req.URL.Path, "/bottest-token/sendVideo"; got != want {
			t.Fatalf("request path = %q, want %q", got, want)
		}
		if got := req.Header.Get("Content-Type"); !strings.HasPrefix(got, "multipart/form-data; boundary=") {
			t.Fatalf("Content-Type = %q, want multipart/form-data", got)
		}

		if err := req.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("ParseMultipartForm() error = %v, want nil", err)
		}
		if got, want := req.FormValue("chat_id"), "111"; got != want {
			t.Fatalf("chat_id = %q, want %q", got, want)
		}
		if got, want := req.FormValue("reply_to_message_id"), "222"; got != want {
			t.Fatalf("reply_to_message_id = %q, want %q", got, want)
		}

		file, header, err := req.FormFile("video")
		if err != nil {
			t.Fatalf("FormFile(video) error = %v, want nil", err)
		}
		defer file.Close()

		fileBytes, err := io.ReadAll(file)
		if err != nil {
			t.Fatalf("ReadAll(video) error = %v, want nil", err)
		}
		if got, want := string(fileBytes), "video-bytes"; got != want {
			t.Fatalf("video file contents = %q, want %q", got, want)
		}
		if got, want := header.Filename, "clip.mp4"; got != want {
			t.Fatalf("video filename = %q, want %q", got, want)
		}

		return newHTTPResponse(http.StatusOK, `{
			"ok": true,
			"result": {
				"message_id": 777,
				"chat": {"id": 111}
			}
		}`), nil
	})

	message, err := client.SendVideo(context.Background(), 111, 222, videoPath)
	if err != nil {
		t.Fatalf("SendVideo() error = %v, want nil", err)
	}
	if message == nil {
		t.Fatal("SendVideo() message = nil, want non-nil")
	}
	if got, want := message.MessageID, int64(777); got != want {
		t.Fatalf("message.MessageID = %d, want %d", got, want)
	}
}

func TestTelegramAPIClientSendAudio(t *testing.T) {
	audioPath := filepath.Join(t.TempDir(), "clip.mp3")
	if err := os.WriteFile(audioPath, []byte("audio-bytes"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v, want nil", err)
	}

	client := newTelegramAPIClientForTest(t, func(req *http.Request) (*http.Response, error) {
		if got, want := req.Method, http.MethodPost; got != want {
			t.Fatalf("request method = %q, want %q", got, want)
		}
		if got, want := req.URL.Path, "/bottest-token/sendAudio"; got != want {
			t.Fatalf("request path = %q, want %q", got, want)
		}

		if err := req.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("ParseMultipartForm() error = %v, want nil", err)
		}
		if got, want := req.FormValue("chat_id"), "555"; got != want {
			t.Fatalf("chat_id = %q, want %q", got, want)
		}
		if got := req.FormValue("reply_to_message_id"); got != "" {
			t.Fatalf("reply_to_message_id = %q, want empty string", got)
		}

		file, header, err := req.FormFile("audio")
		if err != nil {
			t.Fatalf("FormFile(audio) error = %v, want nil", err)
		}
		defer file.Close()

		fileBytes, err := io.ReadAll(file)
		if err != nil {
			t.Fatalf("ReadAll(audio) error = %v, want nil", err)
		}
		if got, want := string(fileBytes), "audio-bytes"; got != want {
			t.Fatalf("audio file contents = %q, want %q", got, want)
		}
		if got, want := header.Filename, "clip.mp3"; got != want {
			t.Fatalf("audio filename = %q, want %q", got, want)
		}

		return newHTTPResponse(http.StatusOK, `{
			"ok": true,
			"result": {
				"message_id": 888,
				"chat": {"id": 555}
			}
		}`), nil
	})

	message, err := client.SendAudio(context.Background(), 555, 0, audioPath)
	if err != nil {
		t.Fatalf("SendAudio() error = %v, want nil", err)
	}
	if message == nil {
		t.Fatal("SendAudio() message = nil, want non-nil")
	}
	if got, want := message.MessageID, int64(888); got != want {
		t.Fatalf("message.MessageID = %d, want %d", got, want)
	}
}

func TestTelegramAPIClientSendMediaRequiresFilePath(t *testing.T) {
	client := newTelegramAPIClientForTest(t, func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected HTTP request: %s %s", req.Method, req.URL.String())
		return nil, nil
	})

	_, err := client.SendVideo(context.Background(), 1, 2, "   ")
	if err == nil {
		t.Fatal("SendVideo() error = nil, want non-nil")
	}
	if got, want := err.Error(), "video path is required"; got != want {
		t.Fatalf("SendVideo() error = %q, want %q", got, want)
	}
}

func TestTelegramAPIClientSendMediaRejectsFilesLargerThanTelegramLimit(t *testing.T) {
	videoPath := filepath.Join(t.TempDir(), "too-large.mp4")

	file, err := os.Create(videoPath)
	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close() error = %v, want nil", err)
	}
	if err := os.Truncate(videoPath, telegramBotMaxUploadSizeBytes+1); err != nil {
		t.Fatalf("Truncate() error = %v, want nil", err)
	}

	client := newTelegramAPIClientForTest(t, func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected HTTP request: %s %s", req.Method, req.URL.String())
		return nil, nil
	})

	_, err = client.SendVideo(context.Background(), 1, 2, videoPath)
	if err == nil {
		t.Fatal("SendVideo() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), `video file "`) {
		t.Fatalf("SendVideo() error = %q, want it to mention the video file", err.Error())
	}
	if !strings.Contains(err.Error(), "50 MB") {
		t.Fatalf("SendVideo() error = %q, want it to mention the 50 MB limit", err.Error())
	}
}

func TestTelegramAPIClientReturnsTelegramAPIError(t *testing.T) {
	client := newTelegramAPIClientForTest(t, func(req *http.Request) (*http.Response, error) {
		return newHTTPResponse(http.StatusOK, `{
			"ok": false,
			"error_code": 400,
			"description": "Bad Request: chat not found"
		}`), nil
	})

	_, err := client.SendText(context.Background(), 1, 2, "hello")
	if err == nil {
		t.Fatal("SendText() error = nil, want non-nil")
	}
	if got, want := err.Error(), "telegram API error 400: Bad Request: chat not found"; got != want {
		t.Fatalf("SendText() error = %q, want %q", got, want)
	}
}

func TestTelegramAPIClientReturnsHTTPError(t *testing.T) {
	client := newTelegramAPIClientForTest(t, func(req *http.Request) (*http.Response, error) {
		return newHTTPResponse(http.StatusInternalServerError, "upstream exploded"), nil
	})

	_, err := client.ReceiveUpdates(context.Background(), 0, 0)
	if err == nil {
		t.Fatal("ReceiveUpdates() error = nil, want non-nil")
	}
	if got, want := err.Error(), "telegram API returned HTTP 500: upstream exploded"; got != want {
		t.Fatalf("ReceiveUpdates() error = %q, want %q", got, want)
	}
}
