# Momentum + Flux + Claude Code = ❤️ 

> [!IMPORTANT]
> Momentum was a fun and rewarding project to build, but it is no longer actively supported. It has been superseded by [Heretic](https://github.com/sirsjg/heretic), a significantly more powerful successor. New users are encouraged to use Heretic instead.

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE) [![GitHub release](https://img.shields.io/github/v/release/sirsjg/momentum)](https://github.com/sirsjg/momentum/releases) ![Go](https://img.shields.io/badge/Go-00ADD8?style=flat&logo=go&logoColor=white) ![macOS](https://img.shields.io/badge/macOS-000000?style=flat&logo=apple&logoColor=white) ![Linux](https://img.shields.io/badge/Linux-FCC624?style=flat&logo=linux&logoColor=black)

> [!WARNING]
> This tool is experimental and not ready for production use. 

The perfect companion to Flux. Because once the board starts moving, it shouldn’t stop.

## Prerequisites

Before installing Momentum, ensure you have:

- **[Claude Code](https://docs.anthropic.com/en/docs/claude-code)** - Anthropic's CLI for Claude
- **[Flux](https://github.com/sirsjg/flux)** - REST API server running and accessible (default: `http://localhost:3000`)

Momentum connects to Flux's REST API and SSE endpoint, not its MCP HTTP
endpoint. The current Flux REST contract is supported, including private
projects, project-scoped API keys, and authenticated live-update events.

## Install

### Installer (Linux & macOS)

Install the latest release to `~/.local/bin`:

```bash
curl -fsSL https://raw.githubusercontent.com/sirsjg/momentum/main/install.sh | sh
```

The installer detects your operating system and architecture, then verifies the
download against the release checksum before installing it. If `~/.local/bin`
is not already on your `PATH`, follow the command it prints after installation.

Set `MOMENTUM_INSTALL_DIR` to use another destination or `MOMENTUM_VERSION` to
install a specific release:

```bash
curl -fsSL https://raw.githubusercontent.com/sirsjg/momentum/main/install.sh | \
  MOMENTUM_INSTALL_DIR="$HOME/bin" MOMENTUM_VERSION=1.4.0 sh
```

### Homebrew (optional; macOS & Linux)

If you already use [Homebrew](https://brew.sh):

```bash
brew tap sirsjg/momentum
brew install --cask momentum
```

Pre-built archives and checksums are also available on the
[releases page](https://github.com/sirsjg/momentum/releases).

## Features

> [!NOTE]
> Currently only Claude Code is supported. Future releases will add support for other agents such as Codex.

### Agent Orchestration
- **Automatic task execution** - Watches for tasks and spawns Claude Code agents automatically
- **Async & sync modes** - Run multiple agents in parallel or sequentially (`--execution-mode`)
- **Graceful cancellation** - Stop agents cleanly with SIGINT handling

### Terminal UI
- **Multi-panel dashboard** - Monitor multiple running agents simultaneously
- **Real-time output streaming** - Watch agent progress with parsed JSON output
- **Keyboard navigation** - Tab between panels, scroll with j/k, stop/close agents
- **Auto-update notifications** - Get notified when new versions are available

### Flux Integration
- **Smart task selection** - Automatically picks unblocked todo tasks from auto-enabled epics
- **Flexible filtering** - Filter by `--project`, `--epic`, or `--task`
- **Real-time sync** - Server-Sent Events (SSE) for instant task updates
- **Workflow automation** - Automatic status transitions (todo → in_progress → done)

## Usage

### Basic Usage

```bash
# Watch all projects for tasks on a Flux server running in open mode
momentum

# Current Flux projects are private by default; authenticated servers require
# a server key or a project-scoped key with write access
momentum --api-key flx_your_key

# Watch a specific project
momentum --project myproject

# Watch a specific epic
momentum --epic epic-456

# Work on a specific task
momentum --task task-789
```

### Execution Modes

```bash
# Run agents in parallel (default)
momentum --project myproject --execution-mode async

# Run agents sequentially (one at a time)
momentum --project myproject --execution-mode sync
```

You can also toggle between modes at runtime by pressing `m` in the TUI.

### Custom Flux Server

```bash
# Connect to a different Flux server
momentum --base-url http://flux.example.com:3000 --project myproject

# The full Flux API server is locked by default. Pass the value configured in
# FLUX_API_KEY, or a stored Flux API key with access to the target project.
momentum --base-url http://flux.example.com:3000 --api-key flx_your_key --project myproject

# The lightweight `flux serve` command defaults to port 3589. It does not
# expose SSE, so Momentum automatically uses REST polling instead.
momentum --base-url http://localhost:3589 --poll-interval 10s

# Adjust the REST fallback interval used while waiting for tasks
momentum --poll-interval 10s
```

Momentum sends `--api-key` as a Bearer token for REST requests and as Flux's
documented `token` query parameter for the SSE event stream.

Flux's `FLUX_MCP_PASSWORD` and OAuth flow secure the MCP HTTP endpoint only;
they do not authenticate Momentum to the REST API. For keyless local use, the
full Flux server must have no configured keys and be explicitly started with
`FLUX_ALLOW_ANONYMOUS=1`.

### Keyboard Controls

| Key | Action |
|-----|--------|
| `Tab` | Cycle focus between agent panels |
| `Shift+Tab` | Cycle focus backwards |
| `j` / `↓` | Scroll down in focused panel |
| `k` / `↑` | Scroll up in focused panel |
| `m` | Toggle execution mode (async/sync) |
| `s` / `Esc` | Stop the focused agent |
| `x` / `c` | Close a finished panel |
| `q` / `Ctrl+C` | Quit |
