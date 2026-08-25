package cmd

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/maoshuhua/pavo-cli/internal/api"
	canvascore "github.com/maoshuhua/pavo-cli/internal/canvas"
	"github.com/maoshuhua/pavo-cli/internal/output"
	"github.com/spf13/cobra"
)

func newCanvasArtifactCommand(stdout, stderr io.Writer, deps *dependencies, options *canvasScopeOptions) *cobra.Command {
	command := &cobra.Command{Use: "artifact", Short: "List, download, delete, or save canvas artifacts", Args: cobra.NoArgs}
	command.AddCommand(newCanvasArtifactListCommand(stdout, stderr, deps, options))
	command.AddCommand(newCanvasArtifactDeleteCommand(stdout, stderr, deps, options))
	command.AddCommand(newCanvasArtifactSaveCommand(stdout, stderr, deps, options))
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}

func newCanvasArtifactListCommand(stdout, stderr io.Writer, deps *dependencies, options *canvasScopeOptions) *cobra.Command {
	var category, downloadDir string
	var page, pageSize int
	command := &cobra.Command{
		Use: "list", Short: "List artifacts grouped by generation date", Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			scope, err := resolveCanvasScope(options)
			if err != nil {
				return err
			}
			if strings.TrimSpace(downloadDir) != "" && !filepath.IsAbs(downloadDir) {
				return errors.New("--download-dir 必须是绝对路径")
			}
			artifacts, err := deps.api.ListCanvasArtifacts(command.Context(), scope.ProjectUUID, scope.CanvasUUID, category, page, pageSize)
			if err != nil {
				return err
			}
			if strings.TrimSpace(downloadDir) != "" {
				for groupIndex := range artifacts.Groups {
					group := &artifacts.Groups[groupIndex]
					for itemIndex := range group.List {
						item := &group.List[itemIndex]
						if strings.TrimSpace(item.URL) == "" {
							continue
						}
						item.LocalPath = ""
						item.DownloadError = ""
						extension := canvasResultExtension(map[string]any{"mime_type": item.MIMEType}, item.URL)
						base := safeFilenamePart(item.NodeName)
						if base == "" {
							base = safeFilenamePart(item.ArtifactUUID)
						}
						if base == "" {
							base = "artifact"
						}
						filename := fmt.Sprintf("%s-%03d-%s%s", safeFilenamePart(group.Date), itemIndex+1, base, extension)
						result, downloadErr := deps.api.DownloadResult(command.Context(), api.DownloadResultOptions{URL: item.URL, OutputPath: filepath.Join(downloadDir, filename)})
						if downloadErr != nil {
							item.DownloadError = downloadErr.Error()
						} else {
							item.LocalPath = result.OutputPath
						}
					}
				}
			}
			return output.WriteJSON(stdout, struct {
				ProjectUUID string `json:"project_uuid"`
				CanvasUUID  string `json:"canvas_uuid,omitempty"`
				Note        string `json:"pagination_note"`
				*api.CanvasArtifactList
			}{ProjectUUID: scope.ProjectUUID, CanvasUUID: scope.CanvasUUID, Note: "page_size 表示每页有产物的日期组数量，pagination.total 是日期总数", CanvasArtifactList: artifacts})
		},
	}
	flags := command.Flags()
	flags.StringVar(&category, "category", "all", "all, images, or videos")
	flags.IntVar(&page, "page", 1, "date-group page number")
	flags.IntVar(&pageSize, "page-size", 10, "number of non-empty date groups per page (1-100)")
	flags.StringVar(&downloadDir, "download-dir", "", "absolute directory to download every URL in this page")
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}

func newCanvasArtifactDeleteCommand(stdout, stderr io.Writer, deps *dependencies, options *canvasScopeOptions) *cobra.Command {
	var yes bool
	command := &cobra.Command{Use: "delete UUID [UUID...]", Short: "Idempotently soft-delete one or more artifact history records", Args: cobra.RangeArgs(1, 100), RunE: func(command *cobra.Command, args []string) error {
		if !yes {
			return errors.New("删除历史产物前请传 --yes；该操作不删除节点资源、已保存资产或对象存储")
		}
		scope, err := resolveCanvasScope(options)
		if err != nil {
			return err
		}
		var result any
		if len(args) == 1 {
			result, err = deps.api.DeleteCanvasArtifact(command.Context(), scope.ProjectUUID, args[0])
		} else {
			result, err = deps.api.BatchDeleteCanvasArtifacts(command.Context(), scope.ProjectUUID, args)
		}
		if err != nil {
			return err
		}
		return output.WriteJSON(stdout, struct {
			ProjectUUID string `json:"project_uuid"`
			Deleted     any    `json:"deleted"`
			Scope       string `json:"scope"`
		}{scope.ProjectUUID, result, "artifact_history_only"})
	}}
	command.Flags().BoolVar(&yes, "yes", false, "confirm soft deletion of artifact history records")
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}

func newCanvasArtifactSaveCommand(stdout, stderr io.Writer, deps *dependencies, options *canvasScopeOptions) *cobra.Command {
	var resourceIndex int
	var name string
	command := &cobra.Command{Use: "save NODE", Short: "Save one node resource into My Assets", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		if resourceIndex < 0 {
			return errors.New("--resource-index 不能小于 0")
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
		canvasUUID := firstNonEmptyString(scope.CanvasUUID, detail.CurrentCanvas.CanvasUUID)
		result, err := deps.api.CreateCanvasMediaAssets(command.Context(), scope.ProjectUUID, api.CanvasMediaAssetsRequest{CanvasUUID: canvasUUID, Items: []api.CanvasMediaAssetItem{{NodeKey: node.NodeKey, ResourceIndex: resourceIndex, Name: name}}})
		if err != nil {
			return err
		}
		return output.WriteJSON(stdout, struct {
			ProjectUUID   string `json:"project_uuid"`
			CanvasUUID    string `json:"canvas_uuid"`
			NodeKey       string `json:"node_key"`
			ResourceIndex int    `json:"resource_index"`
			Result        any    `json:"result"`
		}{scope.ProjectUUID, canvasUUID, node.NodeKey, resourceIndex, result})
	}}
	command.Flags().IntVar(&resourceIndex, "resource-index", 0, "zero-based index in node data.url")
	command.Flags().StringVar(&name, "name", "", "optional saved asset name")
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}
