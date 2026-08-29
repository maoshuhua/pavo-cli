package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/maoshuhua/pavo-cli/internal/api"
	canvascore "github.com/maoshuhua/pavo-cli/internal/canvas"
	"github.com/maoshuhua/pavo-cli/internal/output"
	"github.com/spf13/cobra"
)

type canvasStoryboardOutput struct {
	ProjectUUID string                           `json:"project_uuid"`
	CanvasUUID  string                           `json:"canvas_uuid"`
	CanvasURL   string                           `json:"canvas_url,omitempty"`
	Version     int64                            `json:"version,omitempty"`
	Operation   string                           `json:"operation"`
	DryRun      bool                             `json:"dry_run,omitempty"`
	NodeKey     string                           `json:"node_key"`
	Storyboard  *canvascore.Storyboard           `json:"storyboard,omitempty"`
	Assets      []canvascore.StoryboardAsset     `json:"assets,omitempty"`
	GroupKey    string                           `json:"group_key,omitempty"`
	Request     *api.CanvasBatchRequest          `json:"request,omitempty"`
	Run         *canvasRunOutput                 `json:"run,omitempty"`
	Changed     *bool                            `json:"changed,omitempty"`
	Lint        *canvascore.StoryboardLintResult `json:"lint,omitempty"`
}

func newCanvasStoryboardCommand(stdout, stderr io.Writer, deps *dependencies, scopeOptions *canvasScopeOptions) *cobra.Command {
	command := &cobra.Command{Use: "storyboard", Short: "Create and compile structured CLI-only storyboards", Args: cobra.NoArgs}
	command.AddCommand(newCanvasStoryboardCreateCommand(stdout, stderr, deps, scopeOptions))
	command.AddCommand(newCanvasStoryboardSchemaCommand(stdout, stderr))
	command.AddCommand(newCanvasStoryboardProfileCommand(stdout, stderr))
	command.AddCommand(newCanvasStoryboardTemplateCommand(stdout, stderr))
	command.AddCommand(newCanvasStoryboardLintCommand(stdout, stderr))
	command.AddCommand(newCanvasStoryboardCompileCommand(stdout, stderr))
	command.AddCommand(newCanvasStoryboardGenerateCommand(stdout, stderr, deps, scopeOptions))
	command.AddCommand(newCanvasStoryboardFinalizeCommand(stdout, stderr, deps, scopeOptions))
	command.AddCommand(newCanvasStoryboardImportCommand(stdout, stderr, deps, scopeOptions))
	command.AddCommand(newCanvasStoryboardShowCommand(stdout, stderr, deps, scopeOptions))
	command.AddCommand(newCanvasStoryboardValidateCommand(stdout, stderr, deps, scopeOptions))
	command.AddCommand(newCanvasStoryboardExportCommand(stdout, stderr, deps, scopeOptions))
	command.AddCommand(newCanvasStoryboardBuildCommand(stdout, stderr, deps, scopeOptions))
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}

func newCanvasStoryboardSchemaCommand(stdout, stderr io.Writer) *cobra.Command {
	command := &cobra.Command{
		Use:   "schema",
		Short: "Print the canonical pavo.storyboard/v1 JSON Schema",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return output.WriteJSON(stdout, canvascore.StoryboardJSONSchema())
		},
	}
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}

func writeStoryboardJSON(stdout io.Writer, destination, operation string, value any) error {
	destination = strings.TrimSpace(destination)
	if destination == "" || destination == "-" {
		return output.WriteJSON(stdout, value)
	}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(destination, append(raw, '\n'), 0o600); err != nil {
		return err
	}
	return output.WriteJSON(stdout, map[string]any{"operation": operation, "output": destination})
}

func readStoryboardRaw(path string, stdin io.Reader) ([]byte, error) {
	if strings.TrimSpace(path) == "-" {
		return io.ReadAll(stdin)
	}
	return os.ReadFile(path)
}

