package canvas

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

type ToolSpecs struct {
	Version     string          `json:"version"`
	NodeConfigs ToolNodeConfigs `json:"nodeConfigs"`
	TextModes   []TextModeSpec  `json:"textModes"`
	TextModels  []TextModelSpec `json:"textModels"`
}

type ToolNodeConfigs struct {
	Guide map[string][]ToolConfig `json:"guide"`
	Skill map[string][]ToolConfig `json:"skill"`
	Mode  map[string][]ToolConfig `json:"mode"`
	Node  json.RawMessage         `json:"node,omitempty"`
}

type ToolConfig struct {
	Code      string          `json:"code"`
	Name      string          `json:"name"`
	NodeType  string          `json:"node_type"`
	IconURL   string          `json:"icon_url,omitempty"`
	SortOrder int             `json:"sort_order,omitempty"`
	Extra     ToolConfigExtra `json:"extra"`
}

type ToolConfigExtra struct {
	ConfigName     string                 `json:"config_name,omitempty"`
	ConfigPrompt   string                 `json:"config_prompt,omitempty"`
	SkillContent   string                 `json:"skill_content,omitempty"`
	SkillPrompt    string                 `json:"skill_prompt,omitempty"`
	NodeList       []ToolConfigNode       `json:"node_list,omitempty"`
	ConnectionList []ToolConfigConnection `json:"connection_list,omitempty"`
}

type ToolConfigNode struct {
	NodeKey    string             `json:"node_key"`
	ActionType string             `json:"action_type,omitempty"`
	InputType  string             `json:"input_type,omitempty"`
	NodeType   string             `json:"node_type"`
	NodeStatus string             `json:"node_status,omitempty"`
	Name       string             `json:"name,omitempty"`
	Content    string             `json:"content,omitempty"`
	Data       ToolConfigNodeData `json:"data"`
}

type ToolConfigNodeData struct {
	URL      string         `json:"url,omitempty"`
	Content  string         `json:"content,omitempty"`
	Params   map[string]any `json:"params,omitempty"`
	Settings map[string]any `json:"settings,omitempty"`
}

type ToolConfigConnection struct {
	Source        string `json:"source,omitempty"`
	Target        string `json:"target,omitempty"`
	SourceNodeKey string `json:"source_node_key,omitempty"`
	TargetNodeKey string `json:"target_node_key,omitempty"`
	Role          string `json:"role,omitempty"`
}

func (connection ToolConfigConnection) EffectiveSource() string {
	return firstString(strings.TrimSpace(connection.Source), strings.TrimSpace(connection.SourceNodeKey))
}

func (connection ToolConfigConnection) EffectiveTarget() string {
	return firstString(strings.TrimSpace(connection.Target), strings.TrimSpace(connection.TargetNodeKey))
}

type TextModeSpec struct {
	ModeCode   string   `json:"modeCode"`
	ModelCodes []string `json:"modelCodes"`
}

type TextModelSpec struct {
	ModelCode         string   `json:"modelCode"`
	ModelAlias        string   `json:"modelAlias"`
	IsOnline          *bool    `json:"isOnline,omitempty"`
	InputModalities   []string `json:"inputModalities,omitempty"`
	SubscriptionLevel float64  `json:"subscriptionLevel,omitempty"`
}

type Shortcut struct {
	Kind            string          `json:"kind"`
	Code            string          `json:"code"`
	Name            string          `json:"name"`
	NodeType        string          `json:"node_type"`
	CreatesOutput   bool            `json:"creates_output"`
	RequiredInputs  []ShortcutInput `json:"required_inputs"`
	ToolSpecVersion string          `json:"tool_spec_version,omitempty"`
	Config          ToolConfig      `json:"-"`
}

type ShortcutInput struct {
	Key        string `json:"key"`
	Name       string `json:"name,omitempty"`
	NodeType   string `json:"node_type,omitempty"`
	InputType  string `json:"input_type,omitempty"`
	HasExample bool   `json:"has_example"`
}

func ParseToolSpecs(raw json.RawMessage) (*ToolSpecs, error) {
	var specs ToolSpecs
	if err := json.Unmarshal(raw, &specs); err != nil {
		return nil, fmt.Errorf("解析 canvas tool-specs 失败: %w", err)
	}
	if specs.NodeConfigs.Guide == nil {
		specs.NodeConfigs.Guide = map[string][]ToolConfig{}
	}
	if specs.NodeConfigs.Skill == nil {
		specs.NodeConfigs.Skill = map[string][]ToolConfig{}
	}
	if specs.NodeConfigs.Mode == nil {
		specs.NodeConfigs.Mode = map[string][]ToolConfig{}
	}
	return &specs, nil
}

