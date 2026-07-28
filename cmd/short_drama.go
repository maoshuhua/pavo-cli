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

const (
	defaultShortDramaImageModel = "agnes-image"
	defaultShortDramaVideoModel = "agnes-video"
)

func newShortDramaCommand(stdout, stderr io.Writer, deps *dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "short-drama",
		Short: "Create and continue PAVO short-drama conversations",
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.AddCommand(newShortDramaStartCommand(stdout, stderr, deps))
	cmd.AddCommand(newShortDramaReplyCommand(stdout, stderr, deps))
	cmd.AddCommand(newShortDramaResumeCommand(stdout, stderr, deps))
	cmd.AddCommand(newShortDramaStatusCommand(stdout, stderr, deps))
	cmd.AddCommand(newShortDramaResultCommand(stdout, stderr, deps))
	return cmd
}

func newShortDramaStartCommand(stdout, stderr io.Writer, deps *dependencies) *cobra.Command {
	var prompt string
	var filePaths []string
	var imageModelCode string
	var videoModelCode string
	var downloadDir string
	var raw bool
	var liveAssets bool
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Create a conversation and submit its first short-drama turn",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			prompt = strings.TrimSpace(prompt)
			if prompt == "" {
				return errors.New("缺少必填参数 --prompt")
			}
			attachments, err := uploadStreamAttachments(cmd.Context(), filePaths, deps)
			if err != nil {
				return err
			}
			options, err := shortDramaStreamOptions(imageModelCode, videoModelCode, attachments)
			if err != nil {
				return err
			}
			conversationID, err := deps.api.CreateConversation(cmd.Context(), prompt)
			if err != nil {
				return err
			}
			result, err := runStreamWithRecovery(cmd.Context(), stdout, stderr, deps, conversationID, prompt, options, 0, streamRunOptions{raw: raw, liveAssets: liveAssets, downloadDir: downloadDir}, true)
			if err != nil {
				return fmt.Errorf("短剧会话 %q 的首轮提交失败；可用 pavo short-drama resume --conversation-id %q 恢复：%w", conversationID, conversationID, err)
			}
			if err := downloadStreamResults(cmd.Context(), deps, result, downloadDir); err != nil {
				return err
			}
			return writeStreamResult(stdout, result, liveAssets)
		},
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	flags := cmd.Flags()
	flags.StringVar(&prompt, "prompt", "", "short-drama idea or instruction")
	flags.StringArrayVar(&filePaths, "file", nil, "local attachment to upload before the short-drama turn; repeat for multiple files")
	flags.StringVar(&imageModelCode, "image-model-code", defaultShortDramaImageModel, "image model code used by the short-drama agent")
	flags.StringVar(&videoModelCode, "video-model-code", defaultShortDramaVideoModel, "video model code used by the short-drama agent")
	flags.StringVar(&downloadDir, "download-dir", "", "directory to save successful generated results")
	flags.BoolVar(&raw, "raw", false, "write every raw stream event to stderr")
	flags.BoolVar(&liveAssets, "live-assets", false, "write each completed image or video as asset_ready JSONL to stdout")
	return cmd
}

func newShortDramaReplyCommand(stdout, stderr io.Writer, deps *dependencies) *cobra.Command {
	var conversationID string
	var prompt string
	var filePaths []string
	var imageModelCode string
	var videoModelCode string
	var downloadDir string
	var raw bool
	var liveAssets bool
	cmd := &cobra.Command{
		Use:   "reply",
		Short: "Submit the next turn in an existing short-drama conversation",
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
			options, err := shortDramaStreamOptions(imageModelCode, videoModelCode, attachments)
			if err != nil {
				return err
			}
			result, err := runStreamWithRecovery(cmd.Context(), stdout, stderr, deps, conversationID, prompt, options, 0, streamRunOptions{raw: raw, liveAssets: liveAssets, downloadDir: downloadDir}, true)
			if err != nil {
				return err
			}
			if err := downloadStreamResults(cmd.Context(), deps, result, downloadDir); err != nil {
				return err
			}
			return writeStreamResult(stdout, result, liveAssets)
		},
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	flags := cmd.Flags()
	flags.StringVar(&conversationID, "conversation-id", "", "short-drama conversation ID returned by start")
	flags.StringVar(&prompt, "prompt", "", "user reply or next short-drama instruction")
	flags.StringArrayVar(&filePaths, "file", nil, "local attachment to upload before the short-drama turn; repeat for multiple files")
	flags.StringVar(&imageModelCode, "image-model-code", defaultShortDramaImageModel, "image model code used by the short-drama agent")
	flags.StringVar(&videoModelCode, "video-model-code", defaultShortDramaVideoModel, "video model code used by the short-drama agent")
	flags.StringVar(&downloadDir, "download-dir", "", "directory to save successful generated results")
	flags.BoolVar(&raw, "raw", false, "write every raw stream event to stderr")
	flags.BoolVar(&liveAssets, "live-assets", false, "write each completed image or video as asset_ready JSONL to stdout")
	return cmd
}