func newCanvasStoryboardProfileCommand(stdout, stderr io.Writer) *cobra.Command {
	command := &cobra.Command{Use: "profile", Short: "List or inspect deterministic storyboard creative profiles", Args: cobra.NoArgs}
	list := &cobra.Command{
		Use: "list", Short: "List storyboard creative profiles", Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return output.WriteJSON(stdout, map[string]any{"profiles": canvascore.StoryboardProfiles()})
		},
	}
	show := &cobra.Command{
		Use: "show CODE", Short: "Show one storyboard creative profile", Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			profile, err := canvascore.FindStoryboardProfile(args[0])
			if err != nil {
				return err
			}
			return output.WriteJSON(stdout, profile)
		},
	}
	list.SetOut(stdout)
	list.SetErr(stderr)
	show.SetOut(stdout)
	show.SetErr(stderr)
	command.AddCommand(list, show)
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}

func newCanvasStoryboardTemplateCommand(stdout, stderr io.Writer) *cobra.Command {
	var profile, destination string
	var shots int
	command := &cobra.Command{
		Use: "template", Short: "Write an editable pavo.storyboard/v1 scaffold without contacting PAVO", Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			storyboard, err := canvascore.NewStoryboardTemplate(profile, shots)
			if err != nil {
				return err
			}
			return writeStoryboardJSON(stdout, destination, "storyboard.template", storyboard)
		},
	}
	command.Flags().StringVar(&profile, "profile", "cinematic", "creative profile: auto, cinematic, commercial, animation, or documentary")
	command.Flags().IntVar(&shots, "shots", 6, "number of editable shot rows")
	command.Flags().StringVarP(&destination, "output", "o", "-", "output JSON path; - writes to stdout")
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}

func newCanvasStoryboardLintCommand(stdout, stderr io.Writer) *cobra.Command {
	var strict bool
	command := &cobra.Command{
		Use: "lint FILE", Short: "Validate storyboard Schema and continuity quality without contacting PAVO; FILE may be -", Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			raw, err := readStoryboardRaw(args[0], command.InOrStdin())
			if err != nil {
				return err
			}
			lint := canvascore.LintStoryboard(raw)
			if err := output.WriteJSON(stdout, lint); err != nil {
				return err
			}
			if !lint.Valid {
				return errors.New("storyboard lint 发现 Schema error")
			}
			if strict && !lint.QualityReady {
				return errors.New("storyboard lint --strict 发现质量或连续性 warning")
			}
			return nil
		},
	}
	command.Flags().BoolVar(&strict, "strict", false, "return a non-zero status when quality warnings exist")
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}

func newCanvasStoryboardCompileCommand(stdout, stderr io.Writer) *cobra.Command {
	var kind, destination string
	var strict bool
	command := &cobra.Command{
		Use: "compile FILE", Short: "Compile each shot into deterministic image/video prompts without contacting PAVO; FILE may be -", Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			raw, err := readStoryboardRaw(args[0], command.InOrStdin())
			if err != nil {
				return err
			}
			lint := canvascore.LintStoryboard(raw)
			if !lint.Valid {
				return errors.New("storyboard compile 前必须先修复 `storyboard lint` 报告的 Schema error")
			}
			if strict && !lint.QualityReady {
				return errors.New("storyboard compile --strict 拒绝包含质量或连续性 warning 的输入")
			}
			compiled, err := canvascore.CompileStoryboard(lint.Storyboard, kind)
			if err != nil {
				return err
			}
			return writeStoryboardJSON(stdout, destination, "storyboard.compile", compiled)
		},
	}
	command.Flags().StringVar(&kind, "kind", "all", "prompt kind: all, image, or video")
	command.Flags().BoolVar(&strict, "strict", false, "refuse quality or continuity warnings")
	command.Flags().StringVarP(&destination, "output", "o", "-", "output JSON path; - writes to stdout")
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}

func storyboardCanvasURL(deps *dependencies, scope canvascore.Scope, detail *api.CanvasProjectDetail) (string, string) {
	canvasUUID := firstNonEmptyString(scope.CanvasUUID, detail.CurrentCanvas.CanvasUUID)
	return canvasUUID, canvascore.BuildURL(deps.config.AppBaseURL, canvascore.ProjectIDFromDetail(detail), scope.ProjectUUID, canvasUUID)
}

