package canvas

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/maoshuhua/pavo-cli/internal/api"
)

type DAGRunNode struct {
	NodeKey      string                        `json:"node_key"`
	Name         string                        `json:"name"`
	NodeType     string                        `json:"node_type"`
	Dependencies []string                      `json:"dependencies"`
	BatchOrder   int                           `json:"batch_order"`
	RequestID    string                        `json:"request_id"`
	Status       string                        `json:"status"`
	TaskID       string                        `json:"task_id,omitempty"`
	Error        string                        `json:"error,omitempty"`
	Progress     *api.CanvasGenerationProgress `json:"progress,omitempty"`
}

type DAGRun struct {
	SchemaVersion    int          `json:"schema_version"`
	ExecutionBatchID string       `json:"execution_batch_id"`
	PlanID           string       `json:"plan_id"`
	PlanHash         string       `json:"plan_hash"`
	ProjectID        string       `json:"project_id,omitempty"`
	ProjectUUID      string       `json:"project_uuid"`
	CanvasUUID       string       `json:"canvas_uuid"`
	CanvasURL        string       `json:"canvas_url,omitempty"`
	CreatedAt        time.Time    `json:"created_at"`
	UpdatedAt        time.Time    `json:"updated_at"`
	Status           string       `json:"status"`
	MaxParallel      int          `json:"max_parallel"`
	Download         bool         `json:"download"`
	OutputDir        string       `json:"output_dir,omitempty"`
	RunPath          string       `json:"run_path,omitempty"`
	Nodes            []DAGRunNode `json:"nodes"`
}

func PavoWorkspaceDirectory(start string) (string, error) {
	if _, path, err := FindBinding(start); err == nil {
		return filepath.Dir(path), nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	abs, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	return filepath.Join(abs, BindingDirectory), nil
}

func validateStoreID(value, prefix string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("标识不能为空")
	}
	if filepath.Base(value) != value || strings.ContainsAny(value, `/\\`) {
		return "", errors.New("标识不能包含路径")
	}
	if prefix != "" && !strings.HasPrefix(value, prefix) {
		return "", fmt.Errorf("标识必须以 %s 开头", prefix)
	}
	return value, nil
}

func SaveDAGPlan(pavoDirectory string, plan *DAGPlan) (string, error) {
	if plan == nil {
		return "", errors.New("DAG plan 为空")
	}
	id, err := validateStoreID(plan.PlanID, "plan-")
	if err != nil {
		return "", err
	}
	path := filepath.Join(pavoDirectory, "canvas-plans", id+".json")
	plan.PlanPath = path
	if err := writeJSONAtomic(path, plan); err != nil {
		return "", err
	}
	return path, nil
}

func LoadDAGPlan(pavoDirectory, planID string) (*DAGPlan, string, error) {
	id, err := validateStoreID(strings.TrimSuffix(strings.TrimSpace(planID), ".json"), "plan-")
	if err != nil {
		return nil, "", err
	}
	path := filepath.Join(pavoDirectory, "canvas-plans", id+".json")
	var plan DAGPlan
	if err := readCompatibleDAGPlan(path, &plan); err != nil {
		return nil, path, err
	}
	if plan.SchemaVersion != DAGSchemaVersion {
		return nil, path, fmt.Errorf("不支持的 DAG plan schema_version %d", plan.SchemaVersion)
	}
	plan.PlanPath = path
	return &plan, path, nil
}

func SaveDAGRun(pavoDirectory string, run *DAGRun) (string, error) {
	if run == nil {
		return "", errors.New("DAG run 为空")
	}
	id, err := validateStoreID(run.ExecutionBatchID, "run-")
	if err != nil {
		return "", err
	}
	run.UpdatedAt = time.Now().UTC()
	path := filepath.Join(pavoDirectory, "canvas-runs", id+".json")
	run.RunPath = path
	if err := writeJSONAtomic(path, run); err != nil {
		return "", err
	}
	return path, nil
}

