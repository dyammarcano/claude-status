//go:build windows

package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	runKeyPath   = `Software\Microsoft\Windows\CurrentVersion\Run`
	runValueName = "claude-status-monitor"
)

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Manage the auto-start monitor (runs hidden at logon)",
	Long: "Register claude-status to start automatically when you log in. It runs as a\n" +
		"hidden background process in your user session (so it can still show toasts —\n" +
		"a true Windows service runs in session 0 and cannot), logging to a file.",
}

var serviceInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Register the monitor to start hidden at logon",
	RunE:  runServiceInstall,
}

var serviceUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove the logon auto-start entry",
	RunE:  runServiceUninstall,
}

var serviceStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show whether auto-start is installed",
	RunE:  runServiceStatus,
}

func init() {
	rootCmd.AddCommand(serviceCmd)
	serviceCmd.AddCommand(serviceInstallCmd, serviceUninstallCmd, serviceStatusCmd)
}

// runValue is the Run-key command line: this executable, run hidden in the background.
func runValue() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate executable: %w", err)
	}

	return fmt.Sprintf(`"%s" --background --interval=%s`, exe, flagInterval), nil
}

func runServiceInstall(cmd *cobra.Command, _ []string) error {
	value, err := runValue()
	if err != nil {
		return err
	}

	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open Run key: %w", err)
	}

	defer func() { _ = k.Close() }()

	if err := k.SetStringValue(runValueName, value); err != nil {
		return fmt.Errorf("set Run value: %w", err)
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Installed — the monitor starts hidden at your next logon:\n  %s = %s\n", runValueName, value)
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Start it now without re-logging in: run  claude-status --background  (it self-hides).")

	return nil
}

func runServiceUninstall(cmd *cobra.Command, _ []string) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open Run key: %w", err)
	}

	defer func() { _ = k.Close() }()

	if err := k.DeleteValue(runValueName); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return fmt.Errorf("delete Run value: %w", err)
	}

	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Uninstalled — auto-start removed.")

	return nil
}

func runServiceStatus(cmd *cobra.Command, _ []string) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return fmt.Errorf("open Run key: %w", err)
	}

	defer func() { _ = k.Close() }()

	val, _, err := k.GetStringValue(runValueName)
	if errors.Is(err, registry.ErrNotExist) {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Auto-start: NOT installed.")

		return nil
	}

	if err != nil {
		return fmt.Errorf("read Run value: %w", err)
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Auto-start: installed\n  %s\n", val)

	return nil
}

// hideConsole hides the console window of the current process (used in --background
// mode so the logon-started monitor has no visible window).
func hideConsole() {
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	user32 := windows.NewLazySystemDLL("user32.dll")

	hwnd, _, _ := kernel32.NewProc("GetConsoleWindow").Call()
	if hwnd == 0 {
		return
	}

	const swHide = 0

	ret, _, _ := user32.NewProc("ShowWindow").Call(hwnd, swHide)
	_ = ret
}
