package cmd

import (
	"errors"
	"io"
	"strings"

	"github.com/maoshuhua/pavo-cli/internal/api"
	"github.com/maoshuhua/pavo-cli/internal/output"
	"github.com/spf13/cobra"
)

type conversationOutput struct {
	ConversationID string `json:"conversation_id"`
}

type conversationStatusOutput struct {
	ConversationID string `json:"conversation_id"`
	IsRunning      bool   `json:"is_running"`
	RequestID      string `json:"request_id"`
}

type conversationResultOutput struct {
	ConversationID string                 `json:"conversation_id"`
	IsRunning      bool                   `json:"is_running"`
	Results        []api.GenerationResult `json:"results"`
}

func newConversationCommand(stdout, stderr io.Writer, deps *dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "conversation",
		Short: "Manage PAVO conversations",
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.AddCommand(newCreateConversationCommand(stdout, stderr, deps))
	cmd.AddCommand(newConversationStatusCommand(stdout, stderr, deps))
	cmd.AddCommand(newConversationResultCommand(stdout, stderr, deps))
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

func newConversationStatusCommand(stdout, stderr io.Writer, deps *dependencies) *cobra.Command {
	var conversationID string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Get the current running state of a PAVO conversation",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conversationID = strings.TrimSpace(conversationID)
			if conversationID == "" {
				return errors.New("缺少必填参数 --conversation-id")
			}
			status, err := deps.api.GetConversationStatus(cmd.Context(), conversationID)
			if err != nil {
				return err
			}
			id := strings.TrimSpace(string(status.ConversationID))
			if id == "" {
				id = conversationID
			}
			return output.WriteJSON(stdout, conversationStatusOutput{
				ConversationID: id,
				IsRunning:      status.IsRunning,
				RequestID:      status.RequestID,
			})
		},
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.Flags().StringVar(&conversationID, "conversation-id", "", "conversation ID to inspect")
	return cmd
}

func newConversationResultCommand(stdout, stderr io.Writer, deps *dependencies) *cobra.Command {
	var conversationID string
	cmd := &cobra.Command{
		Use:   "result",
		Short: "Read durable generated results after a stream has completed",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conversationID = strings.TrimSpace(conversationID)
			if conversationID == "" {
				return errors.New("缺少必填参数 --conversation-id")
			}
			history, err := deps.api.GetConversationHistory(cmd.Context(), conversationID)
			if err != nil {
				return err
			}
			results := history.LatestGenerationResults()
			if results == nil {
				results = []api.GenerationResult{}
			}
			return output.WriteJSON(stdout, conversationResultOutput{
				ConversationID: conversationID,
				IsRunning:      history.IsRunning,
				Results:        results,
			})
		},
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.Flags().StringVar(&conversationID, "conversation-id", "", "conversation ID whose results should be read")
	return cmd
}
