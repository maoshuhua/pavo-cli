package cmd

import (
	"errors"
	"io"
	"strings"

	"github.com/maoshuhua/pavo-cli/internal/api"
	canvascore "github.com/maoshuhua/pavo-cli/internal/canvas"
	"github.com/maoshuhua/pavo-cli/internal/output"
	"github.com/spf13/cobra"
)

type canvasGroupOutput struct {
	ProjectUUID string   `json:"project_uuid"`
	CanvasUUID  string   `json:"canvas_uuid"`
	Version     int64    `json:"version"`
	GroupKey    string   `json:"group_key"`
	MemberKeys  []string `json:"member_keys"`
	Operation   string   `json:"operation"`
}

func newCanvasGroupCommand(stdout, stderr io.Writer, deps *dependencies, options *canvasScopeOptions) *cobra.Command {
	command := &cobra.Command{Use: "group", Short: "Group or ungroup canvas nodes", Args: cobra.NoArgs}
	command.AddCommand(newCanvasGroupCreateCommand(stdout, stderr, deps, options))
	command.AddCommand(newCanvasGroupUngroupCommand(stdout, stderr, deps, options))
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}

func newCanvasGroupCreateCommand(stdout, stderr io.Writer, deps *dependencies, options *canvasScopeOptions) *cobra.Command {
	var name, modeCode, border, fill string
	var padding float64
	command := &cobra.Command{
		Use: "create NODE NODE [NODE...]", Short: "Create a frontend-compatible group around nodes", Args: cobra.MinimumNArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			scope, err := resolveCanvasScope(options)
			if err != nil {
				return err
			}
			groupKey, err := canvascore.NewNodeKey("group", "")
			if err != nil {
				return err
			}
			var members []string
			result, err := canvascore.ApplyMutation(command.Context(), deps.api, scope, func(detail *api.CanvasProjectDetail) (*api.CanvasBatchRequest, error) {
				working, cloneErr := canvascore.CloneDetail(detail)
				if cloneErr != nil {
					return nil, cloneErr
				}
				_, resolvedMembers, groupErr := canvascore.GroupNodes(working, args, canvascore.GroupOptions{NodeKey: groupKey, Name: name, ModeCode: modeCode, BorderColor: border, FillColor: fill, Padding: padding})
				if groupErr != nil {
					return nil, groupErr
				}
				members = resolvedMembers
				return canvascore.DiffDetails(detail, working)
			})
			if err != nil {
				return err
			}
			return output.WriteJSON(stdout, canvasGroupOutput{ProjectUUID: scope.ProjectUUID, CanvasUUID: scope.CanvasUUID, Version: mutationVersion(result), GroupKey: groupKey, MemberKeys: members, Operation: "group.create"})
		},
	}
	flags := command.Flags()
	flags.StringVar(&name, "name", "", "group title")
	flags.StringVar(&modeCode, "mode-code", "", "optional frontend group mode code")
	flags.StringVar(&border, "border", "#0ABCCF", "group border color")
	flags.StringVar(&fill, "fill", "#FBFBFB1A", "group fill color")
	flags.Float64Var(&padding, "padding", canvascore.DefaultGroupPadding, "padding around grouped nodes")
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}

func newCanvasGroupUngroupCommand(stdout, stderr io.Writer, deps *dependencies, options *canvasScopeOptions) *cobra.Command {
	var yes bool
	command := &cobra.Command{
		Use: "ungroup GROUP", Short: "Remove a group and restore child absolute positions", Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if !yes {
				return errors.New("解组会删除 group 节点；确认后请传 --yes")
			}
			scope, err := resolveCanvasScope(options)
			if err != nil {
				return err
			}
			var groupKey string
			var members []string
			result, err := canvascore.ApplyMutation(command.Context(), deps.api, scope, func(detail *api.CanvasProjectDetail) (*api.CanvasBatchRequest, error) {
				working, cloneErr := canvascore.CloneDetail(detail)
				if cloneErr != nil {
					return nil, cloneErr
				}
				groupKey, members, cloneErr = canvascore.UngroupNode(working, strings.TrimSpace(args[0]))
				if cloneErr != nil {
					return nil, cloneErr
				}
				return canvascore.DiffDetails(detail, working)
			})
			if err != nil {
				return err
			}
			return output.WriteJSON(stdout, canvasGroupOutput{ProjectUUID: scope.ProjectUUID, CanvasUUID: scope.CanvasUUID, Version: mutationVersion(result), GroupKey: groupKey, MemberKeys: members, Operation: "group.ungroup"})
		},
	}
	command.Flags().BoolVar(&yes, "yes", false, "confirm deleting the group node")
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}
