package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/maoshuhua/pavo-cli/internal/api"
	"github.com/maoshuhua/pavo-cli/internal/output"
	"github.com/spf13/cobra"
)

const (
	maxAutomaticResumeAttempts = 3
	resumeRetryDelay           = time.Second
)

type streamRunOptions struct {
	raw         bool
	liveAssets  bool
	downloadDir string
}

// streamAssetReadyOutput is emitted as one JSONL record for each downloaded
// image or video when --live-assets is enabled.
type streamAssetReadyOutput struct {
	Type           string             `json:"type"`
	ConversationID string             `json:"conversation_id"`
	Seq            int64              `json:"seq"`
	Asset          api.GeneratedAsset `json:"asset"`
}

// streamCompleteOutput is the final JSONL record when --live-assets is
// enabled. Its result has the same shape as the legacy final JSON output.
type streamCompleteOutput struct {
	Type   string            `json:"type"`
	Result *api.StreamOutput `json:"result"`
}

func newStreamCommand(stdout, stderr io.Writer, deps *dependencies) *cobra.Command {
	var conversationID string
	var prompt string
	var filePaths []string
	var downloadDir string
	var raw bool
	cmd := &cobra.Command{
		Use:   "stream",
		Short: "Start a PAVO design generation and reconnect if its stream drops",
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
			result, err := runStreamWithRecovery(cmd.Context(), stdout, stderr, deps, conversationID, prompt, api.StreamOptions{
				Mode:  api.StreamModeDesign,
				Files: attachments,
			}, 0, streamRunOptions{raw: raw}, true)
			if err != nil {
				return err
			}
			if err := downloadStreamResults(cmd.Context(), deps, result, downloadDir); err != nil {
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
	flags.StringVar(&downloadDir, "download-dir", "", "directory to save successful generated results")
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

func newResumeCommand(stdout, stderr io.Writer, deps *dependencies) *cobra.Command {
	var conversationID string
	var fromSeq int64
	var downloadDir string
	var raw bool
	cmd := &cobra.Command{
		Use:   "resume",
		Short: "Resume an existing PAVO generation without submitting a new job",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conversationID = strings.TrimSpace(conversationID)
			if conversationID == "" {
				return errors.New("缺少必填参数 --conversation-id")
			}
			if fromSeq < 0 {
				return errors.New("from_seq 不能为负数")
			}
			result, err := runStreamWithRecovery(cmd.Context(), stdout, stderr, deps, conversationID, "", api.StreamOptions{}, fromSeq, streamRunOptions{raw: raw}, false)
			if err != nil {
				return err
			}
			if err := downloadStreamResults(cmd.Context(), deps, result, downloadDir); err != nil {
				return err
			}
			return output.WriteJSON(stdout, result)
		},
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	flags := cmd.Flags()
	flags.StringVar(&conversationID, "conversation-id", "", "conversation ID to reconnect")
	flags.Int64Var(&fromSeq, "from-seq", 0, "only replay events with seq greater than this value")
	flags.StringVar(&downloadDir, "download-dir", "", "directory to save successful generated results")
	flags.BoolVar(&raw, "raw", false, "write every raw stream event to stderr")
	return cmd
}

// downloadStreamResults saves successful generated assets when the caller
// requests a local handoff. The stream API intentionally returns remote URLs;
// this optional step makes them usable by desktop chat renderers that require
// absolute local file paths.
func downloadStreamResults(ctx context.Context, deps *dependencies, result *api.StreamOutput, rawDir string) error {
	downloadDir := strings.TrimSpace(rawDir)
	if downloadDir == "" || result == nil {
		return nil
	}
	absDir, err := filepath.Abs(downloadDir)
	if err != nil {
		return fmt.Errorf("解析下载目录失败: %w", err)
	}
	if len(result.Assets) == 0 && len(result.Results) > 0 {
		result.Assets = make([]api.GeneratedAsset, len(result.Results))
		for index, generated := range result.Results {
			result.Assets[index] = api.GeneratedAsset{Result: generated}
		}
	}
	for index := range result.Assets {
		asset := &result.Assets[index]
		generated := &asset.Result
		if !generated.Success || strings.TrimSpace(generated.URL) == "" {
			continue
		}
		if localPath := strings.TrimSpace(generated.LocalPath); localPath != "" {
			if info, err := os.Stat(localPath); err == nil && !info.IsDir() {
				setResultLocalPath(result, generated, localPath)
				continue
			}
		}
		outputPath := filepath.Join(absDir, generatedAssetFilename(*asset, index))
		if err := downloadGeneratedAsset(ctx, deps, *asset, outputPath); err != nil {
			return fmt.Errorf("下载第 %d 个生成结果失败: %w", index+1, err)
		}
		setResultLocalPath(result, generated, outputPath)
	}
	return nil
}

func downloadGeneratedAsset(ctx context.Context, deps *dependencies, asset api.GeneratedAsset, outputPath string) error {
	_, err := deps.api.DownloadResult(ctx, api.DownloadResultOptions{
		URL:        asset.Result.URL,
		OutputPath: outputPath,
		Force:      true,
	})
	return err
}

func setResultLocalPath(result *api.StreamOutput, generated *api.GenerationResult, localPath string) {
	generated.LocalPath = localPath
	for resultIndex := range result.Results {
		if sameGeneratedResult(result.Results[resultIndex], *generated) {
			result.Results[resultIndex].LocalPath = localPath
		}
	}
}

func generatedAssetFilename(asset api.GeneratedAsset, index int) string {
	parts := make([]string, 0, 3)
	if value := safeFilenamePart(asset.Group); value != "" {
		parts = append(parts, value)
	}
	if value := safeFilenamePart(asset.ItemID); value != "" {
		parts = append(parts, value)
	}
	if value := safeFilenamePart(asset.Title); value != "" {
		parts = append(parts, value)
	}
	if len(parts) == 0 {
		if value := safeFilenamePart(asset.Kind); value != "" {
			parts = append(parts, value)
		}
	}
	if len(parts) == 0 {
		parts = append(parts, "result")
	}
	return fmt.Sprintf("%02d-%s%s", index+1, strings.Join(parts, "-"), generatedResultExtension(asset.Result.Mimetype))
}

func safeFilenamePart(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var builder strings.Builder
	previousSeparator := false
	for _, char := range value {
		if unicode.IsLetter(char) || unicode.IsDigit(char) {
			builder.WriteRune(char)
			previousSeparator = false
			continue
		}
		if !previousSeparator {
			builder.WriteByte('-')
			previousSeparator = true
		}
	}
	return strings.Trim(builder.String(), "-.")
}

func sameGeneratedResult(left, right api.GenerationResult) bool {
	leftURL := strings.TrimSpace(left.URL)
	rightURL := strings.TrimSpace(right.URL)
	if leftURL != "" || rightURL != "" {
		return leftURL == rightURL
	}
	return strings.TrimSpace(left.Base64) != "" && left.Base64 == right.Base64
}

func generatedResultExtension(mimetype string) string {
	extensions, err := mime.ExtensionsByType(strings.TrimSpace(mimetype))
	if err == nil && len(extensions) > 0 {
		return extensions[0]
	}
	return ".bin"
}

func runStreamWithRecovery(
	ctx context.Context,
	stdout io.Writer,
	stderr io.Writer,
	deps *dependencies,
	conversationID string,
	prompt string,
	streamOptions api.StreamOptions,
	fromSeq int64,
	runOptions streamRunOptions,
	start bool,
) (*api.StreamOutput, error) {
	lastSeq := fromSeq
	collector := api.NewStreamCollector(conversationID)
	liveDownloadDir := strings.TrimSpace(runOptions.downloadDir)
	if runOptions.liveAssets && liveDownloadDir != "" {
		var err error
		liveDownloadDir, err = filepath.Abs(liveDownloadDir)
		if err != nil {
			return nil, fmt.Errorf("解析下载目录失败: %w", err)
		}
	}
	liveAssetIndex := 0
	handler := func(event *api.StreamEvent) error {
		if event.Seq > lastSeq {
			lastSeq = event.Seq
		}
		assets := collector.AddAndGetNewAssets(event)
		if err := writeStreamEvent(stderr, runOptions.raw, event); err != nil {
			return err
		}
		if !runOptions.liveAssets {
			return nil
		}
		for _, asset := range assets {
			if !asset.Result.Success || (strings.TrimSpace(asset.Result.URL) == "" && strings.TrimSpace(asset.Result.Base64) == "") {
				continue
			}
			liveAssetIndex++
			if liveDownloadDir != "" && strings.TrimSpace(asset.Result.URL) != "" {
				outputPath := filepath.Join(liveDownloadDir, generatedAssetFilename(asset, liveAssetIndex-1))
				if err := downloadGeneratedAsset(ctx, deps, asset, outputPath); err != nil {
					return fmt.Errorf("下载实时生成结果失败: %w", err)
				}
				asset.Result.LocalPath = outputPath
				collector.SetAssetLocalPath(asset, outputPath)
			}
			if err := output.WriteJSON(stdout, streamAssetReadyOutput{
				Type:           "asset_ready",
				ConversationID: conversationID,
				Seq:            event.Seq,
				Asset:          asset,
			}); err != nil {
				return err
			}
		}
		return nil
	}

	resume := !start
	for attempts := 0; ; attempts++ {
		var (
			result *api.StreamOutput
			err    error
		)
		if resume {
			result, err = deps.api.Resume(ctx, conversationID, lastSeq, handler)
		} else {
			result, err = deps.api.StreamWithOptions(ctx, conversationID, prompt, streamOptions, handler)
		}
		if err == nil {
			if combined := collector.Output(); combined != nil && combined.TerminalType != "" {
				return combined, nil
			}
			return result, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if !api.IsRecoverableStreamError(err) {
			return nil, err
		}
		if attempts >= maxAutomaticResumeAttempts {
			return nil, fmt.Errorf("PAVO 流多次断开，仍可稍后运行 pavo resume --conversation-id %q：%w", conversationID, err)
		}
		wasBusy := !resume && api.IsAgentStreamBusy(err)
		if wasBusy {
			fmt.Fprintln(stderr, "已有生成任务在运行，正在连接其现有流…")
		} else {
			fmt.Fprintln(stderr, "PAVO 流已断开，正在从已接收的位置恢复…")
		}
		resume = true
		if wasBusy {
			continue
		}
		if err := waitForResumeRetry(ctx); err != nil {
			return nil, err
		}
	}
}

func writeStreamResult(stdout io.Writer, result *api.StreamOutput, liveAssets bool) error {
	if liveAssets {
		return output.WriteJSON(stdout, streamCompleteOutput{Type: "complete", Result: result})
	}
	return output.WriteJSON(stdout, result)
}

func writeStreamEvent(stderr io.Writer, raw bool, event *api.StreamEvent) error {
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

func waitForResumeRetry(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(resumeRetryDelay):
		return nil
	}
}
