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

const telegramAPIBaseURL = "https://api.telegram.org"

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
	defer file.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if err := writer.WriteField("chat_id", strconv.FormatInt(chatID, 10)); err != nil {
		return fmt.Errorf("write Telegram multipart field chat_id: %w", err)
	}
	if replyToMessageID > 0 {
		if err := writer.WriteField("reply_to_message_id", strconv.FormatInt(replyToMessageID, 10)); err != nil {
			return fmt.Errorf("write Telegram multipart field reply_to_message_id: %w", err)
		}
	}

	part, err := writer.CreateFormFile(fieldName, filepath.Base(filePath))
	if err != nil {
		return fmt.Errorf("create Telegram multipart file field %s: %w", fieldName, err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return fmt.Errorf("copy %s file %q into request: %w", fieldName, filePath, err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("finalize Telegram multipart request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.methodURL(method), &body)
	if err != nil {
		return fmt.Errorf("build Telegram request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	return c.do(req, result)
}

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

func (c *TelegramAPIClient) methodURL(method string) string {
	return fmt.Sprintf("%s/bot%s/%s", c.baseURL, c.token, method)
}
