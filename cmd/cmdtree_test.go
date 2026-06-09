package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// newNestedRoot builds a synthetic command tree to exercise recursion,
// description truncation, and the Long-fallback branches without touching
// the real rootCmd.
func newNestedRoot() *cobra.Command {
	root := &cobra.Command{Use: "synthroot", Short: "synthetic root"}

	// Long description, no Short -> exercises the desc==""->Long fallback
	// plus the >maxDescLen truncation path in printCommands.
	parent := &cobra.Command{
		Use:  "parent",
		Long: "this is a very long description that definitely exceeds the maximum length allowed",
	}

	child := &cobra.Command{Use: "child", Short: "a child command"}
	parent.AddCommand(child)

	sibling := &cobra.Command{Use: "sibling", Short: "a sibling command"}

	root.AddCommand(parent)
	root.AddCommand(sibling)

	return root
}

func TestPrintCommandsNested(t *testing.T) {
	root := newNestedRoot()

	var buf bytes.Buffer

	printCommands(&buf, root.Commands(), "")

	out := buf.String()
	if !strings.Contains(out, "parent") {
		t.Errorf("nested printCommands missing parent\noutput: %q", out)
	}

	if !strings.Contains(out, "child") {
		t.Errorf("nested printCommands missing recursed child\noutput: %q", out)
	}

	// Truncated long description ends with ellipsis.
	if !strings.Contains(out, "...") {
		t.Errorf("expected truncated description with ellipsis\noutput: %q", out)
	}
}

func TestPrintVerboseCommandsNested(t *testing.T) {
	root := newNestedRoot()

	var buf bytes.Buffer

	printVerboseCommands(&buf, root.Commands(), "")

	out := buf.String()
	if !strings.Contains(out, "child") {
		t.Errorf("nested verbose missing recursed child\noutput: %q", out)
	}

	// Long-fallback description should be present.
	if !strings.Contains(out, "Description: this is a very long") {
		t.Errorf("verbose missing Long fallback description\noutput: %q", out)
	}
}

func TestBuildCommandDetailNested(t *testing.T) {
	root := newNestedRoot()

	detail := buildCommandDetail(root)
	if len(detail.Subcommands) != 2 {
		t.Fatalf("expected 2 subcommands, got %d", len(detail.Subcommands))
	}

	var parentDetail *CommandDetail

	for i := range detail.Subcommands {
		if detail.Subcommands[i].Name == "parent" {
			parentDetail = &detail.Subcommands[i]
		}
	}

	if parentDetail == nil {
		t.Fatal("parent subcommand detail not found")
	}

	if len(parentDetail.Subcommands) != 1 {
		t.Errorf("expected parent to have 1 subcommand, got %d", len(parentDetail.Subcommands))
	}
}

func resetCmdtreeFlags() {
	cmdtreeVerbose = true
	cmdtreeBrief = false
	cmdtreeCommand = ""
	cmdtreeJSON = false
}

func TestCmdtreeRunE(t *testing.T) {
	t.Run("verbose default", func(t *testing.T) {
		resetCmdtreeFlags()
		defer resetCmdtreeFlags()

		var buf bytes.Buffer

		cmd := newBufCmd(&buf)

		if err := cmdtreeCmd.RunE(cmd, nil); err != nil {
			t.Fatalf("RunE verbose: %v", err)
		}

		out := buf.String()
		if !strings.Contains(out, "# Command Tree") {
			t.Errorf("missing tree header\noutput: %q", out)
		}

		// Verbose output prints "Usage:" detail lines.
		if !strings.Contains(out, "Usage:") {
			t.Errorf("verbose output missing Usage detail")
		}
	})

	t.Run("brief", func(t *testing.T) {
		resetCmdtreeFlags()
		defer resetCmdtreeFlags()

		cmdtreeBrief = true

		var buf bytes.Buffer

		cmd := newBufCmd(&buf)

		if err := cmdtreeCmd.RunE(cmd, nil); err != nil {
			t.Fatalf("RunE brief: %v", err)
		}

		out := buf.String()
		if !strings.Contains(out, "# Command Tree") {
			t.Errorf("missing tree header in brief\noutput: %q", out)
		}

		if !strings.Contains(out, "version") {
			t.Errorf("brief output missing version command")
		}
	})

	t.Run("json", func(t *testing.T) {
		resetCmdtreeFlags()
		defer resetCmdtreeFlags()

		cmdtreeJSON = true

		var buf bytes.Buffer

		cmd := newBufCmd(&buf)

		if err := cmdtreeCmd.RunE(cmd, nil); err != nil {
			t.Fatalf("RunE json: %v", err)
		}

		var detail CommandDetail
		if err := json.Unmarshal(buf.Bytes(), &detail); err != nil {
			t.Fatalf("output is not valid JSON: %v\noutput: %q", err, buf.String())
		}

		if detail.Name != "claude-status" {
			t.Errorf("detail.Name = %q, want claude-status", detail.Name)
		}

		if len(detail.Subcommands) == 0 {
			t.Error("detail.Subcommands is empty")
		}
	})

	t.Run("specific command", func(t *testing.T) {
		resetCmdtreeFlags()
		defer resetCmdtreeFlags()

		cmdtreeCommand = "version"

		var buf bytes.Buffer

		cmd := newBufCmd(&buf)

		if err := cmdtreeCmd.RunE(cmd, nil); err != nil {
			t.Fatalf("RunE specific command: %v", err)
		}

		out := buf.String()
		if !strings.Contains(out, "# version") {
			t.Errorf("single-command output missing version header\noutput: %q", out)
		}
	})
}

