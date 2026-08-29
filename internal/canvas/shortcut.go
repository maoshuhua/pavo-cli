package canvas

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type ShortcutApplyOptions struct {
	Source          string
	Target          string
	Name            string
	Prompt          string
	Model           string
	Inputs          map[string]string
	UseExampleInput bool
}

type ShortcutModelResolver func(nodeType, configuredModel string) (string, error)

type ShortcutPlan struct {
	Shortcut   Shortcut           `json:"shortcut"`
	Operations []*NDJSONOperation `json:"operations"`
	RunRef     string             `json:"run_ref,omitempty"`
}

func toolNodeData(node ToolConfigNode, shortcut Shortcut, promptOverride string) (map[string]any, string, error) {
	data := map[string]any{
		"pavo_shortcut": map[string]any{
			"code": shortcut.Code, "kind": shortcut.Kind, "tool_spec_version": shortcut.ToolSpecVersion,
		},
	}
	params := map[string]any{}
	configuredModel := ""
	for key, value := range node.Data.Params {
		switch key {
		case "model":
			configuredModel, _ = value.(string)
		case "prompt", "skill", "url":
			// Converted to first-class prompt segments or input nodes below.
		default:
			if stringValue, ok := value.(string); ok && strings.TrimSpace(stringValue) == "" {
				continue
			}
			params[key] = value
		}
	}
	if len(node.Data.Settings) > 0 {
		settings := map[string]any{}
		for key, value := range node.Data.Settings {
			if stringValue, ok := value.(string); ok && strings.TrimSpace(stringValue) == "" {
				continue
			}
			settings[key] = value
		}
		if len(settings) > 0 {
			params["settings"] = settings
		}
	}
	if len(params) > 0 {
		data["params"] = params
	}
	if skill, _ := node.Data.Params["skill"].(string); strings.TrimSpace(skill) != "" {
		if err := AddSkillPrompt(data, skill); err != nil {
			return nil, "", err
		}
	}
	prompt, _ := node.Data.Params["prompt"].(string)
	if strings.TrimSpace(promptOverride) != "" {
		prompt = promptOverride
	}
	content := firstString(strings.TrimSpace(node.Data.Content), strings.TrimSpace(node.Content))
	if strings.TrimSpace(promptOverride) != "" && node.NodeType == "text" {
		content = promptOverride
	}
	if content != "" {
		data["content"] = content
		if node.NodeType == "text" && strings.TrimSpace(prompt) == "" {
			prompt = content
		}
	}
	if strings.TrimSpace(prompt) != "" {
		ReplaceTextPrompt(data, prompt)
	}
	if node.NodeStatus == "edit" {
		data["isExecutable"] = false
	}
	if shortcut.Kind == "skill" {
		data["preset"] = shortcut.Code
		if err := AddSkillPrompt(data, shortcut.Code); err != nil {
			return nil, "", err
		}
	}
	return data, strings.TrimSpace(configuredModel), nil
}

func encodeShortcutData(data map[string]any) (json.RawMessage, error) {
	encoded, err := EncodeObject(data)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(encoded), nil
}

func stringPointer(value string) *string {
	copy := value
	return &copy
}

func optionalStringPointer(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return stringPointer(value)
}

func shortcutInputBinding(options ShortcutApplyOptions, input ToolConfigNode, index int) string {
	keys := []string{input.NodeKey, input.Name, fmt.Sprintf("input%d", index+1)}
	if index == 0 {
		keys = append(keys, "first")
	}
	if index == 1 {
		keys = append(keys, "last")
	}
	for _, key := range keys {
		if value := strings.TrimSpace(options.Inputs[key]); value != "" {
			return value
		}
	}
	return ""
}

func resolveShortcutModel(resolver ShortcutModelResolver, nodeType, override, configured string) (string, error) {
	candidate := firstString(strings.TrimSpace(override), strings.TrimSpace(configured))
	if resolver == nil {
		return candidate, nil
	}
	return resolver(nodeType, candidate)
}

