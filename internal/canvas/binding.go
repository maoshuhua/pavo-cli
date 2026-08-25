package canvas

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	BindingSchemaVersion = 1
	BindingDirectory     = ".pavo"
	BindingFilename      = "canvas.json"
)

type Binding struct {
	SchemaVersion int    `json:"schema_version"`
	ProjectUUID   string `json:"project_uuid"`
	CanvasUUID    string `json:"canvas_uuid"`
	SessionID     string `json:"session_id"`
}

func (binding Binding) Validate() error {
	if binding.SchemaVersion != BindingSchemaVersion {
		return fmt.Errorf("不支持的画布绑定 schema_version %d", binding.SchemaVersion)
	}
	if strings.TrimSpace(binding.ProjectUUID) == "" {
		return errors.New("画布绑定缺少 project_uuid")
	}
	if strings.TrimSpace(binding.CanvasUUID) == "" {
		return errors.New("画布绑定缺少 canvas_uuid")
	}
	if strings.TrimSpace(binding.SessionID) == "" {
		return errors.New("画布绑定缺少 session_id")
	}
	return nil
}

func BindingPath(directory string) string {
	return filepath.Join(directory, BindingDirectory, BindingFilename)
}

// FindBinding searches the current directory and its parents. This lets a
// repository-level binding work from nested package directories.
func FindBinding(startDirectory string) (*Binding, string, error) {
	startDirectory = strings.TrimSpace(startDirectory)
	if startDirectory == "" {
		var err error
		startDirectory, err = os.Getwd()
		if err != nil {
			return nil, "", fmt.Errorf("获取当前目录失败: %w", err)
		}
	}
	current, err := filepath.Abs(startDirectory)
	if err != nil {
		return nil, "", fmt.Errorf("解析画布绑定起始目录失败: %w", err)
	}
	for {
		path := BindingPath(current)
		binding, readErr := ReadBinding(path)
		if readErr == nil {
			return binding, path, nil
		}
		if !errors.Is(readErr, os.ErrNotExist) {
			return nil, path, readErr
		}
		parent := filepath.Dir(current)
		if parent == current {
			return nil, "", os.ErrNotExist
		}
		current = parent
	}
}

func ReadBinding(path string) (*Binding, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var binding Binding
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&binding); err != nil {
		return nil, fmt.Errorf("解析画布绑定 %s 失败: %w", path, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, fmt.Errorf("解析画布绑定 %s 失败: 包含多余 JSON 值", path)
		}
		return nil, fmt.Errorf("解析画布绑定 %s 失败: %w", path, err)
	}
	if err := binding.Validate(); err != nil {
		return nil, fmt.Errorf("画布绑定 %s 无效: %w", path, err)
	}
	return &binding, nil
}

func WriteBinding(directory string, binding Binding) (string, error) {
	binding.SchemaVersion = BindingSchemaVersion
	binding.ProjectUUID = strings.TrimSpace(binding.ProjectUUID)
	binding.CanvasUUID = strings.TrimSpace(binding.CanvasUUID)
	binding.SessionID = strings.TrimSpace(binding.SessionID)
	if err := binding.Validate(); err != nil {
		return "", err
	}
	directory, err := filepath.Abs(directory)
	if err != nil {
		return "", fmt.Errorf("解析绑定目录失败: %w", err)
	}
	path := BindingPath(directory)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", fmt.Errorf("创建画布绑定目录失败: %w", err)
	}
	data, err := json.MarshalIndent(binding, "", "  ")
	if err != nil {
		return "", fmt.Errorf("编码画布绑定失败: %w", err)
	}
	data = append(data, '\n')
	temporary, err := os.CreateTemp(filepath.Dir(path), ".canvas-*.tmp")
	if err != nil {
		return "", fmt.Errorf("创建画布绑定临时文件失败: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return "", fmt.Errorf("设置画布绑定文件权限失败: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return "", fmt.Errorf("写入画布绑定失败: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return "", fmt.Errorf("关闭画布绑定临时文件失败: %w", err)
	}
	// Windows cannot rename over an existing file. The binding is not secret,
	// but write it atomically when possible and fall back to a direct rewrite.
	if err := os.Rename(temporaryPath, path); err != nil {
		if writeErr := os.WriteFile(path, data, 0o600); writeErr != nil {
			return "", fmt.Errorf("保存画布绑定失败: %w", writeErr)
		}
	}
	return path, nil
}

func RemoveBinding(path string) error {
	if strings.TrimSpace(path) == "" {
		return os.ErrNotExist
	}
	if err := os.Remove(path); err != nil {
		return err
	}
	return nil
}
