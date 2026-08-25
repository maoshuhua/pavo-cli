package cmd

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/maoshuhua/pavo-cli/internal/api"
	canvascore "github.com/maoshuhua/pavo-cli/internal/canvas"
	"github.com/maoshuhua/pavo-cli/internal/output"
	"github.com/spf13/cobra"
)

func newCanvasProjectCommand(stdout, stderr io.Writer, deps *dependencies, scopeOptions *canvasScopeOptions) *cobra.Command {
	command := &cobra.Command{Use: "project", Short: "Manage canvas projects", Args: cobra.NoArgs}
	command.AddCommand(newCanvasProjectListCommand(stdout, stderr, deps))
	command.AddCommand(newCanvasProjectCreateCommand(stdout, stderr, deps))
	command.AddCommand(newCanvasProjectShowCommand(stdout, stderr, deps, scopeOptions))
	command.AddCommand(newCanvasProjectUpdateCommand(stdout, stderr, deps, scopeOptions))
	command.AddCommand(newCanvasProjectDuplicateCommand(stdout, stderr, deps, scopeOptions))
	command.AddCommand(newCanvasProjectDeleteCommand(stdout, stderr, deps, scopeOptions))
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}

func newCanvasProjectListCommand(stdout, stderr io.Writer, deps *dependencies) *cobra.Command {
	command := &cobra.Command{
		Use:   "list",
		Short: "List canvas projects owned by the current user",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			entries, err := deps.api.ListCanvasProjects(command.Context())
			if err != nil {
				return err
			}
			for index := range entries.Items {
				entry := &entries.Items[index]
				entry.CanvasURL = canvascore.BuildURL(
					deps.config.AppBaseURL,
					canvascore.ProjectIDFromEntry(*entry),
					entry.ProjectUUID,
					canvascore.CanvasUUIDFromEntry(*entry),
				)
			}
			return output.WriteJSON(stdout, entries)
		},
	}
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}

func bindCreatedCanvas(created *api.CanvasProjectCreated, appBaseURL string) (*canvasBindingOutput, error) {
	directory, err := currentDirectory()
	if err != nil {
		return nil, err
	}
	sessionID, err := canvascore.RandomUUID()
	if err != nil {
		return nil, err
	}
	path, err := canvascore.WriteBinding(directory, canvascore.Binding{
		ProjectUUID: created.ProjectUUID,
		CanvasUUID:  created.CanvasUUID,
		SessionID:   sessionID,
	})
	if err != nil {
		return nil, err
	}
	projectID := canvascore.ProjectIDFromCreated(created)
	return &canvasBindingOutput{
		ProjectID:   projectID,
		ProjectUUID: created.ProjectUUID,
		CanvasUUID:  created.CanvasUUID,
		CanvasURL:   canvascore.BuildURL(appBaseURL, projectID, created.ProjectUUID, created.CanvasUUID),
		SessionID:   sessionID,
		BindingPath: path,
	}, nil
}

func newCanvasProjectCreateCommand(stdout, stderr io.Writer, deps *dependencies) *cobra.Command {
	var title string
	var coverURL string
	var use bool
	command := &cobra.Command{
		Use:   "create",
		Short: "Create a canvas project and its default canvas",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			title = strings.TrimSpace(title)
			if title == "" {
				return errors.New("缺少必填参数 --title")
			}
			created, err := deps.api.CreateCanvasProject(command.Context(), title, coverURL)
			if err != nil {
				return err
			}
			var binding *canvasBindingOutput
			if use {
				binding, err = bindCreatedCanvas(created, deps.config.AppBaseURL)
				if err != nil {
					return fmt.Errorf("项目已创建 project_uuid=%s canvas_uuid=%s，但写入工作区绑定失败: %w", created.ProjectUUID, created.CanvasUUID, err)
				}
			}
			projectID := canvascore.ProjectIDFromCreated(created)
			return output.WriteJSON(stdout, struct {
				Created   *api.CanvasProjectCreated `json:"created"`
				Binding   *canvasBindingOutput      `json:"binding,omitempty"`
				ProjectID string                    `json:"project_id,omitempty"`
				CanvasURL string                    `json:"canvas_url,omitempty"`
			}{
				Created:   created,
				Binding:   binding,
				ProjectID: projectID,
				CanvasURL: canvascore.BuildURL(deps.config.AppBaseURL, projectID, created.ProjectUUID, created.CanvasUUID),
			})
		},
	}
	flags := command.Flags()
	flags.StringVar(&title, "title", "", "project title")
	flags.StringVar(&coverURL, "cover-url", "", "optional project cover URL")
	flags.BoolVar(&use, "use", false, "bind the current workspace to the created project")
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}

