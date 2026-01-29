// Package tui provides the Bubble Tea TUI for The Council of Legends.
package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Column represents a Kanban column.
type Column int

const (
	ColumnBacklog Column = iota
	ColumnInProgress
	ColumnReview
	ColumnDone
)

func (c Column) String() string {
	switch c {
	case ColumnBacklog:
		return "Backlog"
	case ColumnInProgress:
		return "In Progress"
	case ColumnReview:
		return "Review"
	case ColumnDone:
		return "Done"
	default:
		return "Unknown"
	}
}

// KanbanCard represents a task card on the board.
type KanbanCard struct {
	ID          string
	Title       string
	Description string
	AssignedTo  string   // AI ID or "user"
	Column      Column
	IsBlocking  bool     // Only relevant for user tasks
	IsUserTask  bool     // True if assigned to human
	StreamBuf   strings.Builder
	FullHistory strings.Builder
	Progress    float64  // 0.0-1.0
	DependsOn   []string
	BlockedBy   []string // Computed: tasks that block this one
	Done        bool
	Error       error
}

// NewKanbanCard creates a new Kanban card.
func NewKanbanCard(id, title, assignedTo string) *KanbanCard {
	return &KanbanCard{
		ID:         id,
		Title:      title,
		AssignedTo: assignedTo,
		Column:     ColumnBacklog,
		IsUserTask: assignedTo == "user",
	}
}

// IsBlocked returns true if this card is blocked by incomplete dependencies.
func (c *KanbanCard) IsBlocked() bool {
	return len(c.BlockedBy) > 0
}

// CardStyle returns the appropriate style for this card's state.
func (c *KanbanCard) CardStyle(styles Styles, selected bool, width int) lipgloss.Style {
	baseStyle := styles.Panel.Width(width - 2)

	// Border color based on state
	var borderColor lipgloss.Color
	switch {
	case selected:
		borderColor = colorBorderActive
	case c.Error != nil:
		borderColor = colorError
	case c.IsBlocked():
		borderColor = colorError
	case c.IsUserTask && c.Column == ColumnInProgress:
		borderColor = colorWarning // Amber for "waiting on you"
	case c.Column == ColumnInProgress:
		borderColor = GetAIColor(c.AssignedTo)
	case c.Column == ColumnReview:
		borderColor = colorPrimary
	case c.Column == ColumnDone:
		borderColor = colorSuccess
	default:
		borderColor = colorBorder
	}

	return baseStyle.BorderForeground(borderColor)
}

// Render renders the card content.
func (c *KanbanCard) Render(styles Styles, selected bool, width, maxLines int) string {
	style := c.CardStyle(styles, selected, width)

	// Header: title
	title := truncateString(c.Title, width-4)

	// Assignee line
	var assignee string
	if c.IsUserTask {
		assignee = styles.Warning.Render("You")
	} else {
		assignee = styles.AINameStyle(c.AssignedTo).Render(c.AssignedTo)
	}

	// Status indicator
	var status string
	switch {
	case c.Error != nil:
		status = styles.Error.Render("Error")
	case c.IsBlocked():
		status = styles.Error.Render("Blocked")
	case c.Column == ColumnInProgress && c.IsUserTask:
		status = styles.Warning.Render("Waiting on you")
	case c.Column == ColumnInProgress:
		status = renderProgressBar(c.Progress, width-4)
	case c.Column == ColumnDone:
		status = styles.Success.Render("Done")
	}

	// Stream preview (truncated)
	preview := ""
	if c.Column == ColumnInProgress && c.StreamBuf.Len() > 0 {
		lines := strings.Split(c.StreamBuf.String(), "\n")
		previewLines := maxLines - 4 // Leave room for title, assignee, status
		if previewLines > 0 && len(lines) > 0 {
			if len(lines) > previewLines {
				lines = lines[len(lines)-previewLines:]
			}
			preview = strings.Join(lines, "\n")
			preview = truncateLines(preview, width-4)
		}
	}

	// Build card content
	var content strings.Builder
	content.WriteString(title)
	content.WriteString("\n")
	content.WriteString(assignee)
	if status != "" {
		content.WriteString("\n")
		content.WriteString(status)
	}
	if preview != "" {
		content.WriteString("\n")
		content.WriteString(styles.Muted.Render(preview))
	}

	return style.Render(content.String())
}

func renderProgressBar(progress float64, width int) string {
	barWidth := width - 2
	if barWidth < 5 {
		barWidth = 5
	}
	filled := int(progress * float64(barWidth))
	empty := barWidth - filled

	bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)
	return bar
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func truncateLines(s string, maxWidth int) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if len(line) > maxWidth {
			lines[i] = line[:maxWidth-3] + "..."
		}
	}
	return strings.Join(lines, "\n")
}
