# Flux + Momentum = ❤️ 
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE) ![Go](https://img.shields.io/badge/Go-00ADD8?style=flat&logo=go&logoColor=white)

> [!WARNING]
> This tool is experimental and not ready for production use. 

The perfect companion to Flux. Because once the board starts moving, it shouldn’t stop.

## Features

> [!NOTE]
> Currently only Claude Code is supported. Future releases will add support for other agents such as Codex.

### Headless Mode
- **Smart task selection** - Automatically picks the newest unblocked todo task
- **Flexible filtering** - Filter by `--project`, `--epic`, or `--task`

### Workflow Operations
- **Batch status transitions** - Start, complete, or reset multiple tasks at once
- **Dependency awareness** - Blocked tasks are visually distinguished

### Flux Integration
- Full REST client for Projects, Epics, and Tasks
- Real-time sync via Server-Sent Events (SSE)

## Install

### Homebrew (macOS & Linux)

Requires [Homebrew](https://brew.sh) to be installed.

```bash
brew tap sirsjg/momentum
brew install momentum
```