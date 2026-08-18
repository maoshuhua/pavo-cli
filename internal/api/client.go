package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/maoshuhua/pavo-cli/internal/config"
)

type TokenProvider func() (string, error)

const (
	pavoPlatform   = "1"
	pavoUserAgent  = "PAVO-CLI/1.0"
	phoneAuthScene = "phone_auth"
	StreamBusyCode = "070301"
)

type Client struct {
	baseURL          string
	httpClient       *http.Client
	streamHTTPClient *http.Client
	tokenProvider    TokenProvider
	paths            *config.Paths
}

type APIError struct {
	Code       string
	Message    string
	HTTPStatus int
}

func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	switch {
	case e.HTTPStatus > 0 && e.Code != "":
		return fmt.Sprintf("PAVO API 返回 HTTP %d，code=%s: %s", e.HTTPStatus, e.Code, e.Message)
	case e.HTTPStatus > 0:
		return fmt.Sprintf("PAVO API 返回 HTTP %d: %s", e.HTTPStatus, e.Message)
	case e.Code != "":
		return fmt.Sprintf("PAVO API 返回 code=%s: %s", e.Code, e.Message)
	default:
		return e.Message
	}
}

func NewClient(baseURL string, timeout time.Duration, paths *config.Paths, tokenProvider TokenProvider) *Client {
	return &Client{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		httpClient: &http.Client{
			Timeout: timeout,
		},
		// A generation stream can legitimately outlive the request timeout used
		// for ordinary APIs. The service emits heartbeats and the caller can
		// reconnect via Resume when a transport error occurs.
		streamHTTPClient: &http.Client{},
		tokenProvider:    tokenProvider,
		paths:            paths,
	}
}

func (c *Client) SendPhoneVerificationCode(ctx context.Context, countryCode, phoneNumber string) error {
	countryCode = normalizeCountryCode(countryCode)
	phoneNumber = strings.TrimSpace(phoneNumber)
	if countryCode == "" {
		return errors.New("country_code 不能为空")
	}
	if phoneNumber == "" {
		return errors.New("phone_number 不能为空")
	}
	body := SendPhoneCodeRequest{
		CountryCode: countryCode,
		PhoneNumber: phoneNumber,
		Scene:       phoneAuthScene,
	}
	var response struct {
		Code    string         `json:"code"`
		Message string         `json:"message"`
		Data    map[string]any `json:"data"`
	}
	if err := c.doJSON(ctx, http.MethodPost, c.paths.SendPhoneCode, body, false, &response); err != nil {
		return err
	}
	return validateEnvelope(response.Code, response.Message)
}

func (c *Client) LoginWithPhoneOTP(ctx context.Context, countryCode, phoneNumber, verificationCode string) (*LoginResult, error) {
	countryCode = normalizeCountryCode(countryCode)
	phoneNumber = strings.TrimSpace(phoneNumber)
	verificationCode = strings.TrimSpace(verificationCode)
	if countryCode == "" {
		return nil, errors.New("country_code 不能为空")
	}
	if phoneNumber == "" {
		return nil, errors.New("phone_number 不能为空")
	}
	if verificationCode == "" {
		return nil, errors.New("verification_code 不能为空")
	}
	body := PhoneOTPLoginRequest{
		CountryCode:      countryCode,
		PhoneNumber:      phoneNumber,
		VerificationCode: verificationCode,
	}
	var response struct {
		Code    string    `json:"code"`
		Message string    `json:"message"`
		Data    LoginData `json:"data"`
	}
	if err := c.doJSON(ctx, http.MethodPost, c.paths.PhoneOTPLogin, body, false, &response); err != nil {
		return nil, err
	}
	if err := validateEnvelope(response.Code, response.Message); err != nil {
		return nil, err
	}
	response.Data.AccessToken = strings.TrimSpace(response.Data.AccessToken)
	if response.Data.AccessToken == "" {
		return nil, errors.New("登录响应缺少 data.access_token")
	}
	return &LoginResult{
		AccessToken: response.Data.AccessToken,
		UserInfo:    response.Data.UserInfo,
	}, nil
}

func normalizeCountryCode(countryCode string) string {
	return strings.TrimPrefix(strings.TrimSpace(countryCode), "+")
}

func (c *Client) CreateConversation(ctx context.Context, prompt string) (string, error) {
	title, err := conversationTitle(prompt)
	if err != nil {
		return "", err
	}
	body := CreateConversationRequest{
		Title:    title,
		FolderID: "",
		KBStrict: false,
	}
	var response struct {
		Code    string                 `json:"code"`
		Message string                 `json:"message"`
		Data    CreateConversationData `json:"data"`
	}
	if err := c.doJSON(ctx, http.MethodPost, c.paths.Conversation, body, true, &response); err != nil {
		return "", err
	}
	if err := validateEnvelope(response.Code, response.Message); err != nil {
		return "", err
	}
	conversationID := strings.TrimSpace(string(response.Data.ConversationID))
	if conversationID == "" {
		return "", errors.New("创建 conversation 响应缺少 data.conversation_id")
	}
	return conversationID, nil
}

// IsStreamBusy reports whether the service rejected a second stream
// submission because this conversation already has one in progress.
func IsStreamBusy(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Code == StreamBusyCode
}

