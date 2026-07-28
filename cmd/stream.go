package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/maoshuhua/pavo-cli/internal/api"
	"github.com/maoshuhua/pavo-cli/internal/output"
	"github.com/spf13/cobra"
)

const (
	maxAutomaticResumeAttempts = 3
	resumeRetryDelay           = time.Second
)

func newStreamCommand(stdout, stderr io.Writer, deps *dependencies) *cobra.Command {
	var conversationID string
	var prompt string
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
			result, err := runStreamWithRecovery(cmd.Context(), stderr, deps, conversationID, prompt, 0, raw, true)
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

func newResumeCommand(stdout, stderr io.Writer, deps *dependencies) *cobra.Command {
	var conversationID string
	var fromSeq int64
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
			result, err := runStreamWithRecovery(cmd.Context(), stderr, deps, conversationID, "", fromSeq, raw, false)
			if err != nil {
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
	flags.BoolVar(&raw, "raw", false, "write every raw stream event to stderr")
	return cmd
}

func runStreamWithRecovery(
	ctx context.Context,
	stderr io.Writer,
	deps *dependencies,
	conversationID string,
	prompt string,
	fromSeq int64,
	raw bool,
	start bool,
) (*api.StreamOutput, error) {
	lastSeq := fromSeq
	handler := func(event *api.StreamEvent) error {
		if event.Seq > lastSeq {
			lastSeq = event.Seq
		}
		return writeStreamEvent(stderr, raw, event)
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
			result, err = deps.api.Stream(ctx, conversationID, prompt, handler)
		}
		if err == nil {
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
