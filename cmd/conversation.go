package cmd

import (
	"errors"
	"io"
	"strings"

	"github.com/maoshuhua/pavo-cli/internal/output"
	"github.com/spf13/cobra"
)

type conversationOutput struct {
	ConversationID string `json:"conversation_id"`
}

func newConversationCommand(stdout, stderr io.Writer, deps *dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "conversation",
		Short: "Manage PAVO conversations",
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.AddCommand(newCreateConversationCommand(stdout, stderr, deps))
	return cmd
}

func newCreateConversationCommand(stdout, stderr io.Writer, deps *dependencies) *cobra.Command {
	var prompt string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a PAVO conversation",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			prompt = strings.TrimSpace(prompt)
			if prompt == "" {
				return errors.New("缺少必填参数 --prompt")
			}
			conversationID, err := deps.api.CreateConversation(cmd.Context(), prompt)
			if err != nil {
				return err
			}
			return output.WriteJSON(stdout, conversationOutput{ConversationID: conversationID})
		},
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.Flags().StringVar(&prompt, "prompt", "", "prompt used to build the conversation title")
	return cmd
}