// GetConversationStatus returns the lightweight Redis-backed running state for
// a conversation. Callers can use it to decide whether resume is appropriate.
func (c *Client) GetConversationStatus(ctx context.Context, conversationID string) (*ConversationStatus, error) {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return nil, errors.New("conversation_id 不能为空")
	}
	if c.paths == nil || strings.TrimSpace(c.paths.ConversationRunning) == "" {
		return nil, errors.New("PAVO API 未配置 conversation running 路径")
	}
	var response struct {
		Code    string             `json:"code"`
		Message string             `json:"message"`
		Data    ConversationStatus `json:"data"`
	}
	if err := c.getJSON(ctx, c.paths.ConversationRunning, url.Values{"conversation_id": {conversationID}}, &response); err != nil {
		return nil, err
	}
	if err := validateEnvelope(response.Code, response.Message); err != nil {
		return nil, err
	}
	return &response.Data, nil
}

// GetConversationHistory reads durable conversation data after the Redis stream
// replay window has expired.
func (c *Client) GetConversationHistory(ctx context.Context, conversationID string) (*ConversationHistory, error) {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return nil, errors.New("conversation_id 不能为空")
	}
	if c.paths == nil || strings.TrimSpace(c.paths.ConversationHistory) == "" {
		return nil, errors.New("PAVO API 未配置 conversation history 路径")
	}
	var response struct {
		Code    string              `json:"code"`
		Message string              `json:"message"`
		Data    ConversationHistory `json:"data"`
	}
	if err := c.getJSON(ctx, c.paths.ConversationHistory, url.Values{"conversation_id": {conversationID}}, &response); err != nil {
		return nil, err
	}
	if err := validateEnvelope(response.Code, response.Message); err != nil {
		return nil, err
	}
	return &response.Data, nil
}

func conversationTitle(prompt string) (string, error) {
	content := strings.TrimSpace(prompt)
	if content == "" {
		return "", errors.New("prompt 不能为空")
	}
	parts := []struct {
		Type    string `json:"type"`
		Content string `json:"content"`
	}{
		{Type: "text", Content: content},
	}
	data, err := json.Marshal(parts)
	if err != nil {
		return "", fmt.Errorf("编码 conversation title 失败: %w", err)
	}
	return string(data), nil
}

func (c *Client) doJSON(ctx context.Context, method, path string, body any, authenticated bool, out any) error {
	requestURL, err := c.resolveURL(path)
	if err != nil {
		return err
	}
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("编码请求体失败: %w", err)
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, requestURL, reader)
	if err != nil {
		return fmt.Errorf("构造 %s 请求失败: %w", method, err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	setPAVOHeaders(req)
	if authenticated {
		if err := c.authorize(req); err != nil {
			return err
		}
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s 请求失败: %w", method, requestURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		return responseError(resp)
	}
	if out == nil {
		_, err := io.Copy(io.Discard, resp.Body)
		return err
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("解析 API 响应失败: %w", err)
	}
	return nil
}

func (c *Client) getJSON(ctx context.Context, path string, query url.Values, out any) error {
	requestURL, err := c.resolveURL(path)
	if err != nil {
		return err
	}
	parsedURL, err := url.Parse(requestURL)
	if err != nil {
		return fmt.Errorf("解析 API URL 失败: %w", err)
	}
	parsedURL.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		return fmt.Errorf("构造 GET 请求失败: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	setPAVOHeaders(req)
	if err := c.authorize(req); err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s 请求失败: %w", parsedURL.String(), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		return responseError(resp)
	}
	if out == nil {
		_, err := io.Copy(io.Discard, resp.Body)
		return err
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("解析 API 响应失败: %w", err)
	}
	return nil
}

func setPAVOHeaders(req *http.Request) {
	req.Header.Set("User-Agent", pavoUserAgent)
	req.Header.Set("X-Platform", pavoPlatform)
}

func (c *Client) authorize(req *http.Request) error {
	if c.tokenProvider == nil {
		return errors.New("缺少 Token Provider")
	}
	token, err := c.tokenProvider()
	if err != nil {
		return err
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return errors.New("access token 为空，请先运行 pavo login")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return nil
}

func (c *Client) resolveURL(path string) (string, error) {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		if _, err := url.ParseRequestURI(path); err != nil {
			return "", fmt.Errorf("解析 API URL 失败: %w", err)
		}
		return path, nil
	}
	if c.baseURL == "" {
		return "", errors.New("PAVO API base URL 为空")
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return c.baseURL + path, nil
}

func validateEnvelope(code, message string) error {
	if code == SuccessCode {
		return nil
	}
	if strings.TrimSpace(code) == "" {
		return errors.New("API 响应缺少业务 code")
	}
	return &APIError{Code: code, Message: strings.TrimSpace(message)}
}

func responseError(resp *http.Response) error {
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	message := http.StatusText(resp.StatusCode)
	var envelope struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if json.Unmarshal(data, &envelope) == nil {
		if strings.TrimSpace(envelope.Message) != "" {
			message = strings.TrimSpace(envelope.Message)
		}
	}
	if message == "" {
		message = http.StatusText(resp.StatusCode)
	}
	return &APIError{
		Code:       strings.TrimSpace(envelope.Code),
		Message:    message,
		HTTPStatus: resp.StatusCode,
	}
}
