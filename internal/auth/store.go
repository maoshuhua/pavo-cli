package auth

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var ErrNotLoggedIn = errors.New("尚未登录，请先运行 pavo login")

type User struct {
	ID           string `json:"id,omitempty"`
	Username     string `json:"username,omitempty"`
	AvatarURL    string `json:"avatar_url,omitempty"`
	Email        string `json:"email,omitempty"`
	CountryCode  string `json:"country_code,omitempty"`
	PhoneNumber  string `json:"phone_number,omitempty"`
	AuthProvider string `json:"auth_provider,omitempty"`
	AppID        string `json:"app_id,omitempty"`
	IsActive     bool   `json:"is_active"`
	IsNewAccount bool   `json:"is_new_account"`
	CreatedAt    int64  `json:"created_at,omitempty"`
}

type Session struct {
	AccessToken string `json:"access_token"`
	ExpiresAt   int64  `json:"expires_at,omitempty"`
	User        User   `json:"user_info"`
}

type Store interface {
	Load() (*Session, error)
	Save(*Session) error
	Clear() error
	Path() string
}

type FileStore struct {
	path string
}

func NewFileStore(path string) *FileStore {
	return &FileStore{path: filepath.Clean(path)}
}

func NewDefaultFileStore(override string) (*FileStore, error) {
	path := strings.TrimSpace(override)
	if path == "" {
		var err error
		path, err = defaultPath()
		if err != nil {
			return nil, err
		}
	}
	return NewFileStore(path), nil
}

func (s *FileStore) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

func (s *FileStore) Load() (*Session, error) {
	if s == nil || strings.TrimSpace(s.path) == "" {
		return nil, ErrNotLoggedIn
	}
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotLoggedIn
	}
	if err != nil {
		return nil, fmt.Errorf("读取登录信息失败: %w", err)
	}
	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("解析登录信息失败: %w", err)
	}
	session.AccessToken = strings.TrimSpace(session.AccessToken)
	if session.AccessToken == "" {
		return nil, ErrNotLoggedIn
	}
	return &session, nil
}

func (s *FileStore) Save(session *Session) error {
	if s == nil || strings.TrimSpace(s.path) == "" {
		return errors.New("登录信息存储路径为空")
	}
	if session == nil || strings.TrimSpace(session.AccessToken) == "" {
		return errors.New("拒绝保存空 access token")
	}
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("编码登录信息失败: %w", err)
	}
	data = append(data, '\n')
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("创建登录信息目录失败: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".session-*")
	if err != nil {
		return fmt.Errorf("创建临时登录文件失败: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("设置登录文件权限失败: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("写入登录信息失败: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("关闭登录信息文件失败: %w", err)
	}
	if runtime.GOOS == "windows" {
		_ = os.Remove(s.path)
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		return fmt.Errorf("保存登录信息失败: %w", err)
	}
	return nil
}

func (s *FileStore) Clear() error {
	if s == nil || strings.TrimSpace(s.path) == "" {
		return nil
	}
	if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("删除登录信息失败: %w", err)
	}
	return nil
}

func ResolveToken(environmentToken string, store Store) (token string, source string, err error) {
	if token := strings.TrimSpace(environmentToken); token != "" {
		return token, "environment", nil
	}
	if store == nil {
		return "", "", ErrNotLoggedIn
	}
	session, err := store.Load()
	if err != nil {
		return "", "", err
	}
	return session.AccessToken, "stored", nil
}

func JWTExpiresAt(token string) int64 {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 {
		return 0
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return 0
	}
	var claims struct {
		ExpiresAt int64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return 0
	}
	return claims.ExpiresAt
}

func defaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("定位系统配置目录失败: %w", err)
	}
	return filepath.Join(dir, "pavo", "config.json"), nil
}
