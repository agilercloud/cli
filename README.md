# agiler

Command-line interface for [Agiler](https://agiler.io) — manage projects, files, backups, domains, and more from the terminal.

> **Status: pre-1.0, unstable.** This CLI is in active development. Expect rough edges, incomplete coverage of the API, and breaking changes between releases without deprecation windows. Pin a specific version if you need stability, and report issues on the [tracker](https://github.com/agilercloud/cli/issues). Not yet recommended for production automation.

## Install

**Homebrew (macOS/Linux):**

```sh
brew install agilercloud/tap/agiler
```

**Install script (macOS/Linux):**

```sh
curl -fsSL https://raw.githubusercontent.com/agilercloud/cli/main/install.sh | sh
```

Installs the latest release to `~/.local/bin/agiler`. Override with `AGILER_VERSION=v0.1.2` or `AGILER_INSTALL_DIR=/usr/local/bin` (prefix the env vars before the pipeline, e.g. `curl ... | AGILER_VERSION=v0.1.2 sh`). Script verifies the SHA-256 checksum before installing.

**From source:**

```sh
go install github.com/agilercloud/cli/cmd/agiler@latest
```

**Binary downloads:** see [Releases](https://github.com/agilercloud/cli/releases).

## Upgrading

```sh
agiler upgrade              # install the latest release
agiler upgrade --check      # report current vs. latest, no changes
agiler upgrade --version v0.1.2   # install a specific tag (supports downgrade)
agiler upgrade --force      # override refusals (dev build, non-canonical path, same version)
```

`agiler upgrade` only self-updates installs it can safely manage (the `install.sh` / manual-download path). For installs from Homebrew or `go install`, it prints the correct native upgrade command instead. The command hits the GitHub Releases API directly and verifies the SHA-256 of the downloaded archive before replacing the binary. Set `GITHUB_TOKEN` if you hit the unauthenticated rate limit (60 requests/hour per IP).

## Quickstart

```sh
agiler login                                  # interactive, masked input
# or, for CI:
printf '%s' "$AGILER_API_KEY" | agiler login

agiler config set project-id <project-uuid>
agiler status
agiler projects list
agiler logs
```

Generate an API key in the [Agiler dashboard](https://agiler.io) under *Settings → API Keys*.

`agiler login` is the safe way to store an API key: on a TTY it reads with echo disabled so the key never lands in shell history; on a pipe it reads one line from stdin. If a script needs to bypass the login flow entirely, `agiler config set api-key -` also reads the value from stdin.

## Configuration

The CLI resolves its config from the first file found:

1. `--config <path>` flag
2. `./agiler.toml`
3. `~/.config/agiler/config.toml` (or `$AGILER_CONFIG_DIR/config.toml`)
4. `/etc/agiler/config.toml`

Config format:

```toml
api_key      = "ak_..."
api_base     = "https://api.agiler.io"   # optional; defaults to production
workspace_id = "..."                     # optional; default workspace
project_id   = "..."                     # optional; default project for project-scoped commands
```

Environment variables `AGILER_API_KEY`, `AGILER_API_BASE`, `AGILER_WORKSPACE_ID`, and `AGILER_PROJECT_ID` override config values. Command-line flags `--api-key`, `--api-base`, `--workspace`, and `--project`/`-p` override both.

Project-scoped commands (`logs`, `sql`, `files`, `backups`, `variables`, `domains`, `rules`, `usage`) target the project resolved from `--project`, `AGILER_PROJECT_ID`, or `project_id` in config. Setting it once in config means you don't repeat the project ID on every command.

## Commands

**Project operations** (target the configured project):

```
agiler logs                Tail or search project logs
agiler sql                 Execute SQL and inspect prior runs
agiler files               Browse and transfer project files
agiler backups             List, create, restore, download backups; manage policy
agiler variables           Manage environment variables
agiler domains             Manage custom domains
agiler rules               Manage project rules (templates at 'rules templates options')
agiler usage               Project resource usage
```

**Resources:**

```
agiler projects            Manage projects (list, get, create, update, delete)
agiler workspaces          Manage workspaces (list, get, create, members)
```

**Reference:**

```
agiler regions             List available regions
agiler runtimes            List available runtimes
```

**Account:**

```
agiler login               Store an API key (masked input on a TTY, stdin on a pipe)
agiler whoami              Show the authenticated user and effective scopes
agiler billing             View billing state, transactions, statements
agiler notifications       List and dismiss notifications
agiler config              Manage CLI configuration
```

**Maintenance:**

```
agiler status              Check API status
agiler upgrade             Upgrade this CLI to the latest release
agiler version             Print CLI version
```

Run `agiler <command> --help` for details on any subcommand.

## Shell completion

Generate a completion script for your shell:

```sh
# bash (Linux):
agiler completion bash | sudo tee /etc/bash_completion.d/agiler

# bash (macOS, with Homebrew bash-completion):
agiler completion bash > "$(brew --prefix)/etc/bash_completion.d/agiler"

# zsh:
agiler completion zsh > "${fpath[1]}/_agiler"

# fish:
agiler completion fish > ~/.config/fish/completions/agiler.fish

# powershell:
agiler completion powershell | Out-String | Invoke-Expression
```

After installing the script, project, workspace, region, and runtime IDs auto-complete from the live API. Completion is also wired for `--project`, `--workspace`, `--region`, and `--runtime` flag values.

## License

[MIT](LICENSE)
