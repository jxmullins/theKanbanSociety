// Package budget provides token cost tracking for AI interactions.
package budget

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// PricingTier represents token pricing for a model.
type PricingTier struct {
	Provider       string
	Model          string
	InputPer1K     float64 // Cost per 1K input tokens
	OutputPer1K    float64 // Cost per 1K output tokens
	CachedPer1K    float64 // Cost per 1K cached tokens (if applicable)
}

// DefaultPricing returns pricing for common models (as of early 2025).
// Prices in USD.
func DefaultPricing() []PricingTier {
	return []PricingTier{
		// Anthropic
		{Provider: "anthropic", Model: "claude-opus-4-5-20250929", InputPer1K: 0.015, OutputPer1K: 0.075},
		{Provider: "anthropic", Model: "claude-sonnet-4-5-20250929", InputPer1K: 0.003, OutputPer1K: 0.015},
		{Provider: "anthropic", Model: "claude-haiku-3-5-20241022", InputPer1K: 0.0008, OutputPer1K: 0.004},

		// OpenAI
		{Provider: "openai", Model: "gpt-5.2", InputPer1K: 0.01, OutputPer1K: 0.03},
		{Provider: "openai", Model: "gpt-5.2-codex", InputPer1K: 0.01, OutputPer1K: 0.03},
		{Provider: "openai", Model: "o3", InputPer1K: 0.015, OutputPer1K: 0.06},
		{Provider: "openai", Model: "o4-mini", InputPer1K: 0.003, OutputPer1K: 0.012},

		// Google
		{Provider: "google", Model: "gemini-3-pro-preview", InputPer1K: 0.00125, OutputPer1K: 0.005},
		{Provider: "google", Model: "gemini-3-flash-preview", InputPer1K: 0.000075, OutputPer1K: 0.0003},

		// Groq (very low cost)
		{Provider: "groq", Model: "llama-4-maverick", InputPer1K: 0.0002, OutputPer1K: 0.0002},
		{Provider: "groq", Model: "llama-3.3-70b-versatile", InputPer1K: 0.00059, OutputPer1K: 0.00079},

		// DeepSeek (very low cost)
		{Provider: "deepseek", Model: "deepseek-chat", InputPer1K: 0.00014, OutputPer1K: 0.00028},
		{Provider: "deepseek", Model: "deepseek-reasoner", InputPer1K: 0.00055, OutputPer1K: 0.00219},

		// Mistral
		{Provider: "mistral", Model: "mistral-large-2512", InputPer1K: 0.002, OutputPer1K: 0.006},
		{Provider: "mistral", Model: "codestral-latest", InputPer1K: 0.001, OutputPer1K: 0.003},

		// xAI
		{Provider: "xai", Model: "grok-4-1-fast-reasoning", InputPer1K: 0.003, OutputPer1K: 0.015},
		{Provider: "xai", Model: "grok-code-fast-1", InputPer1K: 0.003, OutputPer1K: 0.015},

		// Local (free)
		{Provider: "ollama", Model: "*", InputPer1K: 0, OutputPer1K: 0},
		{Provider: "lmstudio", Model: "*", InputPer1K: 0, OutputPer1K: 0},
	}
}

// Usage represents token usage for a single interaction.
type Usage struct {
	Timestamp    time.Time `json:"timestamp"`
	Provider     string    `json:"provider"`
	Model        string    `json:"model"`
	AIID         string    `json:"ai_id"`
	InputTokens  int       `json:"input_tokens"`
	OutputTokens int       `json:"output_tokens"`
	CachedTokens int       `json:"cached_tokens,omitempty"`
	Cost         float64   `json:"cost"`
	Phase        string    `json:"phase,omitempty"`
}

// Tracker tracks budget and usage across a session.
type Tracker struct {
	mu           sync.Mutex
	usages       []Usage
	pricing      map[string]PricingTier // key: provider/model
	budget       float64                // Max budget (0 = unlimited)
	totalCost    float64
	showCosts    bool
}

// NewTracker creates a new budget tracker.
func NewTracker() *Tracker {
	t := &Tracker{
		usages:  []Usage{},
		pricing: make(map[string]PricingTier),
	}

	// Load default pricing
	for _, p := range DefaultPricing() {
		key := fmt.Sprintf("%s/%s", p.Provider, p.Model)
		t.pricing[key] = p
	}

	return t
}

// SetBudget sets a maximum budget limit.
func (t *Tracker) SetBudget(budget float64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.budget = budget
}

// SetShowCosts enables/disables cost display.
func (t *Tracker) SetShowCosts(show bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.showCosts = show
}

// GetPricing returns the pricing for a model.
func (t *Tracker) GetPricing(provider, model string) (PricingTier, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Try exact match
	key := fmt.Sprintf("%s/%s", provider, model)
	if p, ok := t.pricing[key]; ok {
		return p, true
	}

	// Try wildcard match
	key = fmt.Sprintf("%s/*", provider)
	if p, ok := t.pricing[key]; ok {
		return p, true
	}

	return PricingTier{}, false
}

