package updatecmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

const defaultPackage = "@pavo-dev/cli"

func NewCommand(stdout, stderr io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update PAVO CLI and its skills",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runUpdate(stdout, stderr)
		},
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	return cmd
}

func runUpdate(stdout, stderr io.Writer) error {
	pkg := strings.TrimSpace(os.Getenv("PAVO_CLI_INSTALL_PACKAGE"))
	if pkg == "" {
		pkg = defaultPackage + "@latest"
	}
	fmt.Fprintf(stderr, "Updating PAVO CLI via npm: %s\n", pkg)
	restore, err := prepareSelfReplace()
	if err != nil {
		return fmt.Errorf("准备替换当前可执行文件失败: %w", err)
	}
	if err := runInheritEnv(stderr, []string{"PAVO_CLI_SKIP_SKILLS=1"}, "npm", "install", "-g", pkg); err != nil {
		restore()
		return fmt.Errorf("更新 PAVO CLI 失败: %w", err)
	}
	root, err := globalPackageRoot(defaultPackage)
	if err != nil {
		return fmt.Errorf("定位全局 npm 包失败: %w", err)
	}
	fmt.Fprintln(stderr, "Removing the retired PAVO legacy skill...")
	if err := runInherit(stderr, "node", filepath.Join(root, "scripts", "skills.js"), "remove-legacy"); err != nil {
		return fmt.Errorf("移除旧 PAVO Skill 失败: %w", err)
	}
	fmt.Fprintln(stderr, "Updating PAVO skills...")
	if err := runInherit(stderr, "npx", "-y", "skills", "add", root, "-g", "-y", "--skill", "*"); err != nil {
		return fmt.Errorf("更新 PAVO Skill 失败: %w", err)
	}
	fmt.Fprintln(stdout, "PAVO CLI and skills updated")
	return nil
}

func globalPackageRoot(pkg string) (string, error) {
	out, err := command("npm", "root", "-g").Output()
	if err != nil {
		return "", err
	}
	npmRoot := strings.TrimSpace(string(out))
	if npmRoot == "" {
		return "", fmt.Errorf("npm root -g 输出为空")
	}
	return filepath.Join(append([]string{npmRoot}, strings.Split(pkg, "/")...)...), nil
}

func runInherit(stderr io.Writer, name string, args ...string) error {
	return runInheritEnv(stderr, nil, name, args...)
}

func runInheritEnv(stderr io.Writer, env []string, name string, args ...string) error {
	cmd := command(name, args...)
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	cmd.Stdout = stderr
	cmd.Stderr = stderr
	return cmd.Run()
}

func command(name string, args ...string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		cmdArgs := append([]string{"/c", name}, args...)
		return exec.Command("cmd.exe", cmdArgs...)
	}
	return exec.Command(name, args...)
}

func prepareSelfReplace() (func(), error) {
	if runtime.GOOS != "windows" {
		return func() {}, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	if filepath.Base(exe) != "pavo.exe" || !strings.Contains(exe, "node_modules") {
		return func() {}, nil
	}
	old := exe + ".old"
	_ = os.Remove(old)
	if err := os.Rename(exe, old); err != nil {
		return nil, err
	}
	return func() {
		if _, err := os.Stat(exe); err == nil {
			return
		}
		_ = os.Rename(old, exe)
	}, nil
}