func newCanvasStoryboardCreateCommand(stdout, stderr io.Writer, deps *dependencies, scopeOptions *canvasScopeOptions) *cobra.Command {
	var title, brief, model, profile string
	var shots int
	var x, y float64
	command := &cobra.Command{
		Use:   "create",
		Short: "Create an executable text node with the strict storyboard generation prompt",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			title, brief = strings.TrimSpace(title), strings.TrimSpace(brief)
			if title == "" || brief == "" {
				return errors.New("缺少必填参数 --title 或 --brief")
			}
			if shots <= 0 || shots > 100 {
				return errors.New("--shots 必须在 1 到 100 之间")
			}
			if _, err := canvascore.FindStoryboardProfile(profile); err != nil {
				return err
			}
			specs, err := getCanvasToolSpecs(command.Context(), deps)
			if err != nil {
				return err
			}
			resolvedModel, err := shortcutModelResolver(command.Context(), deps, specs)("text", model)
			if err != nil {
				return err
			}
			scope, err := resolveCanvasScope(scopeOptions)
			if err != nil {
				return err
			}
			var nodeKey, canvasUUID, canvasURL string
			result, err := canvascore.ApplyMutation(command.Context(), deps.api, scope, func(detail *api.CanvasProjectDetail) (*api.CanvasBatchRequest, error) {
				canvasUUID, canvasURL = storyboardCanvasURL(deps, scope, detail)
				data := map[string]any{
					"pavo_storyboard_request": map[string]any{"schema_version": canvascore.StoryboardSchemaVersion, "title": title, "brief": brief, "shots": shots, "profile": profile},
					"pavo_storyboard_schema":  canvascore.StoryboardSchemaVersion,
				}
				options := canvascore.NewNodeOptions{Type: "text", Name: title + " · Storyboard", Prompt: canvascore.StoryboardGenerationPromptWithProfile(title, brief, shots, profile), Model: resolvedModel, Width: 480, Height: 560, Data: data}
				if command.Flags().Changed("x") {
					options.X = &x
				}
				if command.Flags().Changed("y") {
					options.Y = &y
				}
				item, err := canvascore.NewNode(detail, options)
				if err != nil {
					return nil, err
				}
				nodeKey = item.NodeKey
				request := canvascore.NewBatchRequest()
				request.Nodes.Create = append(request.Nodes.Create, *item)
				return request, nil
			})
			if err != nil {
				return err
			}
			return output.WriteJSON(stdout, canvasStoryboardOutput{ProjectUUID: scope.ProjectUUID, CanvasUUID: canvasUUID, CanvasURL: canvasURL, Version: mutationVersion(result), Operation: "storyboard.create", NodeKey: nodeKey})
		},
	}
	flags := command.Flags()
	flags.StringVar(&title, "title", "", "storyboard title")
	flags.StringVar(&brief, "brief", "", "story brief and creative requirements")
	flags.IntVar(&shots, "shots", 8, "exact number of shots to request")
	flags.StringVar(&model, "model", "", "live text model code; defaults to the first online text_common model")
	flags.StringVar(&profile, "profile", "auto", "creative profile: auto, cinematic, commercial, animation, or documentary")
	flags.Float64Var(&x, "x", 0, "node X position")
	flags.Float64Var(&y, "y", 0, "node Y position")
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}

