package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/maoshuhua/pavo-cli/internal/api"
	canvascore "github.com/maoshuhua/pavo-cli/internal/canvas"
	"github.com/maoshuhua/pavo-cli/internal/output"
	"github.com/spf13/cobra"
)

func newCanvasDAGCommand(stdout, stderr io.Writer, deps *dependencies, options *canvasScopeOptions) *cobra.Command {
	command := &cobra.Command{Use: "dag", Short: "Plan and execute canvas generation as a DAG", Args: cobra.NoArgs}
	command.AddCommand(newCanvasDAGPlanCommand(stdout, stderr, deps, options))
	command.AddCommand(newCanvasDAGRunCommand(stdout, stderr, deps))
	command.AddCommand(newCanvasDAGStatusCommand(stdout, stderr, deps))
	command.AddCommand(newCanvasDAGResumeCommand(stdout, stderr, deps))
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}

func dagScope(group, target string, all bool) (canvascore.DAGScope, error) {
	count := 0
	if strings.TrimSpace(group) != "" {
		count++
	}
	if strings.TrimSpace(target) != "" {
		count++
	}
	if all {
		count++
	}
	if count != 1 {
		return canvascore.DAGScope{}, errors.New("必须且只能传 --group、--target 或 --all 之一")
	}
	if all {
		return canvascore.DAGScope{Mode: "all"}, nil
	}
	if strings.TrimSpace(group) != "" {
		return canvascore.DAGScope{Mode: "group", Reference: strings.TrimSpace(group)}, nil
	}
	return canvascore.DAGScope{Mode: "target", Reference: strings.TrimSpace(target)}, nil
}

func buildDAGPlan(detail *api.CanvasProjectDetail, projectUUID, canvasUUID, appBaseURL string, scope canvascore.DAGScope) (*canvascore.DAGPlan, error) {
	plan, err := canvascore.BuildDAGPlan(detail, projectUUID, canvasUUID, scope)
	if err != nil {
		return nil, err
	}
	plan.CanvasURL = canvascore.BuildURL(appBaseURL, plan.ProjectID, plan.ProjectUUID, plan.CanvasUUID)
	return plan, nil
}

func currentPavoDirectory() (string, error) {
	directory, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return canvascore.PavoWorkspaceDirectory(directory)
}

func newCanvasDAGPlanCommand(stdout, stderr io.Writer, deps *dependencies, options *canvasScopeOptions) *cobra.Command {
	var group, target string
	var all bool
	command := &cobra.Command{
		Use: "plan", Short: "Resolve and persist an executable canvas topology", Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			selection, err := dagScope(group, target, all)
			if err != nil {
				return err
			}
			scope, err := resolveCanvasScope(options)
			if err != nil {
				return err
			}
			detail, err := deps.api.GetCanvasProjectDetail(command.Context(), scope.ProjectUUID, scope.CanvasUUID)
			if err != nil {
				return err
			}
			canvasUUID := firstNonEmptyString(scope.CanvasUUID, detail.CurrentCanvas.CanvasUUID)
			plan, err := buildDAGPlan(detail, scope.ProjectUUID, canvasUUID, deps.config.AppBaseURL, selection)
			if err != nil {
				return err
			}
			pavoDirectory, err := currentPavoDirectory()
			if err != nil {
				return err
			}
			path, err := canvascore.SaveDAGPlan(pavoDirectory, plan)
			if err != nil {
				return err
			}
			plan.PlanPath = path
			return output.WriteJSON(stdout, plan)
		},
	}
	flags := command.Flags()
	flags.StringVar(&group, "group", "", "group node_key or exact title")
	flags.StringVar(&target, "target", "", "target node_key or exact title, including executable ancestors")
	flags.BoolVar(&all, "all", false, "plan all executable nodes")
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}

type dagExecutionOptions struct {
	maxParallel       int
	download          bool
	outputDir         string
	interval, timeout time.Duration
}

func loadCurrentDAGPlan(ctx context.Context, deps *dependencies, plan *canvascore.DAGPlan) (*canvascore.DAGPlan, error) {
	detail, err := deps.api.GetCanvasProjectDetail(ctx, plan.ProjectUUID, plan.CanvasUUID)
	if err != nil {
		return nil, err
	}
	current, err := buildDAGPlan(detail, plan.ProjectUUID, plan.CanvasUUID, deps.config.AppBaseURL, plan.Scope)
	if err != nil {
		return nil, err
	}
	if current.PlanHash != plan.PlanHash {
		return nil, errors.New("replan_required: 画布结构或节点参数已变化；请重新运行 canvas dag plan")
	}
	plan.ProjectID = current.ProjectID
	plan.CanvasURL = current.CanvasURL
	return current, nil
}

