package scotus

import (
	"context"
	"strings"

	"github.com/jxmullins/thekanbansociety/internal/config"
	"github.com/jxmullins/thekanbansociety/internal/provider"
)

// ChiefSelector handles Chief Justice selection.
type ChiefSelector struct {
	registry *provider.Registry
	config   *config.Config
}

// NewChiefSelector creates a new Chief Justice selector.
func NewChiefSelector(registry *provider.Registry, cfg *config.Config) *ChiefSelector {
	return &ChiefSelector{
		registry: registry,
		config:   cfg,
	}
}

// JusticeStrength represents strengths relevant to CJ selection.
type JusticeStrength struct {
	AIID        string
	DisplayName string
	Strengths   []string
	CJScore     int // Base score for CJ role
}

// GetJusticeStrengths returns the known strengths of potential justices.
func GetJusticeStrengths() []JusticeStrength {
	return []JusticeStrength{
		{
			AIID:        "claude",
			DisplayName: "Claude",
			Strengths: []string{
				"nuanced reasoning",
				"ethics",
				"balanced analysis",
				"constitutional interpretation",
				"mediating disagreements",
			},
			CJScore: 10, // Claude is well-suited to lead deliberations
		},
		{
			AIID:        "gpt",
			DisplayName: "GPT",
			Strengths: []string{
				"structured analysis",
				"precedent research",
				"clear opinions",
				"systematic reasoning",
			},
			CJScore: 8,
		},
		{
			AIID:        "gemini",
			DisplayName: "Gemini",
			Strengths: []string{
				"research",
				"information synthesis",
				"broad perspective",
				"fact-checking",
			},
			CJScore: 7,
		},
		{
			AIID:        "groq",
			DisplayName: "Groq (Llama)",
			Strengths: []string{
				"quick analysis",
				"direct reasoning",
				"efficient deliberation",
			},
			CJScore: 5,
		},
	}
}

// Select chooses the best Chief Justice for a given case.
func (s *ChiefSelector) Select(ctx context.Context, resolution string, justices []string) (string, error) {
	resolutionLower := strings.ToLower(resolution)
	strengths := GetJusticeStrengths()

	var bestCJ string
	bestScore := -1

	for _, js := range strengths {
		// Check if this justice is in the panel
		available := false
		for _, j := range justices {
			if j == js.AIID {
				available = true
				break
			}
		}
		if !available {
			continue
		}

		// Start with base CJ score
		score := js.CJScore

		// Add bonuses based on case type
		score += s.getCaseTypeBonus(resolutionLower, js.AIID)

		// Add strength keyword matches
		for _, strength := range js.Strengths {
			if strings.Contains(resolutionLower, strength) {
				score += 2
			}
			for _, word := range strings.Fields(strength) {
				if strings.Contains(resolutionLower, word) {
					score++
				}
			}
		}

		if score > bestScore {
			bestScore = score
			bestCJ = js.AIID
		}
	}

	// Default to claude if no clear winner
	if bestCJ == "" {
		for _, j := range justices {
			if j == "claude" {
				return "claude", nil
			}
		}
		if len(justices) > 0 {
			return justices[0], nil
		}
	}

	return bestCJ, nil
}

// getCaseTypeBonus returns bonus points based on case type.
func (s *ChiefSelector) getCaseTypeBonus(resolution, aiID string) int {
	bonus := 0

	// Ethics/rights cases favor Claude
	ethicsKeywords := []string{"ethics", "rights", "freedom", "privacy", "constitutional", "liberty", "discrimination"}
	for _, kw := range ethicsKeywords {
		if strings.Contains(resolution, kw) {
			if aiID == "claude" {
				bonus += 5
			}
			break
		}
	}

	// Technical/legal procedure cases favor GPT
	technicalKeywords := []string{"procedure", "jurisdiction", "standing", "statutory", "regulation", "administrative"}
	for _, kw := range technicalKeywords {
		if strings.Contains(resolution, kw) {
			if aiID == "gpt" {
				bonus += 4
			}
			break
		}
	}

	// Research-heavy cases favor Gemini
	researchKeywords := []string{"historical", "precedent", "comparative", "international", "analysis"}
	for _, kw := range researchKeywords {
		if strings.Contains(resolution, kw) {
			if aiID == "gemini" {
				bonus += 3
			}
			break
		}
	}

	return bonus
}

// SelectWithAnalysis selects a CJ and returns analysis.
func (s *ChiefSelector) SelectWithAnalysis(ctx context.Context, resolution string, justices []string) (string, string, error) {
	cj, err := s.Select(ctx, resolution, justices)
	if err != nil {
		return "", "", err
	}

	strengths := GetJusticeStrengths()
	var cjStrength JusticeStrength
	for _, js := range strengths {
		if js.AIID == cj {
			cjStrength = js
			break
		}
	}

	analysis := "Selected " + cjStrength.DisplayName + " as Chief Justice based on strengths in: " +
		strings.Join(cjStrength.Strengths[:min(3, len(cjStrength.Strengths))], ", ")

	return cj, analysis, nil
}

// PreviousCJHistory tracks CJ selections for rotation if desired.
type PreviousCJHistory struct {
	Selections []string
}

// NewPreviousCJHistory creates a new history tracker.
func NewPreviousCJHistory() *PreviousCJHistory {
	return &PreviousCJHistory{
		Selections: []string{},
	}
}

// Add records a CJ selection.
func (h *PreviousCJHistory) Add(cj string) {
	h.Selections = append(h.Selections, cj)
}

// GetRotationCandidate returns a CJ that hasn't been selected recently.
func (h *PreviousCJHistory) GetRotationCandidate(justices []string) string {
	if len(h.Selections) == 0 {
		if len(justices) > 0 {
			return justices[0]
		}
		return ""
	}

	// Find justice with fewest recent selections
	counts := make(map[string]int)
	for _, j := range justices {
		counts[j] = 0
	}

	// Only look at recent history
	lookback := min(10, len(h.Selections))
	for i := len(h.Selections) - lookback; i < len(h.Selections); i++ {
		if _, ok := counts[h.Selections[i]]; ok {
			counts[h.Selections[i]]++
		}
	}

	// Find minimum
	minCount := -1
	var candidate string
	for j, count := range counts {
		if minCount == -1 || count < minCount {
			minCount = count
			candidate = j
		}
	}

	return candidate
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