func finalizeStoryboardNode(ctx context.Context, deps *dependencies, scope canvascore.Scope, nodeRef string) (*canvascore.Storyboard, string, string, string, int64, error) {
	var storyboard *canvascore.Storyboard
	var nodeKey, canvasUUID, canvasURL string
	result, err := canvascore.ApplyMutation(ctx, deps.api, scope, func(detail *api.CanvasProjectDetail) (*api.CanvasBatchRequest, error) {
		canvasUUID, canvasURL = storyboardCanvasURL(deps, scope, detail)
		node, err := canvascore.FindNode(detail, nodeRef)
		if err != nil {
			return nil, err
		}
		if string(node.NodeType) != "text" {
			return nil, fmt.Errorf("节点 %s 不是 text 节点", node.NodeKey)
		}
		storyboard, err = canvascore.StoryboardFromNode(*node)
		if err != nil {
			return nil, err
		}
		data, err := canvascore.NodeData(*node)
		if err != nil {
			return nil, err
		}
		if err := canvascore.SetStoryboardNodeData(data, storyboard); err != nil {
			return nil, err
		}
		item, err := canvascore.WriteItemFromNode(*node, data)
		if err != nil {
			return nil, err
		}
		nodeKey = node.NodeKey
		request := canvascore.NewBatchRequest()
		request.Nodes.Update = append(request.Nodes.Update, item)
		return request, nil
	})
	if err != nil {
		return nil, "", "", "", 0, err
	}
	return storyboard, nodeKey, canvasUUID, canvasURL, mutationVersion(result), nil
}

func newCanvasStoryboardFinalizeCommand(stdout, stderr io.Writer, deps *dependencies, scopeOptions *canvasScopeOptions) *cobra.Command {
	command := &cobra.Command{
		Use:   "finalize NODE",
		Short: "Parse generated text as storyboard JSON, validate it, and persist the Schema",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			scope, err := resolveCanvasScope(scopeOptions)
			if err != nil {
				return err
			}
			storyboard, nodeKey, canvasUUID, canvasURL, version, err := finalizeStoryboardNode(command.Context(), deps, scope, args[0])
			if err != nil {
				return err
			}
			return output.WriteJSON(stdout, canvasStoryboardOutput{ProjectUUID: scope.ProjectUUID, CanvasUUID: canvasUUID, CanvasURL: canvasURL, Version: version, Operation: "storyboard.finalize", NodeKey: nodeKey, Storyboard: storyboard})
		},
	}
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}

func newCanvasStoryboardGenerateCommand(stdout, stderr io.Writer, deps *dependencies, scopeOptions *canvasScopeOptions) *cobra.Command {
	var force bool
	var interval, timeout time.Duration
	command := &cobra.Command{
		Use:   "generate NODE",
		Short: "Run a storyboard request node, wait, then validate and finalize its JSON",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			scope, err := resolveCanvasScope(scopeOptions)
			if err != nil {
				return err
			}
			runResult, err := executeCanvasNodeRun(command.Context(), stderr, deps, scope, args[0], canvasNodeRunOptions{Wait: true, Force: force, Interval: interval, Timeout: timeout})
			if err != nil {
				return err
			}
			storyboard, nodeKey, canvasUUID, canvasURL, version, err := finalizeStoryboardNode(command.Context(), deps, scope, runResult.NodeKey)
			if err != nil {
				return err
			}
			return output.WriteJSON(stdout, canvasStoryboardOutput{ProjectUUID: scope.ProjectUUID, CanvasUUID: canvasUUID, CanvasURL: canvasURL, Version: version, Operation: "storyboard.generate", NodeKey: nodeKey, Storyboard: storyboard, Run: runResult})
		},
	}
	command.Flags().BoolVar(&force, "force", false, "resubmit even when the node contains an existing task_id")
	command.Flags().DurationVar(&interval, "interval", 3*time.Second, "progress polling interval")
	command.Flags().DurationVar(&timeout, "timeout", 30*time.Minute, "maximum generation wait time")
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}

func readStoryboardFile(path string, stdin io.Reader) (*canvascore.Storyboard, error) {
	var raw []byte
	var err error
	if strings.TrimSpace(path) == "-" {
		raw, err = io.ReadAll(stdin)
	} else {
		raw, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, err
	}
	return canvascore.ParseStoryboard(raw)
}

