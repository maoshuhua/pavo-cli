package cmd

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"

	canvascore "github.com/maoshuhua/pavo-cli/internal/canvas"
	"github.com/maoshuhua/pavo-cli/internal/output"
	"github.com/spf13/cobra"
)

type canvasScopeOptions struct {
	projectUUID string
	canvasUUID  string
}

type canvasBindingOutput struct {
	ProjectID   string `json:"project_id,omitempty"`
	ProjectUUID string `json:"project_uuid"`
	CanvasUUID  string `json:"canvas_uuid"`
	CanvasURL   string `json:"canvas_url,omitempty"`
	SessionID   string `json:"session_id"`
	BindingPath string `json:"binding_path"`
}

func newCanvasCommand(stdout, stderr io.Writer, deps *dependencies) *cobra.Command {
	scopeOptions := &canvasScopeOptions{}
	command := &cobra.Command{
		Use:   "canvas",
		Short: "Create and operate Pixa infinite-canvas projects",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return command.Help()
		},
	}
	command.SetOut(stdout)
	command.SetErr(stderr)
	command.PersistentFlags().StringVar(&scopeOptions.projectUUID, "project", "", "canvas project UUID (defaults to .pavo/canvas.json)")
	command.PersistentFlags().StringVar(&scopeOptions.canvasUUID, "canvas", "", "canvas UUID (defaults to the bound/current canvas)")
	command.AddCommand(newCanvasUseCommand(stdout, stderr, deps, scopeOptions))
	command.AddCommand(newCanvasUnuseCommand(stdout, stderr))
	command.AddCommand(newCanvasStatusCommand(stdout, stderr, deps, scopeOptions))
	command.AddCommand(newCanvasProjectCommand(stdout, stderr, deps, scopeOptions))
	command.AddCommand(newCanvasNodeCommand(stdout, stderr, deps, scopeOptions))
	command.AddCommand(newCanvasEdgeCommand(stdout, stderr, deps, scopeOptions))
	command.AddCommand(newCanvasGroupCommand(stdout, stderr, deps, scopeOptions))
	command.AddCommand(newCanvasApplyCommand(stdout, stderr, deps, scopeOptions))
	command.AddCommand(newCanvasDAGCommand(stdout, stderr, deps, scopeOptions))
	command.AddCommand(newCanvasArtifactCommand(stdout, stderr, deps, scopeOptions))
	command.AddCommand(newCanvasUploadCommand(stdout, stderr, deps, scopeOptions))
	command.AddCommand(newCanvasModelCommand(stdout, stderr, deps))
	command.AddCommand(newCanvasToolSpecsCommand(stdout, stderr, deps))
	command.AddCommand(newCanvasRunCommand(stdout, stderr, deps, scopeOptions))
	command.AddCommand(newCanvasTaskCommand(stdout, stderr, deps))
	return command
}

func currentDirectory() (string, error) {
	directory, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return directory, nil
}

func resolveCanvasScope(options *canvasScopeOptions) (canvascore.Scope, error) {
	directory, err := currentDirectory()
	if err != nil {
		return canvascore.Scope{}, err
	}
	return canvascore.ResolveScope(directory, options.projectUUID, options.canvasUUID)
}

func newCanvasUseCommand(stdout, stderr io.Writer, deps *dependencies, options *canvasScopeOptions) *cobra.Command {
	command := &cobra.Command{
		Use:   "use",
		Short: "Bind the current workspace to a canvas project",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			projectUUID := strings.TrimSpace(options.projectUUID)
			if projectUUID == "" {
				return errors.New("缺少必填参数 --project")
			}
			detail, err := deps.api.GetCanvasProjectDetail(command.Context(), projectUUID, strings.TrimSpace(options.canvasUUID))
			if err != nil {
				return err
			}
			canvasUUID := strings.TrimSpace(options.canvasUUID)
			if canvasUUID == "" {
				canvasUUID = strings.TrimSpace(detail.CurrentCanvas.CanvasUUID)
			}
			if canvasUUID == "" {
				return errors.New("画布详情缺少 current_canvas.canvas_uuid")
			}
			sessionID, err := canvascore.RandomUUID()
			if err != nil {
				return err
			}
			directory, err := currentDirectory()
			if err != nil {
				return err
			}
			path, err := canvascore.WriteBinding(directory, canvascore.Binding{
				ProjectUUID: projectUUID,
				CanvasUUID:  canvasUUID,
				SessionID:   sessionID,
			})
			if err != nil {
				return err
			}
			projectID := canvascore.ProjectIDFromDetail(detail)
			return output.WriteJSON(stdout, canvasBindingOutput{
				ProjectID:   projectID,
				ProjectUUID: projectUUID,
				CanvasUUID:  canvasUUID,
				CanvasURL:   canvascore.BuildURL(deps.config.AppBaseURL, projectID, projectUUID, canvasUUID),
				SessionID:   sessionID,
				BindingPath: path,
			})
		},
	}
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}

