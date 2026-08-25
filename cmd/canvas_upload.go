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

func canvasMediaType(contentType string) (string, error) {
	switch {
	case strings.HasPrefix(contentType, "image/"):
		return "image", nil
	case strings.HasPrefix(contentType, "video/"):
		return "video", nil
	case strings.HasPrefix(contentType, "audio/"):
		return "audio", nil
	default:
		return "", errors.New("画布上传仅支持图片、视频或音频")
	}
}

func newCanvasUploadCommand(stdout, stderr io.Writer, deps *dependencies, options *canvasScopeOptions) *cobra.Command {
	var filePath, name string
	var x, y float64
	command := &cobra.Command{
		Use:   "upload",
		Short: "Upload a local media file and create a canvas upload node",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			filePath = strings.TrimSpace(filePath)
			if filePath == "" {
				return errors.New("缺少必填参数 --file")
			}
			scope, err := resolveCanvasScope(options)
			if err != nil {
				return err
			}
			uploaded, err := deps.api.UploadCanvasFile(command.Context(), filePath)
			if err != nil {
				return err
			}
			mediaType, err := canvasMediaType(uploaded.ContentType)
			if err != nil {
				return err
			}
			if strings.TrimSpace(name) == "" {
				name = filepath.Base(uploaded.Filename)
			}
			var nodeKey string
			result, err := canvascore.ApplyMutation(command.Context(), deps.api, scope, func(detail *api.CanvasProjectDetail) (*api.CanvasBatchRequest, error) {
				newOptions := canvascore.NewNodeOptions{
					Type: "upload", Name: name, MediaType: mediaType,
					Data: map[string]any{"url": []any{uploaded.PublicURL}},
				}
				if command.Flags().Changed("x") {
					newOptions.X = &x
				}
				if command.Flags().Changed("y") {
					newOptions.Y = &y
				}
				item, createErr := canvascore.NewNode(detail, newOptions)
				if createErr != nil {
					return nil, createErr
				}
				nodeKey = item.NodeKey
				request := canvascore.NewBatchRequest()
				request.Nodes.Create = append(request.Nodes.Create, *item)
				return request, nil
			})
			if err != nil {
				return fmt.Errorf("文件已上传 public_url=%s，但创建画布节点失败: %w", uploaded.PublicURL, err)
			}
			return output.WriteJSON(stdout, struct {
				ProjectUUID string                `json:"project_uuid"`
				CanvasUUID  string                `json:"canvas_uuid,omitempty"`
				Version     int64                 `json:"version"`
				NodeKey     string                `json:"node_key"`
				Upload      *api.FileUploadResult `json:"upload"`
			}{ProjectUUID: scope.ProjectUUID, CanvasUUID: scope.CanvasUUID, Version: mutationVersion(result), NodeKey: nodeKey, Upload: uploaded})
		},
	}
	flags := command.Flags()
	flags.StringVar(&filePath, "file", "", "local image, video, or audio file")
	flags.StringVar(&name, "name", "", "upload node title; defaults to the filename")
	flags.Float64Var(&x, "x", 0, "node X position")
	flags.Float64Var(&y, "y", 0, "node Y position")
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}
