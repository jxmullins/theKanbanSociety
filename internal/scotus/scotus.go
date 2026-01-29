// Package scotus provides a judicial-style debate mode inspired by the Supreme Court.
package scotus

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jxmullins/thekanbansociety/internal/config"
	"github.com/jxmullins/thekanbansociety/internal/provider"
)

// Opinion represents a justice's opinion type.
type Opinion string

const (
	OpinionMajority    Opinion = "majority"
	OpinionConcurrence Opinion = "concurrence"
	OpinionDissent     Opinion = "dissent"
)

// Vote represents a justice's vote on the resolution.
type Vote struct {
	JusticeID string
	Position  bool   // true = affirm, false = reject
	Opinion   Opinion
	Reasoning string
}

// Resolution holds the formal resolution derived from the topic.
type Resolution struct {
	Original     string   // Original topic
	Formal       string   // Formal yes/no resolution
	KeyDimensions []string // Key aspects to consider
}

// Options configures a SCOTUS session.
type Options struct {
	Topic         string
	Justices      []string
	ChiefJustice  string // Forced CJ, or empty for selection
	Rounds        int
	DeriveResolution bool
	Verbose       bool
	OutputDir     string
}

// Session holds the state of a SCOTUS debate session.
type Session struct {
	ID           string
	Resolution   Resolution
	ChiefJustice string
	Justices     []string
	Votes        []Vote
	MajorityOpinion string
	DissentOpinion  string
	StartTime    time.Time
}

// Runner orchestrates SCOTUS debate sessions.
type Runner struct {
	registry *provider.Registry
	config   *config.Config
}

// NewRunner creates a new SCOTUS runner.
func NewRunner(registry *provider.Registry, cfg *config.Config) *Runner {
	return &Runner{
		registry: registry,
		config:   cfg,
	}
}

// Run executes a SCOTUS debate session.
func (r *Runner) Run(ctx context.Context, opts Options) error {
	session := &Session{
		ID:        fmt.Sprintf("scotus_%d", time.Now().Unix()),
		Justices:  opts.Justices,
		StartTime: time.Now(),
	}

	r.printHeader(opts)

	// Step 1: Derive formal resolution (if enabled)
	if opts.DeriveResolution {
		resolution, err := r.deriveResolution(ctx, opts.Topic)
		if err != nil {
			return fmt.Errorf("deriving resolution: %w", err)
		}
		session.Resolution = resolution
	} else {
		session.Resolution = Resolution{
			Original: opts.Topic,
			Formal:   opts.Topic,
		}
	}

	fmt.Printf("Resolution: %s\n\n", session.Resolution.Formal)

	// Step 2: Select Chief Justice
	cj, err := r.selectChiefJustice(ctx, opts, session)
	if err != nil {
		return fmt.Errorf("selecting Chief Justice: %w", err)
	}
	session.ChiefJustice = cj
	fmt.Printf("Chief Justice: %s\n\n", r.getDisplayName(cj))

	// Step 3: Opening arguments
	if err := r.runOpeningArguments(ctx, opts, session); err != nil {
		return fmt.Errorf("opening arguments: %w", err)
	}

	// Step 4: CJ-led questioning
	if err := r.runCJQuestioning(ctx, opts, session); err != nil {
		return fmt.Errorf("CJ questioning: %w", err)
	}

	// Step 5: Deliberation rounds
	for round := 2; round <= opts.Rounds; round++ {
		if err := r.runDeliberationRound(ctx, opts, session, round); err != nil {
			return fmt.Errorf("deliberation round %d: %w", round, err)
		}
	}

	// Step 6: Collect votes
	if err := r.collectVotes(ctx, opts, session); err != nil {
		return fmt.Errorf("collecting votes: %w", err)
	}

	// Step 7: Write opinions
	if err := r.writeOpinions(ctx, opts, session); err != nil {
		return fmt.Errorf("writing opinions: %w", err)
	}

	// Step 8: Deliver ruling
	r.deliverRuling(session)

	return nil
}

func (r *Runner) printHeader(opts Options) {
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println("        THE COUNCIL OF LEGENDS")
	fmt.Println("           SCOTUS Mode")
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println()
	fmt.Printf("Case: %s\n", opts.Topic)
	fmt.Printf("Justices: %d\n", len(opts.Justices))
	fmt.Println()
}