func newShortDramaResumeCommand(stdout, stderr io.Writer, deps *dependencies) *cobra.Command {
	var conversationID string
	var fromSeq int64
	var downloadDir string
	var raw bool
	var liveAssets bool
	cmd := &cobra.Command{
		Use:   "resume",
		Short: "Reconnect to an active short-drama turn without submitting another reply",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conversationID = strings.TrimSpace(conversationID)
			if conversationID == "" {
				return errors.New("缺少必填参数 --conversation-id")
			}
			if fromSeq < 0 {
				return errors.New("from_seq 不能为负数")
			}
			result, err := runStreamWithRecovery(cmd.Context(), stdout, stderr, deps, conversationID, "", api.StreamOptions{}, fromSeq, streamRunOptions{raw: raw, liveAssets: liveAssets, downloadDir: downloadDir}, false)
			if err != nil {
				return err
			}
			if err := downloadStreamResults(cmd.Context(), deps, result, downloadDir); err != nil {
				return err
			}
			return writeStreamResult(stdout, result, liveAssets)
		},
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	flags := cmd.Flags()
	flags.StringVar(&conversationID, "conversation-id", "", "short-drama conversation ID to reconnect")
	flags.Int64Var(&fromSeq, "from-seq", 0, "only replay events with seq greater than this value")
	flags.StringVar(&downloadDir, "download-dir", "", "directory to save successful generated results")
	flags.BoolVar(&raw, "raw", false, "write every raw stream event to stderr")
	flags.BoolVar(&liveAssets, "live-assets", false, "write each completed image or video as asset_ready JSONL to stdout")
	return cmd
}

func newShortDramaStatusCommand(stdout, stderr io.Writer, deps *dependencies) *cobra.Command {
	var conversationID string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Get the current running state of a short-drama conversation",
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
	cmd.Flags().StringVar(&conversationID, "conversation-id", "", "short-drama conversation ID to inspect")
	return cmd
}

func newShortDramaResultCommand(stdout, stderr io.Writer, deps *dependencies) *cobra.Command {
	var conversationID string
	var downloadDir string
	cmd := &cobra.Command{
		Use:   "result",
		Short: "Read durable generated results from a short-drama conversation",
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
	cmd.Flags().StringVar(&conversationID, "conversation-id", "", "short-drama conversation ID whose results should be read")
	cmd.Flags().StringVar(&downloadDir, "download-dir", "", "directory to save successful generated results")
	return cmd
}

func shortDramaStreamOptions(imageModelCode, videoModelCode string, files []api.ChatAttachment) (api.StreamOptions, error) {
	imageModelCode = strings.TrimSpace(imageModelCode)
	videoModelCode = strings.TrimSpace(videoModelCode)
	if imageModelCode == "" || videoModelCode == "" {
		return api.StreamOptions{}, errors.New("短剧需要非空的 --image-model-code 和 --video-model-code")
	}
	return api.StreamOptions{
		Mode:  api.StreamModeShortDrama,
		Files: files,
		ExtraContext: &api.StreamExtraContext{AgentParams: &api.StreamAgentParams{
			ImageModelCode: imageModelCode,
			VideoModelCode: videoModelCode,
		}},
	}, nil
}