// RecordUsage records token usage and calculates cost.
func (t *Tracker) RecordUsage(provider, model, aiID string, inputTokens, outputTokens int, phase string) Usage {
	t.mu.Lock()
	defer t.mu.Unlock()

	pricing, _ := t.GetPricing(provider, model)

	cost := (float64(inputTokens)/1000)*pricing.InputPer1K +
		(float64(outputTokens)/1000)*pricing.OutputPer1K

	usage := Usage{
		Timestamp:    time.Now(),
		Provider:     provider,
		Model:        model,
		AIID:         aiID,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		Cost:         cost,
		Phase:        phase,
	}

	t.usages = append(t.usages, usage)
	t.totalCost += cost

	return usage
}

// GetTotalCost returns the total cost so far.
func (t *Tracker) GetTotalCost() float64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.totalCost
}

// GetRemainingBudget returns remaining budget (or -1 if unlimited).
func (t *Tracker) GetRemainingBudget() float64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.budget == 0 {
		return -1
	}
	return t.budget - t.totalCost
}

// IsOverBudget returns whether we've exceeded the budget.
func (t *Tracker) IsOverBudget() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.budget > 0 && t.totalCost > t.budget
}

// GetUsages returns all recorded usages.
func (t *Tracker) GetUsages() []Usage {
	t.mu.Lock()
	defer t.mu.Unlock()
	result := make([]Usage, len(t.usages))
	copy(result, t.usages)
	return result
}

// GetUsagesByProvider returns usages grouped by provider.
func (t *Tracker) GetUsagesByProvider() map[string][]Usage {
	t.mu.Lock()
	defer t.mu.Unlock()

	result := make(map[string][]Usage)
	for _, u := range t.usages {
		result[u.Provider] = append(result[u.Provider], u)
	}
	return result
}

// GetCostByProvider returns total cost per provider.
func (t *Tracker) GetCostByProvider() map[string]float64 {
	t.mu.Lock()
	defer t.mu.Unlock()

	result := make(map[string]float64)
	for _, u := range t.usages {
		result[u.Provider] += u.Cost
	}
	return result
}

// GetTokensByProvider returns total tokens per provider.
func (t *Tracker) GetTokensByProvider() map[string]struct{ Input, Output int } {
	t.mu.Lock()
	defer t.mu.Unlock()

	result := make(map[string]struct{ Input, Output int })
	for _, u := range t.usages {
		entry := result[u.Provider]
		entry.Input += u.InputTokens
		entry.Output += u.OutputTokens
		result[u.Provider] = entry
	}
	return result
}

// Summary returns a summary of usage.
func (t *Tracker) Summary() string {
	t.mu.Lock()
	defer t.mu.Unlock()

	if len(t.usages) == 0 {
		return "No usage recorded"
	}

	var totalInput, totalOutput int
	for _, u := range t.usages {
		totalInput += u.InputTokens
		totalOutput += u.OutputTokens
	}

	summary := fmt.Sprintf("Usage Summary:\n")
	summary += fmt.Sprintf("  Interactions: %d\n", len(t.usages))
	summary += fmt.Sprintf("  Input tokens: %d\n", totalInput)
	summary += fmt.Sprintf("  Output tokens: %d\n", totalOutput)
	summary += fmt.Sprintf("  Total cost: $%.4f\n", t.totalCost)

	if t.budget > 0 {
		summary += fmt.Sprintf("  Budget: $%.4f (%.1f%% used)\n",
			t.budget, (t.totalCost/t.budget)*100)
	}

	return summary
}

// SaveToFile saves usage data to a JSON file.
func (t *Tracker) SaveToFile(path string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	data, err := json.MarshalIndent(struct {
		Usages    []Usage `json:"usages"`
		TotalCost float64 `json:"total_cost"`
		Budget    float64 `json:"budget,omitempty"`
	}{
		Usages:    t.usages,
		TotalCost: t.totalCost,
		Budget:    t.budget,
	}, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// LoadFromFile loads usage data from a JSON file.
func (t *Tracker) LoadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var loaded struct {
		Usages    []Usage `json:"usages"`
		TotalCost float64 `json:"total_cost"`
		Budget    float64 `json:"budget"`
	}

	if err := json.Unmarshal(data, &loaded); err != nil {
		return err
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	t.usages = loaded.Usages
	t.totalCost = loaded.TotalCost
	t.budget = loaded.Budget

	return nil
}

// EstimateCost estimates cost for a given number of tokens.
func (t *Tracker) EstimateCost(provider, model string, inputTokens, outputTokens int) float64 {
	pricing, ok := t.GetPricing(provider, model)
	if !ok {
		return 0
	}

	return (float64(inputTokens)/1000)*pricing.InputPer1K +
		(float64(outputTokens)/1000)*pricing.OutputPer1K
}
