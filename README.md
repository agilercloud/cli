# agiler

Command-line interface for [Agiler](https://agiler.io) — manage projects, files, backups, domains, and more from the terminal.

## Install

**Homebrew (macOS/Linux):**

```sh
brew install agilercloud/tap/agiler
```

**From source:**

```sh
go install github.com/agilercloud/cli/cmd/agiler@latest
```

**Binary downloads:** see [Releases](https://github.com/agilercloud/cli/releases).

## Quickstart

```sh
agiler config set api-key ak_your_key_here
agiler status
agiler projects list
```

Generate an API key in the [Agiler dashboard](https://agiler.io) under *Settings → API Keys*.

## Configuration

The CLI resolves its config from the first file found:

1. `--config <path>` flag
2. `./agiler.toml`
3. `~/.config/agiler/config.toml` (or `$AGILER_CONFIG_DIR/config.toml`)
4. `/etc/agiler/config.toml`

Config format:

```toml
api_key  = "ak_..."
api_base = "https://api.agiler.io"   # optional; defaults to production
```

Environment variables `AGILER_API_KEY` and `AGILER_API_BASE` override config values. Command-line flags `--api-key` and `--api-base` override both.

## Commands

```
agiler status              Check API status
agiler config              Manage CLI configuration
agiler projects            Manage projects (list, get, create, update, delete)
agiler projects variables  Manage environment variables
agiler projects domains    Manage custom domains
agiler projects files      Browse and transfer project files
agiler projects backups    List, create, restore, download backups
agiler projects sql        Run SQL queries against a project database
agiler projects rules      Manage project rules
agiler projects logs       Tail project logs
agiler projects usage      Project resource usage
agiler runtimes            List available runtimes
agiler regions             List available regions
agiler rules               Rule templates
```

Run `agiler <command> --help` for details on any subcommand.

## License

[MIT](LICENSE)
