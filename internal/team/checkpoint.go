package team

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// CheckpointType represents the type of checkpoint.
type CheckpointType string

const (
	CheckpointPlanApproval CheckpointType = "plan_approval"
	CheckpointMilestone    CheckpointType = "milestone"
	CheckpointReview       CheckpointType = "review"
	CheckpointDelivery     CheckpointType = "delivery"
)

// Checkpoint represents a point where user approval may be required.
type Checkpoint struct {
	ID          string         `json:"id"`
	Type        CheckpointType `json:"type"`
	Description string         `json:"description"`
	Phase       Phase          `json:"phase"`
	CreatedAt   time.Time      `json:"created_at"`
	Approved    bool           `json:"approved"`
	ApprovedAt  *time.Time     `json:"approved_at,omitempty"`
	Notes       string         `json:"notes,omitempty"`
	Data        interface{}    `json:"data,omitempty"`
}

// NewCheckpoint creates a new checkpoint.
func NewCheckpoint(checkpointType CheckpointType, description string, phase Phase) Checkpoint {
	return Checkpoint{
		ID:          fmt.Sprintf("cp_%d", time.Now().UnixNano()),
		Type:        checkpointType,
		Description: description,
		Phase:       phase,
		CreatedAt:   time.Now(),
	}
}

// Approve marks the checkpoint as approved.
func (c *Checkpoint) Approve(notes string) {
	c.Approved = true
	now := time.Now()
	c.ApprovedAt = &now
	c.Notes = notes
}

// Reject marks the checkpoint as rejected.
func (c *Checkpoint) Reject(notes string) {
	c.Approved = false
	c.Notes = notes
}

// CheckpointManager manages checkpoints for a session.
type CheckpointManager struct {
	checkpoints []Checkpoint
	level       CheckpointLevel
	saveDir     string
}

// NewCheckpointManager creates a new checkpoint manager.
func NewCheckpointManager(level CheckpointLevel, saveDir string) *CheckpointManager {
	return &CheckpointManager{
		checkpoints: []Checkpoint{},
		level:       level,
		saveDir:     saveDir,
	}
}

// Add adds a checkpoint.
func (m *CheckpointManager) Add(checkpoint Checkpoint) {
	m.checkpoints = append(m.checkpoints, checkpoint)
}

// RequiresApproval returns whether a checkpoint type requires approval at the current level.
func (m *CheckpointManager) RequiresApproval(checkpointType CheckpointType) bool {
	switch m.level {
	case CheckpointNone:
		return false
	case CheckpointMajor:
		return checkpointType == CheckpointPlanApproval || checkpointType == CheckpointDelivery
	case CheckpointAll:
		return true
	default:
		return true
	}
}

// GetPending returns all pending (unapproved) checkpoints.
func (m *CheckpointManager) GetPending() []Checkpoint {
	var pending []Checkpoint
	for _, c := range m.checkpoints {
		if !c.Approved {
			pending = append(pending, c)
		}
	}
	return pending
}

// GetAll returns all checkpoints.
func (m *CheckpointManager) GetAll() []Checkpoint {
	return m.checkpoints
}

// Save persists checkpoints to disk.
func (m *CheckpointManager) Save() error {
	if m.saveDir == "" {
		return nil
	}

	if err := os.MkdirAll(m.saveDir, 0755); err != nil {
		return fmt.Errorf("creating checkpoint directory: %w", err)
	}

	path := filepath.Join(m.saveDir, "checkpoints.json")
	data, err := json.MarshalIndent(m.checkpoints, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling checkpoints: %w", err)
	}

	if err := os.WriteFile(path, data, 0640); err != nil {
		return fmt.Errorf("writing checkpoints: %w", err)
	}

	return nil
}

// Load reads checkpoints from disk.
func (m *CheckpointManager) Load() error {
	if m.saveDir == "" {
		return nil
	}

	path := filepath.Join(m.saveDir, "checkpoints.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading checkpoints: %w", err)
	}

	if err := json.Unmarshal(data, &m.checkpoints); err != nil {
		return fmt.Errorf("unmarshaling checkpoints: %w", err)
	}

	return nil
}

// Milestone represents a major progress point in the project.
type Milestone struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Phase       Phase     `json:"phase"`
	CreatedAt   time.Time `json:"created_at"`
	Completed   bool      `json:"completed"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Artifacts   []string  `json:"artifacts,omitempty"`
}

// NewMilestone creates a new milestone.
func NewMilestone(name, description string, phase Phase) Milestone {
	return Milestone{
		ID:          fmt.Sprintf("ms_%d", time.Now().UnixNano()),
		Name:        name,
		Description: description,
		Phase:       phase,
		CreatedAt:   time.Now(),
	}
}

// Complete marks the milestone as completed.
func (m *Milestone) Complete(artifacts []string) {
	m.Completed = true
	now := time.Now()
	m.CompletedAt = &now
	m.Artifacts = artifacts
}

// MilestoneTracker tracks milestones for a session.
type MilestoneTracker struct {
	milestones []Milestone
	saveDir    string
}

// NewMilestoneTracker creates a new milestone tracker.
func NewMilestoneTracker(saveDir string) *MilestoneTracker {
	return &MilestoneTracker{
		milestones: []Milestone{},
		saveDir:    saveDir,
	}
}

// Add adds a milestone.
func (t *MilestoneTracker) Add(milestone Milestone) {
	t.milestones = append(t.milestones, milestone)
}

// GetAll returns all milestones.
func (t *MilestoneTracker) GetAll() []Milestone {
	return t.milestones
}

// GetCompleted returns completed milestones.
func (t *MilestoneTracker) GetCompleted() []Milestone {
	var completed []Milestone
	for _, m := range t.milestones {
		if m.Completed {
			completed = append(completed, m)
		}
	}
	return completed
}

// GetPending returns pending milestones.
func (t *MilestoneTracker) GetPending() []Milestone {
	var pending []Milestone
	for _, m := range t.milestones {
		if !m.Completed {
			pending = append(pending, m)
		}
	}
	return pending
}

// Progress returns the completion percentage.
func (t *MilestoneTracker) Progress() float64 {
	if len(t.milestones) == 0 {
		return 0
	}
	completed := len(t.GetCompleted())
	return float64(completed) / float64(len(t.milestones)) * 100
}

// Save persists milestones to disk.
func (t *MilestoneTracker) Save() error {
	if t.saveDir == "" {
		return nil
	}

	if err := os.MkdirAll(t.saveDir, 0755); err != nil {
		return fmt.Errorf("creating milestone directory: %w", err)
	}

	path := filepath.Join(t.saveDir, "milestones.json")
	data, err := json.MarshalIndent(t.milestones, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling milestones: %w", err)
	}

	if err := os.WriteFile(path, data, 0640); err != nil {
		return fmt.Errorf("writing milestones: %w", err)
	}

	return nil
}

// Load reads milestones from disk.
func (t *MilestoneTracker) Load() error {
	if t.saveDir == "" {
		return nil
	}

	path := filepath.Join(t.saveDir, "milestones.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading milestones: %w", err)
	}

	if err := json.Unmarshal(data, &t.milestones); err != nil {
		return fmt.Errorf("unmarshaling milestones: %w", err)
	}

	return nil
}
