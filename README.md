# Momentum + Flux + Claude Code = ❤️ 

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE) [![GitHub release](https://img.shields.io/github/v/release/sirsjg/momentum)](https://github.com/sirsjg/momentum/releases) ![Go](https://img.shields.io/badge/Go-00ADD8?style=flat&logo=go&logoColor=white) ![macOS](https://img.shields.io/badge/macOS-000000?style=flat&logo=apple&logoColor=white) ![Linux](https://img.shields.io/badge/Linux-FCC624?style=flat&logo=linux&logoColor=black)

> [!WARNING]
> This tool is experimental and not ready for production use. 

The perfect companion to Flux. Because once the board starts moving, it shouldn’t stop.

## Install

### Homebrew (macOS & Linux)

Requires [Homebrew](https://brew.sh) to be installed.

```bash
brew tap sirsjg/momentum
brew install momentum
```

## Features

> [!NOTE]
> Currently only Claude Code is supported. Future releases will add support for other agents such as Codex.

### Agent Orchestration
- **Automatic task execution** - Watches for tasks and spawns Claude Code agents automatically
- **Async & sync modes** - Run multiple agents in parallel or sequentially (`--execution-mode`)
- **Graceful cancellation** - Stop agents cleanly with SIGINT handling

### Terminal UI
- **Multi-panel dashboard** - Monitor multiple running agents simultaneously
- **Live output panel** - Stream parsed agent output with follow/scroll controls
- **Keyboard navigation** - Move between panels, stop/close agents, toggle modes
- **Auto-update notifications** - Get notified when new versions are available

### Flux Integration
- **Smart task selection** - Automatically picks unblocked todo tasks from auto-enabled epics
- **Flexible filtering** - Filter by `--project`, `--epic`, or `--task`
- **Real-time sync** - Server-Sent Events (SSE) for instant task updates
- **Workflow automation** - Automatic status transitions (todo → in_progress → done)

## Usage

### Basic Usage

```bash
# Watch all projects for tasks
momentum

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
```

### Keyboard Controls

| Key | Action |
|-----|--------|
| `Tab` / `Shift+Tab` | Cycle focus between agent panels |
| `j` / `k` / `↓` / `↑` | Move selection |
| `pgup` / `pgdn` | Scroll output |
| `f` | Jump back to follow mode |
| `m` | Toggle execution mode (async/sync) |
| `s` | Stop the focused agent |
| `x` / `c` | Close the focused panel |
| `q` / `Ctrl+C` | Quit |
