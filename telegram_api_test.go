package main

import (
	"context"
	"errors"
	"io"
	"mime"
	"mime/multipart"
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

type errReadCloser struct {
	err error
}

func (r errReadCloser) Read(_ []byte) (int, error) {
	return 0, r.err
}

func (r errReadCloser) Close() error {
	return nil
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

func readMultipartRequest(
	t *testing.T,
	req *http.Request,
) (map[string]string, string, string, string) {
	t.Helper()

	mediaType, params, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
	if err != nil {
		t.Fatalf("ParseMediaType() error = %v, want nil", err)
	}
	if got, want := mediaType, "multipart/form-data"; got != want {
		t.Fatalf("media type = %q, want %q", got, want)
	}

	reader := multipart.NewReader(req.Body, params["boundary"])
	fields := make(map[string]string)
	var fileFieldName string
	var fileName string
	var fileContents string

	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("NextPart() error = %v, want nil", err)
		}

		partBytes, err := io.ReadAll(part)
		if err != nil {
			t.Fatalf("ReadAll(part %q) error = %v, want nil", part.FormName(), err)
		}

		if part.FileName() == "" {
			fields[part.FormName()] = string(partBytes)
			continue
		}

		fileFieldName = part.FormName()
		fileName = part.FileName()
		fileContents = string(partBytes)
	}

	return fields, fileFieldName, fileName, fileContents
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

func TestNewTelegramAPIClientUsesDefaultHTTPClient(t *testing.T) {
	client, err := NewTelegramAPIClient("test-token", nil)
	if err != nil {
		t.Fatalf("NewTelegramAPIClient() error = %v, want nil", err)
	}
	if got, want := client.httpClient, http.DefaultClient; got != want {
		t.Fatalf("client.httpClient = %p, want %p", got, want)
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

func TestTelegramAPIClientReceiveUpdatesWithoutOptionalQueryParams(t *testing.T) {
	client := newTelegramAPIClientForTest(t, func(req *http.Request) (*http.Response, error) {
		if got, want := req.Method, http.MethodGet; got != want {
			t.Fatalf("request method = %q, want %q", got, want)
		}
		if got, want := req.URL.Path, "/bottest-token/getUpdates"; got != want {
			t.Fatalf("request path = %q, want %q", got, want)
		}
		if got := req.URL.RawQuery; got != "" {
			t.Fatalf("raw query = %q, want empty string", got)
		}

		return newHTTPResponse(http.StatusOK, `{
			"ok": true,
			"result": []
		}`), nil
	})

	updates, err := client.ReceiveUpdates(context.Background(), 0, 0)
	if err != nil {
		t.Fatalf("ReceiveUpdates() error = %v, want nil", err)
	}
	if got, want := len(updates), 0; got != want {
		t.Fatalf("len(updates) = %d, want %d", got, want)
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

func TestTelegramAPIClientGetMeReturnsHTTPError(t *testing.T) {
	client := newTelegramAPIClientForTest(t, func(req *http.Request) (*http.Response, error) {
		return newHTTPResponse(http.StatusBadGateway, "upstream gateway error"), nil
	})

	_, err := client.GetMe(context.Background())
	if err == nil {
		t.Fatal("GetMe() error = nil, want non-nil")
	}
	if got, want := err.Error(), "telegram API returned HTTP 502: upstream gateway error"; got != want {
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

		fields, fileFieldName, fileName, fileContents := readMultipartRequest(t, req)
		if got, want := fields["chat_id"], "111"; got != want {
			t.Fatalf("chat_id = %q, want %q", got, want)
		}
		if got, want := fields["reply_to_message_id"], "222"; got != want {
			t.Fatalf("reply_to_message_id = %q, want %q", got, want)
		}
		if got, want := fileFieldName, "video"; got != want {
			t.Fatalf("file field name = %q, want %q", got, want)
		}
		if got, want := fileName, "clip.mp4"; got != want {
			t.Fatalf("video filename = %q, want %q", got, want)
		}
		if got, want := fileContents, "video-bytes"; got != want {
			t.Fatalf("video file contents = %q, want %q", got, want)
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

		fields, fileFieldName, fileName, fileContents := readMultipartRequest(t, req)
		if got, want := fields["chat_id"], "555"; got != want {
			t.Fatalf("chat_id = %q, want %q", got, want)
		}
		if got := fields["reply_to_message_id"]; got != "" {
			t.Fatalf("reply_to_message_id = %q, want empty string", got)
		}
		if got, want := fileFieldName, "audio"; got != want {
			t.Fatalf("file field name = %q, want %q", got, want)
		}
		if got, want := fileName, "clip.mp3"; got != want {
			t.Fatalf("audio filename = %q, want %q", got, want)
		}
		if got, want := fileContents, "audio-bytes"; got != want {
			t.Fatalf("audio file contents = %q, want %q", got, want)
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

func TestTelegramAPIClientSendAudioReturnsOpenFileError(t *testing.T) {
	client := newTelegramAPIClientForTest(t, func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected HTTP request: %s %s", req.Method, req.URL.String())
		return nil, nil
	})

	_, err := client.SendAudio(context.Background(), 1, 2, "/path/that/does/not/exist.mp3")
	if err == nil {
		t.Fatal("SendAudio() error = nil, want non-nil")
	}
	if got := err.Error(); !strings.Contains(got, `open audio file "/path/that/does/not/exist.mp3"`) {
		t.Fatalf("SendAudio() error = %q, want it to mention the audio file open failure", got)
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

func TestTelegramAPIClientPostJSONReturnsMarshalError(t *testing.T) {
	client := newTelegramAPIClientForTest(t, func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected HTTP request: %s %s", req.Method, req.URL.String())
		return nil, nil
	})

	err := client.postJSON(
		context.Background(),
		"sendMessage",
		map[string]any{"bad": func() {}},
		nil,
	)
	if err == nil {
		t.Fatal("postJSON() error = nil, want non-nil")
	}
	if got := err.Error(); !strings.Contains(got, "marshal Telegram payload: json: unsupported type: func()") {
		t.Fatalf("postJSON() error = %q, want it to mention marshal failure", got)
	}
}

func TestTelegramAPIClientDoWithNilResult(t *testing.T) {
	client := newTelegramAPIClientForTest(t, func(req *http.Request) (*http.Response, error) {
		return newHTTPResponse(http.StatusOK, `{
			"ok": true,
			"result": {
				"ignored": true
			}
		}`), nil
	})

	req, err := http.NewRequest(http.MethodGet, client.methodURL("getMe"), nil)
	if err != nil {
		t.Fatalf("http.NewRequest() error = %v, want nil", err)
	}

	err = client.do(req, nil)
	if err != nil {
		t.Fatalf("do() error = %v, want nil", err)
	}
}

func TestTelegramAPIClientDoReturnsReadResponseError(t *testing.T) {
	wantErr := errors.New("read exploded")
	client := newTelegramAPIClientForTest(t, func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       errReadCloser{err: wantErr},
		}, nil
	})

	req, err := http.NewRequest(http.MethodGet, client.methodURL("getMe"), nil)
	if err != nil {
		t.Fatalf("http.NewRequest() error = %v, want nil", err)
	}

	err = client.do(req, &TelegramAPIUser{})
	if err == nil {
		t.Fatal("do() error = nil, want non-nil")
	}
	if got, want := err.Error(), "read Telegram response: read exploded"; got != want {
		t.Fatalf("do() error = %q, want %q", got, want)
	}
}
