package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/maoshuhua/pavo-cli/internal/api"
	canvascore "github.com/maoshuhua/pavo-cli/internal/canvas"
	"github.com/maoshuhua/pavo-cli/internal/output"
	"github.com/spf13/cobra"
)

type canvasTaskOutput struct {
	TaskID       string          `json:"task_id"`
	Status       int             `json:"status"`
	StatusText   string          `json:"status_text,omitempty"`
	Progress     float64         `json:"progress_percent"`
	Terminal     bool            `json:"terminal"`
	Failed       bool            `json:"failed"`
	ErrorCode    string          `json:"error_code,omitempty"`
	ErrorMessage string          `json:"error_message,omitempty"`
	Version      int64           `json:"version,omitempty"`
	TaskResult   json.RawMessage `json:"task_result,omitempty"`
}

type canvasRunOutput struct {
	ProjectID   string `json:"project_id,omitempty"`
	ProjectUUID string `json:"project_uuid"`
	CanvasUUID  string `json:"canvas_uuid"`
	CanvasURL   string `json:"canvas_url,omitempty"`
	NodeKey     string `json:"node_key"`
	Task        any    `json:"task"`
	SyncError   string `json:"sync_error,omitempty"`
}

func taskOutput(progress api.CanvasGenerationProgress) canvasTaskOutput {
	return canvasTaskOutput{
		TaskID:       strings.TrimSpace(string(progress.TaskID)),
		Status:       progress.Status,
		StatusText:   progress.StatusText,
		Progress:     progress.ProgressPercent,
		Terminal:     progress.Terminal(),
		Failed:       progress.Failed(),
		ErrorCode:    progress.ErrorCode,
		ErrorMessage: progress.ErrorMessage,
		Version:      int64(progress.Version),
		TaskResult:   canvascore.DecodeTaskResult(progress.TaskResult),
	}
}

func getCanvasTask(ctx context.Context, deps *dependencies, taskID string) (*api.CanvasGenerationProgress, error) {
	data, err := deps.api.GetCanvasGenerationProgress(ctx, []string{taskID})
	if err != nil {
		return nil, err
	}
	progress := canvascore.FindProgress(data, taskID)
	if progress == nil {
		return nil, fmt.Errorf("进度响应中找不到 task_id %q", taskID)
	}
	return progress, nil
}

func waitCanvasTask(ctx context.Context, deps *dependencies, stderr io.Writer, taskID string, interval time.Duration) (*api.CanvasGenerationProgress, error) {
	if interval <= 0 {
		return nil, errors.New("--interval 必须大于 0")
	}
	missingCount := 0
	lastStatus := -1
	lastProgress := -1.0
	for {
		data, err := deps.api.GetCanvasGenerationProgress(ctx, []string{taskID})
		if err != nil {
			return nil, err
		}
		progress := canvascore.FindProgress(data, taskID)
		if progress == nil {
			missingCount++
			if missingCount >= 3 {
				return nil, fmt.Errorf("连续 3 次进度响应中找不到 task_id %q", taskID)
			}
		} else {
			missingCount = 0
			if progress.Status != lastStatus || progress.ProgressPercent != lastProgress {
				fmt.Fprintf(stderr, "task %s: status=%d progress=%.0f%%\n", taskID, progress.Status, progress.ProgressPercent)
				lastStatus = progress.Status
				lastProgress = progress.ProgressPercent
			}
			if progress.Terminal() {
				return progress, nil
			}
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, fmt.Errorf("等待画布任务 %s 结束: %w", taskID, ctx.Err())
		case <-timer.C:
		}
	}
}