func newCanvasProjectShowCommand(stdout, stderr io.Writer, deps *dependencies, options *canvasScopeOptions) *cobra.Command {
	command := &cobra.Command{
		Use:   "show",
		Short: "Show project metadata, nodes, connections, and version",
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
			projectID := canvascore.ProjectIDFromDetail(detail)
			return output.WriteJSON(stdout, struct {
				*api.CanvasProjectDetail
				ProjectID string `json:"project_id,omitempty"`
				CanvasURL string `json:"canvas_url,omitempty"`
			}{
				CanvasProjectDetail: detail,
				ProjectID:           projectID,
				CanvasURL:           canvascore.BuildURL(deps.config.AppBaseURL, projectID, scope.ProjectUUID, canvasUUID),
			})
		},
	}
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}

func newCanvasProjectUpdateCommand(stdout, stderr io.Writer, deps *dependencies, options *canvasScopeOptions) *cobra.Command {
	var title string
	var coverURL string
	command := &cobra.Command{
		Use:   "update",
		Short: "Update a canvas project title or cover",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			scope, err := resolveCanvasScope(options)
			if err != nil {
				return err
			}
			request := api.UpdateCanvasProjectRequest{}
			if command.Flags().Changed("title") {
				value := strings.TrimSpace(title)
				if value == "" {
					return errors.New("--title 不能为空")
				}
				request.Title = &value
			}
			if command.Flags().Changed("cover-url") {
				value := strings.TrimSpace(coverURL)
				request.CoverURL = &value
			}
			if request.Title == nil && request.CoverURL == nil {
				return errors.New("至少指定 --title 或 --cover-url")
			}
			data, err := deps.api.UpdateCanvasProject(command.Context(), scope.ProjectUUID, request)
			if err != nil {
				return err
			}
			return output.WriteJSON(stdout, map[string]any{"project_uuid": scope.ProjectUUID, "updated": true, "data": data})
		},
	}
	command.Flags().StringVar(&title, "title", "", "new project title")
	command.Flags().StringVar(&coverURL, "cover-url", "", "new project cover URL; pass an empty value to clear it")
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}

func newCanvasProjectDuplicateCommand(stdout, stderr io.Writer, deps *dependencies, options *canvasScopeOptions) *cobra.Command {
	var use bool
	command := &cobra.Command{
		Use:   "duplicate",
		Short: "Duplicate a canvas project, including nodes and connections",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			scope, err := resolveCanvasScope(options)
			if err != nil {
				return err
			}
			created, err := deps.api.DuplicateCanvasProject(command.Context(), scope.ProjectUUID)
			if err != nil {
				return err
			}
			var binding *canvasBindingOutput
			if use {
				binding, err = bindCreatedCanvas(created, deps.config.AppBaseURL)
				if err != nil {
					return fmt.Errorf("项目已复制 project_uuid=%s canvas_uuid=%s，但写入工作区绑定失败: %w", created.ProjectUUID, created.CanvasUUID, err)
				}
			}
			projectID := canvascore.ProjectIDFromCreated(created)
			return output.WriteJSON(stdout, struct {
				Created   *api.CanvasProjectCreated `json:"created"`
				Binding   *canvasBindingOutput      `json:"binding,omitempty"`
				ProjectID string                    `json:"project_id,omitempty"`
				CanvasURL string                    `json:"canvas_url,omitempty"`
			}{
				Created:   created,
				Binding:   binding,
				ProjectID: projectID,
				CanvasURL: canvascore.BuildURL(deps.config.AppBaseURL, projectID, created.ProjectUUID, created.CanvasUUID),
			})
		},
	}
	command.Flags().BoolVar(&use, "use", false, "bind the current workspace to the duplicated project")
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}

func newCanvasProjectDeleteCommand(stdout, stderr io.Writer, deps *dependencies, options *canvasScopeOptions) *cobra.Command {
	var yes bool
	command := &cobra.Command{
		Use:   "delete",
		Short: "Delete a canvas project",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if !yes {
				return errors.New("删除项目不可恢复；确认后请传 --yes")
			}
			scope, err := resolveCanvasScope(options)
			if err != nil {
				return err
			}
			data, err := deps.api.DeleteCanvasProject(command.Context(), scope.ProjectUUID)
			if err != nil {
				return err
			}
			return output.WriteJSON(stdout, map[string]any{"project_uuid": scope.ProjectUUID, "deleted": true, "data": data})
		},
	}
	command.Flags().BoolVar(&yes, "yes", false, "confirm irreversible project deletion")
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}
