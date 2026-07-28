package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/maoshuhua/pavo-cli/internal/api"
	"github.com/maoshuhua/pavo-cli/internal/output"
	"github.com/spf13/cobra"
)

func newStreamCommand(stdout, stderr io.Writer, deps *dependencies) *cobra.Command {
	var conversationID string
	var prompt string
	var filePaths []string
	var raw bool
	cmd := &cobra.Command{
		Use:   "stream",
		Short: "Stream a PAVO design generation",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conversationID = strings.TrimSpace(conversationID)
			prompt = strings.TrimSpace(prompt)
			if conversationID == "" {
				return errors.New("缺少必填参数 --conversation-id")
			}
			if prompt == "" {
				return errors.New("缺少必填参数 --prompt")
			}
			attachments, err := uploadStreamAttachments(cmd.Context(), filePaths, deps)
			if err != nil {
				return err
			}
			handler := func(event *api.StreamEvent) error {
				if len(event.Raw) > 0 {
					if raw {
						_, err := fmt.Fprintln(stderr, string(event.Raw))
						return err
					}
					_, err := fmt.Fprintf(stderr, "[%d] %s %s\n", event.Seq, event.Type, event.Raw)
					return err
				}
				if event.Type != "" {
					_, err := fmt.Fprintf(stderr, "[%d] %s\n", event.Seq, event.Type)
					return err
				}
				return nil
			}
			result, err := deps.api.StreamWithFiles(cmd.Context(), conversationID, prompt, attachments, handler)
			if err != nil {
				return err
			}
			return output.WriteJSON(stdout, result)
		},
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	flags := cmd.Flags()
	flags.StringVar(&conversationID, "conversation-id", "", "conversation ID returned by conversation create")
	flags.StringVar(&prompt, "prompt", "", "generation prompt")
	flags.StringArrayVar(&filePaths, "file", nil, "local attachment to upload before generation; repeat for multiple files")
	flags.BoolVar(&raw, "raw", false, "write every raw stream event to stderr")
	return cmd
}

func uploadStreamAttachments(ctx context.Context, paths []string, deps *dependencies) ([]api.ChatAttachment, error) {
	attachments := make([]api.ChatAttachment, 0, len(paths))
	for _, rawPath := range paths {
		path := strings.TrimSpace(rawPath)
		if path == "" {
			return nil, errors.New("--file 不能为空")
		}
		result, err := deps.api.UploadFile(ctx, path)
		if err != nil {
			return nil, fmt.Errorf("上传附件 %q 失败: %w", path, err)
		}
		attachments = append(attachments, api.ChatAttachment{
			MimeType: result.ContentType,
			URL:      result.PublicURL,
			Filename: result.Filename,
		})
	}
	return attachments, nil
}
