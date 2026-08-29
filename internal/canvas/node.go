package canvas

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/maoshuhua/pavo-cli/internal/api"
)

var supportedNodeTypes = map[string]bool{
	"text": true, "image": true, "video": true, "audio": true,
	"upload": true, "directorNode": true, "videoComposition": true, "group": true,
}

type NewNodeOptions struct {
	NodeKey   string
	Type      string
	Name      string
	X         *float64
	Y         *float64
	Width     float64
	Height    float64
	MediaType string
	Prompt    string
	Model     string
	Data      map[string]any
}

func DecodeObject(data json.RawMessage) (map[string]any, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return map[string]any{}, nil
	}
	if trimmed[0] == '"' {
		var encoded string
		if err := json.Unmarshal(trimmed, &encoded); err != nil {
			return nil, fmt.Errorf("解析节点 data 字符串失败: %w", err)
		}
		trimmed = []byte(encoded)
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.UseNumber()
	var result map[string]any
	if err := decoder.Decode(&result); err != nil {
		return nil, fmt.Errorf("节点 data 必须是 JSON object: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, errors.New("节点 data 必须只包含一个 JSON object")
		}
		return nil, fmt.Errorf("节点 data 包含无效的尾随内容: %w", err)
	}
	if result == nil {
		result = map[string]any{}
	}
	return result, nil
}

func ParseObject(text string) (map[string]any, error) {
	if strings.TrimSpace(text) == "" {
		return map[string]any{}, nil
	}
	return DecodeObject(json.RawMessage(text))
}

func MergeObject(target, patch map[string]any) map[string]any {
	if target == nil {
		target = map[string]any{}
	}
	for key, value := range patch {
		target[key] = value
	}
	return target
}

func EncodeObject(data map[string]any) (string, error) {
	encoded, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("编码节点 data 失败: %w", err)
	}
	return string(encoded), nil
}

func NodeData(node api.CanvasNode) (map[string]any, error) {
	return DecodeObject(node.Data)
}

func NodeMediaType(node api.CanvasNode) string {
	data, err := NodeData(node)
	if err != nil {
		return ""
	}
	value, _ := data["mediaType"].(string)
	return value
}

func FindNode(detail *api.CanvasProjectDetail, reference string) (*api.CanvasNode, error) {
	reference = strings.TrimSpace(reference)
	if reference == "" {
		return nil, errors.New("节点引用不能为空")
	}
	for index := range detail.NodeList {
		if detail.NodeList[index].NodeKey == reference {
			return &detail.NodeList[index], nil
		}
	}
	matches := make([]*api.CanvasNode, 0, 2)
	for index := range detail.NodeList {
		node := &detail.NodeList[index]
		name := strings.TrimSpace(node.Name)
		if data, err := NodeData(*node); err == nil {
			if title, ok := data["title"].(string); ok && strings.TrimSpace(title) != "" {
				name = strings.TrimSpace(title)
			}
		}
		if name == reference {
			matches = append(matches, node)
		}
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("找不到节点 %q；请使用 node_key 或精确名称", reference)
	case 1:
		return matches[0], nil
	default:
		keys := make([]string, 0, len(matches))
		for _, node := range matches {
			keys = append(keys, node.NodeKey)
		}
		return nil, fmt.Errorf("节点名称 %q 不唯一，请改用 node_key（候选: %s）", reference, strings.Join(keys, ", "))
	}
}

func defaultNodeName(nodeType string, detail *api.CanvasProjectDetail) string {
	base := map[string]string{
		"text": "文本节点", "image": "图片节点", "video": "视频节点", "audio": "音频节点",
		"upload": "上传节点", "directorNode": "导演台", "videoComposition": "视频合成", "group": "分组",
	}[nodeType]
	max := 0
	for _, node := range detail.NodeList {
		if string(node.NodeType) != nodeType {
			continue
		}
		data, _ := NodeData(node)
		title, _ := data["title"].(string)
		if title == "" {
			title = node.Name
		}
		if !strings.HasPrefix(title, base) {
			continue
		}
		value, err := strconv.Atoi(strings.TrimPrefix(title, base))
		if err == nil && value > max {
			max = value
		}
	}
	return base + strconv.Itoa(max+1)
}

func nextPosition(detail *api.CanvasProjectDetail) (float64, float64) {
	if len(detail.NodeList) == 0 {
		return 100, 80
	}
	maxX := 100.0
	for _, node := range detail.NodeList {
		value, err := strconv.ParseFloat(string(node.Position.PositionX), 64)
		if err == nil && value > maxX {
			maxX = value
		}
	}
	return maxX + 350, 80
}

