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

type canvasMutationOutput struct {
	ProjectUUID  string `json:"project_uuid"`
	CanvasUUID   string `json:"canvas_uuid,omitempty"`
	Version      int64  `json:"version"`
	NodeKey      string `json:"node_key,omitempty"`
	ConnectionID string `json:"connection_id,omitempty"`
	Operation    string `json:"operation"`
}

func mutationVersion(result *api.CanvasBatchResult) int64 {
	if result == nil {
		return 0
	}
	return int64(result.Version)
}

func newCanvasNodeCommand(stdout, stderr io.Writer, deps *dependencies, scopeOptions *canvasScopeOptions) *cobra.Command {
	command := &cobra.Command{Use: "node", Short: "Inspect and mutate canvas nodes", Args: cobra.NoArgs}
	command.AddCommand(newCanvasNodeListCommand(stdout, stderr, deps, scopeOptions))
	command.AddCommand(newCanvasNodeGetCommand(stdout, stderr, deps, scopeOptions))
	command.AddCommand(newCanvasNodeCreateCommand(stdout, stderr, deps, scopeOptions))
	command.AddCommand(newCanvasNodeUpdateCommand(stdout, stderr, deps, scopeOptions))
	command.AddCommand(newCanvasNodeDeleteCommand(stdout, stderr, deps, scopeOptions))
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}

func newCanvasNodeListCommand(stdout, stderr io.Writer, deps *dependencies, options *canvasScopeOptions) *cobra.Command {
	command := &cobra.Command{
		Use:   "list",
		Short: "List nodes in the effective canvas",
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
			return output.WriteJSON(stdout, struct {
				ProjectUUID string           `json:"project_uuid"`
				CanvasUUID  string           `json:"canvas_uuid"`
				Version     int64            `json:"version"`
				Nodes       []api.CanvasNode `json:"nodes"`
			}{ProjectUUID: scope.ProjectUUID, CanvasUUID: detail.CurrentCanvas.CanvasUUID, Version: int64(detail.Version), Nodes: detail.NodeList})
		},
	}
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}

func newCanvasNodeGetCommand(stdout, stderr io.Writer, deps *dependencies, options *canvasScopeOptions) *cobra.Command {
	command := &cobra.Command{
		Use:   "get NODE",
		Short: "Get one node by exact node_key or exact title",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
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
			return output.WriteJSON(stdout, node)
		},
	}
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}

func newCanvasNodeCreateCommand(stdout, stderr io.Writer, deps *dependencies, options *canvasScopeOptions) *cobra.Command {
	var nodeType string
	var name string
	var prompt string
	var model string
	var dataJSON string
	var mediaType string
	var x, y, width, height float64
	command := &cobra.Command{
		Use:   "create",
		Short: "Create a node using the frontend-compatible data contract",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			nodeType = strings.TrimSpace(nodeType)
			if nodeType == "" {
				return errors.New("缺少必填参数 --type")
			}
			data, err := canvascore.ParseObject(dataJSON)
			if err != nil {
				return fmt.Errorf("--data 无效: %w", err)
			}
			if scene := canvascore.ModelScene(nodeType); scene != "" && strings.TrimSpace(model) != "" {
				modelOptions, modelErr := deps.api.GetCanvasModelOptions(command.Context(), scene)
				if modelErr != nil {
					return modelErr
				}
				if modelErr := canvascore.ApplyModelConfiguration(data, nodeType, model, modelOptions); modelErr != nil {
					return modelErr
				}
			}
			scope, err := resolveCanvasScope(options)
			if err != nil {
				return err
			}
			var createdKey string
			result, err := canvascore.ApplyMutation(command.Context(), deps.api, scope, func(detail *api.CanvasProjectDetail) (*api.CanvasBatchRequest, error) {
				newOptions := canvascore.NewNodeOptions{
					Type: nodeType, Name: name, Prompt: prompt, Model: model,
					Width: width, Height: height, MediaType: strings.TrimSpace(mediaType), Data: data,
				}
				if command.Flags().Changed("x") {
					newOptions.X = &x
				}
				if command.Flags().Changed("y") {
					newOptions.Y = &y
				}
				item, err := canvascore.NewNode(detail, newOptions)
				if err != nil {
					return nil, err
				}
				createdKey = item.NodeKey
				request := canvascore.NewBatchRequest()
				request.Nodes.Create = append(request.Nodes.Create, *item)
				return request, nil
			})
			if err != nil {
				return err
			}
			return output.WriteJSON(stdout, canvasMutationOutput{ProjectUUID: scope.ProjectUUID, CanvasUUID: scope.CanvasUUID, Version: mutationVersion(result), NodeKey: createdKey, Operation: "node.create"})
		},
	}
	flags := command.Flags()
	flags.StringVar(&nodeType, "type", "", "node type: text, image, video, audio, upload, directorNode, videoComposition, or group")
	flags.StringVar(&name, "name", "", "node title; defaults to the next frontend-style title")
	flags.StringVar(&prompt, "prompt", "", "text prompt to put in data.params.prompt")
	flags.StringVar(&model, "model", "", "model code to put in data.params.model")
	flags.StringVar(&mediaType, "media-type", "image", "upload media type: image, video, or audio")
	flags.StringVar(&dataJSON, "data", "{}", "additional node data as a JSON object")
	flags.Float64Var(&x, "x", 0, "node X position")
	flags.Float64Var(&y, "y", 0, "node Y position")
	flags.Float64Var(&width, "width", 280, "measured node width")
	flags.Float64Var(&height, "height", 280, "measured node height")
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}

