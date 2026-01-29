// Package team provides multi-AI team collaboration for task completion.
package team

import "time"

// EventType represents the type of team event.
type EventType int

const (
	EventPhaseChanged EventType = iota
	EventPMSelected
	EventPMDecision
	EventTaskCreated
	EventTaskStarted
	EventTaskProgress
	EventTaskCompleted
	EventTaskMovedToReview
	EventPMApproved
	EventUserTaskCreated
	EventUserTaskBlocking
	EventUserTaskCompleted
	EventError
	EventSessionComplete
)

func (e EventType) String() string {
	switch e {
	case EventPhaseChanged:
		return "PhaseChanged"
	case EventPMSelected:
		return "PMSelected"
	case EventPMDecision:
		return "PMDecision"
	case EventTaskCreated:
		return "TaskCreated"
	case EventTaskStarted:
		return "TaskStarted"
	case EventTaskProgress:
		return "TaskProgress"
	case EventTaskCompleted:
		return "TaskCompleted"
	case EventTaskMovedToReview:
		return "TaskMovedToReview"
	case EventPMApproved:
		return "PMApproved"
	case EventUserTaskCreated:
		return "UserTaskCreated"
	case EventUserTaskBlocking:
		return "UserTaskBlocking"
	case EventUserTaskCompleted:
		return "UserTaskCompleted"
	case EventError:
		return "Error"
	case EventSessionComplete:
		return "SessionComplete"
	default:
		return "Unknown"
	}
}

// Event represents a team workflow event.
type Event struct {
	Type      EventType
	Timestamp time.Time
	TaskID    string
	Actor     string      // "pm", AI ID, or "user"
	Data      interface{}
}

// TaskCreatedData contains data for TaskCreated events.
type TaskCreatedData struct {
	Title       string
	Description string
	AssignedTo  string
	IsBlocking  bool   // Only for user tasks
	DependsOn   []string
}

// TaskProgressData contains data for TaskProgress events.
type TaskProgressData struct {
	Content  string  // Streaming chunk
	Progress float64 // 0.0-1.0
}

// PMDecisionData contains data for PMDecision events.
type PMDecisionData struct {
	WorkMode    WorkMode
	PlanSummary string
	Tasks       []PlanStep
}

// PhaseChangedData contains data for PhaseChanged events.
type PhaseChangedData struct {
	OldPhase Phase
	NewPhase Phase
}

// PMSelectedData contains data for PMSelected events.
type PMSelectedData struct {
	PMID        string
	DisplayName string
}

// ErrorData contains data for Error events.
type ErrorData struct {
	Error   error
	TaskID  string
	Message string
}

// UserTaskCompletedData contains data for UserTaskCompleted events.
type UserTaskCompletedData struct {
	Notes string // Optional notes from user
}

// NewEvent creates a new event with the current timestamp.
func NewEvent(eventType EventType, actor string, data interface{}) Event {
	return Event{
		Type:      eventType,
		Timestamp: time.Now(),
		Actor:     actor,
		Data:      data,
	}
}

// NewTaskEvent creates a new event for a specific task.
func NewTaskEvent(eventType EventType, taskID, actor string, data interface{}) Event {
	return Event{
		Type:      eventType,
		Timestamp: time.Now(),
		TaskID:    taskID,
		Actor:     actor,
		Data:      data,
	}
}
