# Momentum CLI Plan (for coding agent)

## Goals
- Build a cross-platform CLI named `momentum` for Flux.
- Provide two modes:
  - Interactive terminal UI to browse projects, select one, view epics + todo tasks, choose tasks, then "start working".
  - Headless mode that starts on any new todo task, or a specific project/epic/task if provided.
- React to Flux SSE `data-changed` events and refresh data from REST.
- Flags-only configuration (no config files, no auth).

## Non-goals (for MVP)
- Authentication, user accounts, or token storage.
- Full Flux feature parity; focus on projects/epics/tasks + status transitions.
- Persistent local state beyond runtime (optional in later phases).

## Assumptions / Open Questions
- Flux exposes SSE at `GET /api/events` with `data-changed` events.
- Flux REST endpoints exist to list projects, epics, tasks, and update task status.
- "Start working" means setting a task status to `in_progress` (confirm exact endpoint).
- "Todo" status is the string `todo`.

## Language Choice
- Go (cross-platform binaries, good SSE/HTTP support, CLI ergonomics).

## CLI Surface (MVP)
```
momentum interactive
momentum headless --project-id <id> [--epic-id <id> | --task-id <id>]
momentum headless [--project-id <id>]
momentum --help
```

### Global flags
- `--base-url` (default `http://localhost:3000`)
- `--poll-interval` (fallback if SSE fails; e.g. 10s)
- `--log-level` (info, warn, debug)

### Interactive flags
- `--project-id` (skip project selection)

### Headless flags
- `--project-id`
- `--epic-id`
- `--task-id`
- `--start-latest` (if multiple todo tasks, choose most recently created)

## Architecture
### Modules
- `cmd/momentum`: Cobra or `flag`-based command wiring.
- `internal/fluxclient`: HTTP client for REST + SSE subscription.
- `internal/model`: Project/Epic/Task structs.
- `internal/tui`: Interactive terminal UI (project picker, task list).
- `internal/runner`: "Start working" executor for selected tasks.

### SSE Strategy
- Connect to `GET /api/events` using `net/http`.
- Parse SSE lines; on `event: data-changed` trigger refresh.
- Keep a reconnect loop with exponential backoff.
- If SSE fails repeatedly, fall back to polling using `--poll-interval`.

### REST Strategy
- `GET /api/projects`
- `GET /api/projects/:projectId/epics`
- `GET /api/projects/:projectId/tasks`
- `PATCH /api/tasks/:taskId` to set `status: in_progress` (confirm).

### Data Flow
- Interactive: load projects -> choose -> load epics + tasks (todo only) -> allow selection -> mark selected tasks as `in_progress`.
- Headless:
  - If `--task-id`: start that task.
  - If `--epic-id`: start first todo task in epic.
  - Else if `--project-id`: start first todo task in project.
  - Else: wait for SSE `data-changed`, then pick newest todo task across all projects.

## TUI Behavior (Interactive)
- Layout: left pane projects, right pane epics/tasks filtered by `todo`.
- Toggle selection of tasks; "Start working" triggers update for selected tasks.
- Keyboard: arrows/jk, space to toggle, enter to confirm, q to quit.
- Use `pterm` for cross-platform UI rendering.

## Error Handling
- Graceful SSE reconnect with jittered backoff.
- REST errors surfaced in UI/status bar and logged.
- Headless mode logs errors and continues waiting for events.

## Testing
- Unit tests for SSE parser and REST client (mock server).
- Table tests for task selection logic in headless mode.

## Phases
1. MVP headless mode (REST + SSE, no TUI).
2. Interactive TUI with project/epic/task selection.
3. Nice-to-haves: filters, search, task details, retry UX.

## Flux Task Plan Integration
- After this plan is accepted, create tasks in Flux project "Momentum" (ID: `5wkj2r9`).
