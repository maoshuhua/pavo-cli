package canvas

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/maoshuhua/pavo-cli/internal/api"
)

const maxNDJSONLineBytes = 4 * 1024 * 1024

var ndjsonAliasPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*$`)

type NDJSONOperation struct {
	Line             int             `json:"-"`
	Op               string          `json:"op"`
	Alias            string          `json:"as,omitempty"`
	Ref              string          `json:"ref,omitempty"`
	Type             string          `json:"type,omitempty"`
	Name             *string         `json:"name,omitempty"`
	Prompt           *string         `json:"prompt,omitempty"`
	Model            *string         `json:"model,omitempty"`
	MediaType        string          `json:"media_type,omitempty"`
	Data             json.RawMessage `json:"data,omitempty"`
	ReplaceData      bool            `json:"replace_data,omitempty"`
	Unset            []string        `json:"unset,omitempty"`
	X                *float64        `json:"x,omitempty"`
	Y                *float64        `json:"y,omitempty"`
	Width            *float64        `json:"width,omitempty"`
	Height           *float64        `json:"height,omitempty"`
	Parent           *string         `json:"parent,omitempty"`
	Source           string          `json:"source,omitempty"`
	Target           string          `json:"target,omitempty"`
	ID               string          `json:"id,omitempty"`
	SourceHandle     string          `json:"source_handle,omitempty"`
	TargetHandle     string          `json:"target_handle,omitempty"`
	SourcePortType   string          `json:"source_port_type,omitempty"`
	TargetPortType   string          `json:"target_port_type,omitempty"`
	Role             string          `json:"role,omitempty"`
	MediaOrder       int             `json:"media_order,omitempty"`
	ConnectionType   string          `json:"connection_type,omitempty"`
	ColorKey         string          `json:"color_key,omitempty"`
	Selectable       *bool           `json:"selectable,omitempty"`
	Deletable        *bool           `json:"deletable,omitempty"`
	Style            json.RawMessage `json:"style,omitempty"`
	Members          []string        `json:"members,omitempty"`
	ModeCode         string          `json:"mode_code,omitempty"`
	Border           string          `json:"border,omitempty"`
	Fill             string          `json:"fill,omitempty"`
	Padding          float64         `json:"padding,omitempty"`
	GeneratedNodeKey string          `json:"-"`
	GeneratedEdgeID  string          `json:"-"`
}

var ndjsonAllowedFields = map[string]map[string]bool{
	"node.create":   fields("op", "as", "type", "name", "prompt", "model", "media_type", "data", "x", "y", "width", "height", "parent"),
	"node.update":   fields("op", "ref", "name", "prompt", "model", "data", "replace_data", "unset", "x", "y", "width", "height", "parent"),
	"node.delete":   fields("op", "ref"),
	"edge.add":      fields("op", "as", "source", "target", "id", "source_handle", "target_handle", "source_port_type", "target_port_type", "role", "media_order", "connection_type", "color_key", "selectable", "deletable", "style"),
	"edge.delete":   fields("op", "id"),
	"group.create":  fields("op", "as", "members", "name", "mode_code", "border", "fill", "padding"),
	"group.ungroup": fields("op", "ref"),
}

func fields(values ...string) map[string]bool {
	result := map[string]bool{}
	for _, value := range values {
		result[value] = true
	}
	return result
}

func ParseNDJSON(reader io.Reader) ([]*NDJSONOperation, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), maxNDJSONLineBytes)
	operations := []*NDJSONOperation{}
	line := 0
	for scanner.Scan() {
		line++
		raw := bytes.TrimSpace(scanner.Bytes())
		if len(raw) == 0 {
			continue
		}
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		var object map[string]json.RawMessage
		if err := decoder.Decode(&object); err != nil {
			return nil, fmt.Errorf("NDJSON 第 %d 行不是有效 JSON object: %w", line, err)
		}
		var trailing any
		if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("NDJSON 第 %d 行包含多余 JSON 值", line)
		}
		opRaw, ok := object["op"]
		if !ok {
			return nil, fmt.Errorf("NDJSON 第 %d 行缺少 op", line)
		}
		var opName string
		if err := json.Unmarshal(opRaw, &opName); err != nil {
			return nil, fmt.Errorf("NDJSON 第 %d 行 op 必须是字符串", line)
		}
		opName = strings.TrimSpace(opName)
		allowed, ok := ndjsonAllowedFields[opName]
		if !ok {
			return nil, fmt.Errorf("NDJSON 第 %d 行不支持 op %q", line, opName)
		}
		for key := range object {
			if !allowed[key] {
				return nil, fmt.Errorf("NDJSON 第 %d 行 op %s 包含未知字段 %q", line, opName, key)
			}
		}
		var operation NDJSONOperation
		strict := json.NewDecoder(bytes.NewReader(raw))
		strict.UseNumber()
		strict.DisallowUnknownFields()
		if err := strict.Decode(&operation); err != nil {
			return nil, fmt.Errorf("NDJSON 第 %d 行 op %s 参数无效: %w", line, opName, err)
		}
		operation.Line = line
		operation.Op = opName
		if operation.Alias != "" && !ndjsonAliasPattern.MatchString(operation.Alias) {
			return nil, fmt.Errorf("NDJSON 第 %d 行 as %q 无效", line, operation.Alias)
		}
		operations = append(operations, &operation)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("读取 NDJSON 失败: %w", err)
	}
	if len(operations) == 0 {
		return nil, errors.New("stdin 中没有 NDJSON 操作")
	}
	return operations, nil
}

func NDJSONHasDestructiveOperations(operations []*NDJSONOperation) bool {
	for _, operation := range operations {
		switch operation.Op {
		case "node.delete", "edge.delete", "group.ungroup":
			return true
		}
	}
	return false
}

type NDJSONApplyResult struct {
	Request *api.CanvasBatchRequest `json:"request"`
	Aliases map[string]string       `json:"aliases"`
	Counts  map[string]int          `json:"counts"`
}

func ApplyNDJSON(detail *api.CanvasProjectDetail, operations []*NDJSONOperation) (*NDJSONApplyResult, error) {
	return ApplyNDJSONWithModelConfigurator(detail, operations, nil)
}

type NDJSONModelConfigurator func(nodeType, model string, data map[string]any) error

func ApplyNDJSONWithModelConfigurator(detail *api.CanvasProjectDetail, operations []*NDJSONOperation, configureModel NDJSONModelConfigurator) (*NDJSONApplyResult, error) {
	working, err := CloneDetail(detail)
	if err != nil {
		return nil, err
	}
	aliases := map[string]string{}
	edgeAliases := map[string]string{}
	counts := map[string]int{}
	resolve := func(reference string) (string, error) {
		reference = strings.TrimSpace(reference)
		if strings.HasPrefix(reference, "$") {
			value, ok := aliases[strings.TrimPrefix(reference, "$")]
			if !ok {
				return "", fmt.Errorf("别名 %q 尚未定义", reference)
			}
			return value, nil
		}
		return reference, nil
	}
	for _, operation := range operations {
		if err := applyNDJSONOperation(working, operation, aliases, edgeAliases, resolve, configureModel); err != nil {
			return nil, fmt.Errorf("NDJSON 第 %d 行 op %s: %w", operation.Line, operation.Op, err)
		}
		counts[operation.Op]++
	}
	if err := ValidateGraphStructure(working); err != nil {
		return nil, err
	}
	request, err := DiffDetails(detail, working)
	if err != nil {
		return nil, err
	}
	if BatchRequestEmpty(request) {
		return nil, errors.New("NDJSON 没有产生画布变更")
	}
	for alias, value := range edgeAliases {
		aliases[alias] = value
	}
	return &NDJSONApplyResult{Request: request, Aliases: aliases, Counts: counts}, nil
}

func applyNDJSONOperation(detail *api.CanvasProjectDetail, operation *NDJSONOperation, aliases, edgeAliases map[string]string, resolve func(string) (string, error), configureModel NDJSONModelConfigurator) error {
	switch operation.Op {
	case "node.create":
		if strings.TrimSpace(operation.Type) == "" {
			return errors.New("缺少 type")
		}
		if operation.Alias != "" {
			if _, exists := aliases[operation.Alias]; exists {
				return fmt.Errorf("别名 %q 重复", operation.Alias)
			}
			if _, exists := edgeAliases[operation.Alias]; exists {
				return fmt.Errorf("别名 %q 重复", operation.Alias)
			}
		}
		if operation.GeneratedNodeKey == "" {
			key, err := NewNodeKey(operation.Type, operation.MediaType)
			if err != nil {
				return err
			}
			operation.GeneratedNodeKey = key
		}
		data := map[string]any{}
		if len(bytes.TrimSpace(operation.Data)) > 0 {
			var err error
			data, err = DecodeObject(operation.Data)
			if err != nil {
				return err
			}
		}
		name, prompt, model := "", "", ""
		if operation.Name != nil {
			name = *operation.Name
		}
		if operation.Prompt != nil {
			prompt = *operation.Prompt
		}
		if operation.Model != nil {
			model = *operation.Model
			if configureModel != nil {
				if err := configureModel(operation.Type, model, data); err != nil {
					return err
				}
				model = ""
			}
		}
		width, height := 280.0, 280.0
		if operation.Width != nil {
			width = *operation.Width
		}
		if operation.Height != nil {
			height = *operation.Height
		}
		item, err := NewNode(detail, NewNodeOptions{NodeKey: operation.GeneratedNodeKey, Type: operation.Type, Name: name, Prompt: prompt, Model: model, MediaType: operation.MediaType, X: operation.X, Y: operation.Y, Width: width, Height: height, Data: data})
		if err != nil {
			return err
		}
		if operation.Parent != nil {
			parentKey, err := resolve(*operation.Parent)
			if err != nil {
				return err
			}
			if parentKey != "" {
				parent := nodeByKey(detail, parentKey)
				if parent == nil || string(parent.NodeType) != "group" {
					return fmt.Errorf("parent %q 不是已有 group", parentKey)
				}
			}
			item.ParentKey = parentKey
			nodeData, _ := DecodeObject(json.RawMessage(item.Data))
			if parentKey == "" {
				delete(nodeData, "parent_key")
			} else {
				nodeData["parent_key"] = parentKey
			}
			item.Data, _ = EncodeObject(nodeData)
		}
		detail.NodeList = append(detail.NodeList, NodeFromWriteItem(*item))
		if operation.Alias != "" {
			aliases[operation.Alias] = item.NodeKey
		}
	case "node.update":
		ref, err := resolve(operation.Ref)
		if err != nil {
			return err
		}
		node, err := FindNode(detail, ref)
		if err != nil {
			return err
		}
		data := map[string]any{}
		if !operation.ReplaceData {
			data, err = NodeData(*node)
			if err != nil {
				return err
			}
		}
		if len(bytes.TrimSpace(operation.Data)) > 0 {
			patch, patchErr := DecodeObject(operation.Data)
			if patchErr != nil {
				return patchErr
			}
			MergeObject(data, patch)
		}
		data["node_key"] = node.NodeKey
		if operation.Name != nil {
			value := strings.TrimSpace(*operation.Name)
			if value == "" {
				return errors.New("name 不能为空")
			}
			node.Name = value
			data["title"] = value
			data["name"] = value
		}
		if operation.Prompt != nil {
			SetPrompt(data, string(node.NodeType), *operation.Prompt)
		}
		if operation.Model != nil {
			if configureModel != nil {
				if err := configureModel(string(node.NodeType), *operation.Model, data); err != nil {
					return err
				}
			} else {
				SetModel(data, *operation.Model)
			}
		}
		for _, key := range operation.Unset {
			key = strings.TrimSpace(key)
			if key == "node_key" {
				return errors.New("不能 unset node_key")
			}
			delete(data, key)
		}
		encoded, _ := EncodeObject(data)
		node.Data = json.RawMessage(encoded)
		if operation.X != nil {
			node.Position.PositionX = api.ScalarString(formatNumber(*operation.X))
		}
		if operation.Y != nil {
			node.Position.PositionY = api.ScalarString(formatNumber(*operation.Y))
		}
		if operation.Width != nil {
			node.Measured.Width = api.ScalarString(formatNumber(*operation.Width))
		}
		if operation.Height != nil {
			node.Measured.Height = api.ScalarString(formatNumber(*operation.Height))
		}
		if operation.Parent != nil {
			parentKey, parentErr := resolve(*operation.Parent)
			if parentErr != nil {
				return parentErr
			}
			if parentKey != "" {
				parent := nodeByKey(detail, parentKey)
				if parent == nil || string(parent.NodeType) != "group" {
					return fmt.Errorf("parent %q 不是已有 group", parentKey)
				}
			}
			node.ParentKey = parentKey
			data, _ = NodeData(*node)
			if parentKey == "" {
				delete(data, "parent_key")
			} else {
				data["parent_key"] = parentKey
			}
			encoded, _ = EncodeObject(data)
			node.Data = json.RawMessage(encoded)
		}
	case "node.delete":
		ref, err := resolve(operation.Ref)
		if err != nil {
			return err
		}
		node, err := FindNode(detail, ref)
		if err != nil {
			return err
		}
		return DeleteNodeFromDetail(detail, node.NodeKey)
	case "edge.add":
		if operation.Alias != "" {
			if _, exists := edgeAliases[operation.Alias]; exists {
				return fmt.Errorf("连线别名 %q 重复", operation.Alias)
			}
			if _, exists := aliases[operation.Alias]; exists {
				return fmt.Errorf("别名 %q 重复", operation.Alias)
			}
		}
		source, err := resolve(operation.Source)
		if err != nil {
			return err
		}
		target, err := resolve(operation.Target)
		if err != nil {
			return err
		}
		if operation.GeneratedEdgeID == "" {
			operation.GeneratedEdgeID = strings.TrimSpace(operation.ID)
		}
		style := operation.Style
		if len(bytes.TrimSpace(style)) > 0 {
			var value map[string]any
			if err := json.Unmarshal(style, &value); err != nil {
				return fmt.Errorf("style 必须是 JSON object: %w", err)
			}
		}
		id, err := AddConnection(detail, source, target, EdgeOptions{ConnectionID: operation.GeneratedEdgeID, SourceHandle: operation.SourceHandle, TargetHandle: operation.TargetHandle, SourcePortType: operation.SourcePortType, TargetPortType: operation.TargetPortType, Role: operation.Role, MediaOrder: operation.MediaOrder, ConnectionType: operation.ConnectionType, ColorKey: operation.ColorKey, Selectable: operation.Selectable, Deletable: operation.Deletable, Style: style})
		if err != nil {
			return err
		}
		operation.GeneratedEdgeID = id
		if operation.Alias != "" {
			edgeAliases[operation.Alias] = id
		}
	case "edge.delete":
		id := strings.TrimSpace(operation.ID)
		if strings.HasPrefix(id, "$") {
			value, ok := edgeAliases[strings.TrimPrefix(id, "$")]
			if !ok {
				return fmt.Errorf("连线别名 %q 尚未定义", id)
			}
			id = value
		}
		return DeleteConnection(detail, id)
	case "group.create":
		if len(operation.Members) < 2 {
			return errors.New("members 至少包含两个节点")
		}
		members := make([]string, len(operation.Members))
		for index, ref := range operation.Members {
			resolved, err := resolve(ref)
			if err != nil {
				return err
			}
			members[index] = resolved
		}
		if operation.Alias != "" {
			if _, exists := aliases[operation.Alias]; exists {
				return fmt.Errorf("别名 %q 重复", operation.Alias)
			}
			if _, exists := edgeAliases[operation.Alias]; exists {
				return fmt.Errorf("别名 %q 重复", operation.Alias)
			}
		}
		if operation.GeneratedNodeKey == "" {
			key, err := NewNodeKey("group", "")
			if err != nil {
				return err
			}
			operation.GeneratedNodeKey = key
		}
		name := ""
		if operation.Name != nil {
			name = *operation.Name
		}
		key, _, err := GroupNodes(detail, members, GroupOptions{NodeKey: operation.GeneratedNodeKey, Name: name, ModeCode: operation.ModeCode, BorderColor: operation.Border, FillColor: operation.Fill, Padding: operation.Padding})
		if err != nil {
			return err
		}
		if operation.Alias != "" {
			aliases[operation.Alias] = key
		}
	case "group.ungroup":
		ref, err := resolve(operation.Ref)
		if err != nil {
			return err
		}
		_, _, err = UngroupNode(detail, ref)
		return err
	}
	return nil
}

func ValidateGraphStructure(detail *api.CanvasProjectDetail) error {
	nodes := map[string]api.CanvasNode{}
	for _, node := range detail.NodeList {
		if node.NodeKey == "" {
			return errors.New("画布包含空 node_key")
		}
		if _, exists := nodes[node.NodeKey]; exists {
			return fmt.Errorf("node_key %q 重复", node.NodeKey)
		}
		nodes[node.NodeKey] = node
	}
	connections := map[string]bool{}
	for _, connection := range detail.ConnectionList {
		if connections[connection.ConnectionID] {
			return fmt.Errorf("connection_id %q 重复", connection.ConnectionID)
		}
		connections[connection.ConnectionID] = true
		if _, ok := nodes[connection.SourceNodeKey]; !ok {
			return fmt.Errorf("连线 %s 的 source 不存在", connection.ConnectionID)
		}
		if _, ok := nodes[connection.TargetNodeKey]; !ok {
			return fmt.Errorf("连线 %s 的 target 不存在", connection.ConnectionID)
		}
	}
	for _, node := range detail.NodeList {
		if node.ParentKey == "" {
			continue
		}
		parent, ok := nodes[node.ParentKey]
		if !ok {
			return fmt.Errorf("节点 %s 的 parent_key %s 不存在", node.NodeKey, node.ParentKey)
		}
		if string(parent.NodeType) != "group" {
			return fmt.Errorf("节点 %s 的 parent_key %s 不是 group", node.NodeKey, node.ParentKey)
		}
		if _, _, err := AbsolutePosition(detail, node.NodeKey); err != nil {
			return err
		}
	}
	return nil
}
