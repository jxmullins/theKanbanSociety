// The Council of Legends - Assessment CLI
// Entry point for AI model assessment and comparison
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/jxmullins/thekanbansociety/internal/config"
	"github.com/jxmullins/thekanbansociety/internal/provider"
)

var (
	cfgFile   string
	models    []string
	outputDir string
	verbose   bool
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "assess",
	Short: "The Council of Legends - AI Assessment Tool",
	Long: `Assess and compare AI models using standardized tests.

The assessment tool helps you:
- Compare response quality across models
- Measure response latency
- Test specific capabilities
- Generate comparison reports

Example:
  assess run --models claude,gpt,gemini
  assess baseline --models claude
  assess compare results/baseline.json results/new.json`,
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run assessment on specified models",
	RunE:  runAssessment,
}

var baselineCmd = &cobra.Command{
	Use:   "baseline",
	Short: "Generate baseline assessment",
	RunE:  runBaseline,
}

var compareCmd = &cobra.Command{
	Use:   "compare [file1] [file2]",
	Short: "Compare two assessment results",
	Args:  cobra.ExactArgs(2),
	RunE:  runCompare,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	runCmd.Flags().StringSliceVar(&models, "models", nil, "models to assess")
	runCmd.Flags().StringVarP(&outputDir, "output", "o", "./assessments", "output directory")

	baselineCmd.Flags().StringSliceVar(&models, "models", nil, "models to assess")
	baselineCmd.Flags().StringVarP(&outputDir, "output", "o", "./assessments", "output directory")

	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(baselineCmd)
	rootCmd.AddCommand(compareCmd)
}

// AssessmentResult holds results for a single model.
type AssessmentResult struct {
	ModelID      string         `json:"model_id"`
	Provider     string         `json:"provider"`
	Timestamp    time.Time      `json:"timestamp"`
	Tests        []TestResult   `json:"tests"`
	Summary      ResultSummary  `json:"summary"`
}

// TestResult holds results for a single test.
type TestResult struct {
	Name        string        `json:"name"`
	Category    string        `json:"category"`
	Prompt      string        `json:"prompt"`
	Response    string        `json:"response"`
	Latency     time.Duration `json:"latency_ms"`
	TokensUsed  int           `json:"tokens_used"`
	Score       float64       `json:"score"`
	Notes       string        `json:"notes,omitempty"`
}

// ResultSummary summarizes assessment results.
type ResultSummary struct {
	TotalTests    int           `json:"total_tests"`
	AverageScore  float64       `json:"average_score"`
	AverageLatency time.Duration `json:"average_latency_ms"`
	TotalTokens   int           `json:"total_tokens"`
}

// StandardTests returns the standard assessment test suite.
func StandardTests() []struct {
	Name     string
	Category string
	Prompt   string
} {
	return []struct {
		Name     string
		Category string
		Prompt   string
	}{
		{
			Name:     "Basic Reasoning",
			Category: "reasoning",
			Prompt:   "If all roses are flowers, and some flowers fade quickly, can we conclude that some roses fade quickly? Explain your reasoning.",
		},
		{
			Name:     "Code Generation",
			Category: "coding",
			Prompt:   "Write a Python function that checks if a string is a palindrome. Include docstring and example usage.",
		},
		{
			Name:     "Creative Writing",
			Category: "creative",
			Prompt:   "Write a haiku about artificial intelligence.",
		},
		{
			Name:     "Fact Recall",
			Category: "knowledge",
			Prompt:   "What is the capital of Australia and what is its approximate population?",
		},
		{
			Name:     "Math Problem",
			Category: "math",
			Prompt:   "A train travels from City A to City B at 60 mph, and returns at 40 mph. What is the average speed for the round trip?",
		},
		{
			Name:     "Summarization",
			Category: "comprehension",
			Prompt:   "Summarize the concept of machine learning in exactly three sentences for a non-technical audience.",
		},
		{
			Name:     "Instruction Following",
			Category: "instruction",
			Prompt:   "List exactly 5 prime numbers between 10 and 50, one per line, with no additional text.",
		},
		{
			Name:     "Ethical Reasoning",
			Category: "ethics",
			Prompt:   "Should autonomous vehicles prioritize passenger safety over pedestrian safety? Provide a balanced analysis.",
		},
	}
}

func loadConfig() (*config.Config, error) {
	if cfgFile != "" {
		return config.Load(cfgFile)
	}

	locations := []string{
		"./config/config.yaml",
		"./config.yaml",
		filepath.Join(os.Getenv("HOME"), ".config/council/config.yaml"),
	}

	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			return config.Load(loc)
		}
	}

	return nil, fmt.Errorf("no config file found")
}

func setupProviders(cfg *config.Config) *provider.Registry {
	registry := provider.NewRegistry()

	registry.Register(provider.NewAnthropicProvider(""))
	registry.Register(provider.NewOpenAIProvider(""))
	registry.Register(provider.NewGoogleProvider(""))
	registry.Register(provider.NewGroqProvider(""))
	registry.Register(provider.NewDeepSeekProvider(""))
	registry.Register(provider.NewMistralProvider(""))
	registry.Register(provider.NewXAIProvider(""))
	registry.Register(provider.NewOllamaProvider("", ""))
	registry.Register(provider.NewLMStudioProvider("", ""))

	registry.RegisterModels(cfg.Models)

	return registry
}

