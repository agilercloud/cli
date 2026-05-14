// Package app wires concrete dependencies into an App value that the CLI
// command tree reads from. Tests construct an App with fakes and invoke
// Run directly; main constructs one via Wire.
package app

import (
	"io"

	"github.com/agilercloud/cli/internal/api"
	"github.com/agilercloud/cli/internal/clock"
	"github.com/agilercloud/cli/internal/config"
	"github.com/agilercloud/cli/internal/fsx"
	"github.com/agilercloud/cli/internal/output"
)

// App is the dependency bundle passed to every command constructor.
//
// API is assigned by PersistentPreRunE — commands that run before config
// loads (version, help, upgrade, config subcommands) see a nil API.
// Output is assigned from the --format / --quiet flag values in PersistentPreRunE.
type App struct {
	Version      string
	API          *api.Client
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

	// OutputFormat is the raw --format flag value (parsed in initOutput);
	// OutputQuiet is the raw --quiet flag value.
	OutputFormat string
	OutputQuiet  bool

	In  io.Reader
	Out io.Writer
	Err io.Writer
}
