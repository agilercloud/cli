package app

import (
	"os"

	"github.com/agilercloud/cli/internal/api"
	"github.com/agilercloud/cli/internal/clock"
	"github.com/agilercloud/cli/internal/fsx"
	"github.com/agilercloud/cli/internal/output"
)

// Wire constructs a baseline App with real I/O streams and the given
// version. Flags, config, and the API client are populated by the root
// command's PersistentPreRunE after flag parsing.
func Wire(version string) (*App, error) {
	return &App{
		Version: version,
		In:      os.Stdin,
		Out:     os.Stdout,
		Err:     os.Stderr,
		Output:  output.New(output.ModeText, os.Stdout, os.Stderr),
		FS:      fsx.OSFS{},
		Clock:   clock.Real{},
	}, nil
}

// Ensure *api.Client satisfies the APIClient interface.
var _ APIClient = (*api.Client)(nil)
