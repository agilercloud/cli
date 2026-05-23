package cli

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func withInteractiveStub(t *testing.T, interactive bool) {
	t.Helper()
	prev := isInteractiveFn
	isInteractiveFn = func(io.Reader) bool { return interactive }
	t.Cleanup(func() { isInteractiveFn = prev })
}

func TestConfirmDestructiveYes(t *testing.T) {
	withInteractiveStub(t, true)
	a, _, errBuf := newTestApp(t)
	a.In = strings.NewReader("y\n")

	if err := confirmDestructive(a, "Delete thing? (y/N) "); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !strings.Contains(errBuf.String(), "Delete thing?") {
		t.Errorf("prompt not written to stderr: %q", errBuf.String())
	}
}

func TestConfirmDestructiveYesLong(t *testing.T) {
	withInteractiveStub(t, true)
	a, _, _ := newTestApp(t)
	a.In = strings.NewReader("YES\n")

	if err := confirmDestructive(a, "prompt "); err != nil {
		t.Fatalf("expected nil error for 'YES', got %v", err)
	}
}

func TestConfirmDestructiveDeclines(t *testing.T) {
	withInteractiveStub(t, true)
	a, _, _ := newTestApp(t)
	a.In = strings.NewReader("n\n")

	err := confirmDestructive(a, "prompt ")
	if !errors.Is(err, errAborted) {
		t.Fatalf("expected errAborted, got %v", err)
	}
}

func TestConfirmDestructiveEOFDeclines(t *testing.T) {
	withInteractiveStub(t, true)
	a, _, _ := newTestApp(t)
	a.In = strings.NewReader("")

	err := confirmDestructive(a, "prompt ")
	if !errors.Is(err, errAborted) {
		t.Fatalf("expected errAborted on EOF, got %v", err)
	}
}

func TestConfirmDestructiveNonInteractiveErrors(t *testing.T) {
	withInteractiveStub(t, false)
	a, _, _ := newTestApp(t)
	a.In = strings.NewReader("y\n")

	err := confirmDestructive(a, "prompt ")
	if err == nil {
		t.Fatal("expected non-interactive error, got nil")
	}
	if !strings.Contains(err.Error(), "--yes") {
		t.Errorf("error should mention --yes, got %q", err.Error())
	}
}

func TestConfirmOrSkipBypassesWithYesFlag(t *testing.T) {
	withInteractiveStub(t, false)
	a, _, errBuf := newTestApp(t)
	a.In = strings.NewReader("")

	cmd := &cobra.Command{Use: "x"}
	addYesFlag(cmd)
	if err := cmd.ParseFlags([]string{"--yes"}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	if err := confirmOrSkip(a, cmd, "prompt "); err != nil {
		t.Fatalf("expected --yes to bypass prompt, got %v", err)
	}
	if errBuf.Len() != 0 {
		t.Errorf("--yes should suppress prompt output, got %q", errBuf.String())
	}
}

func TestConfirmOrSkipPromptsWithoutYesFlag(t *testing.T) {
	withInteractiveStub(t, true)
	a, _, _ := newTestApp(t)
	a.In = strings.NewReader("y\n")

	cmd := &cobra.Command{Use: "x"}
	addYesFlag(cmd)
	if err := cmd.ParseFlags([]string{}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	if err := confirmOrSkip(a, cmd, "prompt "); err != nil {
		t.Fatalf("expected confirm to succeed, got %v", err)
	}
}
