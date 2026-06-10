package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dyammarcano/claude-status/internal/monitor"
	"github.com/dyammarcano/claude-status/internal/notify"
	"github.com/dyammarcano/claude-status/internal/statuspage"
	"github.com/spf13/cobra"
)

const (
	defaultURL       = "https://status.claude.com/api/v2/status.json"
	defaultUserAgent = "claude-status-monitor/dev"
	httpTimeout      = 10 * time.Second
)

var (
	flagInterval time.Duration
	flagURL      string
)

var rootCmd = &cobra.Command{
	Use:   "claude-status",
	Short: "Monitor status.claude.com and toast on state changes",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if flagInterval < 10*time.Second {
			return fmt.Errorf("--interval must be at least 10s, got %s", flagInterval)
		}

		logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		client := statuspage.NewClient(flagURL, defaultUserAgent, httpTimeout)
		notifier := notify.New("claude-status")

		m := &monitor.Monitor{
			Fetcher:  client,
			Notifier: notifier,
			Interval: flagInterval,
			Logger:   logger,
		}

		logger.Info("starting monitor", "url", flagURL, "interval", flagInterval)

		return m.Run(ctx)
	},
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().DurationVar(&flagInterval, "interval", 60*time.Second, "polling interval (min 10s)")
	rootCmd.PersistentFlags().StringVar(&flagURL, "url", defaultURL, "Statuspage v2 status.json URL")
	rootCmd.Version = GetVersionJSON()
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "verbose output")
}