func (r *Runner) deriveResolution(ctx context.Context, topic string) (Resolution, error) {
	fmt.Println("Deriving formal resolution...")

	// Use first available justice to derive resolution
	prompt := fmt.Sprintf(`Convert this topic into a formal yes/no resolution for judicial review:

Topic: %s

Respond with:
RESOLUTION: [A clear yes/no question that can be ruled upon]
KEY_DIMENSIONS:
- [First key aspect to consider]
- [Second key aspect]
- [Third key aspect]`, topic)

	resp, err := r.registry.Invoke(ctx, "claude", provider.Request{
		Prompt:       prompt,
		SystemPrompt: "You are a legal clerk preparing a case for judicial review.",
	})
	if err != nil {
		return Resolution{}, err
	}

	// Parse response
	resolution := Resolution{Original: topic}
	lines := strings.Split(resp.Content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "RESOLUTION:") {
			resolution.Formal = strings.TrimPrefix(line, "RESOLUTION:")
			resolution.Formal = strings.TrimSpace(resolution.Formal)
		} else if strings.HasPrefix(line, "- ") {
			resolution.KeyDimensions = append(resolution.KeyDimensions,
				strings.TrimPrefix(line, "- "))
		}
	}

	if resolution.Formal == "" {
		resolution.Formal = topic
	}

	fmt.Printf("Formal Resolution: %s\n\n", resolution.Formal)
	return resolution, nil
}

func (r *Runner) selectChiefJustice(ctx context.Context, opts Options, session *Session) (string, error) {
	if opts.ChiefJustice != "" {
		return opts.ChiefJustice, nil
	}

	selector := NewChiefSelector(r.registry, r.config)
	return selector.Select(ctx, session.Resolution.Formal, opts.Justices)
}

func (r *Runner) runOpeningArguments(ctx context.Context, opts Options, session *Session) error {
	fmt.Println("───────────────────────────────────────────────────────")
	fmt.Println("  Opening Arguments")
	fmt.Println("───────────────────────────────────────────────────────")
	fmt.Println()

	for _, justice := range session.Justices {
		fmt.Printf("Justice %s:\n", r.getDisplayName(justice))

		prompt := fmt.Sprintf(`You are a Supreme Court Justice hearing arguments on this resolution:

Resolution: %s

Original case: %s

Present your initial analysis and any questions you would pose to the parties.
Consider constitutional principles, precedents, and practical implications.`,
			session.Resolution.Formal, session.Resolution.Original)

		resp, err := r.registry.Invoke(ctx, justice, provider.Request{
			Prompt:       prompt,
			SystemPrompt: r.getJusticeSystemPrompt(justice),
		})
		if err != nil {
			fmt.Printf("[Failed: %v]\n\n", err)
			continue
		}

		fmt.Println(resp.Content)
		fmt.Println()
	}

	return nil
}

func (r *Runner) runCJQuestioning(ctx context.Context, opts Options, session *Session) error {
	fmt.Println("───────────────────────────────────────────────────────")
	fmt.Println("  Chief Justice Questioning")
	fmt.Println("───────────────────────────────────────────────────────")
	fmt.Println()

	// CJ poses follow-up questions
	prompt := fmt.Sprintf(`As Chief Justice, you've heard opening arguments on:

Resolution: %s

Pose 2-3 pointed questions to guide the deliberation. Focus on:
- Unresolved tensions in the arguments
- Critical precedents or principles at stake
- Practical implications of ruling either way`, session.Resolution.Formal)

	resp, err := r.registry.Invoke(ctx, session.ChiefJustice, provider.Request{
		Prompt:       prompt,
		SystemPrompt: "You are the Chief Justice of the Supreme Court, leading deliberations.",
	})
	if err != nil {
		return err
	}

	fmt.Printf("Chief Justice %s:\n", r.getDisplayName(session.ChiefJustice))
	fmt.Println(resp.Content)
	fmt.Println()

	return nil
}

func (r *Runner) runDeliberationRound(ctx context.Context, opts Options, session *Session, round int) error {
	fmt.Println("───────────────────────────────────────────────────────")
	fmt.Printf("  Deliberation Round %d\n", round)
	fmt.Println("───────────────────────────────────────────────────────")
	fmt.Println()

	for _, justice := range session.Justices {
		fmt.Printf("Justice %s:\n", r.getDisplayName(justice))

		prompt := fmt.Sprintf(`Deliberation round %d on:

Resolution: %s

Respond to the Chief Justice's questions and engage with other justices' positions.
Clarify or refine your position.`, round, session.Resolution.Formal)

		resp, err := r.registry.Invoke(ctx, justice, provider.Request{
			Prompt:       prompt,
			SystemPrompt: r.getJusticeSystemPrompt(justice),
		})
		if err != nil {
			fmt.Printf("[Failed: %v]\n\n", err)
			continue
		}

		fmt.Println(resp.Content)
		fmt.Println()
	}

	return nil
}

