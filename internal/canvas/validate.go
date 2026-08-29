package canvas

import (
	"fmt"
	"strings"

	"github.com/maoshuhua/pavo-cli/internal/api"
)

type ValidationIssue struct {
	Severity string `json:"severity"`
	Path     string `json:"path"`
	Message  string `json:"message"`
}

type NodeValidation struct {
	NodeKey  string            `json:"node_key"`
	NodeType string            `json:"node_type"`
	Name     string            `json:"name,omitempty"`
	Valid    bool              `json:"valid"`
	Issues   []ValidationIssue `json:"issues"`
}

type CanvasModelValidator func(nodeType, modelCode string) error

func ValidateCanvasNode(node api.CanvasNode, specs *ToolSpecs, validateModel CanvasModelValidator) NodeValidation {
	result := NodeValidation{NodeKey: node.NodeKey, NodeType: string(node.NodeType), Name: node.Name, Valid: true, Issues: []ValidationIssue{}}
	add := func(severity, path, message string) {
		result.Issues = append(result.Issues, ValidationIssue{Severity: severity, Path: path, Message: message})
		if severity == "error" {
			result.Valid = false
		}
	}
	data, err := NodeData(node)
	if err != nil {
		add("error", "data", err.Error())
		return result
	}
	params := objectField(data, "params")
	for index, segment := range PromptSegments(data) {
		path := fmt.Sprintf("data.params.prompt[%d]", index)
		kind, _ := segment["type"].(string)
		switch strings.TrimSpace(kind) {
		case "text":
			content, _ := segment["content"].(string)
			if strings.TrimSpace(content) == "" {
				add("error", path+".content", "text segment 不能为空")
			}
		case "skill":
			code, _ := segment["code"].(string)
			if strings.TrimSpace(code) == "" {
				add("error", path+".code", "skill code 不能为空")
			} else if specs != nil && !specs.HasSkill(code, string(node.NodeType)) {
				add("error", path+".code", fmt.Sprintf("实时 tool-specs 的 %s skill 中找不到 %q", node.NodeType, code))
			}
		case "image", "video", "audio":
			if url, _ := segment["url"].(string); strings.TrimSpace(url) == "" {
				add("error", path+".url", kind+" segment 缺少 url")
			}
		case "":
			add("error", path+".type", "不能为空")
		default:
			add("warning", path+".type", fmt.Sprintf("未知 segment type %q，已保留供前向兼容", kind))
		}
	}
	if IsNodeExecutable(node) {
		model, _ := params["model"].(string)
		if strings.TrimSpace(model) == "" {
			add("warning", "data.params.model", "可执行节点没有显式模型，将依赖服务端默认值")
		} else if string(node.NodeType) == "text" {
			if specs != nil {
				textModel := specs.FindTextModel(model)
				if textModel == nil {
					add("error", "data.params.model", fmt.Sprintf("实时 textModels 中找不到 %q", model))
				} else if textModel.IsOnline != nil && !*textModel.IsOnline {
					add("error", "data.params.model", fmt.Sprintf("文本模型 %q 当前未上线", model))
				}
			}
		} else if ModelScene(string(node.NodeType)) != "" && validateModel != nil {
			if err := validateModel(string(node.NodeType), model); err != nil {
				add("error", "data.params.model", err.Error())
			}
		}
	}
	if _, exists := data["pavo_storyboard"]; exists {
		storyboard, err := StoryboardFromNode(node)
		if err != nil {
			add("error", "data.pavo_storyboard", err.Error())
		} else {
			for _, issue := range storyboard.Validate() {
				add("error", "data.pavo_storyboard."+issue.Path, issue.Message)
			}
		}
	}
	return result
}
