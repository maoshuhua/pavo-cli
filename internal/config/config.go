package config

import (
	"os"
	"strings"
	"time"
)

const (
	DefaultBaseURL     = "https://api.pavo-ai.cn"
	DefaultHTTPTimeout = 10 * time.Minute

	SendPhoneCodePath       = "/api/v1/user/code/send"
	PhoneOTPLoginPath       = "/api/v1/user/auth/phone-otp"
	ConversationPath        = "/api/v1/chat/conversation"
	StreamPath              = "/api/v1/chat/stream"
	ResumeStreamPath        = "/api/v1/chat/stream/resume"
	ConversationHistoryPath = "/api/v1/chat/conversation/history"
	ConversationRunningPath = "/api/v1/chat/conversation/running"
	PresignedURLPath        = "/api/v1/file/presigned-url"
	ModeSupportModelsPath   = "/api/v1/pixa/mode_support_models"

	EnvAPIBaseURL       = "PAVO_API_BASE_URL"
	EnvAccessToken      = "PAVO_ACCESS_TOKEN"
	EnvVerificationCode = "PAVO_VERIFICATION_CODE"
	EnvHTTPTimeout      = "PAVO_HTTP_TIMEOUT"
	EnvConfigFile       = "PAVO_CONFIG_FILE"
)

type Config struct {
	BaseURL          string
	HTTPTimeout      time.Duration
	AccessToken      string
	VerificationCode string
	ConfigFile       string
	Paths            *Paths
}

type Paths struct {
	SendPhoneCode       string
	PhoneOTPLogin       string
	Conversation        string
	Stream              string
	ResumeStream        string
	ConversationHistory string
	ConversationRunning string
	PresignedURL        string
	ModeSupportModels   string
}

func Load() *Config {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv(EnvAPIBaseURL)), "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Config{
		BaseURL:          baseURL,
		HTTPTimeout:      resolveTimeout(os.Getenv(EnvHTTPTimeout)),
		AccessToken:      strings.TrimSpace(os.Getenv(EnvAccessToken)),
		VerificationCode: os.Getenv(EnvVerificationCode),
		ConfigFile:       strings.TrimSpace(os.Getenv(EnvConfigFile)),
		Paths: &Paths{
			SendPhoneCode:       SendPhoneCodePath,
			PhoneOTPLogin:       PhoneOTPLoginPath,
			Conversation:        ConversationPath,
			Stream:              StreamPath,
			ResumeStream:        ResumeStreamPath,
			ConversationHistory: ConversationHistoryPath,
			ConversationRunning: ConversationRunningPath,
			PresignedURL:        PresignedURLPath,
			ModeSupportModels:   ModeSupportModelsPath,
		},
	}
}

func resolveTimeout(raw string) time.Duration {
	value := strings.TrimSpace(raw)
	if value == "" {
		return DefaultHTTPTimeout
	}
	timeout, err := time.ParseDuration(value)
	if err != nil || timeout <= 0 {
		return DefaultHTTPTimeout
	}
	return timeout
}
