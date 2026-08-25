package canvas

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/maoshuhua/pavo-cli/internal/api"
)

func phase2Node(key, nodeType, name, x, y string, executable bool) api.CanvasNode {
	data, _ := json.Marshal(map[string]any{"node_key": key, "title": name, "isExecutable": executable, "params": map[string]any{"model": "model-x", "count": 1}})
	return api.CanvasNode{NodeKey: key, NodeType: api.ScalarString(nodeType), Name: name, Position: api.CanvasNodePosition{PositionX: api.ScalarString(x), PositionY: api.ScalarString(y)}, Measured: api.CanvasNodeMeasured{Width: "280", Height: "280"}, Data: data}
}

func TestGroupNodesAndUngroupRestoresCoordinates(t *testing.T) {
	detail := &api.CanvasProjectDetail{NodeList: []api.CanvasNode{phase2Node("i-a", "image", "A", "100", "80", true), phase2Node("i-b", "image", "B", "500", "80", true)}, ConnectionList: []api.CanvasConnection{}}
	groupKey, members, err := GroupNodes(detail, []string{"i-a", "i-b"}, GroupOptions{NodeKey: "g-fixed", Name: "镜头组"})
	if err != nil {
		t.Fatal(err)
	}
	if groupKey != "g-fixed" || len(members) != 2 {
		t.Fatalf("group=%q members=%v", groupKey, members)
	}
	group := nodeByKey(detail, "g-fixed")
	if group == nil || group.Position.PositionX != "68" || group.Position.PositionY != "48" || group.Measured.Width != "744" || group.Measured.Height != "344" {
		t.Fatalf("group=%#v", group)
	}
	if nodeByKey(detail, "i-a").ParentKey != "g-fixed" || nodeByKey(detail, "i-a").Position.PositionX != "32" || nodeByKey(detail, "i-b").Position.PositionX != "432" {
		t.Fatalf("children=%#v", detail.NodeList)
	}
	if _, _, err := UngroupNode(detail, "g-fixed"); err != nil {
		t.Fatal(err)
	}
	if nodeByKey(detail, "g-fixed") != nil || nodeByKey(detail, "i-a").Position.PositionX != "100" || nodeByKey(detail, "i-b").Position.PositionX != "500" || nodeByKey(detail, "i-a").ParentKey != "" {
		t.Fatalf("ungrouped=%#v", detail.NodeList)
	}
}

func TestApplyNDJSONAliasesCompileOneAtomicBatch(t *testing.T) {
	operations, err := ParseNDJSON(strings.NewReader(strings.Join([]string{
		`{"op":"node.create","as":"prompt","type":"text","name":"提示"}`,
		`{"op":"node.create","as":"image","type":"image","name":"画面","model":"model-x"}`,
		`{"op":"edge.add","as":"flow","source":"$prompt","target":"$image","role":"prompt"}`,
	}, "\n")))
	if err != nil {
		t.Fatal(err)
	}
	result, err := ApplyNDJSON(&api.CanvasProjectDetail{NodeList: []api.CanvasNode{}, ConnectionList: []api.CanvasConnection{}}, operations)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Request.Nodes.Create) != 2 || len(result.Request.Connections.Create) != 1 || result.Aliases["prompt"] == "" || result.Aliases["image"] == "" || result.Aliases["flow"] == "" {
		t.Fatalf("result=%#v", result)
	}
	connection := result.Request.Connections.Create[0]
	if connection.Source != result.Aliases["prompt"] || connection.Target != result.Aliases["image"] || connection.Role != "prompt" || !connection.Selectable || !connection.Deletable {
		t.Fatalf("connection=%#v", connection)
	}
}

func TestParseNDJSONRejectsUnknownOperationField(t *testing.T) {
	_, err := ParseNDJSON(strings.NewReader(`{"op":"node.delete","ref":"i-a","force":true}`))
	if err == nil || !strings.Contains(err.Error(), "未知字段") {
		t.Fatalf("error=%v", err)
	}
}