func newCanvasUnuseCommand(stdout, stderr io.Writer) *cobra.Command {
	command := &cobra.Command{
		Use:   "unuse",
		Short: "Remove the nearest workspace canvas binding",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			directory, err := currentDirectory()
			if err != nil {
				return err
			}
			binding, path, err := canvascore.FindBinding(directory)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return errors.New("当前目录及其父目录没有 .pavo/canvas.json")
				}
				return err
			}
			if err := canvascore.RemoveBinding(path); err != nil {
				return err
			}
			return output.WriteJSON(stdout, map[string]any{"removed": true, "binding_path": path, "project_uuid": binding.ProjectUUID, "canvas_uuid": binding.CanvasUUID})
		},
	}
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}

func newCanvasStatusCommand(stdout, stderr io.Writer, deps *dependencies, options *canvasScopeOptions) *cobra.Command {
	command := &cobra.Command{
		Use:   "status",
		Short: "Show the effective canvas workspace binding",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			scope, err := resolveCanvasScope(options)
			if err != nil {
				return err
			}
			detail, err := deps.api.GetCanvasProjectDetail(command.Context(), scope.ProjectUUID, scope.CanvasUUID)
			if err != nil {
				return err
			}
			canvasUUID := firstNonEmptyString(scope.CanvasUUID, detail.CurrentCanvas.CanvasUUID)
			scope.CanvasUUID = canvasUUID
			projectID := canvascore.ProjectIDFromDetail(detail)
			return output.WriteJSON(stdout, struct {
				canvascore.Scope
				ProjectID string `json:"project_id,omitempty"`
				CanvasURL string `json:"canvas_url,omitempty"`
			}{
				Scope:     scope,
				ProjectID: projectID,
				CanvasURL: canvascore.BuildURL(deps.config.AppBaseURL, projectID, scope.ProjectUUID, canvasUUID),
			})
		},
	}
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}

func newCanvasModelCommand(stdout, stderr io.Writer, deps *dependencies) *cobra.Command {
	command := &cobra.Command{Use: "model", Short: "Inspect canvas model options", Args: cobra.NoArgs}
	var scene string
	list := &cobra.Command{
		Use:   "list",
		Short: "List live model options for a canvas scene",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			scene = strings.TrimSpace(scene)
			if scene == "" {
				return errors.New("缺少必填参数 --scene")
			}
			data, err := deps.api.GetCanvasModelOptions(command.Context(), scene)
			if err != nil {
				return err
			}
			return output.WriteJSON(stdout, struct {
				Scene  string          `json:"scene"`
				Models json.RawMessage `json:"models"`
			}{Scene: scene, Models: data})
		},
	}
	list.Flags().StringVar(&scene, "scene", "", "scene code, for example canvas_image, canvas_video, or canvas_audio")
	list.SetOut(stdout)
	list.SetErr(stderr)
	command.AddCommand(list)
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}

func newCanvasToolSpecsCommand(stdout, stderr io.Writer, deps *dependencies) *cobra.Command {
	command := &cobra.Command{
		Use:   "tool-specs",
		Short: "Show the live canvas node and text-tool specifications",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			data, err := deps.api.GetCanvasToolSpecs(command.Context())
			if err != nil {
				return err
			}
			return output.WriteJSON(stdout, data)
		},
	}
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}
