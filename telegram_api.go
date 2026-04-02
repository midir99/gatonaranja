package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// telegramAPIBaseURL is the base URL for Telegram Bot API HTTP requests.
const telegramAPIBaseURL = "https://api.telegram.org"

// telegramBotMaxUploadSizeBytes is the current Telegram Bot API upload limit
// enforced before attempting media uploads.
const telegramBotMaxUploadSizeBytes = 50 * 1024 * 1024

// ErrTelegramMediaTooLarge reports that a media file exceeds Telegram's bot
// upload size limit.
var ErrTelegramMediaTooLarge = errors.New("telegram media file is too large")

// TelegramAPIClient is a small stdlib-only client for the Telegram Bot API.
type TelegramAPIClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// TelegramAPIResponse is the envelope returned by the Telegram Bot API.
type TelegramAPIResponse[T any] struct {
	OK          bool   `json:"ok"`
	Result      T      `json:"result"`
	ErrorCode   int    `json:"error_code"`
	Description string `json:"description"`
}

// TelegramAPIUpdate represents a Telegram update returned by getUpdates.
type TelegramAPIUpdate struct {
	UpdateID int64               `json:"update_id"`
	Message  *TelegramAPIMessage `json:"message,omitempty"`
}

// TelegramAPIMessage represents a Telegram message returned by the Bot API.
type TelegramAPIMessage struct {
	MessageID int64            `json:"message_id"`
	From      *TelegramAPIUser `json:"from,omitempty"`
	Chat      TelegramAPIChat  `json:"chat"`
	Text      string           `json:"text,omitempty"`
}

// TelegramAPIUser represents a Telegram user.
type TelegramAPIUser struct {
	ID       int64  `json:"id"`
	UserName string `json:"username,omitempty"`
}

// TelegramAPIChat represents a Telegram chat.
type TelegramAPIChat struct {
	ID int64 `json:"id"`
}

// NewTelegramAPIClient creates a stdlib-only Telegram Bot API client.
func NewTelegramAPIClient(token string, httpClient *http.Client) (*TelegramAPIClient, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New("telegram bot token is required")
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &TelegramAPIClient{
		baseURL:    telegramAPIBaseURL,
		token:      token,
		httpClient: httpClient,
	}, nil
}

// ReceiveUpdates receives updates from Telegram using long polling.
func (c *TelegramAPIClient) ReceiveUpdates(
	ctx context.Context,
	offset int64,
	timeoutSeconds int,
) ([]TelegramAPIUpdate, error) {
	values := url.Values{}
	if offset > 0 {
		values.Set("offset", strconv.FormatInt(offset, 10))
	}
	if timeoutSeconds > 0 {
		values.Set("timeout", strconv.Itoa(timeoutSeconds))
	}

	var updates []TelegramAPIUpdate
	if err := c.get(ctx, "getUpdates", values, &updates); err != nil {
		return nil, err
	}
	return updates, nil
}

