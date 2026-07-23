package tests

import (
	"encoding/base64"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/maoshuhua/pavo-cli/internal/auth"
)

func TestAuthFileStoreRoundTripAndClear(t *testing.T) {
	store := auth.NewFileStore(filepath.Join(t.TempDir(), "pavo", "config.json"))
	want := &auth.Session{
		AccessToken: "token",
		ExpiresAt:   123,
		User:        auth.User{ID: "user-1", Email: "u@example.com"},
	}
	if err := store.Save(want); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.AccessToken != want.AccessToken || got.User.Email != want.User.Email {
		t.Fatalf("Load() = %#v", got)
	}
	if err := store.Clear(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); !errors.Is(err, auth.ErrNotLoggedIn) {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestAuthResolveTokenPrefersEnvironment(t *testing.T) {
	store := auth.NewFileStore(filepath.Join(t.TempDir(), "config.json"))
	if err := store.Save(&auth.Session{AccessToken: "stored"}); err != nil {
		t.Fatal(err)
	}
	token, source, err := auth.ResolveToken(" environment ", store)
	if err != nil {
		t.Fatal(err)
	}
	if token != "environment" || source != "environment" {
		t.Fatalf("ResolveToken() = %q, %q", token, source)
	}
}

func TestAuthJWTExpiresAt(t *testing.T) {
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"exp":1787196872}`))
	token := fmt.Sprintf("header.%s.signature", payload)
	if got := auth.JWTExpiresAt(token); got != 1787196872 {
		t.Fatalf("JWTExpiresAt() = %d", got)
	}
}
