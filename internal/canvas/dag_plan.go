package canvas

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/maoshuhua/pavo-cli/internal/api"
)

const DAGSchemaVersion = 1

type DAGScope struct {
	Mode      string `json:"mode"`
	Reference string `json:"reference,omitempty"`
}

type DAGPlanNode struct {
	NodeKey      string   `json:"node_key"`
	Name         string   `json:"name"`
	NodeType     string   `json:"node_type"`
	Dependencies []string `json:"dependencies"`
	BatchOrder   int      `json:"batch_order"`
	RequestID    string   `json:"request_id"`
	ContentHash  string   `json:"content_hash"`
}

type DAGPlan struct {
	SchemaVersion int           `json:"schema_version"`
	PlanID        string        `json:"plan_id"`
	CreatedAt     time.Time     `json:"created_at"`
	ProjectID     string        `json:"project_id,omitempty"`
	ProjectUUID   string        `json:"project_uuid"`
	CanvasUUID    string        `json:"canvas_uuid"`
	CanvasURL     string        `json:"canvas_url,omitempty"`
	CanvasVersion int64         `json:"canvas_version"`
	Scope         DAGScope      `json:"scope"`
	PlanHash      string        `json:"plan_hash"`
	Levels        [][]string    `json:"levels"`
	Nodes         []DAGPlanNode `json:"nodes"`
	PlanPath      string        `json:"plan_path,omitempty"`
}

func BuildDAGPlan(detail *api.CanvasProjectDetail, projectUUID, canvasUUID string, scope DAGScope) (*DAGPlan, error) {
	if detail == nil {
		return nil, errors.New("画布详情为空")
	}
	selected, normalizedScope, err := selectDAGNodes(detail, scope)
	if err != nil {
		return nil, err
	}
	if len(selected) == 0 {
		return nil, errors.New("所选范围没有可执行节点")
	}
	levels, dependencies, err := topologicalLevels(detail, selected)
	if err != nil {
		return nil, err
	}
	planID, err := RandomUUID()
	if err != nil {
		return nil, err
	}
	plan := &DAGPlan{SchemaVersion: DAGSchemaVersion, PlanID: "plan-" + planID, CreatedAt: time.Now().UTC(), ProjectID: ProjectIDFromDetail(detail), ProjectUUID: strings.TrimSpace(projectUUID), CanvasUUID: firstString(canvasUUID, detail.CurrentCanvas.CanvasUUID), CanvasVersion: int64(detail.Version), Scope: normalizedScope, Levels: levels}
	order := 1
	for _, level := range levels {
		for _, key := range level {
			node := nodeByKey(detail, key)
			if node == nil {
				return nil, fmt.Errorf("计划节点 %s 不存在", key)
			}
			requestID, idErr := RequestID()
			if idErr != nil {
				return nil, idErr
			}
			contentHash, hashErr := hashPlanNode(detail, *node, dependencies[key])
			if hashErr != nil {
				return nil, hashErr
			}
			plan.Nodes = append(plan.Nodes, DAGPlanNode{NodeKey: key, Name: node.Name, NodeType: string(node.NodeType), Dependencies: dependencies[key], BatchOrder: order, RequestID: requestID, ContentHash: contentHash})
			order++
		}
	}
	plan.PlanHash, err = ComputeDAGPlanHash(plan)
	return plan, err
}

func DAGNodeKeys(detail *api.CanvasProjectDetail, scope DAGScope) ([]string, DAGScope, error) {
	selected, normalized, err := selectDAGNodes(detail, scope)
	if err != nil {
		return nil, DAGScope{}, err
	}
	levels, _, err := topologicalLevels(detail, selected)
	if err != nil {
		return nil, DAGScope{}, err
	}
	keys := []string{}
	for _, level := range levels {
		keys = append(keys, level...)
	}
	return keys, normalized, nil
}