func contextWithTaskTimeout(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc, error) {
	if timeout <= 0 {
		return nil, nil, errors.New("--timeout 必须大于 0")
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	return ctx, cancel, nil
}

func syncCanvasRunNode(ctx context.Context, deps *dependencies, scope canvascore.Scope, nodeKey string, patch func(map[string]any, string)) error {
	_, err := canvascore.ApplyMutation(ctx, deps.api, scope, func(detail *api.CanvasProjectDetail) (*api.CanvasBatchRequest, error) {
		node, findErr := canvascore.FindNode(detail, nodeKey)
		if findErr != nil {
			return nil, findErr
		}
		data, decodeErr := canvascore.NodeData(*node)
		if decodeErr != nil {
			return nil, decodeErr
		}
		patch(data, string(node.NodeType))
		item, itemErr := canvascore.WriteItemFromNode(*node, data)
		if itemErr != nil {
			return nil, itemErr
		}
		request := canvascore.NewBatchRequest()
		request.Nodes.Update = append(request.Nodes.Update, item)
		return request, nil
	})
	return err
}

func newCanvasRunCommand(stdout, stderr io.Writer, deps *dependencies, options *canvasScopeOptions) *cobra.Command {
	var wait bool
	var force bool
	var download bool
	var outputDir string
	var interval, timeout time.Duration
	command := &cobra.Command{
		Use:   "run NODE",
		Short: "Create a generation task for one executable canvas node",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			download = download || strings.TrimSpace(outputDir) != ""
			if download && !wait {
				return errors.New("--download/--output-dir 需要等待任务终态，不能与 --wait=false 同时使用")
			}
			scope, err := resolveCanvasScope(options)
			if err != nil {
				return err
			}
			detail, err := deps.api.GetCanvasProjectDetail(command.Context(), scope.ProjectUUID, scope.CanvasUUID)
			if err != nil {
				return err
			}
			node, err := canvascore.FindNode(detail, args[0])
			if err != nil {
				return err
			}
			data, err := canvascore.NodeData(*node)
			if err != nil {
				return err
			}
			if !canvascore.IsNodeExecutable(*node) {
				return fmt.Errorf("节点 %s 不可执行", node.NodeKey)
			}
			if taskID, ok := data["task_id"].(string); ok && taskID != "" && taskID != "-1" && !force {
				return fmt.Errorf("节点已有 task_id %s；如确认要重复提交请传 --force", taskID)
			}
			requestID, err := canvascore.RequestID()
			if err != nil {
				return err
			}
			canvasUUID := strings.TrimSpace(scope.CanvasUUID)
			if canvasUUID == "" {
				canvasUUID = detail.CurrentCanvas.CanvasUUID
			}
			projectID := canvascore.ProjectIDFromDetail(detail)
			canvasURL := canvascore.BuildURL(deps.config.AppBaseURL, projectID, scope.ProjectUUID, canvasUUID)
			created, err := deps.api.CreateCanvasGeneration(command.Context(), scope.ProjectUUID, api.CreateCanvasGenerationRequest{
				NodeKey:    node.NodeKey,
				RequestID:  requestID,
				CanvasUUID: canvasUUID,
			})
			if err != nil {
				return err
			}
			taskID := created.EffectiveTaskID()
			syncErr := syncCanvasRunNode(command.Context(), deps, scope, node.NodeKey, func(data map[string]any, _ string) {
				canvascore.ApplyGenerationStart(data, *created)
			})
			if syncErr != nil {
				fmt.Fprintf(stderr, "warning: task %s 已创建，但回写节点执行状态失败: %v\n", taskID, syncErr)
			}
			if !wait {
				return output.WriteJSON(stdout, canvasRunOutput{ProjectID: projectID, ProjectUUID: scope.ProjectUUID, CanvasUUID: canvasUUID, CanvasURL: canvasURL, NodeKey: node.NodeKey, Task: created, SyncError: errorText(syncErr)})
			}
			waitContext, cancel, err := contextWithTaskTimeout(command.Context(), timeout)
			if err != nil {
				return err
			}
			defer cancel()
			progress, err := waitCanvasTask(waitContext, deps, stderr, taskID, interval)
			if err != nil {
				return err
			}
			terminalSyncErr := syncCanvasRunNode(command.Context(), deps, scope, node.NodeKey, func(data map[string]any, nodeType string) {
				canvascore.ApplyGenerationTerminal(data, nodeType, *progress)
			})
			if terminalSyncErr != nil {
				fmt.Fprintf(stderr, "warning: task %s 已结束，但回写节点结果失败: %v\n", taskID, terminalSyncErr)
			}
			task := taskOutput(*progress)
			if download {
				if err := downloadCanvasTaskResults(command.Context(), deps, &task, taskID, node.Name, outputDir); err != nil {
					return err
				}
			}
			return output.WriteJSON(stdout, canvasRunOutput{ProjectID: projectID, ProjectUUID: scope.ProjectUUID, CanvasUUID: canvasUUID, CanvasURL: canvasURL, NodeKey: node.NodeKey, Task: task, SyncError: errorText(terminalSyncErr)})
		},
	}
	flags := command.Flags()
	flags.BoolVar(&wait, "wait", true, "wait for a terminal task status")
	flags.BoolVar(&force, "force", false, "submit even when node data contains an active task_id")
	flags.BoolVar(&download, "download", false, "download successful generated assets after the task reaches a terminal state")
	flags.StringVar(&outputDir, "output-dir", "", "directory for downloaded assets; implies --download and defaults to pavo_outputs/canvas/<task_id>")
	flags.DurationVar(&interval, "interval", 3*time.Second, "progress polling interval")
	flags.DurationVar(&timeout, "timeout", 30*time.Minute, "maximum time to wait")
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func newCanvasTaskCommand(stdout, stderr io.Writer, deps *dependencies) *cobra.Command {
	command := &cobra.Command{Use: "task", Short: "Inspect, wait for, or cancel canvas generation tasks", Args: cobra.NoArgs}
	command.AddCommand(newCanvasTaskStatusCommand(stdout, stderr, deps))
	command.AddCommand(newCanvasTaskWaitCommand(stdout, stderr, deps))
	command.AddCommand(newCanvasTaskCancelCommand(stdout, stderr, deps))
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}

func newCanvasTaskStatusCommand(stdout, stderr io.Writer, deps *dependencies) *cobra.Command {
	command := &cobra.Command{
		Use:   "status TASK_ID",
		Short: "Get the current status of one generation task",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			progress, err := getCanvasTask(command.Context(), deps, strings.TrimSpace(args[0]))
			if err != nil {
				return err
			}
			return output.WriteJSON(stdout, taskOutput(*progress))
		},
	}
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}