func ensurePendingDAGRunCurrent(ctx context.Context, deps *dependencies, plan *canvascore.DAGPlan, run *canvascore.DAGRun) error {
	detail, err := deps.api.GetCanvasProjectDetail(ctx, plan.ProjectUUID, plan.CanvasUUID)
	if err != nil {
		return err
	}
	current, err := buildDAGPlan(detail, plan.ProjectUUID, plan.CanvasUUID, deps.config.AppBaseURL, plan.Scope)
	if err != nil {
		return err
	}
	currentByKey := map[string]canvascore.DAGPlanNode{}
	for _, node := range current.Nodes {
		currentByKey[node.NodeKey] = node
	}
	originalByKey := map[string]canvascore.DAGPlanNode{}
	for _, node := range plan.Nodes {
		originalByKey[node.NodeKey] = node
	}
	for _, runNode := range run.Nodes {
		if runNode.Status == "succeeded" || runNode.Status == "failed" || runNode.Status == "skipped" {
			continue
		}
		original, originalOK := originalByKey[runNode.NodeKey]
		currentNode, currentOK := currentByKey[runNode.NodeKey]
		if !originalOK || !currentOK || original.ContentHash != currentNode.ContentHash || strings.Join(original.Dependencies, "\x00") != strings.Join(currentNode.Dependencies, "\x00") {
			return fmt.Errorf("replan_required: 待恢复节点 %s 的结构或参数已变化；请停止旧 run 并重新运行 canvas dag plan", runNode.NodeKey)
		}
	}
	return nil
}

func newCanvasDAGRunCommand(stdout, stderr io.Writer, deps *dependencies) *cobra.Command {
	var planID string
	options := dagExecutionOptions{}
	command := &cobra.Command{
		Use: "run", Short: "Execute a previously persisted DAG plan", Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if strings.TrimSpace(planID) == "" {
				return errors.New("缺少必填参数 --plan")
			}
			if options.maxParallel <= 0 {
				return errors.New("--max-parallel 必须大于 0")
			}
			pavoDirectory, err := currentPavoDirectory()
			if err != nil {
				return err
			}
			plan, _, err := canvascore.LoadDAGPlan(pavoDirectory, planID)
			if err != nil {
				return err
			}
			if _, err := loadCurrentDAGPlan(command.Context(), deps, plan); err != nil {
				return err
			}
			if strings.TrimSpace(options.outputDir) != "" {
				options.outputDir, err = filepath.Abs(options.outputDir)
				if err != nil {
					return err
				}
				options.download = true
			}
			run, err := canvascore.NewDAGRun(plan, options.maxParallel, options.download, options.outputDir)
			if err != nil {
				return err
			}
			if _, err := canvascore.SaveDAGRun(pavoDirectory, run); err != nil {
				return err
			}
			ctx, cancel, err := contextWithTaskTimeout(command.Context(), options.timeout)
			if err != nil {
				return err
			}
			defer cancel()
			executionErr := executeDAGRun(ctx, deps, stderr, pavoDirectory, run, options.interval)
			if writeErr := output.WriteJSON(stdout, run); writeErr != nil {
				return writeErr
			}
			return executionErr
		},
	}
	flags := command.Flags()
	flags.StringVar(&planID, "plan", "", "plan ID from canvas dag plan")
	addDAGExecutionFlags(flags, &options)
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}

func addDAGExecutionFlags(flags interface {
	IntVar(*int, string, int, string)
	BoolVar(*bool, string, bool, string)
	StringVar(*string, string, string, string)
	DurationVar(*time.Duration, string, time.Duration, string)
}, options *dagExecutionOptions) {
	flags.IntVar(&options.maxParallel, "max-parallel", 4, "maximum number of concurrently running generation nodes")
	flags.BoolVar(&options.download, "download", false, "download successful node results")
	flags.StringVar(&options.outputDir, "output-dir", "", "base directory for downloaded DAG results; implies --download")
	flags.DurationVar(&options.interval, "interval", 3*time.Second, "task polling interval")
	flags.DurationVar(&options.timeout, "timeout", 2*time.Hour, "maximum time for this run or resume invocation")
}

type dagNodeOutcome struct {
	index         int
	progress      *api.CanvasGenerationProgress
	err           error
	submitError   bool
	downloadError bool
}

