package cmd

import (
	"errors"
	"io"
	"strings"

	"github.com/maoshuhua/pavo-cli/internal/api"
	"github.com/maoshuhua/pavo-cli/internal/output"
	"github.com/spf13/cobra"
)

type conversationStatusOutput struct {
	ConversationID string `json:"conversation_id"`
	IsRunning      bool   `json:"is_running"`
	RequestID      string `json:"request_id"`
}

type conversationResultOutput struct {
	ConversationID string                 `json:"conversation_id"`
	IsRunning      bool                   `json:"is_running"`
	Results        []api.GenerationResult `json:"results"`
	Assets         []api.GeneratedAsset   `json:"assets,omitempty"`
}

func newConversationCommand(stdout, stderr io.Writer, deps *dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "conversation",
		Short: "Manage PAVO conversations",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.AddCommand(newConversationStatusCommand(stdout, stderr, deps))
	cmd.AddCommand(newConversationResultCommand(stdout, stderr, deps))
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
	var downloadDir string
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
			streamResult := &api.StreamOutput{
				ConversationID: conversationID,
				Results:        history.LatestGenerationResults(),
			}
			if streamResult.Results == nil {
				streamResult.Results = []api.GenerationResult{}
			}
			if err := downloadStreamResults(cmd.Context(), deps, streamResult, downloadDir); err != nil {
				return err
			}
			return output.WriteJSON(stdout, conversationResultOutput{
				ConversationID: conversationID,
				IsRunning:      history.IsRunning,
				Results:        streamResult.Results,
				Assets:         streamResult.Assets,
			})
		},
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.Flags().StringVar(&conversationID, "conversation-id", "", "conversation ID whose results should be read")
	cmd.Flags().StringVar(&downloadDir, "download-dir", "", "directory to save successful generated results")
	return cmd
}
