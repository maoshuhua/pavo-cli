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

type canvasShortcutApplyOutput struct {
	ProjectUUID string                  `json:"project_uuid"`
	CanvasUUID  string                  `json:"canvas_uuid"`
	CanvasURL   string                  `json:"canvas_url,omitempty"`
	Version     int64                   `json:"version,omitempty"`
	DryRun      bool                    `json:"dry_run"`
	Shortcut    canvascore.Shortcut     `json:"shortcut"`
	Aliases     map[string]string       `json:"aliases"`
	Counts      map[string]int          `json:"counts"`
	RunNodeKey  string                  `json:"run_node_key,omitempty"`
	Request     *api.CanvasBatchRequest `json:"request,omitempty"`
	Run         *canvasRunOutput        `json:"run,omitempty"`
}

func getCanvasToolSpecs(ctx context.Context, deps *dependencies) (*canvascore.ToolSpecs, error) {
	raw, err := deps.api.GetCanvasToolSpecs(ctx)
	if err != nil {
		return nil, err
	}
	return canvascore.ParseToolSpecs(raw)
}

func shortcutModelResolver(ctx context.Context, deps *dependencies, specs *canvascore.ToolSpecs) canvascore.ShortcutModelResolver {
	cache := map[string]json.RawMessage{}
	return func(nodeType, configuredModel string) (string, error) {
		configuredModel = strings.TrimSpace(configuredModel)
		if strings.TrimSpace(nodeType) == "text" {
			if configuredModel == "" {
				return specs.DefaultTextModel("text_common")
			}
			model := specs.FindTextModel(configuredModel)
			if model == nil {
				return "", fmt.Errorf("实时 textModels 中找不到 %q", configuredModel)
			}
			if model.IsOnline != nil && !*model.IsOnline {
				return "", fmt.Errorf("文本模型 %q 当前未上线", configuredModel)
			}
			return configuredModel, nil
		}
		scene := canvascore.ModelScene(nodeType)
		if scene == "" {
			return configuredModel, nil
		}
		raw, ok := cache[scene]
		if !ok {
			var err error
			raw, err = deps.api.GetCanvasModelOptions(ctx, scene)
			if err != nil {
				return "", err
			}
			cache[scene] = raw
		}
		if configuredModel == "" {
			return canvascore.FirstAvailableModelCode(raw)
		}
		probe := map[string]any{}
		if err := canvascore.ApplyModelConfiguration(probe, nodeType, configuredModel, raw); err != nil {
			return "", err
		}
		return configuredModel, nil
	}
}

func newCanvasShortcutCommand(stdout, stderr io.Writer, deps *dependencies, scopeOptions *canvasScopeOptions) *cobra.Command {
	command := &cobra.Command{Use: "shortcut", Short: "Discover and apply live PAVO canvas presets", Args: cobra.NoArgs}
	command.AddCommand(newCanvasShortcutListCommand(stdout, stderr, deps))
	command.AddCommand(newCanvasShortcutShowCommand(stdout, stderr, deps))
	command.AddCommand(newCanvasShortcutApplyCommand(stdout, stderr, deps, scopeOptions))
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}

func newCanvasShortcutListCommand(stdout, stderr io.Writer, deps *dependencies) *cobra.Command {
	var kind, nodeType string
	command := &cobra.Command{
		Use:   "list",
		Short: "List guide, skill, and mode shortcuts from live tool specs",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			specs, err := getCanvasToolSpecs(command.Context(), deps)
			if err != nil {
				return err
			}
			shortcuts := []canvascore.Shortcut{}
			for _, shortcut := range specs.Shortcuts() {
				if strings.TrimSpace(kind) != "" && shortcut.Kind != strings.TrimSpace(kind) {
					continue
				}
				if strings.TrimSpace(nodeType) != "" && shortcut.NodeType != strings.TrimSpace(nodeType) {
					continue
				}
				shortcuts = append(shortcuts, shortcut)
			}
			return output.WriteJSON(stdout, map[string]any{"tool_spec_version": specs.Version, "shortcuts": shortcuts})
		},
	}
	command.Flags().StringVar(&kind, "kind", "", "filter by guide, skill, or mode")
	command.Flags().StringVar(&nodeType, "type", "", "filter by canvas node type")
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}

func newCanvasShortcutShowCommand(stdout, stderr io.Writer, deps *dependencies) *cobra.Command {
	command := &cobra.Command{
		Use:   "show CODE",
		Short: "Show one normalized shortcut and its required inputs",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			specs, err := getCanvasToolSpecs(command.Context(), deps)
			if err != nil {
				return err
			}
			shortcut, err := specs.FindShortcut(args[0])
			if err != nil {
				return err
			}
			return output.WriteJSON(stdout, shortcut)
		},
	}
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}

func parseShortcutInputs(values []string) (map[string]string, error) {
	result := map[string]string{}
	for _, value := range values {
		key, ref, ok := strings.Cut(value, "=")
		key, ref = strings.TrimSpace(key), strings.TrimSpace(ref)
		if !ok || key == "" || ref == "" {
			return nil, fmt.Errorf("--input %q 无效，应为 KEY=NODE", value)
		}
		if _, exists := result[key]; exists {
			return nil, fmt.Errorf("--input key %q 重复", key)
		}
		result[key] = ref
	}
	return result, nil
}

func resolvePlanRunRef(ref string, aliases map[string]string) string {
	ref = strings.TrimSpace(ref)
	if strings.HasPrefix(ref, "$") {
		return aliases[strings.TrimPrefix(ref, "$")]
	}
	return ref
}

