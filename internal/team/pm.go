package team

import (
	"context"
	"strings"

	"github.com/jxmullins/thekanbansociety/internal/config"
	"github.com/jxmullins/thekanbansociety/internal/provider"
)

// PMSelector handles project manager selection based on task analysis.
type PMSelector struct {
	registry *provider.Registry
	config   *config.Config
}

// NewPMSelector creates a new PM selector.
func NewPMSelector(registry *provider.Registry, cfg *config.Config) *PMSelector {
	return &PMSelector{
		registry: registry,
		config:   cfg,
	}
}

// PMStrength represents an AI's strength in a particular area.
type PMStrength struct {
	AIID        string
	DisplayName string
	Strengths   []string
	Score       int
}

// GetPMStrengths returns the known strengths of potential PMs.
func GetPMStrengths() []PMStrength {
	return []PMStrength{
		{
			AIID:        "claude",
			DisplayName: "Claude",
			Strengths: []string{
				"nuanced reasoning",
				"writing",
				"analysis",
				"ethics",
				"explanation",
				"documentation",
				"architecture",
				"design",
			},
		},
		{
			AIID:        "gpt",
			DisplayName: "GPT",
			Strengths: []string{
				"coding",
				"implementation",
				"debugging",
				"api design",
				"system design",
				"data analysis",
				"algorithms",
			},
		},
		{
			AIID:        "gemini",
			DisplayName: "Gemini",
			Strengths: []string{
				"research",
				"multimodal",
				"information synthesis",
				"fact checking",
				"summarization",
				"translation",
			},
		},
		{
			AIID:        "groq",
			DisplayName: "Groq (Llama)",
			Strengths: []string{
				"speed",
				"quick iteration",
				"brainstorming",
				"rapid prototyping",
			},
		},
	}
}

// Select chooses the best PM for a given task.
func (s *PMSelector) Select(ctx context.Context, task string, availableMembers []string) (string, error) {
	// Score each potential PM based on task keywords
	taskLower := strings.ToLower(task)
	strengths := GetPMStrengths()

	var bestPM string
	bestScore := -1

	for _, pm := range strengths {
		// Check if this PM is in available members
		available := false
		for _, member := range availableMembers {
			if member == pm.AIID {
				available = true
				break
			}
		}
		if !available {
			continue
		}

		// Score based on keyword matches
		score := 0
		for _, strength := range pm.Strengths {
			if strings.Contains(taskLower, strength) {
				score += 2
			}
			// Partial matches
			for _, word := range strings.Fields(strength) {
				if strings.Contains(taskLower, word) {
					score++
				}
			}
		}

		// Add task-type bonuses
		score += s.getTaskTypeBonus(taskLower, pm.AIID)

		if score > bestScore {
			bestScore = score
			bestPM = pm.AIID
		}
	}

	// Default to claude if no clear winner or no matches
	if bestPM == "" {
		for _, member := range availableMembers {
			if member == "claude" {
				return "claude", nil
			}
		}
		// Fall back to first available
		if len(availableMembers) > 0 {
			return availableMembers[0], nil
		}
	}

	return bestPM, nil
}

// getTaskTypeBonus returns bonus points for specific task types.
func (s *PMSelector) getTaskTypeBonus(task, aiID string) int {
	bonus := 0

	// Coding tasks favor GPT
	codingKeywords := []string{"code", "implement", "build", "develop", "program", "function", "api", "debug", "fix"}
	for _, kw := range codingKeywords {
		if strings.Contains(task, kw) {
			if aiID == "gpt" {
				bonus += 3
			}
			break
		}
	}

	// Writing/documentation tasks favor Claude
	writingKeywords := []string{"write", "document", "explain", "analyze", "review", "design", "architect", "plan"}
	for _, kw := range writingKeywords {
		if strings.Contains(task, kw) {
			if aiID == "claude" {
				bonus += 3
			}
			break
		}
	}

	// Research tasks favor Gemini
	researchKeywords := []string{"research", "find", "search", "compare", "investigate", "summarize"}
	for _, kw := range researchKeywords {
		if strings.Contains(task, kw) {
			if aiID == "gemini" {
				bonus += 3
			}
			break
		}
	}

	// Quick/iterative tasks favor Groq
	quickKeywords := []string{"quick", "fast", "simple", "prototype", "brainstorm", "iterate"}
	for _, kw := range quickKeywords {
		if strings.Contains(task, kw) {
			if aiID == "groq" {
				bonus += 3
			}
			break
		}
	}

	return bonus
}

// SelectWithAnalysis selects a PM and returns the analysis.
func (s *PMSelector) SelectWithAnalysis(ctx context.Context, task string, availableMembers []string) (string, string, error) {
	pm, err := s.Select(ctx, task, availableMembers)
	if err != nil {
		return "", "", err
	}

	// Generate analysis
	strengths := GetPMStrengths()
	var pmStrength PMStrength
	for _, str := range strengths {
		if str.AIID == pm {
			pmStrength = str
			break
		}
	}

	analysis := "Selected " + pmStrength.DisplayName + " as PM based on strengths in: " +
		strings.Join(pmStrength.Strengths[:min(3, len(pmStrength.Strengths))], ", ")

	return pm, analysis, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