func executeDAGRun(ctx context.Context, deps *dependencies, stderr io.Writer, pavoDirectory string, run *canvascore.DAGRun, interval time.Duration) error {
	if interval <= 0 {
		return errors.New("--interval 必须大于 0")
	}
	if run.MaxParallel <= 0 {
		return errors.New("max_parallel 必须大于 0")
	}
	var mutex sync.Mutex
	persist := func() error {
		canvascore.RecomputeDAGRunStatus(run)
		_, err := canvascore.SaveDAGRun(pavoDirectory, run)
		return err
	}
	persistStarted := func(index int, taskID string) error {
		mutex.Lock()
		defer mutex.Unlock()
		run.Nodes[index].TaskID = taskID
		run.Nodes[index].Status = "running"
		run.Nodes[index].Error = ""
		return persist()
	}
	statusByKey := func() map[string]string {
		result := map[string]string{}
		for _, node := range run.Nodes {
			result[node.NodeKey] = node.Status
		}
		return result
	}
	markSkipped := func() bool {
		changed := false
		statuses := statusByKey()
		for index := range run.Nodes {
			node := &run.Nodes[index]
			if node.Status != "pending" {
				continue
			}
			for _, dependency := range node.Dependencies {
				if statuses[dependency] == "failed" || statuses[dependency] == "skipped" {
					node.Status = "skipped"
					node.Error = "upstream_failed:" + dependency
					statuses[node.NodeKey] = "skipped"
					changed = true
					break
				}
			}
		}
		return changed
	}
	if err := persist(); err != nil {
		return err
	}
	outcomes := make(chan dagNodeOutcome, run.MaxParallel)
	attempted := map[int]bool{}
	active := 0
	launch := func(index int) error {
		node := &run.Nodes[index]
		node.Status = "submitting"
		node.Error = ""
		attempted[index] = true
		if err := persist(); err != nil {
			return err
		}
		active++
		go func(snapshot canvascore.DAGRunNode) {
			taskID := strings.TrimSpace(snapshot.TaskID)
			if taskID == "" {
				created, err := deps.api.CreateCanvasGeneration(ctx, run.ProjectUUID, api.CreateCanvasGenerationRequest{NodeKey: snapshot.NodeKey, RequestID: snapshot.RequestID, CanvasUUID: run.CanvasUUID, ExecutionBatchID: run.ExecutionBatchID, BatchOrder: snapshot.BatchOrder, BatchTotal: len(run.Nodes)})
				if err != nil {
					outcomes <- dagNodeOutcome{index: index, err: err, submitError: true}
					return
				}
				taskID = created.EffectiveTaskID()
				if err := persistStarted(index, taskID); err != nil {
					outcomes <- dagNodeOutcome{index: index, err: err}
					return
				}
			}
			progress, err := waitCanvasTask(ctx, deps, stderr, taskID, interval)
			if err != nil {
				outcomes <- dagNodeOutcome{index: index, err: err}
				return
			}
			if run.Download && !progress.Failed() {
				task := taskOutput(*progress)
				nodeDir := ""
				if strings.TrimSpace(run.OutputDir) != "" {
					nodeDir = filepath.Join(run.OutputDir, fmt.Sprintf("%03d-%s", snapshot.BatchOrder, safeFilenamePart(snapshot.Name)))
				}
				if err := downloadCanvasTaskResults(ctx, deps, &task, taskID, snapshot.Name, nodeDir); err != nil {
					outcomes <- dagNodeOutcome{index: index, progress: progress, err: err, downloadError: true}
					return
				}
				progress.TaskResult = task.TaskResult
			}
			outcomes <- dagNodeOutcome{index: index, progress: progress}
		}(*node)
		return nil
	}
	for {
		mutex.Lock()
		if markSkipped() {
			if err := persist(); err != nil {
				mutex.Unlock()
				return err
			}
		}
		statuses := statusByKey()
		launched := false
		for index := range run.Nodes {
			if active >= run.MaxParallel {
				break
			}
			node := &run.Nodes[index]
			if attempted[index] || (node.Status != "pending" && node.Status != "unknown" && node.Status != "submitting" && !(node.Status == "running" && node.TaskID != "")) {
				continue
			}
			ready := true
			for _, dependency := range node.Dependencies {
				if statuses[dependency] != "succeeded" {
					ready = false
					break
				}
			}
			if !ready {
				continue
			}
			if err := launch(index); err != nil {
				mutex.Unlock()
				return err
			}
			launched = true
		}
		if active == 0 && !launched {
			canvascore.RecomputeDAGRunStatus(run)
			saveErr := func() error { _, err := canvascore.SaveDAGRun(pavoDirectory, run); return err }()
			status := run.Status
			mutex.Unlock()
			if saveErr != nil {
				return saveErr
			}
			if status == "running" {
				return errors.New("DAG 仍有状态不明确或未完成的节点；请先运行 canvas dag status，再用 canvas dag resume 继续")
			}
			return nil
		}
		mutex.Unlock()
		outcome := <-outcomes
		mutex.Lock()
		active--
		node := &run.Nodes[outcome.index]
		if outcome.progress != nil {
			node.Progress = outcome.progress
			if outcome.progress.Failed() {
				node.Status = "failed"
				node.Error = firstNonEmptyString(outcome.progress.ErrorMessage, outcome.progress.ErrorCode)
			} else if outcome.progress.Terminal() {
				node.Status = "succeeded"
				node.Error = ""
			}
		}
		if outcome.err != nil {
			node.Error = outcome.err.Error()
			if outcome.downloadError && outcome.progress != nil && outcome.progress.Terminal() && !outcome.progress.Failed() {
				node.Status = "succeeded"
				node.Error = "download_failed:" + outcome.err.Error()
			} else if outcome.submitError {
				var apiErr *api.APIError
				if errors.As(outcome.err, &apiErr) {
					node.Status = "failed"
				} else {
					node.Status = "unknown"
				}
			} else if node.TaskID != "" {
				node.Status = "running"
			} else {
				node.Status = "unknown"
			}
		}
		if err := persist(); err != nil {
			mutex.Unlock()
			return err
		}
		mutex.Unlock()
	}
}