func newCanvasStoryboardImportCommand(stdout, stderr io.Writer, deps *dependencies, scopeOptions *canvasScopeOptions) *cobra.Command {
	var nodeRef, name string
	command := &cobra.Command{
		Use:   "import FILE",
		Short: "Validate and import storyboard JSON into a new or existing text node; FILE may be -",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			storyboard, err := readStoryboardFile(args[0], command.InOrStdin())
			if err != nil {
				return err
			}
			scope, err := resolveCanvasScope(scopeOptions)
			if err != nil {
				return err
			}
			var nodeKey, canvasUUID, canvasURL string
			result, err := canvascore.ApplyMutation(command.Context(), deps.api, scope, func(detail *api.CanvasProjectDetail) (*api.CanvasBatchRequest, error) {
				canvasUUID, canvasURL = storyboardCanvasURL(deps, scope, detail)
				request := canvascore.NewBatchRequest()
				if strings.TrimSpace(nodeRef) == "" {
					data := map[string]any{}
					if err := canvascore.SetStoryboardNodeData(data, storyboard); err != nil {
						return nil, err
					}
					item, err := canvascore.NewNode(detail, canvascore.NewNodeOptions{Type: "text", Name: firstNonEmptyString(name, storyboard.Title+" · Storyboard"), Width: 480, Height: 560, Data: data})
					if err != nil {
						return nil, err
					}
					nodeKey = item.NodeKey
					request.Nodes.Create = append(request.Nodes.Create, *item)
					return request, nil
				}
				node, err := canvascore.FindNode(detail, nodeRef)
				if err != nil {
					return nil, err
				}
				data, err := canvascore.NodeData(*node)
				if err != nil {
					return nil, err
				}
				if err := canvascore.SetStoryboardNodeData(data, storyboard); err != nil {
					return nil, err
				}
				if strings.TrimSpace(name) != "" {
					node.Name, data["title"], data["name"] = strings.TrimSpace(name), strings.TrimSpace(name), strings.TrimSpace(name)
				}
				item, err := canvascore.WriteItemFromNode(*node, data)
				if err != nil {
					return nil, err
				}
				nodeKey = node.NodeKey
				request.Nodes.Update = append(request.Nodes.Update, item)
				return request, nil
			})
			if err != nil {
				return err
			}
			return output.WriteJSON(stdout, canvasStoryboardOutput{ProjectUUID: scope.ProjectUUID, CanvasUUID: canvasUUID, CanvasURL: canvasURL, Version: mutationVersion(result), Operation: "storyboard.import", NodeKey: nodeKey, Storyboard: storyboard})
		},
	}
	command.Flags().StringVar(&nodeRef, "node", "", "existing text node to update; creates a node when omitted")
	command.Flags().StringVar(&name, "name", "", "override the storyboard node title")
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}

func loadStoryboardNode(ctx context.Context, deps *dependencies, scope canvascore.Scope, ref string) (*api.CanvasProjectDetail, *api.CanvasNode, *canvascore.Storyboard, error) {
	detail, err := deps.api.GetCanvasProjectDetail(ctx, scope.ProjectUUID, scope.CanvasUUID)
	if err != nil {
		return nil, nil, nil, err
	}
	node, err := canvascore.FindNode(detail, ref)
	if err != nil {
		return nil, nil, nil, err
	}
	storyboard, err := canvascore.StoryboardFromNode(*node)
	if err != nil {
		return nil, nil, nil, err
	}
	return detail, node, storyboard, nil
}

func newCanvasStoryboardShowCommand(stdout, stderr io.Writer, deps *dependencies, scopeOptions *canvasScopeOptions) *cobra.Command {
	command := &cobra.Command{
		Use: "show NODE", Short: "Show the structured storyboard stored in a text node", Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			scope, err := resolveCanvasScope(scopeOptions)
			if err != nil {
				return err
			}
			detail, node, storyboard, err := loadStoryboardNode(command.Context(), deps, scope, args[0])
			if err != nil {
				return err
			}
			canvasUUID, canvasURL := storyboardCanvasURL(deps, scope, detail)
			return output.WriteJSON(stdout, canvasStoryboardOutput{ProjectUUID: scope.ProjectUUID, CanvasUUID: canvasUUID, CanvasURL: canvasURL, Version: int64(detail.Version), Operation: "storyboard.show", NodeKey: node.NodeKey, Storyboard: storyboard})
		},
	}
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}