func newCanvasShortcutApplyCommand(stdout, stderr io.Writer, deps *dependencies, scopeOptions *canvasScopeOptions) *cobra.Command {
	var source, target, name, prompt, model string
	var inputValues []string
	var useExample, dryRun, run, wait, force, download bool
	var outputDir string
	var interval, timeout time.Duration
	command := &cobra.Command{
		Use:   "apply CODE",
		Short: "Apply a live preset as one atomic canvas mutation",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if dryRun && run {
				return errors.New("--dry-run 不能与 --run 同时使用")
			}
			inputs, err := parseShortcutInputs(inputValues)
			if err != nil {
				return err
			}
			scope, err := resolveCanvasScope(scopeOptions)
			if err != nil {
				return err
			}
			specs, err := getCanvasToolSpecs(command.Context(), deps)
			if err != nil {
				return err
			}
			shortcut, err := specs.FindShortcut(args[0])
			if err != nil {
				return err
			}
			plan, err := canvascore.BuildShortcutPlan(*shortcut, canvascore.ShortcutApplyOptions{Source: source, Target: target, Name: name, Prompt: prompt, Model: model, Inputs: inputs, UseExampleInput: useExample}, shortcutModelResolver(command.Context(), deps, specs))
			if err != nil {
				return err
			}
			configureModel := ndjsonModelConfigurator(command.Context(), deps)
			resultOutput := &canvasShortcutApplyOutput{ProjectUUID: scope.ProjectUUID, CanvasUUID: scope.CanvasUUID, DryRun: dryRun, Shortcut: *shortcut}
			if dryRun {
				detail, err := deps.api.GetCanvasProjectDetail(command.Context(), scope.ProjectUUID, scope.CanvasUUID)
				if err != nil {
					return err
				}
				compiled, err := canvascore.ApplyNDJSONWithModelConfigurator(detail, plan.Operations, configureModel)
				if err != nil {
					return err
				}
				compiled.Request.CanvasUUID = firstNonEmptyString(scope.CanvasUUID, detail.CurrentCanvas.CanvasUUID)
				compiled.Request.Version = int64(detail.Version)
				compiled.Request.SessionID = scope.SessionID
				resultOutput.CanvasUUID = compiled.Request.CanvasUUID
				resultOutput.CanvasURL = canvascore.BuildURL(deps.config.AppBaseURL, canvascore.ProjectIDFromDetail(detail), scope.ProjectUUID, compiled.Request.CanvasUUID)
				resultOutput.Aliases = compiled.Aliases
				resultOutput.Counts = compiled.Counts
				resultOutput.RunNodeKey = resolvePlanRunRef(plan.RunRef, compiled.Aliases)
				resultOutput.Request = compiled.Request
				return output.WriteJSON(stdout, resultOutput)
			}
			var compiled *canvascore.NDJSONApplyResult
			var projectID, canvasUUID string
			mutation, err := canvascore.ApplyMutation(command.Context(), deps.api, scope, func(detail *api.CanvasProjectDetail) (*api.CanvasBatchRequest, error) {
				projectID = canvascore.ProjectIDFromDetail(detail)
				canvasUUID = firstNonEmptyString(scope.CanvasUUID, detail.CurrentCanvas.CanvasUUID)
				var compileErr error
				compiled, compileErr = canvascore.ApplyNDJSONWithModelConfigurator(detail, plan.Operations, configureModel)
				if compileErr != nil {
					return nil, compileErr
				}
				return compiled.Request, nil
			})
			if err != nil {
				return err
			}
			resultOutput.CanvasUUID = canvasUUID
			resultOutput.CanvasURL = canvascore.BuildURL(deps.config.AppBaseURL, projectID, scope.ProjectUUID, canvasUUID)
			resultOutput.Version = mutationVersion(mutation)
			resultOutput.Aliases = compiled.Aliases
			resultOutput.Counts = compiled.Counts
			resultOutput.RunNodeKey = resolvePlanRunRef(plan.RunRef, compiled.Aliases)
			if run {
				if resultOutput.RunNodeKey == "" {
					return errors.New("shortcut 没有可执行目标节点")
				}
				runResult, err := executeCanvasNodeRun(command.Context(), stderr, deps, scope, resultOutput.RunNodeKey, canvasNodeRunOptions{Wait: wait, Force: force, Download: download, OutputDir: outputDir, Interval: interval, Timeout: timeout})
				if err != nil {
					return err
				}
				resultOutput.Run = runResult
			}
			return output.WriteJSON(stdout, resultOutput)
		},
	}
	flags := command.Flags()
	flags.StringVar(&source, "source", "", "source node for a skill shortcut")
	flags.StringVar(&target, "target", "", "existing node to update instead of creating the shortcut self node")
	flags.StringSliceVar(&inputValues, "input", nil, "bind a guide input as KEY=NODE; repeatable")
	flags.StringVar(&name, "name", "", "override the created node or group name")
	flags.StringVar(&prompt, "prompt", "", "override the shortcut text prompt while preserving skill segments")
	flags.StringVar(&model, "model", "", "override and validate the live model code")
	flags.BoolVar(&useExample, "use-example-input", false, "materialize example input URLs supplied by the live guide")
	flags.BoolVar(&dryRun, "dry-run", false, "validate and print the batch without mutating the canvas")
	flags.BoolVar(&run, "run", false, "run the shortcut target after applying the graph mutation")
	flags.BoolVar(&wait, "wait", true, "wait for terminal task status when --run is set")
	flags.BoolVar(&force, "force", false, "allow repeat task submission when --run is set")
	flags.BoolVar(&download, "download", false, "download generated assets after --run completes")
	flags.StringVar(&outputDir, "output-dir", "", "directory for downloaded assets; implies --download")
	flags.DurationVar(&interval, "interval", 3*time.Second, "progress polling interval for --run")
	flags.DurationVar(&timeout, "timeout", 30*time.Minute, "maximum wait time for --run")
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}
