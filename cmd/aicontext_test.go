package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// newBufCmd returns a fresh cobra command whose output is wired to buf.
func newBufCmd(buf *bytes.Buffer) *cobra.Command {
	c := &cobra.Command{Use: "test"}
	c.SetOut(buf)
	c.SetErr(buf)

	return c
}

func resetAIContextFlags() {
	aicontextJSON = false
	aicontextCompact = false
}

func TestRunAIContextMarkdown(t *testing.T) {
	resetAIContextFlags()
	defer resetAIContextFlags()

	var buf bytes.Buffer

	cmd := newBufCmd(&buf)

	if err := runAIContext(cmd, nil); err != nil {
		t.Fatalf("runAIContext (markdown): %v", err)
	}

	out := buf.String()

	for _, want := range []string{
		"# claude-status - AI Context",
		"## Overview",
		"## Commands",
		"## Command Categories",
		"## Project Structure",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("markdown output missing %q", want)
		}
	}

	// rootCmd has version/cmdtree/aicontext subcommands populated by init().
	for _, sub := range []string{"version", "cmdtree", "aicontext"} {
		if !strings.Contains(out, sub) {
			t.Errorf("markdown output missing subcommand %q", sub)
		}
	}
}

func TestRunAIContextCompact(t *testing.T) {
	resetAIContextFlags()
	defer resetAIContextFlags()

	aicontextCompact = true

	var buf bytes.Buffer

	cmd := newBufCmd(&buf)

	if err := runAIContext(cmd, nil); err != nil {
		t.Fatalf("runAIContext (compact): %v", err)
	}

	out := buf.String()

	if !strings.Contains(out, "# claude-status - claude-status is a CLI application") {
		t.Errorf("compact output missing header\noutput: %q", out)
	}

	if !strings.Contains(out, "`claude-status version`") {
		t.Errorf("compact output missing version command line\noutput: %q", out)
	}
}

func TestRunAIContextJSON(t *testing.T) {
	resetAIContextFlags()
	defer resetAIContextFlags()

	aicontextJSON = true

	var buf bytes.Buffer

	cmd := newBufCmd(&buf)

	if err := runAIContext(cmd, nil); err != nil {
		t.Fatalf("runAIContext (json): %v", err)
	}

	var doc aiContextDoc
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %q", err, buf.String())
	}

	if doc.Tool != "claude-status" {
		t.Errorf("doc.Tool = %q, want claude-status", doc.Tool)
	}

	if len(doc.Commands) == 0 {
		t.Error("doc.Commands is empty")
	}

	// Ensure global flags captured (interval/url are persistent on rootCmd).
	if len(doc.GlobalFlags) == 0 {
		t.Error("doc.GlobalFlags is empty")
	}
}

func TestAIBuildCommandInfo(t *testing.T) {
	info := aiBuildCommandInfo(cmdtreeCmd)

	if info.Name != "cmdtree" {
		t.Errorf("info.Name = %q, want cmdtree", info.Name)
	}

	if info.Usage == "" {
		t.Error("info.Usage is empty")
	}

	if len(info.Flags) == 0 {
		t.Error("cmdtree should have local flags")
	}
}

func TestAIWriteCommandMarkdown(t *testing.T) {
	var b strings.Builder

	aiWriteCommandMarkdown(&b, rootCmd.Commands(), "")

	out := b.String()
	if !strings.Contains(out, "### version") {
		t.Errorf("markdown writer missing version heading\noutput: %q", out)
	}

	if !strings.Contains(out, "Usage:") {
		t.Errorf("markdown writer missing usage line")
	}
}

func TestAIWriteCompactCommand(t *testing.T) {
	var b strings.Builder

	aiWriteCompactCommand(&b, versionCmd, "")

	out := b.String()
	if !strings.Contains(out, "`claude-status version`") {
		t.Errorf("compact writer missing command line\noutput: %q", out)
	}
}
