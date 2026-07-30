package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/agilercloud/cli/internal/app"
	"github.com/spf13/cobra"
)

// errAborted is returned by confirmDestructive when the user declines the
// prompt. It bubbles up through cobra so the process exits non-zero.
var errAborted = errors.New("aborted")

// isInteractiveFn is a test seam so unit tests can simulate TTY input from
// an in-memory reader.
var isInteractiveFn = isInteractive

// confirmDestructive prompts the user on a.Err and reads a single line from
// a.In. Only "y" or "yes" (case-insensitive) accepts; anything else, including
// EOF or a non-interactive input, declines. Callers pass --yes to bypass.
func confirmDestructive(a *app.App, prompt string) error {
	if !isInteractiveFn(a.In) {
		return fmt.Errorf("refusing to proceed without confirmation on a non-interactive input; pass --yes to skip the prompt")
	}

	if _, err := fmt.Fprint(a.Err, prompt); err != nil {
		return fmt.Errorf("write confirmation: %w", err)
	}

	reader := bufio.NewReader(a.In)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("read confirmation: %w", err)
	}

	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return nil
	default:
		return errAborted
	}
}

// confirmOrSkip runs the prompt unless --yes was set on cmd. It's the
// one-liner most call sites want.
func confirmOrSkip(a *app.App, cmd *cobra.Command, prompt string) error {
	if yes, _ := cmd.Flags().GetBool("yes"); yes {
		return nil
	}
	return confirmDestructive(a, prompt)
}

// addYesFlag registers the standard --yes/-y bypass on cmd.
func addYesFlag(cmd *cobra.Command) {
	cmd.Flags().BoolP("yes", "y", false, "Skip the interactive confirmation prompt")
}

// isInteractive reports whether r is connected to a terminal. Readers that
// aren't *os.File (e.g. test buffers, pipes) are treated as non-interactive.
func isInteractive(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
