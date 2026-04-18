package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/agilercloud/cli/internal/app"
	"github.com/agilercloud/cli/internal/clock"
	"github.com/agilercloud/cli/internal/fsx"
	"github.com/agilercloud/cli/internal/output"
)

// newTestApp returns an App with in-memory buffers. Callers can inspect
// Out/Err after Run to verify behavior.
func newTestApp(t *testing.T) (*app.App, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	out := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	a := &app.App{
		Version: "v0.0.0-test",
		In:      strings.NewReader(""),
		Out:     out,
		Err:     errBuf,
		Output:  output.New(output.ModeText, out, errBuf),
		FS:      fsx.NewMemFS(),
		Clock:   clock.Real{},
	}
	return a, out, errBuf
}

// TestHelpCommands verifies --help on every top-level command succeeds
// and emits nonempty output. Catches missing registrations and typos.
func TestHelpCommands(t *testing.T) {
	cases := []string{
		"--help",
		"projects --help",
		"projects list --help",
		"projects get --help",
		"projects create --help",
		"projects update --help",
		"projects delete --help",
		"projects variables --help",
		"projects domains --help",
		"projects rules --help",
		"projects files --help",
		"projects backups --help",
		"projects sql --help",
		"projects usage --help",
		"projects logs --help",
		"projects logs tail --help",
		"projects logs search --help",
		"regions --help",
		"regions list --help",
		"regions get --help",
		"runtimes --help",
		"runtimes list --help",
		"runtimes get --help",
		"rules --help",
		"rules options --help",
		"status --help",
		"version --help",
		"upgrade --help",
		"config --help",
		"config get --help",
		"config set --help",
		"config path --help",
	}
	for _, tc := range cases {
		t.Run(tc, func(t *testing.T) {
			a, out, _ := newTestApp(t)
			code := Run(a, context.Background(), strings.Fields(tc))
			if code != 0 {
				t.Errorf("exit %d for %q", code, tc)
			}
			if out.Len() == 0 {
				t.Errorf("empty help output for %q", tc)
			}
		})
	}
}

// TestVersionCommand verifies the version command prints a.Version.
func TestVersionCommand(t *testing.T) {
	a, out, _ := newTestApp(t)
	code := Run(a, context.Background(), []string{"version"})
	if code != 0 {
		t.Fatalf("version exited %d", code)
	}
	if !strings.Contains(out.String(), "v0.0.0-test") {
		t.Errorf("version output missing tag: %q", out.String())
	}
}

// TestUnknownCommand verifies unknown commands exit nonzero.
func TestUnknownCommand(t *testing.T) {
	a, _, _ := newTestApp(t)
	code := Run(a, context.Background(), []string{"not-a-real-command"})
	if code == 0 {
		t.Error("expected nonzero exit for unknown command")
	}
}
