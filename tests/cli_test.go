package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/maoshuhua/pavo-cli/cmd"
	"github.com/maoshuhua/pavo-cli/internal/api"
	"github.com/maoshuhua/pavo-cli/internal/auth"
	"github.com/maoshuhua/pavo-cli/internal/config"
)

func TestCLIBusinessCommandsAreLimitedToProvidedCapabilities(t *testing.T) {
	t.Setenv(config.EnvConfigFile, filepath.Join(t.TempDir(), "config.json"))
	root, err := cmd.NewRootCommand(&bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"login", "conversation", "short-drama", "models", "visuals", "generate", "resume", "upload", "download-result", "update"} {
		command, _, findErr := root.Find([]string{name})
		if findErr != nil || command.Name() != name {
			t.Fatalf("missing command %q: command=%v err=%v", name, command, findErr)
		}
	}
	for _, removed := range []string{"stream", "generate-image", "generate-video", "get-thread"} {
		command, _, findErr := root.Find([]string{removed})
		if findErr == nil && command.Name() == removed {
			t.Fatalf("unexpected removed command %q", removed)
		}
	}
	conversation, _, err := root.Find([]string{"conversation"})
	if err != nil {
		t.Fatal(err)
	}
	for _, child := range conversation.Commands() {
		if child.Name() == "create" {
			t.Fatal("unexpected removed command \"conversation create\"")
		}
	}
	root.SetArgs([]string{"conversation", "create"})
	if err := root.Execute(); err == nil {
		t.Fatal("removed command \"conversation create\" unexpectedly succeeded")
	}
}

func TestCLIPhoneOTPLoginStoresTokenWithoutPrintingIt(t *testing.T) {
	const token = "header.eyJleHAiOjE3ODcxOTY4NzJ9.signature"
	const verificationCode = "654321"
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests = append(requests, request.URL.Path)
		if got := request.Header.Get("Authorization"); got != "" {
			t.Fatalf("login Authorization = %q", got)
		}
		switch request.URL.Path {
		case config.SendPhoneCodePath:
			var body api.SendPhoneCodeRequest
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.CountryCode != "86" || body.PhoneNumber != "13800138000" || body.Scene != "phone_auth" {
				t.Fatalf("send-code body = %#v", body)
			}
			_, _ = writer.Write([]byte(`{"code":"000000","message":"success","data":{}}`))
		case config.PhoneOTPLoginPath:
			var body api.PhoneOTPLoginRequest
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.CountryCode != "86" || body.PhoneNumber != "13800138000" || body.VerificationCode != verificationCode {
				t.Fatalf("login body = %#v", body)
			}
			_, _ = writer.Write([]byte(`{
				"code":"000000",
				"message":"success",
				"data":{
					"access_token":"` + token + `",
					"user_info":{"id":"user-1","phone_number":"13800138000","auth_provider":"phone","is_active":true}
				}
			}`))
		default:
			t.Fatalf("unexpected path %q", request.URL.Path)
		}
	}))
	defer server.Close()

	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv(config.EnvAPIBaseURL, server.URL)
	t.Setenv(config.EnvConfigFile, configPath)
	t.Setenv(config.EnvAccessToken, "")
	t.Setenv(config.EnvVerificationCode, "")

	var sendStdout bytes.Buffer
	root, err := cmd.NewRootCommand(&sendStdout, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{"login", "send-code", "--country-code", "+86", "--phone-number", "13800138000"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sendStdout.String(), `"verification_code_sent":true`) {
		t.Fatalf("send-code stdout = %q", sendStdout.String())
	}

	var stdout bytes.Buffer
	root, err = cmd.NewRootCommand(&stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	root.SetArgs([]string{"login", "--country-code", "86", "--phone-number", "13800138000", "--verification-code", verificationCode})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}

	session, err := auth.NewFileStore(configPath).Load()
	if err != nil {
		t.Fatal(err)
	}
	if session.AccessToken != token || session.User.ID != "user-1" {
		t.Fatalf("session = %#v", session)
	}
	if strings.Contains(stdout.String(), token) || strings.Contains(stdout.String(), verificationCode) {
		t.Fatalf("sensitive login value leaked to stdout: %s", stdout.String())
	}
	if len(requests) != 2 || requests[0] != config.SendPhoneCodePath || requests[1] != config.PhoneOTPLoginPath {
		t.Fatalf("requests = %#v", requests)
	}
}