func newCanvasStoryboardValidateCommand(stdout, stderr io.Writer, deps *dependencies, scopeOptions *canvasScopeOptions) *cobra.Command {
	var strict bool
	command := &cobra.Command{
		Use: "validate NODE", Short: "Validate one persisted storyboard against pavo.storyboard/v1", Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			scope, err := resolveCanvasScope(scopeOptions)
			if err != nil {
				return err
			}
			_, node, storyboard, err := loadStoryboardNode(command.Context(), deps, scope, args[0])
			if err != nil {
				return err
			}
			raw, err := json.Marshal(storyboard)
			if err != nil {
				return err
			}
			lint := canvascore.LintStoryboard(raw)
			if err := output.WriteJSON(stdout, map[string]any{"node_key": node.NodeKey, "lint": lint}); err != nil {
				return err
			}
			if strict && !lint.QualityReady {
				return errors.New("storyboard validate --strict 发现质量或连续性 warning")
			}
			return nil
		},
	}
	command.Flags().BoolVar(&strict, "strict", false, "return a non-zero status when quality warnings exist")
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}

func newCanvasStoryboardExportCommand(stdout, stderr io.Writer, deps *dependencies, scopeOptions *canvasScopeOptions) *cobra.Command {
	var destination string
	command := &cobra.Command{
		Use: "export NODE", Short: "Export a structured storyboard as JSON", Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			scope, err := resolveCanvasScope(scopeOptions)
			if err != nil {
				return err
			}
			_, node, storyboard, err := loadStoryboardNode(command.Context(), deps, scope, args[0])
			if err != nil {
				return err
			}
			raw, err := json.MarshalIndent(storyboard, "", "  ")
			if err != nil {
				return err
			}
			if strings.TrimSpace(destination) == "" || destination == "-" {
				_, err = fmt.Fprintln(stdout, string(raw))
				return err
			}
			if err := os.WriteFile(destination, append(raw, '\n'), 0o600); err != nil {
				return err
			}
			return output.WriteJSON(stdout, map[string]any{"exported": true, "node_key": node.NodeKey, "output": destination})
		},
	}
	command.Flags().StringVarP(&destination, "output", "o", "-", "output JSON path; - writes to stdout")
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}

func resolveCanvasMediaModel(ctx context.Context, deps *dependencies, nodeType, requested string) (string, error) {
	scene := canvascore.ModelScene(nodeType)
	if scene == "" {
		return "", fmt.Errorf("node type %q 没有模型场景", nodeType)
	}
	raw, err := deps.api.GetCanvasModelOptions(ctx, scene)
	if err != nil {
		return "", err
	}
	requested = strings.TrimSpace(requested)
	if requested == "" {
		return canvascore.FirstAvailableModelCode(raw)
	}
	if err := canvascore.ApplyModelConfiguration(map[string]any{}, nodeType, requested, raw); err != nil {
		return "", err
	}
	return requested, nil
}

func checkStoryboardBuildQuality(storyboard *canvascore.Storyboard, strict bool, stderr io.Writer) (*canvascore.StoryboardLintResult, error) {
	raw, err := json.Marshal(storyboard)
	if err != nil {
		return nil, err
	}
	lint := canvascore.LintStoryboard(raw)
	if !lint.Valid {
		return &lint, errors.New("storyboard build 前必须修复 Schema error")
	}
	if lint.Warnings > 0 {
		_, _ = fmt.Fprintf(stderr, "storyboard quality warnings=%d；运行 `pavo canvas storyboard validate NODE --strict` 查看详情\n", lint.Warnings)
	}
	if strict && !lint.QualityReady {
		return &lint, errors.New("storyboard build --strict 拒绝包含质量或连续性 warning 的输入")
	}
	return &lint, nil
}

