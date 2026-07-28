package cmd

import (
	"errors"
	"io"
	"strings"

	"github.com/maoshuhua/pavo-cli/internal/output"
	"github.com/spf13/cobra"
)

func newUploadCommand(stdout, stderr io.Writer, deps *dependencies) *cobra.Command {
	var filePath string
	cmd := &cobra.Command{
		Use:   "upload",
		Short: "Upload a chat attachment and return its public URL",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			filePath = strings.TrimSpace(filePath)
			if filePath == "" {
				return errors.New("缺少必填参数 --file")
			}
			result, err := deps.api.UploadFile(cmd.Context(), filePath)
			if err != nil {
				return err
			}
			return output.WriteJSON(stdout, result)
		},
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.Flags().StringVar(&filePath, "file", "", "local file to upload")
	return cmd
}
