//go:build !windows

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Manage the auto-start monitor (Windows only)",
	RunE: func(_ *cobra.Command, _ []string) error {
		return fmt.Errorf("service auto-start management is only supported on Windows")
	},
}

func init() { rootCmd.AddCommand(serviceCmd) }

// hideConsole is a no-op off Windows.
func hideConsole() {}