func newCanvasTaskWaitCommand(stdout, stderr io.Writer, deps *dependencies) *cobra.Command {
	var interval, timeout time.Duration
	command := &cobra.Command{
		Use:   "wait TASK_ID",
		Short: "Poll one generation task until it succeeds, fails, or times out",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			ctx, cancel, err := contextWithTaskTimeout(command.Context(), timeout)
			if err != nil {
				return err
			}
			defer cancel()
			progress, err := waitCanvasTask(ctx, deps, stderr, strings.TrimSpace(args[0]), interval)
			if err != nil {
				return err
			}
			return output.WriteJSON(stdout, taskOutput(*progress))
		},
	}
	command.Flags().DurationVar(&interval, "interval", 3*time.Second, "progress polling interval")
	command.Flags().DurationVar(&timeout, "timeout", 30*time.Minute, "maximum time to wait")
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}

func newCanvasTaskCancelCommand(stdout, stderr io.Writer, deps *dependencies) *cobra.Command {
	command := &cobra.Command{
		Use:   "cancel TASK_ID",
		Short: "Cancel one canvas generation task",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			result, err := deps.api.CancelCanvasGeneration(command.Context(), strings.TrimSpace(args[0]))
			if err != nil {
				return err
			}
			return output.WriteJSON(stdout, result)
		},
	}
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}
