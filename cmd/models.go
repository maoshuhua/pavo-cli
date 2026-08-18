package cmd

import (
	"errors"
	"io"
	"strings"

	"github.com/maoshuhua/pavo-cli/internal/api"
	"github.com/maoshuhua/pavo-cli/internal/output"
	"github.com/spf13/cobra"
)

type modelsOutput struct {
	Mode   string               `json:"mode"`
	Models []api.SupportedModel `json:"models"`
}

func newModelsCommand(stdout, stderr io.Writer, deps *dependencies) *cobra.Command {
	var mode string
	var modelType string
	var onlineOnly bool
	cmd := &cobra.Command{
		Use:   "models",
		Short: "List the Pixa models currently supported by a generation mode",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			mode = strings.ToLower(strings.TrimSpace(mode))
			modelType = strings.ToLower(strings.TrimSpace(modelType))
			if mode == "" {
				return errors.New("缺少必填参数 --mode")
			}
			if modelType != "" && modelType != "image" && modelType != "video" {
				return errors.New("--type 必须是 image 或 video")
			}
			if modelType != "" && mode != string(api.ModeCodeShortDrama) {
				return errors.New("--type 仅适用于 --mode short_drama")
			}
			models, err := deps.api.ListModeSupportModels(cmd.Context(), api.ModeCode(mode))
			if err != nil {
				return err
			}
			filtered := make([]api.SupportedModel, 0, len(models))
			for _, model := range models {
				if onlineOnly && !model.IsOnline {
					continue
				}
				if modelType != "" && strings.ToLower(strings.TrimSpace(model.Type)) != modelType {
					continue
				}
				filtered = append(filtered, model)
			}
			return output.WriteJSON(stdout, modelsOutput{Mode: mode, Models: filtered})
		},
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	flags := cmd.Flags()
	flags.StringVar(&mode, "mode", "", "mode code: short_drama, generate_image, or generate_video")
	flags.StringVar(&modelType, "type", "", "short_drama model type filter: image or video")
	flags.BoolVar(&onlineOnly, "online-only", false, "only return models whose is_online field is true")
	return cmd
}
