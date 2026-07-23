package cmd

import (
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
			result, err := deps.api.Stream(cmd.Context(), conversationID, prompt, handler)
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
	flags.BoolVar(&raw, "raw", false, "write every raw stream event to stderr")
	return cmd
}
