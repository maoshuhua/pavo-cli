package cmd

import (
	"errors"
	"io"
	"strings"

	"github.com/maoshuhua/pavo-cli/internal/api"
	"github.com/maoshuhua/pavo-cli/internal/output"
	"github.com/spf13/cobra"
)

func newDownloadResultCommand(stdout, stderr io.Writer, deps *dependencies) *cobra.Command {
	var opts api.DownloadResultOptions
	cmd := &cobra.Command{
		Use:   "download-result",
		Short: "Download a generated result URL to a local file",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts.URL = strings.TrimSpace(opts.URL)
			if opts.URL == "" {
				return errors.New("缺少必填参数 --url")
			}
			opts.OutputPath = strings.TrimSpace(opts.OutputPath)
			if opts.OutputPath == "" {
				return errors.New("缺少必填参数 --output-path")
			}
			result, err := deps.api.DownloadResult(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return output.WriteJSON(stdout, result)
		},
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	flags := cmd.Flags()
	flags.StringVar(&opts.URL, "url", "", "generated result URL to download")
	flags.StringVar(&opts.OutputPath, "output-path", "", "local output file path")
	flags.Int64Var(&opts.UpdatedAt, "updated-at", 0, "remote file update time as a Unix timestamp")
	flags.BoolVar(&opts.Force, "force", false, "overwrite an existing local file")
	return cmd
}