func runAssessment(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	if len(models) == 0 {
		models = cfg.GetCouncilMembers()
	}

	registry := setupProviders(cfg)
	ctx := cmd.Context()

	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println("        AI MODEL ASSESSMENT")
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println()
	fmt.Printf("Models: %s\n", strings.Join(models, ", "))
	fmt.Println()

	tests := StandardTests()
	var results []AssessmentResult

	for _, modelID := range models {
		fmt.Printf("Assessing %s...\n", modelID)

		modelCfg, ok := cfg.GetModel(modelID)
		if !ok {
			fmt.Printf("  Model not found, skipping\n")
			continue
		}

		result := AssessmentResult{
			ModelID:   modelID,
			Provider:  modelCfg.Provider,
			Timestamp: time.Now(),
			Tests:     []TestResult{},
		}

		var totalScore float64
		var totalLatency time.Duration
		var totalTokens int

		for _, test := range tests {
			fmt.Printf("  %s... ", test.Name)

			start := time.Now()
			resp, err := registry.Invoke(ctx, modelID, provider.Request{
				Prompt: test.Prompt,
			})
			latency := time.Since(start)

			if err != nil {
				fmt.Printf("FAILED\n")
				result.Tests = append(result.Tests, TestResult{
					Name:     test.Name,
					Category: test.Category,
					Prompt:   test.Prompt,
					Notes:    err.Error(),
					Score:    0,
				})
				continue
			}

			// Simple scoring based on response length and content
			score := scoreResponse(test.Category, resp.Content)

			testResult := TestResult{
				Name:       test.Name,
				Category:   test.Category,
				Prompt:     test.Prompt,
				Response:   resp.Content,
				Latency:    latency,
				TokensUsed: resp.TokensUsed,
				Score:      score,
			}

			result.Tests = append(result.Tests, testResult)

			totalScore += score
			totalLatency += latency
			totalTokens += resp.TokensUsed

			fmt.Printf("%.1f (%.1fs)\n", score, latency.Seconds())
		}

		result.Summary = ResultSummary{
			TotalTests:     len(result.Tests),
			AverageScore:   totalScore / float64(len(result.Tests)),
			AverageLatency: totalLatency / time.Duration(len(result.Tests)),
			TotalTokens:    totalTokens,
		}

		results = append(results, result)
		fmt.Printf("  Average Score: %.2f\n\n", result.Summary.AverageScore)
	}

	// Save results
	if err := os.MkdirAll(outputDir, 0750); err != nil {
		return err
	}

	filename := filepath.Join(outputDir, fmt.Sprintf("assessment_%s.json",
		time.Now().Format("2006-01-02_15-04-05")))

	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(filename, data, 0640); err != nil {
		return err
	}

	fmt.Printf("Results saved to: %s\n", filename)
	return nil
}

func runBaseline(cmd *cobra.Command, args []string) error {
	// Same as run but saves as baseline
	if err := runAssessment(cmd, args); err != nil {
		return err
	}
	fmt.Println("Baseline assessment complete.")
	return nil
}

func runCompare(cmd *cobra.Command, args []string) error {
	file1, file2 := args[0], args[1]

	data1, err := os.ReadFile(file1)
	if err != nil {
		return fmt.Errorf("reading %s: %w", file1, err)
	}

	data2, err := os.ReadFile(file2)
	if err != nil {
		return fmt.Errorf("reading %s: %w", file2, err)
	}

	var results1, results2 []AssessmentResult
	if err := json.Unmarshal(data1, &results1); err != nil {
		return fmt.Errorf("parsing %s: %w", file1, err)
	}
	if err := json.Unmarshal(data2, &results2); err != nil {
		return fmt.Errorf("parsing %s: %w", file2, err)
	}

	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println("        ASSESSMENT COMPARISON")
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println()

	// Build lookup for results2
	lookup := make(map[string]AssessmentResult)
	for _, r := range results2 {
		lookup[r.ModelID] = r
	}

	for _, r1 := range results1 {
		r2, ok := lookup[r1.ModelID]
		if !ok {
			fmt.Printf("%s: no comparison data\n", r1.ModelID)
			continue
		}

		scoreDiff := r2.Summary.AverageScore - r1.Summary.AverageScore
		latencyDiff := r2.Summary.AverageLatency - r1.Summary.AverageLatency

		indicator := "→"
		if scoreDiff > 0.1 {
			indicator = "↑"
		} else if scoreDiff < -0.1 {
			indicator = "↓"
		}

		fmt.Printf("%s:\n", r1.ModelID)
		fmt.Printf("  Score: %.2f → %.2f (%s%.2f)\n",
			r1.Summary.AverageScore, r2.Summary.AverageScore, indicator, scoreDiff)
		fmt.Printf("  Latency: %.1fs → %.1fs\n",
			r1.Summary.AverageLatency.Seconds(), r2.Summary.AverageLatency.Seconds())
		fmt.Printf("  Latency change: %+.1fs\n\n", latencyDiff.Seconds())
	}

	return nil
}

// scoreResponse provides a simple scoring heuristic.
func scoreResponse(category, response string) float64 {
	// Base score
	score := 5.0

	// Length-based adjustments
	words := len(strings.Fields(response))
	if words < 10 {
		score -= 2
	} else if words > 50 && words < 500 {
		score += 1
	}

	// Category-specific checks
	switch category {
	case "coding":
		if strings.Contains(response, "def ") || strings.Contains(response, "func ") {
			score += 2
		}
		if strings.Contains(response, "```") {
			score += 1
		}
	case "math":
		if strings.Contains(response, "=") || strings.Contains(response, "mph") {
			score += 1
		}
	case "instruction":
		lines := strings.Split(strings.TrimSpace(response), "\n")
		if len(lines) == 5 {
			score += 2
		}
	}

	// Clamp score
	if score < 0 {
		score = 0
	}
	if score > 10 {
		score = 10
	}

	return score
}