func refreshDAGRun(ctx context.Context, deps *dependencies, pavoDirectory string, run *canvascore.DAGRun) error {
	ids := []string{}
	indexByTask := map[string]int{}
	for index, node := range run.Nodes {
		if strings.TrimSpace(node.TaskID) != "" && node.Status != "succeeded" && node.Status != "failed" {
			ids = append(ids, node.TaskID)
			indexByTask[node.TaskID] = index
		}
	}
	if len(ids) > 0 {
		data, err := deps.api.GetCanvasGenerationProgress(ctx, ids)
		if err != nil {
			return err
		}
		for progressIndex := range data.Progresses {
			progress := data.Progresses[progressIndex]
			taskID := strings.TrimSpace(string(progress.TaskID))
			index, ok := indexByTask[taskID]
			if !ok {
				continue
			}
			run.Nodes[index].Progress = &progress
			if progress.Terminal() {
				if progress.Failed() {
					run.Nodes[index].Status = "failed"
					run.Nodes[index].Error = firstNonEmptyString(progress.ErrorMessage, progress.ErrorCode)
				} else {
					run.Nodes[index].Status = "succeeded"
					run.Nodes[index].Error = ""
				}
			} else {
				run.Nodes[index].Status = "running"
			}
		}
	}
	canvascore.RecomputeDAGRunStatus(run)
	_, err := canvascore.SaveDAGRun(pavoDirectory, run)
	return err
}

func newCanvasDAGStatusCommand(stdout, stderr io.Writer, deps *dependencies) *cobra.Command {
	command := &cobra.Command{Use: "status RUN_ID", Short: "Refresh task states in a local DAG run manifest", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		pavoDirectory, err := currentPavoDirectory()
		if err != nil {
			return err
		}
		run, _, err := canvascore.LoadDAGRun(pavoDirectory, args[0])
		if err != nil {
			return err
		}
		if err := refreshDAGRun(command.Context(), deps, pavoDirectory, run); err != nil {
			return err
		}
		return output.WriteJSON(stdout, run)
	}}
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}

func newCanvasDAGResumeCommand(stdout, stderr io.Writer, deps *dependencies) *cobra.Command {
	options := dagExecutionOptions{}
	command := &cobra.Command{
		Use: "resume RUN_ID", Short: "Resume unresolved DAG nodes with their original idempotency request IDs", Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			pavoDirectory, err := currentPavoDirectory()
			if err != nil {
				return err
			}
			run, _, err := canvascore.LoadDAGRun(pavoDirectory, args[0])
			if err != nil {
				return err
			}
			plan, _, err := canvascore.LoadDAGPlan(pavoDirectory, run.PlanID)
			if err != nil {
				return err
			}
			if plan.PlanHash != run.PlanHash {
				return errors.New("run manifest 与 plan hash 不一致")
			}
			if err := refreshDAGRun(command.Context(), deps, pavoDirectory, run); err != nil {
				return err
			}
			if run.Status == "succeeded" || run.Status == "completed_with_errors" {
				return output.WriteJSON(stdout, run)
			}
			if err := ensurePendingDAGRunCurrent(command.Context(), deps, plan, run); err != nil {
				return err
			}
			if command.Flags().Changed("max-parallel") {
				if options.maxParallel <= 0 {
					return errors.New("--max-parallel 必须大于 0")
				}
				run.MaxParallel = options.maxParallel
			}
			ctx, cancel, err := contextWithTaskTimeout(command.Context(), options.timeout)
			if err != nil {
				return err
			}
			defer cancel()
			executionErr := executeDAGRun(ctx, deps, stderr, pavoDirectory, run, options.interval)
			if writeErr := output.WriteJSON(stdout, run); writeErr != nil {
				return writeErr
			}
			return executionErr
		},
	}
	flags := command.Flags()
	flags.IntVar(&options.maxParallel, "max-parallel", 4, "override maximum parallel nodes for this resume")
	flags.DurationVar(&options.interval, "interval", 3*time.Second, "task polling interval")
	flags.DurationVar(&options.timeout, "timeout", 2*time.Hour, "maximum time for this resume invocation")
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}