func TestBuildTree(t *testing.T) {
	out := string(buildTree(rootCmd))

	if !strings.Contains(out, rootCmd.Use) {
		t.Errorf("buildTree missing root use line\noutput: %q", out)
	}

	if !strings.Contains(out, "Global flags:") {
		t.Errorf("buildTree missing global flags section")
	}

	if !strings.Contains(out, "version") {
		t.Errorf("buildTree missing version command")
	}
}

func TestBuildVerboseTree(t *testing.T) {
	out := string(buildVerboseTree(rootCmd))

	if !strings.Contains(out, "Global Flags:") {
		t.Errorf("buildVerboseTree missing global flags section\noutput: %q", out)
	}

	if !strings.Contains(out, "Usage:") {
		t.Errorf("buildVerboseTree missing usage details")
	}
}

func TestPrintCommands(t *testing.T) {
	var buf bytes.Buffer

	printCommands(&buf, rootCmd.Commands(), "")

	out := buf.String()
	if !strings.Contains(out, "version") {
		t.Errorf("printCommands missing version\noutput: %q", out)
	}

	if !strings.Contains(out, "#") {
		t.Errorf("printCommands missing description comment marker")
	}
}

func TestPrintVerboseCommands(t *testing.T) {
	var buf bytes.Buffer

	printVerboseCommands(&buf, rootCmd.Commands(), "")

	out := buf.String()
	if !strings.Contains(out, "Usage:") {
		t.Errorf("printVerboseCommands missing Usage\noutput: %q", out)
	}

	if !strings.Contains(out, "Flags:") {
		t.Errorf("printVerboseCommands missing Flags section")
	}
}

func TestCollectFlags(t *testing.T) {
	flags := collectFlags(cmdtreeCmd)
	if len(flags) == 0 {
		t.Fatal("collectFlags returned no flags for cmdtree")
	}

	var found bool

	for _, f := range flags {
		if f.Name == "brief" {
			found = true

			if f.Type != "bool" {
				t.Errorf("brief flag type = %q, want bool", f.Type)
			}
		}
	}

	if !found {
		t.Error("collectFlags missing 'brief' flag")
	}
}

func TestCollectPersistentFlags(t *testing.T) {
	flags := collectPersistentFlags(rootCmd)
	if len(flags) == 0 {
		t.Fatal("collectPersistentFlags returned no flags for root")
	}

	names := make(map[string]bool)
	for _, f := range flags {
		names[f.Name] = true
	}

	for _, want := range []string{"interval", "url"} {
		if !names[want] {
			t.Errorf("collectPersistentFlags missing %q", want)
		}
	}
}

func TestPrintFlagDetail(t *testing.T) {
	t.Run("with shorthand non-bool", func(t *testing.T) {
		var buf bytes.Buffer

		printFlagDetail(&buf, "  ", FlagDetail{
			Name:        "command",
			Shorthand:   "c",
			Type:        "string",
			Description: "pick a command",
		})

		out := buf.String()
		if !strings.Contains(out, "-c, --command string") {
			t.Errorf("flag detail format wrong\noutput: %q", out)
		}

		if !strings.Contains(out, "pick a command") {
			t.Errorf("flag detail missing description")
		}
	})

	t.Run("no shorthand bool", func(t *testing.T) {
		var buf bytes.Buffer

		printFlagDetail(&buf, "  ", FlagDetail{
			Name:        "json",
			Type:        "bool",
			Description: "json output",
		})

		out := buf.String()
		if !strings.Contains(out, "    --json") {
			t.Errorf("bool flag detail format wrong\noutput: %q", out)
		}

		if strings.Contains(out, "bool") {
			t.Errorf("bool type should not be appended\noutput: %q", out)
		}
	})
}

