package tests

import (
	"testing"
	"time"

	"github.com/maoshuhua/pavo-cli/internal/config"
)

func TestConfigUsesDefaults(t *testing.T) {
	t.Setenv(config.EnvAPIBaseURL, "")
	t.Setenv(config.EnvAccessToken, "")
	t.Setenv(config.EnvVerificationCode, "")
	t.Setenv(config.EnvHTTPTimeout, "")
	t.Setenv(config.EnvConfigFile, "")

	cfg := config.Load()
	if cfg.BaseURL != config.DefaultBaseURL {
		t.Fatalf("BaseURL = %q, want %q", cfg.BaseURL, config.DefaultBaseURL)
	}
	if cfg.BaseURL != "https://api.pavo-ai.cn" {
		t.Fatalf("BaseURL = %q, want production PAVO API", cfg.BaseURL)
	}
	if cfg.HTTPTimeout != config.DefaultHTTPTimeout {
		t.Fatalf("HTTPTimeout = %s, want %s", cfg.HTTPTimeout, config.DefaultHTTPTimeout)
	}
	if cfg.HTTPTimeout != 10*time.Minute {
		t.Fatalf("HTTPTimeout = %s, want 10m", cfg.HTTPTimeout)
	}
	if cfg.AccessToken != "" {
		t.Fatalf("AccessToken = %q, want empty", cfg.AccessToken)
	}
	if cfg.Paths.SendPhoneCode != config.SendPhoneCodePath ||
		cfg.Paths.PhoneOTPLogin != config.PhoneOTPLoginPath ||
		cfg.Paths.Conversation != config.ConversationPath ||
		cfg.Paths.Stream != config.StreamPath ||
		cfg.Paths.ResumeStream != config.ResumeStreamPath ||
		cfg.Paths.ConversationHistory != config.ConversationHistoryPath ||
		cfg.Paths.ConversationRunning != config.ConversationRunningPath ||
		cfg.Paths.Visuals != config.VisualsPath {
		t.Fatalf("Paths = %#v", cfg.Paths)
	}
}

func TestConfigReadsEnvironment(t *testing.T) {
	t.Setenv(config.EnvAPIBaseURL, " https://example.test/ ")
	t.Setenv(config.EnvAccessToken, " test-token ")
	t.Setenv(config.EnvVerificationCode, " 654321 ")
	t.Setenv(config.EnvHTTPTimeout, "45s")

	cfg := config.Load()
	if cfg.BaseURL != "https://example.test" {
		t.Fatalf("BaseURL = %q", cfg.BaseURL)
	}
	if cfg.AccessToken != "test-token" {
		t.Fatalf("AccessToken = %q", cfg.AccessToken)
	}
	if cfg.VerificationCode != " 654321 " {
		t.Fatal("VerificationCode must not be trimmed")
	}
	if cfg.HTTPTimeout.String() != "45s" {
		t.Fatalf("HTTPTimeout = %s", cfg.HTTPTimeout)
	}
}