func newCanvasNodeUpdateCommand(stdout, stderr io.Writer, deps *dependencies, options *canvasScopeOptions) *cobra.Command {
	var name, prompt, model, dataJSON string
	var replaceData bool
	var unset []string
	var x, y, width, height float64
	command := &cobra.Command{
		Use:   "update NODE",
		Short: "Merge changes into a node without discarding unknown data fields",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if !command.Flags().Changed("name") && !command.Flags().Changed("prompt") && !command.Flags().Changed("model") && !command.Flags().Changed("data") && len(unset) == 0 && !command.Flags().Changed("x") && !command.Flags().Changed("y") && !command.Flags().Changed("width") && !command.Flags().Changed("height") {
				return errors.New("没有要更新的字段")
			}
			patch, err := canvascore.ParseObject(dataJSON)
			if err != nil {
				return fmt.Errorf("--data 无效: %w", err)
			}
			scope, err := resolveCanvasScope(options)
			if err != nil {
				return err
			}
			var nodeKey string
			result, err := canvascore.ApplyMutation(command.Context(), deps.api, scope, func(detail *api.CanvasProjectDetail) (*api.CanvasBatchRequest, error) {
				node, err := canvascore.FindNode(detail, args[0])
				if err != nil {
					return nil, err
				}
				nodeKey = node.NodeKey
				data := map[string]any{}
				if !replaceData {
					data, err = canvascore.NodeData(*node)
					if err != nil {
						return nil, err
					}
				}
				canvascore.MergeObject(data, patch)
				data["node_key"] = node.NodeKey
				if command.Flags().Changed("name") {
					value := strings.TrimSpace(name)
					if value == "" {
						return nil, errors.New("--name 不能为空")
					}
					data["title"] = value
					data["name"] = value
				}
				if command.Flags().Changed("prompt") {
					canvascore.SetPrompt(data, string(node.NodeType), prompt)
				}
				if command.Flags().Changed("model") {
					if scene := canvascore.ModelScene(string(node.NodeType)); scene != "" {
						modelOptions, modelErr := deps.api.GetCanvasModelOptions(command.Context(), scene)
						if modelErr != nil {
							return nil, modelErr
						}
						if modelErr := canvascore.ApplyModelConfiguration(data, string(node.NodeType), model, modelOptions); modelErr != nil {
							return nil, modelErr
						}
					} else {
						canvascore.SetModel(data, model)
					}
				}
				for _, key := range unset {
					key = strings.TrimSpace(key)
					if key == "node_key" {
						return nil, errors.New("不能 --unset node_key")
					}
					if key != "" {
						delete(data, key)
					}
				}
				item, err := canvascore.WriteItemFromNode(*node, data)
				if err != nil {
					return nil, err
				}
				if command.Flags().Changed("x") {
					item.Position.PositionX = fmt.Sprint(x)
				}
				if command.Flags().Changed("y") {
					item.Position.PositionY = fmt.Sprint(y)
				}
				if command.Flags().Changed("width") {
					item.Measured.Width = fmt.Sprint(width)
				}
				if command.Flags().Changed("height") {
					item.Measured.Height = fmt.Sprint(height)
				}
				request := canvascore.NewBatchRequest()
				request.Nodes.Update = append(request.Nodes.Update, item)
				return request, nil
			})
			if err != nil {
				return err
			}
			return output.WriteJSON(stdout, canvasMutationOutput{ProjectUUID: scope.ProjectUUID, CanvasUUID: scope.CanvasUUID, Version: mutationVersion(result), NodeKey: nodeKey, Operation: "node.update"})
		},
	}
	flags := command.Flags()
	flags.StringVar(&name, "name", "", "new node title")
	flags.StringVar(&prompt, "prompt", "", "new prompt; an empty value clears prompt segments")
	flags.StringVar(&model, "model", "", "new model code")
	flags.StringVar(&dataJSON, "data", "{}", "node data patch as a JSON object")
	flags.BoolVar(&replaceData, "replace-data", false, "replace data instead of merging; node_key is always preserved")
	flags.StringSliceVar(&unset, "unset", nil, "top-level data key to remove; may be repeated")
	flags.Float64Var(&x, "x", 0, "new X position")
	flags.Float64Var(&y, "y", 0, "new Y position")
	flags.Float64Var(&width, "width", 0, "new measured width")
	flags.Float64Var(&height, "height", 0, "new measured height")
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}

