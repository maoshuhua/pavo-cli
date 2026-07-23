package output

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type errorLogEntry struct {
	Time    string `json:"time"`
	Command string `json:"command"`
	Error   string `json:"error"`
}

func AppendError(command string, err error) error {
	if err == nil {
		return nil
	}
	home, pathErr := os.UserHomeDir()
	if pathErr != nil {
		return pathErr
	}
	dir := filepath.Join(home, ".pavo", "logs")
	if mkErr := os.MkdirAll(dir, 0o700); mkErr != nil {
		return mkErr
	}
	entry := errorLogEntry{
		Time:    time.Now().Format(time.RFC3339),
		Command: strings.TrimSpace(command),
		Error:   err.Error(),
	}
	data, marshalErr := json.Marshal(entry)
	if marshalErr != nil {
		return marshalErr
	}
	data = append(data, '\n')
	file, openErr := os.OpenFile(
		filepath.Join(dir, time.Now().Format("2006-01-02")+".log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0o600,
	)
	if openErr != nil {
		return openErr
	}
	defer file.Close()
	_, writeErr := file.Write(data)
	return writeErr
}
