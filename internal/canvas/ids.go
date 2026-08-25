package canvas

import (
	"crypto/rand"
	"fmt"
	"strings"
)

const randomIDCharacters = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_-"

func RandomUUID() (string, error) {
	data := make([]byte, 16)
	if _, err := rand.Read(data); err != nil {
		return "", fmt.Errorf("生成 UUID 失败: %w", err)
	}
	data[6] = (data[6] & 0x0f) | 0x40
	data[8] = (data[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		data[0:4], data[4:6], data[6:8], data[8:10], data[10:16]), nil
}

func RequestID() (string, error) {
	id, err := RandomUUID()
	if err != nil {
		return "", err
	}
	return "req-" + id, nil
}

func nodePrefix(nodeType, mediaType string) (string, error) {
	switch strings.TrimSpace(nodeType) {
	case "text":
		return "t", nil
	case "image":
		return "i", nil
	case "video":
		return "v", nil
	case "audio":
		return "a", nil
	case "directorNode", "videoComposition":
		return "c", nil
	case "group":
		return "g", nil
	case "upload":
		switch strings.TrimSpace(mediaType) {
		case "", "image":
			return "i", nil
		case "video":
			return "v", nil
		case "audio":
			return "a", nil
		default:
			return "", fmt.Errorf("上传节点 mediaType 必须是 image、video 或 audio")
		}
	default:
		return "", fmt.Errorf("不支持的画布节点类型 %q", nodeType)
	}
}

func randomSuffix(length int) (string, error) {
	data := make([]byte, length)
	if _, err := rand.Read(data); err != nil {
		return "", fmt.Errorf("生成随机标识失败: %w", err)
	}
	result := make([]byte, length)
	for index, value := range data {
		result[index] = randomIDCharacters[int(value)%len(randomIDCharacters)]
	}
	return string(result), nil
}

func NewNodeKey(nodeType, mediaType string) (string, error) {
	prefix, err := nodePrefix(nodeType, mediaType)
	if err != nil {
		return "", err
	}
	suffix, err := randomSuffix(12)
	if err != nil {
		return "", err
	}
	return prefix + "-" + suffix, nil
}

func NewConnectionID(sourceType, sourceMediaType, targetType, targetMediaType string) (string, error) {
	sourcePrefix, err := nodePrefix(sourceType, sourceMediaType)
	if err != nil {
		return "", err
	}
	targetPrefix, err := nodePrefix(targetType, targetMediaType)
	if err != nil {
		return "", err
	}
	suffix, err := randomSuffix(3)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("e-%s-%s-%s", sourcePrefix, targetPrefix, suffix), nil
}