func BuildShortcutPlan(shortcut Shortcut, options ShortcutApplyOptions, resolver ShortcutModelResolver) (*ShortcutPlan, error) {
	plan := &ShortcutPlan{Shortcut: shortcut, Operations: []*NDJSONOperation{}}
	operationLine := 1
	appendOperation := func(operation *NDJSONOperation) {
		operation.Line = operationLine
		operationLine++
		plan.Operations = append(plan.Operations, operation)
	}

	switch shortcut.Kind {
	case "skill":
		if strings.TrimSpace(options.Source) == "" {
			return nil, errors.New("skill shortcut 需要 --source")
		}
		node := ToolConfigNode{NodeType: shortcut.NodeType, Name: shortcut.Name, Data: ToolConfigNodeData{Params: map[string]any{"skill": shortcut.Code}}}
		data, configuredModel, err := toolNodeData(node, shortcut, options.Prompt)
		if err != nil {
			return nil, err
		}
		model, err := resolveShortcutModel(resolver, shortcut.NodeType, options.Model, configuredModel)
		if err != nil {
			return nil, err
		}
		raw, err := encodeShortcutData(data)
		if err != nil {
			return nil, err
		}
		name := firstString(strings.TrimSpace(options.Name), shortcut.Name)
		appendOperation(&NDJSONOperation{Op: "node.create", Alias: "shortcut_output", Type: shortcut.NodeType, Name: stringPointer(name), Model: optionalStringPointer(model), Data: raw})
		appendOperation(&NDJSONOperation{Op: "edge.add", Source: options.Source, Target: "$shortcut_output", Role: "reference"})
		plan.RunRef = "$shortcut_output"
		return plan, nil

	case "mode":
		if strings.TrimSpace(options.Target) == "" {
			return nil, errors.New("mode shortcut 需要 --target")
		}
		data := map[string]any{"videoMode": shortcut.Code, "pavo_shortcut": map[string]any{"code": shortcut.Code, "kind": shortcut.Kind, "tool_spec_version": shortcut.ToolSpecVersion}}
		raw, _ := encodeShortcutData(data)
		operation := &NDJSONOperation{Op: "node.update", Ref: options.Target, Data: raw}
		if strings.TrimSpace(options.Model) != "" {
			model, err := resolveShortcutModel(resolver, shortcut.NodeType, options.Model, "")
			if err != nil {
				return nil, err
			}
			operation.Model = stringPointer(model)
		}
		if strings.TrimSpace(options.Prompt) != "" {
			operation.Prompt = stringPointer(options.Prompt)
		}
		appendOperation(operation)
		plan.RunRef = options.Target
		return plan, nil

	case "guide":
		// Continue below.
	default:
		return nil, fmt.Errorf("不支持 shortcut kind %q", shortcut.Kind)
	}

	var selfNode *ToolConfigNode
	inputs := []ToolConfigNode{}
	outputs := []ToolConfigNode{}
	for index := range shortcut.Config.Extra.NodeList {
		node := shortcut.Config.Extra.NodeList[index]
		switch strings.TrimSpace(node.ActionType) {
		case "input":
			inputs = append(inputs, node)
		case "output":
			outputs = append(outputs, node)
		default:
			if node.NodeKey == "self_node" || selfNode == nil {
				copy := node
				selfNode = &copy
			}
		}
	}
	if selfNode == nil {
		return nil, fmt.Errorf("guide shortcut %q 缺少 self_node", shortcut.Code)
	}
	selfRef := strings.TrimSpace(options.Target)
	selfData, selfConfiguredModel, err := toolNodeData(*selfNode, shortcut, options.Prompt)
	if err != nil {
		return nil, err
	}
	selfModel, err := resolveShortcutModel(resolver, selfNode.NodeType, options.Model, selfConfiguredModel)
	if err != nil {
		return nil, err
	}
	selfRaw, err := encodeShortcutData(selfData)
	if err != nil {
		return nil, err
	}
	if selfRef == "" {
		selfRef = "$shortcut_self"
		name := firstString(strings.TrimSpace(options.Name), selfNode.Name, shortcut.Name)
		appendOperation(&NDJSONOperation{Op: "node.create", Alias: "shortcut_self", Type: selfNode.NodeType, Name: stringPointer(name), Model: optionalStringPointer(selfModel), Data: selfRaw})
	} else {
		operation := &NDJSONOperation{Op: "node.update", Ref: selfRef, Data: selfRaw}
		if selfModel != "" {
			operation.Model = stringPointer(selfModel)
		}
		if strings.TrimSpace(options.Prompt) != "" {
			operation.Prompt = stringPointer(options.Prompt)
		}
		appendOperation(operation)
	}

	memberRefs := []string{selfRef}
	nodeRefs := map[string]string{selfNode.NodeKey: selfRef, "self_node": selfRef}
	inputRefs := []string{}
	for index, input := range inputs {
		inputRef := shortcutInputBinding(options, input, index)
		if inputRef == "" {
			url := strings.TrimSpace(firstString(input.Data.URL, stringValue(input.Data.Params["url"])))
			if !options.UseExampleInput || url == "" {
				return nil, fmt.Errorf("shortcut %q 缺少输入 %s；请传 --input %s=NODE", shortcut.Code, firstString(input.Name, input.NodeKey), input.NodeKey)
			}
			alias := fmt.Sprintf("shortcut_input_%d", index+1)
			inputRef = "$" + alias
			mediaType := firstString(strings.TrimSpace(input.NodeType), "image")
			data := map[string]any{"url": []any{url}, "mediaType": mediaType, "pavo_shortcut": map[string]any{"code": shortcut.Code, "kind": "example-input", "tool_spec_version": shortcut.ToolSpecVersion}}
			raw, _ := encodeShortcutData(data)
			appendOperation(&NDJSONOperation{Op: "node.create", Alias: alias, Type: "upload", MediaType: mediaType, Name: stringPointer(firstString(input.Name, fmt.Sprintf("参考素材%d", index+1))), Data: raw})
		}
		memberRefs = append(memberRefs, inputRef)
		inputRefs = append(inputRefs, inputRef)
		nodeRefs[input.NodeKey] = inputRef
	}

	outputRefs := []string{}
	for index, output := range outputs {
		alias := fmt.Sprintf("shortcut_output_%d", index+1)
		outputData, configuredModel, dataErr := toolNodeData(output, shortcut, "")
		if dataErr != nil {
			return nil, dataErr
		}
		model, modelErr := resolveShortcutModel(resolver, output.NodeType, options.Model, configuredModel)
		if modelErr != nil {
			return nil, modelErr
		}
		raw, rawErr := encodeShortcutData(outputData)
		if rawErr != nil {
			return nil, rawErr
		}
		appendOperation(&NDJSONOperation{Op: "node.create", Alias: alias, Type: output.NodeType, Name: stringPointer(firstString(output.Name, shortcut.Name)), Model: optionalStringPointer(model), Data: raw})
		outputRef := "$" + alias
		outputRefs = append(outputRefs, outputRef)
		nodeRefs[output.NodeKey] = outputRef
		memberRefs = append(memberRefs, outputRef)
		plan.RunRef = outputRef
	}
	if len(shortcut.Config.Extra.ConnectionList) > 0 {
		for index, connection := range shortcut.Config.Extra.ConnectionList {
			sourceKey, targetKey := connection.EffectiveSource(), connection.EffectiveTarget()
			sourceRef, sourceOK := nodeRefs[sourceKey]
			targetRef, targetOK := nodeRefs[targetKey]
			if !sourceOK || !targetOK {
				return nil, fmt.Errorf("shortcut %q connection_list[%d] 引用了未知节点 %q → %q", shortcut.Code, index, sourceKey, targetKey)
			}
			appendOperation(&NDJSONOperation{Op: "edge.add", Source: sourceRef, Target: targetRef, Role: firstString(strings.TrimSpace(connection.Role), "reference"), MediaOrder: index})
		}
	} else {
		for index, inputRef := range inputRefs {
			appendOperation(&NDJSONOperation{Op: "edge.add", Source: inputRef, Target: selfRef, Role: "reference", MediaOrder: index})
		}
		for _, outputRef := range outputRefs {
			appendOperation(&NDJSONOperation{Op: "edge.add", Source: selfRef, Target: outputRef, Role: "prompt"})
		}
	}
	if plan.RunRef == "" {
		plan.RunRef = selfRef
	}
	if len(memberRefs) >= 2 {
		appendOperation(&NDJSONOperation{Op: "group.create", Alias: "shortcut_group", Members: memberRefs, Name: stringPointer(shortcut.Name), ModeCode: shortcut.Code})
	}
	return plan, nil
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}
