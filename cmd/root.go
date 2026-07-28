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
	config       *config.Config
	api          *api.Client
	store        auth.Store
	readPassword func() (string, error)
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
		readPassword: func() (string, error) {
			return readPassword(os.Stdin, stderr)
		},
	}
	return newRootCommand(stdout, stderr, deps), nil
}

func newRootCommand(stdout, stderr io.Writer, deps *dependencies) *cobra.Command {
	root := &cobra.Command{
		Use:           "pavo",
		Short:         "PAVO CLI",
		Long:          "PAVO CLI for desktop agents: login, create conversations, and stream design generation.",
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
	root.AddCommand(newStreamCommand(stdout, stderr, deps))
	root.AddCommand(newUploadCommand(stdout, stderr, deps))
	root.AddCommand(newDownloadResultCommand(stdout, stderr, deps))
	root.AddCommand(updatecmd.NewCommand(stdout, stderr))
	localizeFlagErrors(root)
	return root
}

func readPassword(stdin *os.File, stderr io.Writer) (string, error) {
	fmt.Fprint(stderr, "Password: ")
	if term.IsTerminal(int(stdin.Fd())) {
		password, err := term.ReadPassword(int(stdin.Fd()))
		fmt.Fprintln(stderr)
		if err != nil {
			return "", fmt.Errorf("读取密码失败: %w", err)
		}
		return string(password), nil
	}
	scanner := bufio.NewScanner(stdin)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", fmt.Errorf("读取密码失败: %w", err)
		}
		return "", errors.New("没有从标准输入读取到密码")
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
