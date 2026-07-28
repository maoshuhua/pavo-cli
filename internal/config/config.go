package config

import (
	"os"
	"strings"
	"time"
)

const (
	DefaultBaseURL     = "https://api-pixa-test.kiwiar.com"
	DefaultHTTPTimeout = 10 * time.Minute

	LoginPath               = "/api/v1/user/login"
	ConversationPath        = "/api/v1/chat/conversation"
	StreamPath              = "/api/v1/chat/stream"
	ResumeStreamPath        = "/api/v1/chat/stream/resume"
	ConversationHistoryPath = "/api/v1/chat/conversation/history"
	ConversationRunningPath = "/api/v1/chat/conversation/running"
	PresignedURLPath        = "/api/v1/file/presigned-url"

	EnvAPIBaseURL  = "PAVO_API_BASE_URL"
	EnvAccessToken = "PAVO_ACCESS_TOKEN"
	EnvPassword    = "PAVO_PASSWORD"
	EnvHTTPTimeout = "PAVO_HTTP_TIMEOUT"
	EnvConfigFile  = "PAVO_CONFIG_FILE"
)

type Config struct {
	BaseURL     string
	HTTPTimeout time.Duration
	AccessToken string
	Password    string
	ConfigFile  string
	Paths       *Paths
}

type Paths struct {
	Login               string
	Conversation        string
	Stream              string
	ResumeStream        string
	ConversationHistory string
	ConversationRunning string
	PresignedURL        string
}

func Load() *Config {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv(EnvAPIBaseURL)), "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Config{
		BaseURL:     baseURL,
		HTTPTimeout: resolveTimeout(os.Getenv(EnvHTTPTimeout)),
		AccessToken: strings.TrimSpace(os.Getenv(EnvAccessToken)),
		Password:    os.Getenv(EnvPassword),
		ConfigFile:  strings.TrimSpace(os.Getenv(EnvConfigFile)),
		Paths: &Paths{
			Login:               LoginPath,
			Conversation:        ConversationPath,
			Stream:              StreamPath,
			ResumeStream:        ResumeStreamPath,
			ConversationHistory: ConversationHistoryPath,
			ConversationRunning: ConversationRunningPath,
			PresignedURL:        PresignedURLPath,
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