func (specs *ToolSpecs) Shortcuts() []Shortcut {
	if specs == nil {
		return []Shortcut{}
	}
	result := []Shortcut{}
	appendKind := func(kind string, configs map[string][]ToolConfig) {
		types := make([]string, 0, len(configs))
		for nodeType := range configs {
			types = append(types, nodeType)
		}
		sort.Strings(types)
		for _, nodeType := range types {
			for _, config := range configs[nodeType] {
				shortcut := Shortcut{Kind: kind, Code: strings.TrimSpace(config.Code), Name: strings.TrimSpace(config.Name), NodeType: firstString(strings.TrimSpace(config.NodeType), nodeType), ToolSpecVersion: specs.Version, Config: config}
				for _, node := range config.Extra.NodeList {
					if strings.TrimSpace(node.ActionType) == "output" {
						shortcut.CreatesOutput = true
					}
					if strings.TrimSpace(node.ActionType) == "input" {
						shortcut.RequiredInputs = append(shortcut.RequiredInputs, ShortcutInput{Key: node.NodeKey, Name: node.Name, NodeType: node.NodeType, InputType: node.InputType, HasExample: strings.TrimSpace(node.Data.URL) != ""})
					}
					if kind == "guide" && strings.TrimSpace(node.ActionType) == "" && strings.TrimSpace(node.NodeStatus) != "edit" {
						switch strings.TrimSpace(node.NodeType) {
						case "text", "image", "video", "audio":
							shortcut.CreatesOutput = true
						}
					}
				}
				if kind == "skill" {
					shortcut.CreatesOutput = true
					shortcut.RequiredInputs = []ShortcutInput{{Key: "source", Name: "Source", NodeType: nodeType}}
				}
				result = append(result, shortcut)
			}
		}
	}
	appendKind("guide", specs.NodeConfigs.Guide)
	appendKind("skill", specs.NodeConfigs.Skill)
	appendKind("mode", specs.NodeConfigs.Mode)
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Kind != result[j].Kind {
			return result[i].Kind < result[j].Kind
		}
		if result[i].NodeType != result[j].NodeType {
			return result[i].NodeType < result[j].NodeType
		}
		return result[i].Code < result[j].Code
	})
	return result
}

func (specs *ToolSpecs) FindShortcut(code string) (*Shortcut, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, errors.New("shortcut code 不能为空")
	}
	matches := []Shortcut{}
	for _, shortcut := range specs.Shortcuts() {
		if shortcut.Code == code {
			matches = append(matches, shortcut)
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("实时 tool-specs 中找不到 shortcut %q", code)
	}
	if len(matches) > 1 {
		kinds := make([]string, 0, len(matches))
		for _, match := range matches {
			kinds = append(kinds, match.Kind+":"+match.NodeType)
		}
		return nil, fmt.Errorf("shortcut code %q 不唯一（%s）", code, strings.Join(kinds, ", "))
	}
	return &matches[0], nil
}

func (specs *ToolSpecs) HasSkill(code, nodeType string) bool {
	if specs == nil {
		return false
	}
	for configuredType, configs := range specs.NodeConfigs.Skill {
		if nodeType != "" && configuredType != nodeType {
			continue
		}
		for _, config := range configs {
			if strings.TrimSpace(config.Code) == strings.TrimSpace(code) {
				return true
			}
		}
	}
	return false
}

func (specs *ToolSpecs) FindTextModel(code string) *TextModelSpec {
	if specs == nil {
		return nil
	}
	for index := range specs.TextModels {
		if strings.TrimSpace(specs.TextModels[index].ModelCode) == strings.TrimSpace(code) {
			return &specs.TextModels[index]
		}
	}
	return nil
}

func (specs *ToolSpecs) DefaultTextModel(modeCode string) (string, error) {
	if specs == nil {
		return "", errors.New("tool-specs 为空")
	}
	allowed := map[string]bool{}
	for _, mode := range specs.TextModes {
		if strings.TrimSpace(mode.ModeCode) == strings.TrimSpace(modeCode) {
			for _, code := range mode.ModelCodes {
				allowed[strings.TrimSpace(code)] = true
			}
		}
	}
	for _, model := range specs.TextModels {
		if len(allowed) > 0 && !allowed[strings.TrimSpace(model.ModelCode)] {
			continue
		}
		if model.IsOnline != nil && !*model.IsOnline {
			continue
		}
		if strings.TrimSpace(model.ModelCode) != "" {
			return strings.TrimSpace(model.ModelCode), nil
		}
	}
	return "", fmt.Errorf("text mode %q 没有在线模型", modeCode)
}