func selectDAGNodes(detail *api.CanvasProjectDetail, scope DAGScope) (map[string]bool, DAGScope, error) {
	mode := strings.ToLower(strings.TrimSpace(scope.Mode))
	if mode == "" {
		mode = "all"
	}
	executable := map[string]bool{}
	for _, node := range detail.NodeList {
		if IsNodeExecutable(node) {
			executable[node.NodeKey] = true
		}
	}
	predecessors := map[string][]string{}
	for _, connection := range detail.ConnectionList {
		if executable[connection.SourceNodeKey] && executable[connection.TargetNodeKey] {
			predecessors[connection.TargetNodeKey] = append(predecessors[connection.TargetNodeKey], connection.SourceNodeKey)
		}
	}
	selected := map[string]bool{}
	var includeAncestors func(string)
	includeAncestors = func(key string) {
		if selected[key] {
			return
		}
		selected[key] = true
		for _, dependency := range predecessors[key] {
			includeAncestors(dependency)
		}
	}
	switch mode {
	case "all":
		for key := range executable {
			selected[key] = true
		}
		return selected, DAGScope{Mode: "all"}, nil
	case "target":
		node, err := FindNode(detail, scope.Reference)
		if err != nil {
			return nil, DAGScope{}, err
		}
		if !executable[node.NodeKey] {
			return nil, DAGScope{}, fmt.Errorf("目标节点 %s 不可执行", node.NodeKey)
		}
		includeAncestors(node.NodeKey)
		return selected, DAGScope{Mode: "target", Reference: node.NodeKey}, nil
	case "group":
		group, err := FindNode(detail, scope.Reference)
		if err != nil {
			return nil, DAGScope{}, err
		}
		if string(group.NodeType) != "group" {
			return nil, DAGScope{}, fmt.Errorf("节点 %s 不是 group", group.NodeKey)
		}
		var includeChildren func(string)
		includeChildren = func(parent string) {
			for _, node := range detail.NodeList {
				if node.ParentKey != parent {
					continue
				}
				if string(node.NodeType) == "group" {
					includeChildren(node.NodeKey)
				} else if executable[node.NodeKey] {
					includeAncestors(node.NodeKey)
				}
			}
		}
		includeChildren(group.NodeKey)
		return selected, DAGScope{Mode: "group", Reference: group.NodeKey}, nil
	default:
		return nil, DAGScope{}, fmt.Errorf("DAG scope mode %q 无效", scope.Mode)
	}
}

func topologicalLevels(detail *api.CanvasProjectDetail, selected map[string]bool) ([][]string, map[string][]string, error) {
	orderIndex := map[string]int{}
	for index, node := range detail.NodeList {
		orderIndex[node.NodeKey] = index
	}
	dependencies := map[string][]string{}
	outgoing := map[string][]string{}
	indegree := map[string]int{}
	for key := range selected {
		dependencies[key] = []string{}
		indegree[key] = 0
	}
	seenEdges := map[string]bool{}
	for _, connection := range detail.ConnectionList {
		if !selected[connection.SourceNodeKey] || !selected[connection.TargetNodeKey] {
			continue
		}
		edgeKey := connection.SourceNodeKey + "\x00" + connection.TargetNodeKey
		if seenEdges[edgeKey] {
			continue
		}
		seenEdges[edgeKey] = true
		dependencies[connection.TargetNodeKey] = append(dependencies[connection.TargetNodeKey], connection.SourceNodeKey)
		outgoing[connection.SourceNodeKey] = append(outgoing[connection.SourceNodeKey], connection.TargetNodeKey)
		indegree[connection.TargetNodeKey]++
	}
	less := func(a, b string) bool {
		if orderIndex[a] != orderIndex[b] {
			return orderIndex[a] < orderIndex[b]
		}
		return a < b
	}
	for key := range dependencies {
		sort.Slice(dependencies[key], func(i, j int) bool { return less(dependencies[key][i], dependencies[key][j]) })
		sort.Slice(outgoing[key], func(i, j int) bool { return less(outgoing[key][i], outgoing[key][j]) })
	}
	current := []string{}
	for key, degree := range indegree {
		if degree == 0 {
			current = append(current, key)
		}
	}
	sort.Slice(current, func(i, j int) bool { return less(current[i], current[j]) })
	levels := [][]string{}
	processed := 0
	for len(current) > 0 {
		level := append([]string(nil), current...)
		levels = append(levels, level)
		processed += len(level)
		next := []string{}
		for _, key := range level {
			for _, target := range outgoing[key] {
				indegree[target]--
				if indegree[target] == 0 {
					next = append(next, target)
				}
			}
		}
		sort.Slice(next, func(i, j int) bool { return less(next[i], next[j]) })
		current = next
	}
	if processed != len(selected) {
		cycle := findDAGCycle(outgoing, selected)
		return nil, nil, fmt.Errorf("DAG 包含环: %s", strings.Join(cycle, " -> "))
	}
	return levels, dependencies, nil
}

