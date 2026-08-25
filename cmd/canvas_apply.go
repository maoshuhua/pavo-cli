package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/maoshuhua/pavo-cli/internal/api"
	canvascore "github.com/maoshuhua/pavo-cli/internal/canvas"
	"github.com/maoshuhua/pavo-cli/internal/output"
	"github.com/spf13/cobra"
)

type canvasApplyOutput struct {
	ProjectUUID string                  `json:"project_uuid"`
	CanvasUUID  string                  `json:"canvas_uuid"`
	Version     int64                   `json:"version,omitempty"`
	DryRun      bool                    `json:"dry_run"`
	Aliases     map[string]string       `json:"aliases"`
	Counts      map[string]int          `json:"counts"`
	Request     *api.CanvasBatchRequest `json:"request,omitempty"`
}

func ndjsonModelConfigurator(ctx context.Context, deps *dependencies) canvascore.NDJSONModelConfigurator {
	cache := map[string]json.RawMessage{}
	return func(nodeType, model string, data map[string]any) error {
		scene := canvascore.ModelScene(nodeType)
		if scene == "" {
			canvascore.SetModel(data, model)
			return nil
		}
		options, ok := cache[scene]
		if !ok {
			var err error
			options, err = deps.api.GetCanvasModelOptions(ctx, scene)
			if err != nil {
				return err
			}
			cache[scene] = options
		}
		return canvascore.ApplyModelConfiguration(data, nodeType, model, options)
	}
}

func newCanvasApplyCommand(stdout, stderr io.Writer, deps *dependencies, options *canvasScopeOptions) *cobra.Command {
	var stdin, dryRun, yes bool
	command := &cobra.Command{
		Use: "apply", Short: "Atomically apply graph mutations from stdin NDJSON", Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if !stdin {
				return errors.New("当前只支持 --stdin")
			}
			operations, err := canvascore.ParseNDJSON(command.InOrStdin())
			if err != nil {
				return err
			}
			if canvascore.NDJSONHasDestructiveOperations(operations) && !yes {
				return errors.New("NDJSON 包含删除或解组操作；确认后请传 --yes")
			}
			scope, err := resolveCanvasScope(options)
			if err != nil {
				return err
			}
			configureModel := ndjsonModelConfigurator(command.Context(), deps)
			if dryRun {
				detail, detailErr := deps.api.GetCanvasProjectDetail(command.Context(), scope.ProjectUUID, scope.CanvasUUID)
				if detailErr != nil {
					return detailErr
				}
				compiled, compileErr := canvascore.ApplyNDJSONWithModelConfigurator(detail, operations, configureModel)
				if compileErr != nil {
					return compileErr
				}
				compiled.Request.CanvasUUID = firstNonEmptyString(scope.CanvasUUID, detail.CurrentCanvas.CanvasUUID)
				compiled.Request.Version = int64(detail.Version)
				compiled.Request.SessionID = scope.SessionID
				return output.WriteJSON(stdout, canvasApplyOutput{ProjectUUID: scope.ProjectUUID, CanvasUUID: compiled.Request.CanvasUUID, DryRun: true, Aliases: compiled.Aliases, Counts: compiled.Counts, Request: compiled.Request})
			}
			var aliases map[string]string
			var counts map[string]int
			result, err := canvascore.ApplyMutation(command.Context(), deps.api, scope, func(detail *api.CanvasProjectDetail) (*api.CanvasBatchRequest, error) {
				compiled, compileErr := canvascore.ApplyNDJSONWithModelConfigurator(detail, operations, configureModel)
				if compileErr != nil {
					return nil, compileErr
				}
				aliases = compiled.Aliases
				counts = compiled.Counts
				return compiled.Request, nil
			})
			if err != nil {
				return err
			}
			return output.WriteJSON(stdout, canvasApplyOutput{ProjectUUID: scope.ProjectUUID, CanvasUUID: strings.TrimSpace(scope.CanvasUUID), Version: mutationVersion(result), DryRun: false, Aliases: aliases, Counts: counts})
		},
	}
	command.Flags().BoolVar(&stdin, "stdin", false, "read one JSON operation per line from stdin")
	command.Flags().BoolVar(&dryRun, "dry-run", false, "validate and print the batch without mutating the canvas")
	command.Flags().BoolVar(&yes, "yes", false, "confirm destructive operations present in the NDJSON stream")
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
