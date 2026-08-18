package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	updatecmd "github.com/maoshuhua/pavo-cli/cmd/update"
	"github.com/maoshuhua/pavo-cli/internal/api"
	"github.com/maoshuhua/pavo-cli/internal/auth"
	"github.com/maoshuhua/pavo-cli/internal/config"
	"github.com/maoshuhua/pavo-cli/internal/output"
	"github.com/maoshuhua/pavo-cli/internal/version"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

type dependencies struct {
	config               *config.Config
	api                  *api.Client
	store                auth.Store
	readVerificationCode func() (string, error)
}

func Execute() error {
	root, err := NewRootCommand(os.Stdout, os.Stderr)
	if err != nil {
		return err
	}
	err = root.Execute()
	if err != nil {
		_ = output.AppendError(root.CommandPath(), err)
	}
	return err
}

func NewRootCommand(stdout, stderr io.Writer) (*cobra.Command, error) {
	cfg := config.Load()
	store, err := auth.NewDefaultFileStore(cfg.ConfigFile)
	if err != nil {
		return nil, err
	}
	tokenProvider := func() (string, error) {
		token, _, err := auth.ResolveToken(cfg.AccessToken, store)
		return token, err
	}
	client := api.NewClient(cfg.BaseURL, cfg.HTTPTimeout, cfg.Paths, tokenProvider)
	deps := &dependencies{
		config: cfg,
		api:    client,
		store:  store,
		readVerificationCode: func() (string, error) {
			return readVerificationCode(os.Stdin, stderr)
		},
	}
	return newRootCommand(stdout, stderr, deps), nil
}

func newRootCommand(stdout, stderr io.Writer, deps *dependencies) *cobra.Command {
	root := &cobra.Command{
		Use:           "pavo",
		Short:         "PAVO CLI",
		Long:          "PAVO CLI for desktop clients: discover models, query personal creations, and generate images, videos, and short dramas.",
		Version:       version.Current(),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.CompletionOptions.DisableDefaultCmd = true
	root.SetVersionTemplate("{{.Version}}\n")
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.AddCommand(newLoginCommand(stdout, stderr, deps))
	root.AddCommand(newConversationCommand(stdout, stderr, deps))
	root.AddCommand(newShortDramaCommand(stdout, stderr, deps))
	root.AddCommand(newModelsCommand(stdout, stderr, deps))
	root.AddCommand(newVisualsCommand(stdout, stderr, deps))
	root.AddCommand(newGenerateCommand(stdout, stderr, deps))
	root.AddCommand(newResumeCommand(stdout, stderr, deps))
	root.AddCommand(newUploadCommand(stdout, stderr, deps))
	root.AddCommand(newDownloadResultCommand(stdout, stderr, deps))
	root.AddCommand(updatecmd.NewCommand(stdout, stderr))
	localizeFlagErrors(root)
	return root
}

func readVerificationCode(stdin *os.File, stderr io.Writer) (string, error) {
	fmt.Fprint(stderr, "Verification code: ")
	if term.IsTerminal(int(stdin.Fd())) {
		code, err := term.ReadPassword(int(stdin.Fd()))
		fmt.Fprintln(stderr)
		if err != nil {
			return "", fmt.Errorf("读取验证码失败: %w", err)
		}
		return string(code), nil
	}
	scanner := bufio.NewScanner(stdin)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", fmt.Errorf("读取验证码失败: %w", err)
		}
		return "", errors.New("没有从标准输入读取到验证码")
	}
	return scanner.Text(), nil
}

func localizeFlagErrors(cmd *cobra.Command) {
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return localizeFlagError(err)
	})
	for _, child := range cmd.Commands() {
		localizeFlagErrors(child)
	}
}

func localizeFlagError(err error) error {
	msg := err.Error()
	if flag, ok := strings.CutPrefix(msg, "unknown flag: "); ok {
		return fmt.Errorf("未知参数: %s", flag)
	}
	if flag, ok := strings.CutPrefix(msg, "flag needs an argument: "); ok {
		return fmt.Errorf("参数 %s 缺少取值", flag)
	}
	return fmt.Errorf("参数解析失败: %s", msg)
}