func newCanvasNodeDeleteCommand(stdout, stderr io.Writer, deps *dependencies, options *canvasScopeOptions) *cobra.Command {
	var yes bool
	command := &cobra.Command{
		Use:   "delete NODE",
		Short: "Delete a node and its connected edges",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if !yes {
				return errors.New("删除节点会同时删除连线；确认后请传 --yes")
			}
			scope, err := resolveCanvasScope(options)
			if err != nil {
				return err
			}
			var nodeKey string
			result, err := canvascore.ApplyMutation(command.Context(), deps.api, scope, func(detail *api.CanvasProjectDetail) (*api.CanvasBatchRequest, error) {
				node, err := canvascore.FindNode(detail, args[0])
				if err != nil {
					return nil, err
				}
				nodeKey = node.NodeKey
				request := canvascore.NewBatchRequest()
				request.Nodes.Delete = append(request.Nodes.Delete, node.NodeKey)
				updates := map[string]api.CanvasBatchNodeWriteItem{}
				for _, connection := range detail.ConnectionList {
					if connection.SourceNodeKey != node.NodeKey && connection.TargetNodeKey != node.NodeKey {
						continue
					}
					request.Connections.Delete = append(request.Connections.Delete, api.CanvasBatchConnectionDeleteItem{ConnectionID: connection.ConnectionID})
					otherKey := connection.SourceNodeKey
					field := "target"
					removeValue := node.NodeKey
					if otherKey == node.NodeKey {
						otherKey = connection.TargetNodeKey
						field = "source"
					}
					for index := range detail.NodeList {
						other := detail.NodeList[index]
						if other.NodeKey != otherKey {
							continue
						}
						data, decodeErr := canvascore.NodeData(other)
						if decodeErr != nil {
							return nil, decodeErr
						}
						canvascore.RemoveString(data, field, removeValue)
						item, itemErr := canvascore.WriteItemFromNode(other, data)
						if itemErr != nil {
							return nil, itemErr
						}
						updates[other.NodeKey] = item
					}
				}
				for _, item := range updates {
					request.Nodes.Update = append(request.Nodes.Update, item)
				}
				return request, nil
			})
			if err != nil {
				return err
			}
			return output.WriteJSON(stdout, canvasMutationOutput{ProjectUUID: scope.ProjectUUID, CanvasUUID: scope.CanvasUUID, Version: mutationVersion(result), NodeKey: nodeKey, Operation: "node.delete"})
		},
	}
	command.Flags().BoolVar(&yes, "yes", false, "confirm node and connection deletion")
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}
