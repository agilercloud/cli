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
	"golang.org/x/term"
)

// readPasswordFn is a test seam so unit tests can simulate masked TTY input.
var readPasswordFn = func(fd int) ([]byte, error) { return term.ReadPassword(fd) }

func newLoginCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Store an API key in the CLI config",
		Long: "Prompts for an API key and writes it to the CLI config file. " +
			"On a TTY, the prompt reads with echo disabled so the key never reaches the shell history or scrollback. " +
			"On a non-TTY stdin (pipes, CI), reads one line from stdin instead, so " +
			"`printf '%s' \"$AGILER_API_KEY\" | agiler login` works in scripts.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			key, err := readAPIKey(a)
			if err != nil {
				return err
			}
			key = strings.TrimSpace(key)
			if key == "" {
				return errors.New("API key is empty")
			}
			if err := loader(a).Set("api-key", key); err != nil {
				return fmt.Errorf("save api-key: %w", err)
			}
			_, _ = fmt.Fprintln(a.Err, "Logged in.")
			return nil
		},
	}
}

func readAPIKey(a *app.App) (string, error) {
	if isInteractiveFn(a.In) {
		f, ok := a.In.(*os.File)
		if !ok {
			return "", errors.New("interactive input is not a file")
		}
		_, _ = fmt.Fprint(a.Err, "API key: ")
		raw, err := readPasswordFn(int(f.Fd()))
		_, _ = fmt.Fprintln(a.Err)
		if err != nil {
			return "", fmt.Errorf("read api key: %w", err)
		}
		return string(raw), nil
	}

	scanner := bufio.NewScanner(a.In)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
			return "", fmt.Errorf("read api key: %w", err)
		}
		return "", nil
	}
	return scanner.Text(), nil
}