func newCanvasStoryboardBuildCommand(stdout, stderr io.Writer, deps *dependencies, scopeOptions *canvasScopeOptions) *cobra.Command {
	var imageModel, videoModel string
	var withVideo, dryRun, strict bool
	command := &cobra.Command{
		Use: "build NODE", Short: "Compile storyboard shots into stable prompt, asset, edge, and group mutations", Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			scope, err := resolveCanvasScope(scopeOptions)
			if err != nil {
				return err
			}
			imageModel, err = resolveCanvasMediaModel(command.Context(), deps, "image", imageModel)
			if err != nil {
				return err
			}
			if withVideo {
				videoModel, err = resolveCanvasMediaModel(command.Context(), deps, "video", videoModel)
				if err != nil {
					return err
				}
			}
			configure := ndjsonModelConfigurator(command.Context(), deps)
			if dryRun {
				detail, node, storyboard, err := loadStoryboardNode(command.Context(), deps, scope, args[0])
				if err != nil {
					return err
				}
				lint, err := checkStoryboardBuildQuality(storyboard, strict, stderr)
				if err != nil {
					return err
				}
				build, err := canvascore.BuildStoryboardGraph(detail, *node, storyboard, canvascore.StoryboardBuildOptions{ImageModel: imageModel, VideoModel: videoModel, WithVideo: withVideo}, configure)
				if err != nil {
					return err
				}
				build.Request.CanvasUUID = firstNonEmptyString(scope.CanvasUUID, detail.CurrentCanvas.CanvasUUID)
				build.Request.Version = int64(detail.Version)
				build.Request.SessionID = scope.SessionID
				canvasUUID, canvasURL := storyboardCanvasURL(deps, scope, detail)
				changed := build.Changed
				return output.WriteJSON(stdout, canvasStoryboardOutput{ProjectUUID: scope.ProjectUUID, CanvasUUID: canvasUUID, CanvasURL: canvasURL, Operation: "storyboard.build", DryRun: true, NodeKey: node.NodeKey, Assets: build.Assets, GroupKey: build.GroupKey, Request: build.Request, Changed: &changed, Lint: lint})
			}
			var build *canvascore.StoryboardBuildResult
			var nodeKey, canvasUUID, canvasURL string
			var lint *canvascore.StoryboardLintResult
			result, err := canvascore.ApplyMutation(command.Context(), deps.api, scope, func(detail *api.CanvasProjectDetail) (*api.CanvasBatchRequest, error) {
				node, err := canvascore.FindNode(detail, args[0])
				if err != nil {
					return nil, err
				}
				storyboard, err := canvascore.StoryboardFromNode(*node)
				if err != nil {
					return nil, err
				}
				lint, err = checkStoryboardBuildQuality(storyboard, strict, stderr)
				if err != nil {
					return nil, err
				}
				build, err = canvascore.BuildStoryboardGraph(detail, *node, storyboard, canvascore.StoryboardBuildOptions{ImageModel: imageModel, VideoModel: videoModel, WithVideo: withVideo}, configure)
				if err != nil {
					return nil, err
				}
				nodeKey = node.NodeKey
				canvasUUID, canvasURL = storyboardCanvasURL(deps, scope, detail)
				return build.Request, nil
			})
			if err != nil {
				return err
			}
			changed := build.Changed
			return output.WriteJSON(stdout, canvasStoryboardOutput{ProjectUUID: scope.ProjectUUID, CanvasUUID: canvasUUID, CanvasURL: canvasURL, Version: mutationVersion(result), Operation: "storyboard.build", NodeKey: nodeKey, Assets: build.Assets, GroupKey: build.GroupKey, Changed: &changed, Lint: lint})
		},
	}
	command.Flags().StringVar(&imageModel, "image-model", "", "live canvas image model; defaults to the first allowed online model")
	command.Flags().BoolVar(&withVideo, "with-video", false, "also create one video node per shot and connect its key frame")
	command.Flags().StringVar(&videoModel, "video-model", "", "live canvas video model; used with --with-video")
	command.Flags().BoolVar(&dryRun, "dry-run", false, "validate and print the batch without mutating the canvas")
	command.Flags().BoolVar(&strict, "strict", false, "refuse quality or continuity warnings before building nodes")
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}
