package cmd

import (
	"encoding/json"
	"errors"
	"io"
	"strings"

	canvascore "github.com/maoshuhua/pavo-cli/internal/canvas"
	"github.com/maoshuhua/pavo-cli/internal/output"
	"github.com/spf13/cobra"
)

type canvasValidateOutput struct {
	ProjectUUID string                      `json:"project_uuid"`
	CanvasUUID  string                      `json:"canvas_uuid"`
	CanvasURL   string                      `json:"canvas_url,omitempty"`
	Valid       bool                        `json:"valid"`
	GraphError  string                      `json:"graph_error,omitempty"`
	Nodes       []canvascore.NodeValidation `json:"nodes"`
}

func newCanvasValidateCommand(stdout, stderr io.Writer, deps *dependencies, scopeOptions *canvasScopeOptions) *cobra.Command {
	var all, strict bool
	command := &cobra.Command{
		Use:   "validate [NODE]",
		Short: "Validate graph structure, prompt segments, live skills/models, and storyboard Schema",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if len(args) == 0 && !all {
				all = true
			}
			if len(args) == 1 && all {
				return errors.New("NODE 不能与 --all 同时使用")
			}
			scope, err := resolveCanvasScope(scopeOptions)
			if err != nil {
				return err
			}
			detail, err := deps.api.GetCanvasProjectDetail(command.Context(), scope.ProjectUUID, scope.CanvasUUID)
			if err != nil {
				return err
			}
			specs, err := getCanvasToolSpecs(command.Context(), deps)
			if err != nil {
				return err
			}
			cache := map[string]json.RawMessage{}
			validateModel := func(nodeType, model string) error {
				scene := canvascore.ModelScene(nodeType)
				raw, exists := cache[scene]
				if !exists {
					raw, err = deps.api.GetCanvasModelOptions(command.Context(), scene)
					if err != nil {
						return err
					}
					cache[scene] = raw
				}
				return canvascore.ApplyModelConfiguration(map[string]any{}, nodeType, model, raw)
			}
			result := canvasValidateOutput{ProjectUUID: scope.ProjectUUID, CanvasUUID: firstNonEmptyString(scope.CanvasUUID, detail.CurrentCanvas.CanvasUUID), Valid: true, Nodes: []canvascore.NodeValidation{}}
			result.CanvasURL = canvascore.BuildURL(deps.config.AppBaseURL, canvascore.ProjectIDFromDetail(detail), scope.ProjectUUID, result.CanvasUUID)
			if graphErr := canvascore.ValidateGraphStructure(detail); graphErr != nil {
				result.Valid = false
				result.GraphError = graphErr.Error()
			}
			if all {
				for _, node := range detail.NodeList {
					validation := canvascore.ValidateCanvasNode(node, specs, validateModel)
					result.Nodes = append(result.Nodes, validation)
					result.Valid = result.Valid && validation.Valid
				}
			} else {
				node, err := canvascore.FindNode(detail, strings.TrimSpace(args[0]))
				if err != nil {
					return err
				}
				validation := canvascore.ValidateCanvasNode(*node, specs, validateModel)
				result.Nodes = append(result.Nodes, validation)
				result.Valid = result.Valid && validation.Valid
			}
			if err := output.WriteJSON(stdout, result); err != nil {
				return err
			}
			if strict && !result.Valid {
				return errors.New("canvas Schema 校验失败")
			}
			return nil
		},
	}
	command.Flags().BoolVar(&all, "all", false, "validate every node in the current canvas")
	command.Flags().BoolVar(&strict, "strict", false, "return a non-zero exit status when validation errors exist")
	command.SetOut(stdout)
	command.SetErr(stderr)
	return command
}