func NewNode(detail *api.CanvasProjectDetail, options NewNodeOptions) (*api.CanvasBatchNodeWriteItem, error) {
	options.Type = strings.TrimSpace(options.Type)
	if !supportedNodeTypes[options.Type] {
		return nil, fmt.Errorf("--type 必须是 text、image、video、audio、upload、directorNode、videoComposition 或 group")
	}
	if options.Type == "upload" {
		if options.MediaType == "" {
			options.MediaType = "image"
		}
		if options.MediaType != "image" && options.MediaType != "video" && options.MediaType != "audio" {
			return nil, errors.New("--media-type 必须是 image、video 或 audio")
		}
	}
	key := strings.TrimSpace(options.NodeKey)
	if key == "" {
		var err error
		key, err = NewNodeKey(options.Type, options.MediaType)
		if err != nil {
			return nil, err
		}
	}
	name := strings.TrimSpace(options.Name)
	if name == "" {
		name = defaultNodeName(options.Type, detail)
	}
	x, y := nextPosition(detail)
	if options.X != nil {
		x = *options.X
	}
	if options.Y != nil {
		y = *options.Y
	}
	width := options.Width
	if width <= 0 {
		width = 280
	}
	height := options.Height
	if height <= 0 {
		height = 280
	}
	executable := options.Type == "text" || options.Type == "image" || options.Type == "video" || options.Type == "audio" || options.Type == "videoComposition"
	data := map[string]any{
		"node_key":     key,
		"title":        name,
		"name":         name,
		"isExecutable": executable,
	}
	if options.Type != "group" {
		data["progress"] = -1
		data["task_id"] = "-1"
	}
	if options.Type == "text" {
		data["mode"] = "default"
		data["content"] = ""
		data["params"] = map[string]any{"modeCode": "text_common"}
	}
	if options.Type == "upload" {
		data["mediaType"] = options.MediaType
	}
	if options.Type == "group" {
		data["manualBounds"] = false
	}
	MergeObject(data, options.Data)
	data["node_key"] = key
	if _, exists := data["title"]; !exists {
		data["title"] = name
	}
	if options.Prompt != "" {
		SetPrompt(data, options.Type, options.Prompt)
	}
	if options.Model != "" {
		SetModel(data, options.Model)
	}
	if title, ok := data["title"].(string); ok && strings.TrimSpace(title) != "" {
		name = strings.TrimSpace(title)
	}
	parentKey, _ := data["parent_key"].(string)
	item := &api.CanvasBatchNodeWriteItem{NodeKey: key, Type: options.Type, Name: name, ParentKey: strings.TrimSpace(parentKey)}
	item.Position.PositionX = formatNumber(x)
	item.Position.PositionY = formatNumber(y)
	item.Measured.Width = formatNumber(width)
	item.Measured.Height = formatNumber(height)
	encoded, err := EncodeObject(data)
	if err != nil {
		return nil, err
	}
	item.Data = encoded
	return item, nil
}

// IsNodeExecutable follows the generation backend contract. Older frontend
// records marked videoComposition false even though the backend can execute it.
func IsNodeExecutable(node api.CanvasNode) bool {
	if string(node.NodeType) == "videoComposition" {
		return true
	}
	data, err := NodeData(node)
	if err == nil {
		if executable, ok := data["isExecutable"].(bool); ok {
			return executable
		}
	}
	switch string(node.NodeType) {
	case "text", "image", "video", "audio":
		return true
	default:
		return false
	}
}

func SetPrompt(data map[string]any, nodeType, prompt string) {
	prompt = strings.TrimSpace(prompt)
	params := objectField(data, "params")
	if nodeType == "text" {
		data["content"] = prompt
		data["mode"] = "authoring"
		if _, exists := params["modeCode"]; !exists {
			params["modeCode"] = "text_common"
		}
	}
	data["params"] = params
	ReplaceTextPrompt(data, prompt)
}

func SetModel(data map[string]any, model string) {
	params := objectField(data, "params")
	params["model"] = strings.TrimSpace(model)
	data["params"] = params
}

func objectField(data map[string]any, key string) map[string]any {
	if value, ok := data[key].(map[string]any); ok {
		return value
	}
	return map[string]any{}
}

func formatNumber(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func WriteItemFromNode(node api.CanvasNode, data map[string]any) (api.CanvasBatchNodeWriteItem, error) {
	item := api.CanvasBatchNodeWriteItem{
		NodeKey:   node.NodeKey,
		Type:      string(node.NodeType),
		Name:      node.Name,
		ParentKey: node.ParentKey,
	}
	if title, ok := data["title"].(string); ok && strings.TrimSpace(title) != "" {
		item.Name = strings.TrimSpace(title)
	}
	if item.Name == "" {
		item.Name = "未命名节点"
	}
	item.Position.PositionX = firstString(string(node.Position.PositionX), "0")
	item.Position.PositionY = firstString(string(node.Position.PositionY), "0")
	item.Measured.Width = firstString(string(node.Measured.Width), "280")
	item.Measured.Height = firstString(string(node.Measured.Height), "280")
	var err error
	item.Data, err = EncodeObject(data)
	return item, err
}

func firstString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func StringArray(data map[string]any, key string) []string {
	value, ok := data[key]
	if !ok || value == nil {
		return nil
	}
	items, ok := value.([]any)
	if !ok {
		if text, textOK := value.(string); textOK && strings.TrimSpace(text) != "" {
			return []string{strings.TrimSpace(text)}
		}
		return nil
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
			result = append(result, strings.TrimSpace(text))
		}
	}
	return result
}

func AddUniqueString(data map[string]any, key, value string) {
	items := StringArray(data, key)
	for _, item := range items {
		if item == value {
			return
		}
	}
	items = append(items, value)
	values := make([]any, len(items))
	for index := range items {
		values[index] = items[index]
	}
	data[key] = values
}

func RemoveString(data map[string]any, key, value string) {
	items := StringArray(data, key)
	values := make([]any, 0, len(items))
	for _, item := range items {
		if item != value {
			values = append(values, item)
		}
	}
	if len(values) == 0 {
		delete(data, key)
		return
	}
	data[key] = values
}