func TestDAGPlanDiamondAndCycleFailure(t *testing.T) {
	detail := &api.CanvasProjectDetail{CurrentCanvas: api.CanvasInfo{CanvasUUID: "canvas-1"}, NodeList: []api.CanvasNode{
		phase2Node("i-a", "image", "A", "0", "0", true), phase2Node("i-b", "image", "B", "0", "0", true), phase2Node("i-c", "image", "C", "0", "0", true), phase2Node("i-d", "image", "D", "0", "0", true),
	}, ConnectionList: []api.CanvasConnection{
		{ConnectionID: "ab", SourceNodeKey: "i-a", TargetNodeKey: "i-b"}, {ConnectionID: "ac", SourceNodeKey: "i-a", TargetNodeKey: "i-c"}, {ConnectionID: "bd", SourceNodeKey: "i-b", TargetNodeKey: "i-d"}, {ConnectionID: "cd", SourceNodeKey: "i-c", TargetNodeKey: "i-d"},
	}}
	plan, err := BuildDAGPlan(detail, "project-1", "canvas-1", DAGScope{Mode: "all"})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Levels) != 3 || strings.Join(plan.Levels[0], ",") != "i-a" || strings.Join(plan.Levels[1], ",") != "i-b,i-c" || strings.Join(plan.Levels[2], ",") != "i-d" || plan.Nodes[0].BatchOrder != 1 || plan.Nodes[3].BatchOrder != 4 {
		t.Fatalf("levels=%v nodes=%v", plan.Levels, plan.Nodes)
	}
	detail.ConnectionList = append(detail.ConnectionList, api.CanvasConnection{ConnectionID: "da", SourceNodeKey: "i-d", TargetNodeKey: "i-a"})
	_, err = BuildDAGPlan(detail, "project-1", "canvas-1", DAGScope{Mode: "all"})
	if err == nil || !strings.Contains(err.Error(), "DAG 包含环") || !strings.Contains(err.Error(), " -> ") {
		t.Fatalf("error=%v", err)
	}
}

func TestDAGPlanOmitsCreditsAndLoadsLegacyPlan(t *testing.T) {
	detail := &api.CanvasProjectDetail{CurrentCanvas: api.CanvasInfo{CanvasUUID: "canvas-1"}, NodeList: []api.CanvasNode{
		phase2Node("i-a", "image", "A", "0", "0", true),
	}, ConnectionList: []api.CanvasConnection{}}
	plan, err := BuildDAGPlan(detail, "project-1", "canvas-1", DAGScope{Mode: "all"})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	for _, removed := range []string{`"quote"`, `"quote_status"`, `"total_credits"`} {
		if strings.Contains(string(encoded), removed) {
			t.Fatalf("new plan contains removed field %s: %s", removed, encoded)
		}
	}

	pavoDirectory := t.TempDir()
	legacyPath := filepath.Join(pavoDirectory, "canvas-plans", "plan-legacy.json")
	legacy := map[string]any{
		"schema_version": 1, "plan_id": "plan-legacy", "created_at": time.Now().UTC(),
		"project_uuid": "project-1", "canvas_uuid": "canvas-1", "canvas_version": 1,
		"scope": map[string]any{"mode": "all"}, "plan_hash": "legacy-hash",
		"quote_status": "estimated", "total_credits": 12,
		"levels": [][]string{{"i-a"}},
		"nodes": []any{map[string]any{
			"node_key": "i-a", "name": "A", "node_type": "image", "dependencies": []string{},
			"batch_order": 1, "request_id": "req-legacy", "content_hash": "content",
			"quote": map[string]any{"status": "estimated", "required_credits": 12},
		}},
	}
	if err := writeJSONAtomic(legacyPath, legacy); err != nil {
		t.Fatal(err)
	}
	loaded, _, err := LoadDAGPlan(pavoDirectory, "plan-legacy")
	if err != nil {
		t.Fatal(err)
	}
	encoded, err = json.Marshal(loaded)
	if err != nil {
		t.Fatal(err)
	}
	for _, removed := range []string{`"quote"`, `"quote_status"`, `"total_credits"`} {
		if strings.Contains(string(encoded), removed) {
			t.Fatalf("loaded legacy plan exposes removed field %s: %s", removed, encoded)
		}
	}
}
