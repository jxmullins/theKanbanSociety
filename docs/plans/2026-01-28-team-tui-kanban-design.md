# Team TUI with Kanban Board - Design Document

## Overview

Add an interactive TUI to the `team` CLI using the Charm stack (Bubble Tea, Lip Gloss, Huh?, Bubbles). The TUI displays a Kanban board for tracking AI team tasks, with support for user-assigned tasks that bring the human into the collaboration loop.

## Layout

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  THE COUNCIL OF LEGENDS - Team Mode                        Phase: Execution │
├─────────────────────────────────────────────────────────────────────────────┤
│ PM: Claude (Architect)                                          ⏵ Running   │
│ Task: Build a REST API for user management                                  │
│ Mode: divide_conquer │ Progress: ████████░░ 3/5 steps                       │
├───────────────┬───────────────┬───────────────┬───────────────┬─────────────┤
│ Backlog (2)   │ In Progress(2)│ Review (0)    │ Done (1)      │ Active      │
├───────────────┼───────────────┼───────────────┼───────────────┤ Panel       │
│ Cards...      │ Cards...      │ Cards...      │ Cards...      │ (streaming) │
├─────────────────────────────────────────────────────────────────────────────┤
│ [←→] Navigate  [Enter] Details  [Space] Pause  [Tab] Focus panel  [?] Help  │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Regions

| Region | Purpose |
|--------|---------|
| PM Header | Shows PM identity, task summary, work mode, phase, overall progress |
| Kanban Board (~70%) | Four columns with task cards |
| Active Panel (~30%) | Full streaming output from selected/active AI |
| Help Bar | Context-sensitive keyboard shortcuts |

## Kanban Columns

1. **Backlog** - Tasks identified but not started
2. **In Progress** - Currently being worked on (shows which AI)
3. **Review** - Completed, awaiting PM review
4. **Done** - Approved and complete

## Card Model

```go
type KanbanCard struct {
    ID          string
    Title       string       // Short description (fits in card)
    Description string       // Full details (shown in popup)
    AssignedTo  string       // AI ID or "user"
    Column      Column       // Backlog, InProgress, Review, Done
    IsBlocking  bool         // Only relevant for user tasks
    StreamBuf   string       // Current streaming content (truncated in card)
    FullHistory string       // Complete AI output (shown in popup)
    Progress    float64      // 0.0-1.0 for progress bar
    DependsOn   []string     // Task IDs that must complete first
}
```

### Card Visual States

| State | Appearance |
|-------|------------|
| Pending (Backlog) | Muted border, shows assignee |
| Active (In Progress) | AI's color border, streaming text, progress bar |
| Blocked | Red border, shows "Blocked by: Step X" |
| Waiting for User | Amber border, pulsing, "Waiting on you" |
| In Review | Purple border, "Awaiting PM review" |
| Done | Green border, checkmark |

User-assigned cards get amber/gold color with person icon.

## Card Views

Three levels of detail:

1. **Card preview** - Truncated streaming text in the card itself
2. **Active panel** - Full streaming output for focused/active card
3. **Popup overlay** - Complete history with metadata (press Enter)

## Interaction Model

**Hybrid mode**: Watch by default, pause to intervene.

### Keyboard Controls

| Key | Action |
|-----|--------|
| `h/l` or `←/→` | Move between columns |
| `j/k` or `↑/↓` | Move between cards in column |
| `Tab` | Cycle focus: Board → Active Panel → Board |
| `Enter` | Open popup detail view |
| `Esc` | Close popup / exit pause mode |
| `Space` | Pause/Resume execution |
| `q` | Quit (with confirmation) |
| `d` | Mark your task done (quick) |
| `D` | Mark done with notes (opens form) |
| `r` | Reassign task (paused only) |
| `a` | Add new task (paused only) |
| `?` | Toggle help overlay |

### Intervention (Light Touch)

When paused, user can:
- Reassign tasks to different AI
- Add new tasks

Cannot: edit existing tasks, delete tasks, override PM decisions.

## User Tasks

PM can assign tasks to the human user:

- **Blocking**: Execution pauses until user completes
- **Non-blocking**: AIs continue on independent tasks

### User Task Flow

1. PM creates task → `EventUserTaskCreated`
2. Card appears in Backlog with amber styling
3. If blocking, dependent tasks show "Blocked by: User task"
4. User completes with `d` or `D` → `EventUserTaskCompleted`
5. Dependent tasks unblock and continue

## Architecture

### Event-Driven Team Package

Refactor `internal/team` to be UI-agnostic:

```go
type Runner struct {
    Events chan Event  // TUI subscribes here
}

type Event struct {
    Type      EventType
    Timestamp time.Time
    TaskID    string
    Actor     string      // "pm", "claude", "user", etc.
    Data      interface{}
}
```

### Event Types

```go
const (
    EventPhaseChanged      // Analysis → Planning → Execution...
    EventPMDecision        // PM chose work mode, assigned tasks
    EventTaskCreated       // New task added to board
    EventTaskStarted       // AI began working
    EventTaskProgress      // Streaming content update
    EventTaskCompleted     // AI finished
    EventTaskMovedToReview // Awaiting PM review
    EventPMApproved        // PM approved, moves to Done
    EventUserTaskCreated   // Task assigned to human
    EventUserTaskBlocking  // Execution paused waiting on user
    EventUserTaskCompleted // User marked task done
    EventError             // Something went wrong
    EventSessionComplete   // All done
)
```

### File Structure

| File | Purpose |
|------|---------|
| `internal/tui/kanban.go` | Main Kanban board Bubble Tea model |
| `internal/tui/card.go` | Individual card component |
| `internal/tui/popup.go` | Detail popup overlay |
| `internal/tui/teamtui.go` | Orchestrator connecting TUI ↔ team.Runner |

### Changes to Existing Code

- `internal/team/team.go` - Add event channel emission
- `internal/team/events.go` - New file for event types
- `cmd/team/main.go` - Add `--tui` flag

## Integration Flow

```
┌──────────────────┐     ┌──────────────────┐     ┌──────────────────┐
│   cmd/team       │────▶│  internal/tui    │────▶│  internal/team   │
│   --tui flag     │     │  teamtui.go      │     │  Runner (events) │
└──────────────────┘     └──────────────────┘     └──────────────────┘
```

Console mode: small adapter subscribes to events and prints them.
TUI mode: Bubble Tea model subscribes to events and updates UI.