func (r *Runner) collectVotes(ctx context.Context, opts Options, session *Session) error {
	fmt.Println("───────────────────────────────────────────────────────")
	fmt.Println("  Collecting Votes")
	fmt.Println("───────────────────────────────────────────────────────")
	fmt.Println()

	for _, justice := range session.Justices {
		prompt := fmt.Sprintf(`Cast your vote on the resolution:

Resolution: %s

Respond with:
VOTE: AFFIRM or REJECT
REASONING: [One sentence explaining your vote]`, session.Resolution.Formal)

		resp, err := r.registry.Invoke(ctx, justice, provider.Request{
			Prompt:       prompt,
			SystemPrompt: r.getJusticeSystemPrompt(justice),
		})
		if err != nil {
			continue
		}

		vote := Vote{JusticeID: justice}
		lines := strings.Split(resp.Content, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "VOTE:") {
				voteStr := strings.ToUpper(strings.TrimSpace(strings.TrimPrefix(line, "VOTE:")))
				vote.Position = strings.Contains(voteStr, "AFFIRM")
			} else if strings.HasPrefix(line, "REASONING:") {
				vote.Reasoning = strings.TrimSpace(strings.TrimPrefix(line, "REASONING:"))
			}
		}

		session.Votes = append(session.Votes, vote)

		position := "REJECT"
		if vote.Position {
			position = "AFFIRM"
		}
		fmt.Printf("Justice %s: %s\n", r.getDisplayName(justice), position)
	}

	fmt.Println()
	return nil
}

func (r *Runner) writeOpinions(ctx context.Context, opts Options, session *Session) error {
	fmt.Println("───────────────────────────────────────────────────────")
	fmt.Println("  Writing Opinions")
	fmt.Println("───────────────────────────────────────────────────────")
	fmt.Println()

	// Count votes
	affirm := 0
	for _, v := range session.Votes {
		if v.Position {
			affirm++
		}
	}
	majorityPosition := affirm > len(session.Votes)/2

	// Assign opinions
	for i := range session.Votes {
		if session.Votes[i].Position == majorityPosition {
			if session.Votes[i].JusticeID == session.ChiefJustice {
				session.Votes[i].Opinion = OpinionMajority
			} else {
				session.Votes[i].Opinion = OpinionConcurrence
			}
		} else {
			session.Votes[i].Opinion = OpinionDissent
		}
	}

	// CJ writes majority opinion
	fmt.Println("Chief Justice writing majority opinion...")

	majorityPrompt := fmt.Sprintf(`Write the majority opinion of the Court.

Resolution: %s
Vote: %d-%d to %s

Include:
1. Statement of the case
2. Legal reasoning
3. Holding
4. Practical implications`,
		session.Resolution.Formal,
		affirm, len(session.Votes)-affirm,
		map[bool]string{true: "AFFIRM", false: "REJECT"}[majorityPosition])

	majorityResp, err := r.registry.Invoke(ctx, session.ChiefJustice, provider.Request{
		Prompt:       majorityPrompt,
		SystemPrompt: "You are writing the majority opinion for the Supreme Court.",
	})
	if err != nil {
		return err
	}
	session.MajorityOpinion = majorityResp.Content

	// Find dissenter to write dissent
	for _, v := range session.Votes {
		if v.Opinion == OpinionDissent {
			fmt.Printf("Justice %s writing dissent...\n", r.getDisplayName(v.JusticeID))

			dissentPrompt := fmt.Sprintf(`Write the dissenting opinion.

Resolution: %s
You voted to %s while the majority voted to %s.

Explain why you disagree with the majority's reasoning.`,
				session.Resolution.Formal,
				map[bool]string{true: "AFFIRM", false: "REJECT"}[v.Position],
				map[bool]string{true: "AFFIRM", false: "REJECT"}[majorityPosition])

			dissentResp, err := r.registry.Invoke(ctx, v.JusticeID, provider.Request{
				Prompt:       dissentPrompt,
				SystemPrompt: "You are writing a dissenting opinion for the Supreme Court.",
			})
			if err == nil {
				session.DissentOpinion = dissentResp.Content
			}
			break
		}
	}

	return nil
}

func (r *Runner) deliverRuling(session *Session) {
	// Count final vote
	affirm := 0
	for _, v := range session.Votes {
		if v.Position {
			affirm++
		}
	}

	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println("           THE RULING OF THE COURT")
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println()

	ruling := "REJECTED"
	if affirm > len(session.Votes)/2 {
		ruling = "AFFIRMED"
	}

	fmt.Printf("Resolution: %s\n\n", session.Resolution.Formal)
	fmt.Printf("Decision: %s (%d-%d)\n\n", ruling, affirm, len(session.Votes)-affirm)

	fmt.Println("MAJORITY OPINION:")
	fmt.Println(session.MajorityOpinion)
	fmt.Println()

	if session.DissentOpinion != "" {
		fmt.Println("DISSENTING OPINION:")
		fmt.Println(session.DissentOpinion)
	}

	fmt.Println()
	fmt.Printf("Duration: %s\n", time.Since(session.StartTime).Round(time.Second))
}

func (r *Runner) getJusticeSystemPrompt(justiceID string) string {
	return fmt.Sprintf(`You are Justice %s of the Supreme Court.
Approach each case with careful legal analysis, considering:
- Constitutional principles and precedents
- The practical implications of your ruling
- Balancing competing interests and rights

Be thoughtful, precise, and principled in your reasoning.`, r.getDisplayName(justiceID))
}

func (r *Runner) getDisplayName(aiID string) string {
	if modelCfg, ok := r.config.GetModel(aiID); ok && modelCfg.DisplayName != "" {
		return modelCfg.DisplayName
	}
	return aiID
}
