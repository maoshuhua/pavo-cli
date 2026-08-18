package cmd

import (
	"io"
	"strings"

	"github.com/maoshuhua/pavo-cli/internal/api"
	"github.com/maoshuhua/pavo-cli/internal/output"
	"github.com/spf13/cobra"
)

type visualsOutput struct {
	Category   api.VisualCategory   `json:"category"`
	Pagination api.VisualPagination `json:"pagination"`
	Groups     []api.VisualGroup    `json:"groups"`
}

func newVisualsCommand(stdout, stderr io.Writer, deps *dependencies) *cobra.Command {
	var category string
	var page int
	var pageSize int
	cmd := &cobra.Command{
		Use:   "visuals",
		Short: "List the current user's generated images or videos",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return writeVisuals(stdout, deps, cmd, api.VisualCategory(strings.ToLower(strings.TrimSpace(category))), page, pageSize)
		},
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	flags := cmd.Flags()
	flags.StringVar(&category, "category", "", "visual category: images or videos")
	flags.IntVar(&page, "page", 1, "page number starting from 1")
	flags.IntVar(&pageSize, "page-size", 5, "number of visual items per page")
	return cmd
}

func newShortDramaListCommand(stdout, stderr io.Writer, deps *dependencies) *cobra.Command {
	var page int
	var pageSize int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the current user's completed short dramas",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return writeVisuals(stdout, deps, cmd, api.VisualCategoryShortDramaFinal, page, pageSize)
		},
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	flags := cmd.Flags()
	flags.IntVar(&page, "page", 1, "page number starting from 1")
	flags.IntVar(&pageSize, "page-size", 5, "number of short dramas per page")
	return cmd
}

func writeVisuals(stdout io.Writer, deps *dependencies, cmd *cobra.Command, category api.VisualCategory, page, pageSize int) error {
	data, err := deps.api.ListVisuals(cmd.Context(), category, page, pageSize)
	if err != nil {
		return err
	}
	return output.WriteJSON(stdout, visualsOutput{
		Category:   category,
		Pagination: data.Pagination,
		Groups:     data.Groups,
	})
}