func findDAGCycle(outgoing map[string][]string, selected map[string]bool) []string {
	state := map[string]int{}
	stack := []string{}
	position := map[string]int{}
	result := []string{}
	var visit func(string) bool
	visit = func(key string) bool {
		state[key] = 1
		position[key] = len(stack)
		stack = append(stack, key)
		for _, next := range outgoing[key] {
			if state[next] == 0 {
				if visit(next) {
					return true
				}
			} else if state[next] == 1 {
				start := position[next]
				result = append(result, stack[start:]...)
				result = append(result, next)
				return true
			}
		}
		stack = stack[:len(stack)-1]
		delete(position, key)
		state[key] = 2
		return false
	}
	keys := make([]string, 0, len(selected))
	for key := range selected {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if state[key] == 0 && visit(key) {
			break
		}
	}
	return result
}

func hashPlanNode(detail *api.CanvasProjectDetail, node api.CanvasNode, dependencies []string) (string, error) {
	type input struct {
		Connection api.CanvasBatchConnectionWriteItem `json:"connection"`
		Source     *api.CanvasBatchNodeWriteItem      `json:"source,omitempty"`
	}
	data := struct {
		Node         api.CanvasBatchNodeWriteItem `json:"node"`
		Dependencies []string                     `json:"dependencies"`
		Inputs       []input                      `json:"inputs"`
	}{Dependencies: dependencies, Inputs: []input{}}
	nodeData, err := planNodeData(node)
	if err != nil {
		return "", err
	}
	data.Node, err = WriteItemFromNode(node, nodeData)
	if err != nil {
		return "", err
	}
	for _, connection := range detail.ConnectionList {
		if connection.TargetNodeKey != node.NodeKey {
			continue
		}
		item := input{Connection: ConnectionWriteItem(connection)}
		source := nodeByKey(detail, connection.SourceNodeKey)
		if source != nil && !IsNodeExecutable(*source) {
			sourceData, sourceErr := planNodeData(*source)
			if sourceErr != nil {
				return "", sourceErr
			}
			sourceItem, sourceErr := WriteItemFromNode(*source, sourceData)
			if sourceErr != nil {
				return "", sourceErr
			}
			item.Source = &sourceItem
		}
		data.Inputs = append(data.Inputs, item)
	}
	sort.Slice(data.Inputs, func(i, j int) bool {
		return data.Inputs[i].Connection.ConnectionID < data.Inputs[j].Connection.ConnectionID
	})
	encoded, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func planNodeData(node api.CanvasNode) (map[string]any, error) {
	data, err := NodeData(node)
	if err != nil {
		return nil, err
	}
	for _, key := range []string{"status", "progress", "task_id", "copyTaskId", "generation_status", "generation_error_code", "generation_completed_acknowledged", "estimated_generation_time", "isGroupRunning", "groupRunCompleted", "groupRunTotal", "imageGalleryExpanded"} {
		delete(data, key)
	}
	if IsNodeExecutable(node) {
		delete(data, "url")
		if params, ok := data["params"].(map[string]any); ok {
			if prompt, exists := params["prompt"]; exists && prompt != nil {
				delete(data, "content")
			}
			if _, exists := params["duration"]; exists {
				delete(data, "duration")
			}
		}
	}
	return data, nil
}

func ComputeDAGPlanHash(plan *DAGPlan) (string, error) {
	if plan == nil {
		return "", errors.New("DAG plan 为空")
	}
	type hashNode struct {
		NodeKey      string   `json:"node_key"`
		NodeType     string   `json:"node_type"`
		Dependencies []string `json:"dependencies"`
		BatchOrder   int      `json:"batch_order"`
		ContentHash  string   `json:"content_hash"`
	}
	payload := struct {
		ProjectUUID string     `json:"project_uuid"`
		CanvasUUID  string     `json:"canvas_uuid"`
		Scope       DAGScope   `json:"scope"`
		Nodes       []hashNode `json:"nodes"`
	}{ProjectUUID: plan.ProjectUUID, CanvasUUID: plan.CanvasUUID, Scope: plan.Scope}
	for _, node := range plan.Nodes {
		payload.Nodes = append(payload.Nodes, hashNode{NodeKey: node.NodeKey, NodeType: node.NodeType, Dependencies: node.Dependencies, BatchOrder: node.BatchOrder, ContentHash: node.ContentHash})
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}