func TestPrintSingleCommand(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		resetCmdtreeFlags()
		defer resetCmdtreeFlags()

		var buf bytes.Buffer

		cmd := newBufCmd(&buf)

		if err := printSingleCommand(cmd, rootCmd, "cmdtree"); err != nil {
			t.Fatalf("printSingleCommand found: %v", err)
		}

		out := buf.String()
		if !strings.Contains(out, "# cmdtree") {
			t.Errorf("missing command header\noutput: %q", out)
		}

		if !strings.Contains(out, "Flags:") {
			t.Errorf("cmdtree should list flags")
		}

		if !strings.Contains(out, "Subcommands:") {
			// cmdtree has no subcommands; ensure we did not crash and printed flags.
			t.Logf("cmdtree single-command output (no subcommands section expected)")
		}
	})

	t.Run("found root with globals and subs", func(t *testing.T) {
		resetCmdtreeFlags()
		defer resetCmdtreeFlags()

		var buf bytes.Buffer

		cmd := newBufCmd(&buf)

		if err := printSingleCommand(cmd, rootCmd, "claude-status"); err != nil {
			t.Fatalf("printSingleCommand root: %v", err)
		}

		out := buf.String()
		if !strings.Contains(out, "Global Flags:") {
			t.Errorf("root should list global flags\noutput: %q", out)
		}

		if !strings.Contains(out, "Subcommands:") {
			t.Errorf("root should list subcommands\noutput: %q", out)
		}
	})

	t.Run("found json", func(t *testing.T) {
		resetCmdtreeFlags()
		defer resetCmdtreeFlags()

		cmdtreeJSON = true

		var buf bytes.Buffer

		cmd := newBufCmd(&buf)

		if err := printSingleCommand(cmd, rootCmd, "version"); err != nil {
			t.Fatalf("printSingleCommand json: %v", err)
		}

		var detail CommandDetail
		if err := json.Unmarshal(buf.Bytes(), &detail); err != nil {
			t.Fatalf("not valid JSON: %v\noutput: %q", err, buf.String())
		}

		if detail.Name != "version" {
			t.Errorf("detail.Name = %q, want version", detail.Name)
		}
	})

	t.Run("missing", func(t *testing.T) {
		resetCmdtreeFlags()
		defer resetCmdtreeFlags()

		var buf bytes.Buffer

		cmd := newBufCmd(&buf)

		err := printSingleCommand(cmd, rootCmd, "does-not-exist")
		if err == nil {
			t.Fatal("expected error for missing command, got nil")
		}

		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("error %q does not mention 'not found'", err.Error())
		}
	})
}

func TestFindCommand(t *testing.T) {
	t.Run("found root", func(t *testing.T) {
		if got := findCommand(rootCmd, "claude-status"); got == nil {
			t.Error("findCommand failed to match root name")
		}
	})

	t.Run("found sub", func(t *testing.T) {
		got := findCommand(rootCmd, "version")
		if got == nil {
			t.Fatal("findCommand could not find version")
		}

		if got.Name() != "version" {
			t.Errorf("found %q, want version", got.Name())
		}
	})

	t.Run("nil", func(t *testing.T) {
		if got := findCommand(rootCmd, "nope-nope"); got != nil {
			t.Errorf("expected nil for missing command, got %v", got)
		}
	})
}

func TestPrintJSONTree(t *testing.T) {
	var buf bytes.Buffer

	cmd := newBufCmd(&buf)

	if err := printJSONTree(cmd, rootCmd); err != nil {
		t.Fatalf("printJSONTree: %v", err)
	}

	var detail CommandDetail
	if err := json.Unmarshal(buf.Bytes(), &detail); err != nil {
		t.Fatalf("not valid JSON: %v\noutput: %q", err, buf.String())
	}

	if detail.Name != "claude-status" {
		t.Errorf("detail.Name = %q, want claude-status", detail.Name)
	}

	if len(detail.GlobalFlags) == 0 {
		t.Error("detail.GlobalFlags empty")
	}
}

func TestBuildCommandDetail(t *testing.T) {
	detail := buildCommandDetail(rootCmd)

	if detail.Name != "claude-status" {
		t.Errorf("detail.Name = %q, want claude-status", detail.Name)
	}

	if detail.Use == "" {
		t.Error("detail.Use is empty")
	}

	if len(detail.Subcommands) == 0 {
		t.Error("detail.Subcommands empty")
	}
}