// GetMe returns information about the current bot user.
func (c *TelegramAPIClient) GetMe(ctx context.Context) (*TelegramAPIUser, error) {
	var user TelegramAPIUser
	if err := c.get(ctx, "getMe", nil, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

// SendText sends a text message to the given Telegram chat. If
// replyToMessageID != 0, the message is sent as a reply to that message.
func (c *TelegramAPIClient) SendText(
	ctx context.Context,
	chatID int64,
	replyToMessageID int64,
	text string,
) (*TelegramAPIMessage, error) {
	payload := map[string]any{
		"chat_id":             chatID,
		"text":                text,
		"reply_to_message_id": replyToMessageID,
	}
	var message TelegramAPIMessage
	if err := c.postJSON(ctx, "sendMessage", payload, &message); err != nil {
		return nil, err
	}
	return &message, nil
}

// SendVideo sends a video file to the given Telegram chat. If
// replyToMessageID != 0, the video is sent as a reply to that message.
func (c *TelegramAPIClient) SendVideo(
	ctx context.Context,
	chatID int64,
	replyToMessageID int64,
	videoPath string,
) (*TelegramAPIMessage, error) {
	var message TelegramAPIMessage
	if err := c.sendMedia(ctx, "sendVideo", "video", chatID, replyToMessageID, videoPath, &message); err != nil {
		return nil, err
	}
	return &message, nil
}

// SendAudio sends an audio file to the given Telegram chat. If
// replyToMessageID != 0, the audio is sent as a reply to that message.
func (c *TelegramAPIClient) SendAudio(
	ctx context.Context,
	chatID int64,
	replyToMessageID int64,
	audioPath string,
) (*TelegramAPIMessage, error) {
	var message TelegramAPIMessage
	if err := c.sendMedia(ctx, "sendAudio", "audio", chatID, replyToMessageID, audioPath, &message); err != nil {
		return nil, err
	}
	return &message, nil
}

// get sends a Telegram Bot API GET request for the given method and decodes the
// response into result.
func (c *TelegramAPIClient) get(
	ctx context.Context,
	method string,
	values url.Values,
	result any,
) error {
	endpoint := c.methodURL(method)
	if len(values) > 0 {
		endpoint += "?" + values.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("build Telegram request: %w", err)
	}
	return c.do(req, result)
}

// postJSON sends a Telegram Bot API POST request with a JSON payload and
// decodes the response into result.
func (c *TelegramAPIClient) postJSON(
	ctx context.Context,
	method string,
	payload any,
	result any,
) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal Telegram payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.methodURL(method), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build Telegram request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	return c.do(req, result)
}

// sendMedia uploads a local media file to Telegram using the given Bot API
// method and multipart field name.
func (c *TelegramAPIClient) sendMedia(
	ctx context.Context,
	method string,
	fieldName string,
	chatID int64,
	replyToMessageID int64,
	filePath string,
	result any,
) error {
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return fmt.Errorf("%s path is required", fieldName)
	}

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open %s file %q: %w", fieldName, filePath, err)
	}

	fileInfo, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return fmt.Errorf("stat %s file %q: %w", fieldName, filePath, err)
	}
	if fileInfo.Size() > telegramBotMaxUploadSizeBytes {
		_ = file.Close()
		return fmt.Errorf(
			"%w: %s file %q is too large: %d bytes; Telegram Bot API currently supports files up to %d bytes (50 MB)",
			ErrTelegramMediaTooLarge,
			fieldName,
			filePath,
			fileInfo.Size(),
			telegramBotMaxUploadSizeBytes,
		)
	}

	bodyReader, contentType, uploadErrCh := streamTelegramMediaBody(
		file,
		fieldName,
		filePath,
		chatID,
		replyToMessageID,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.methodURL(method), bodyReader)
	if err != nil {
		_ = bodyReader.Close()
		return fmt.Errorf("build Telegram request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)

	err = c.do(req, result)
	uploadErr := <-uploadErrCh
	if err != nil {
		if uploadErr != nil && !errors.Is(uploadErr, io.ErrClosedPipe) {
			return uploadErr
		}
		return err
	}
	if uploadErr != nil {
		return uploadErr
	}
	return nil
}

// streamTelegramMediaBody builds a multipart Telegram upload body and streams
// it through a pipe while the HTTP client consumes it.
func streamTelegramMediaBody(
	file *os.File,
	fieldName string,
	filePath string,
	chatID int64,
	replyToMessageID int64,
) (*io.PipeReader, string, <-chan error) {
	bodyReader, bodyWriter := io.Pipe()
	writer := multipart.NewWriter(bodyWriter)
	uploadErrCh := make(chan error, 1)

	go func() {
		defer close(uploadErrCh)
		defer file.Close()

		fail := func(err error) {
			uploadErrCh <- err
			_ = bodyWriter.CloseWithError(err)
		}

		if err := writer.WriteField("chat_id", strconv.FormatInt(chatID, 10)); err != nil {
			fail(fmt.Errorf("write Telegram multipart field chat_id: %w", err))
			return
		}
		if replyToMessageID > 0 {
			if err := writer.WriteField("reply_to_message_id", strconv.FormatInt(replyToMessageID, 10)); err != nil {
				fail(fmt.Errorf("write Telegram multipart field reply_to_message_id: %w", err))
				return
			}
		}

		part, err := writer.CreateFormFile(fieldName, filepath.Base(filePath))
		if err != nil {
			fail(fmt.Errorf("create Telegram multipart file field %s: %w", fieldName, err))
			return
		}
		if _, err := io.Copy(part, file); err != nil {
			fail(fmt.Errorf("copy %s file %q into request: %w", fieldName, filePath, err))
			return
		}
		if err := writer.Close(); err != nil {
			fail(fmt.Errorf("finalize Telegram multipart request: %w", err))
			return
		}
		uploadErrCh <- nil
		_ = bodyWriter.Close()
	}()

	return bodyReader, writer.FormDataContentType(), uploadErrCh
}

// do executes a Telegram Bot API request, validates the HTTP and API-level
// response, and decodes the result payload when requested.
func (c *TelegramAPIClient) do(req *http.Request, result any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send Telegram request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read Telegram response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var envelope TelegramAPIResponse[json.RawMessage]
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return fmt.Errorf("decode Telegram response envelope: %w", err)
	}

	if !envelope.OK {
		if envelope.Description != "" {
			return fmt.Errorf("telegram API error %d: %s", envelope.ErrorCode, envelope.Description)
		}
		return fmt.Errorf("telegram API error %d", envelope.ErrorCode)
	}

	if result == nil {
		return nil
	}
	if err := json.Unmarshal(envelope.Result, result); err != nil {
		return fmt.Errorf("decode Telegram response result: %w", err)
	}
	return nil
}

// methodURL builds the full Telegram Bot API endpoint URL for the given
// method name.
func (c *TelegramAPIClient) methodURL(method string) string {
	return fmt.Sprintf("%s/bot%s/%s", c.baseURL, c.token, method)
}
