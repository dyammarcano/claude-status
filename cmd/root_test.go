package cmd

import (
	"strings"
	"testing"
	"time"
)

func TestRootCmdIntervalValidation(t *testing.T) {
	orig := flagInterval

	defer func() { flagInterval = orig }()

	flagInterval = 5 * time.Second

	err := rootCmd.RunE(rootCmd, nil)
	if err == nil {
		t.Fatal("expected error for interval below minimum, got nil")
	}

	if !strings.Contains(err.Error(), "interval") {
		t.Errorf("error %q does not mention 'interval'", err.Error())
	}
}

func TestRootCmdPersistentFlags(t *testing.T) {
	t.Run("interval", func(t *testing.T) {
		f := rootCmd.PersistentFlags().Lookup("interval")
		if f == nil {
			t.Fatal("interval persistent flag not registered")
		}

		if f.DefValue != (60 * time.Second).String() {
			t.Errorf("interval default = %q, want %q", f.DefValue, (60 * time.Second).String())
		}
	})

	t.Run("url", func(t *testing.T) {
		f := rootCmd.PersistentFlags().Lookup("url")
		if f == nil {
			t.Fatal("url persistent flag not registered")
		}

		if !strings.Contains(f.DefValue, "status.claude.com") {
			t.Errorf("url default = %q, want it to contain status.claude.com", f.DefValue)
		}
	})
}

func TestRootCmdVersionAssigned(t *testing.T) {
	if rootCmd.Version != GetVersionJSON() {
		t.Errorf("rootCmd.Version = %q, want GetVersionJSON output %q", rootCmd.Version, GetVersionJSON())
	}
}
