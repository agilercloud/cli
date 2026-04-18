// Package app wires concrete dependencies into an App value that the CLI
// command tree reads from. Tests construct an App with fakes and invoke
// Run directly; main constructs one via Wire.
package app

import (
	"context"
	"io"
	"net/http"

	"github.com/agilercloud/cli/internal/clock"
	"github.com/agilercloud/cli/internal/config"
	"github.com/agilercloud/cli/internal/fsx"
	"github.com/agilercloud/cli/internal/output"
)

// APIClient is the subset of *api.Client that CLI commands call.
// Defined at the consumer side so tests can supply fakes without importing
// the concrete type.
type APIClient interface {
	Do(ctx context.Context, method, path string, body io.Reader) (*http.Response, error)
	DoRaw(ctx context.Context, method, path, contentType string, headers map[string]string, body io.Reader) (*http.Response, error)
	DoJSON(ctx context.Context, method, path string, body io.Reader, dest any) error
	DoJSONIdempotent(ctx context.Context, method, path string, body io.Reader, dest any) error
}

// App is the dependency bundle passed to every command constructor.
//
// API is assigned by PersistentPreRunE — commands that run before config
// loads (version, help, upgrade, config subcommands) see a nil API.
// Output is assigned from the --json / --quiet flag values in PersistentPreRunE.
type App struct {
	Version      string
	API          APIClient
	Config       *config.Config
	ConfigLoader config.Loader
	Output       *output.Writer
	FS           fsx.FS
	Clock        clock.Clock

	// FlagConfig is the value of the persistent --config flag, used to
	// resolve config file paths.
	FlagConfig string
	// FlagAPIKey and FlagAPIBase let the --api-key / --api-base flags
	// override config file values at api-client construction time.
	FlagAPIKey  string
	FlagAPIBase string

	// OutputJSON / OutputQuiet are the raw --json / --quiet flag values.
	OutputJSON  bool
	OutputQuiet bool

	In  io.Reader
	Out io.Writer
	Err io.Writer
}
