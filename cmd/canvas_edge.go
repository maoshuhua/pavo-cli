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

func newCanvasEdgeCommand(stdout, stderr io.Writer, deps *dependencies, scopeOptions *canvasScopeOptions) *cobra.Command {
	command := &cobra.Command{Use: "edge", Short: "Inspect and mutate canvas connections", Args: cobra.NoArgs}
	command.AddCommand(newCanvasEdgeListCommand(stdout, stderr, deps, scopeOptions))
	command.AddCommand(newCanvasEdgeAddCommand(stdout, stderr, deps, scopeOptions))
	command.AddCommand(newCanvasEdgeDeleteCommand(stdout, stderr, deps, scopeOptions))
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}

func newCanvasEdgeListCommand(stdout, stderr io.Writer, deps *dependencies, options *canvasScopeOptions) *cobra.Command {
	command := &cobra.Command{
		Use:   "list",
		Short: "List connections in the effective canvas",
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
				ProjectUUID string                 `json:"project_uuid"`
				CanvasUUID  string                 `json:"canvas_uuid"`
				Version     int64                  `json:"version"`
				Connections []api.CanvasConnection `json:"connections"`
			}{ProjectUUID: scope.ProjectUUID, CanvasUUID: detail.CurrentCanvas.CanvasUUID, Version: int64(detail.Version), Connections: detail.ConnectionList})
		},
	}
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}

func newCanvasEdgeAddCommand(stdout, stderr io.Writer, deps *dependencies, options *canvasScopeOptions) *cobra.Command {
	var sourceRef, targetRef, connectionID string
	command := &cobra.Command{
		Use:   "add",
		Short: "Connect two nodes resolved by node_key or exact title",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if strings.TrimSpace(sourceRef) == "" || strings.TrimSpace(targetRef) == "" {
				return errors.New("缺少必填参数 --source 或 --target")
			}
			scope, err := resolveCanvasScope(options)
			if err != nil {
				return err
			}
			var createdID string
			result, err := canvascore.ApplyMutation(command.Context(), deps.api, scope, func(detail *api.CanvasProjectDetail) (*api.CanvasBatchRequest, error) {
				source, err := canvascore.FindNode(detail, sourceRef)
				if err != nil {
					return nil, fmt.Errorf("解析 --source 失败: %w", err)
				}
				target, err := canvascore.FindNode(detail, targetRef)
				if err != nil {
					return nil, fmt.Errorf("解析 --target 失败: %w", err)
				}
				if source.NodeKey == target.NodeKey {
					return nil, errors.New("不能连接节点自身")
				}
				for _, connection := range detail.ConnectionList {
					if connection.SourceNodeKey == source.NodeKey && connection.TargetNodeKey == target.NodeKey {
						return nil, fmt.Errorf("连线已存在: %s", connection.ConnectionID)
					}
				}
				createdID = strings.TrimSpace(connectionID)
				if createdID == "" {
					createdID, err = canvascore.NewConnectionID(string(source.NodeType), canvascore.NodeMediaType(*source), string(target.NodeType), canvascore.NodeMediaType(*target))
					if err != nil {
						return nil, err
					}
				}
				for _, connection := range detail.ConnectionList {
					if connection.ConnectionID == createdID {
						return nil, fmt.Errorf("connection_id %q 已存在", createdID)
					}
				}
				sourceData, err := canvascore.NodeData(*source)
				if err != nil {
					return nil, err
				}
				targetData, err := canvascore.NodeData(*target)
				if err != nil {
					return nil, err
				}
				canvascore.AddUniqueString(sourceData, "target", target.NodeKey)
				canvascore.AddUniqueString(targetData, "source", source.NodeKey)
				sourceItem, err := canvascore.WriteItemFromNode(*source, sourceData)
				if err != nil {
					return nil, err
				}
				targetItem, err := canvascore.WriteItemFromNode(*target, targetData)
				if err != nil {
					return nil, err
				}
				request := canvascore.NewBatchRequest()
				request.Nodes.Update = append(request.Nodes.Update, sourceItem, targetItem)
				request.Connections.Create = append(request.Connections.Create, api.CanvasBatchConnectionWriteItem{
					ConnectionID: createdID, Source: source.NodeKey, Target: target.NodeKey,
					SourceHandle: "source", TargetHandle: "target", ConnectionType: "default", Selectable: true, Deletable: true,
				})
				return request, nil
			})
			if err != nil {
				return err
			}
			return output.WriteJSON(stdout, canvasMutationOutput{ProjectUUID: scope.ProjectUUID, CanvasUUID: scope.CanvasUUID, Version: mutationVersion(result), ConnectionID: createdID, Operation: "edge.add"})
		},
	}
	flags := command.Flags()
	flags.StringVar(&sourceRef, "source", "", "source node_key or exact title")
	flags.StringVar(&targetRef, "target", "", "target node_key or exact title")
	flags.StringVar(&connectionID, "id", "", "explicit connection_id; normally generated automatically")
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}

func newCanvasEdgeDeleteCommand(stdout, stderr io.Writer, deps *dependencies, options *canvasScopeOptions) *cobra.Command {
	var yes bool
	command := &cobra.Command{
		Use:   "delete CONNECTION_ID",
		Short: "Delete a connection and update source/target node data",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if !yes {
				return errors.New("删除连线前请传 --yes")
			}
			scope, err := resolveCanvasScope(options)
			if err != nil {
				return err
			}
			connectionID := strings.TrimSpace(args[0])
			result, err := canvascore.ApplyMutation(command.Context(), deps.api, scope, func(detail *api.CanvasProjectDetail) (*api.CanvasBatchRequest, error) {
				var matched *api.CanvasConnection
				for index := range detail.ConnectionList {
					if detail.ConnectionList[index].ConnectionID == connectionID {
						matched = &detail.ConnectionList[index]
						break
					}
				}
				if matched == nil {
					return nil, fmt.Errorf("找不到连线 %q", connectionID)
				}
				request := canvascore.NewBatchRequest()
				request.Connections.Delete = append(request.Connections.Delete, api.CanvasBatchConnectionDeleteItem{ConnectionID: connectionID})
				for index := range detail.NodeList {
					node := detail.NodeList[index]
					var field, removeValue string
					switch node.NodeKey {
					case matched.SourceNodeKey:
						field, removeValue = "target", matched.TargetNodeKey
					case matched.TargetNodeKey:
						field, removeValue = "source", matched.SourceNodeKey
					default:
						continue
					}
					data, decodeErr := canvascore.NodeData(node)
					if decodeErr != nil {
						return nil, decodeErr
					}
					canvascore.RemoveString(data, field, removeValue)
					item, itemErr := canvascore.WriteItemFromNode(node, data)
					if itemErr != nil {
						return nil, itemErr
					}
					request.Nodes.Update = append(request.Nodes.Update, item)
				}
				return request, nil
			})
			if err != nil {
				return err
			}
			return output.WriteJSON(stdout, canvasMutationOutput{ProjectUUID: scope.ProjectUUID, CanvasUUID: scope.CanvasUUID, Version: mutationVersion(result), ConnectionID: connectionID, Operation: "edge.delete"})
		},
	}
	command.Flags().BoolVar(&yes, "yes", false, "confirm connection deletion")
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}