func LoadDAGRun(pavoDirectory, runID string) (*DAGRun, string, error) {
	id, err := validateStoreID(strings.TrimSuffix(strings.TrimSpace(runID), ".json"), "run-")
	if err != nil {
		return nil, "", err
	}
	path := filepath.Join(pavoDirectory, "canvas-runs", id+".json")
	var run DAGRun
	if err := readCompatibleDAGRun(path, &run); err != nil {
		return nil, path, err
	}
	if run.SchemaVersion != DAGSchemaVersion {
		return nil, path, fmt.Errorf("不支持的 DAG run schema_version %d", run.SchemaVersion)
	}
	run.RunPath = path
	return &run, path, nil
}

func NewDAGRun(plan *DAGPlan, maxParallel int, download bool, outputDir string) (*DAGRun, error) {
	id, err := RandomUUID()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	run := &DAGRun{SchemaVersion: DAGSchemaVersion, ExecutionBatchID: "run-" + id, PlanID: plan.PlanID, PlanHash: plan.PlanHash, ProjectID: plan.ProjectID, ProjectUUID: plan.ProjectUUID, CanvasUUID: plan.CanvasUUID, CanvasURL: plan.CanvasURL, CreatedAt: now, UpdatedAt: now, Status: "pending", MaxParallel: maxParallel, Download: download, OutputDir: strings.TrimSpace(outputDir)}
	for _, node := range plan.Nodes {
		run.Nodes = append(run.Nodes, DAGRunNode{NodeKey: node.NodeKey, Name: node.Name, NodeType: node.NodeType, Dependencies: append([]string(nil), node.Dependencies...), BatchOrder: node.BatchOrder, RequestID: node.RequestID, Status: "pending"})
	}
	return run, nil
}

func RecomputeDAGRunStatus(run *DAGRun) string {
	hasFailure, hasActive := false, false
	for _, node := range run.Nodes {
		switch node.Status {
		case "succeeded":
		case "failed", "skipped":
			hasFailure = true
		default:
			hasActive = true
		}
	}
	switch {
	case hasActive:
		run.Status = "running"
	case hasFailure:
		run.Status = "completed_with_errors"
	default:
		run.Status = "succeeded"
	}
	return run.Status
}

func writeJSONAtomic(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	temporary, err := os.CreateTemp(filepath.Dir(path), ".manifest-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		if writeErr := os.WriteFile(path, data, 0o600); writeErr != nil {
			return writeErr
		}
	}
	return nil
}

func readCompatibleDAGPlan(path string, value *DAGPlan) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("解析 %s 失败: %w", path, err)
	}
	// Plans created before credit estimation was removed contained these
	// fields. Strip only the known legacy fields so all other unknown fields
	// are still rejected by the strict decoder below.
	delete(raw, "quote_status")
	delete(raw, "total_credits")
	if encodedNodes, ok := raw["nodes"]; ok {
		var nodes []map[string]json.RawMessage
		if err := json.Unmarshal(encodedNodes, &nodes); err != nil {
			return fmt.Errorf("解析 %s 失败: %w", path, err)
		}
		for index := range nodes {
			delete(nodes[index], "quote")
		}
		raw["nodes"], err = json.Marshal(nodes)
		if err != nil {
			return err
		}
	}
	data, err = json.Marshal(raw)
	if err != nil {
		return err
	}
	return decodeStrictJSON(path, data, value)
}

func readCompatibleDAGRun(path string, value *DAGRun) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("解析 %s 失败: %w", path, err)
	}
	delete(raw, "total_credits")
	data, err = json.Marshal(raw)
	if err != nil {
		return err
	}
	return decodeStrictJSON(path, data, value)
}

func decodeStrictJSON(path string, data []byte, value any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return fmt.Errorf("解析 %s 失败: %w", path, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("解析 %s 失败: 包含多余 JSON 值", path)
		}
		return fmt.Errorf("解析 %s 失败: %w", path, err)
	}
	return nil
}
