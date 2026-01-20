# FEATURE-PRIORITIZER — PRD Prioritization and Next-Feature Recommendations

## Overview

The Feature Prioritizer analyzes all PRDs in the `docs/prds/` directory and recommends which feature to implement next. It uses Claude to evaluate dependencies, refactoring impact, and codebase state to produce a ranked list of PRDs with detailed reasoning.

The system consists of two parts: a `Prioritizer` component that invokes Claude with a custom agent prompt, and a `choo next-feature` CLI command that surfaces recommendations to the user. PRD frontmatter can include optional `depends_on` hints, but the prioritizer performs its own dependency analysis from PRD content.

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         Feature Prioritizer                              │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   docs/prds/                                                            │
│   ├── auth.md           ──────┐                                         │
│   ├── dashboard.md      ──────┼───▶  Prioritizer  ───▶  PriorityResult  │
│   ├── notifications.md  ──────┤      (Claude)          ├── Recommendations
│   └── settings.md       ──────┘                        ├── DependencyGraph
│                                                        └── Analysis      │
│   specs/                                                                │
│   └── completed/        ───────────▶  (context for what's built)        │
│                                                                         │
│   ┌──────────────┐    ┌──────────────┐    ┌──────────────┐             │
│   │  Load PRDs   │───▶│   Build      │───▶│   Claude     │             │
│   │  & Specs     │    │   Prompt     │    │   Analysis   │             │
│   └──────────────┘    └──────────────┘    └──────────────┘             │
│                                                                         │
│   CLI: choo next-feature                                                │
│   ├── --explain    Show detailed reasoning                              │
│   ├── --top N      Show top N recommendations                           │
│   └── --json       Output as JSON                                       │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

## Requirements

### Functional Requirements

1. Load all PRD markdown files from the specified PRD directory
2. Parse optional `depends_on` hints from PRD YAML frontmatter
3. Load existing specs from `specs/` to understand current codebase state
4. Invoke Claude with the PRD prioritizer agent prompt
5. Return a ranked list of PRDs with priority ordering
6. Include reasoning for each recommendation
7. Generate a dependency graph visualization (ASCII or Mermaid)
8. Support limiting output to top N recommendations (default: 3)
9. Support detailed explanation mode for verbose reasoning
10. Support JSON output for programmatic consumption
11. Handle PRDs with no explicit dependencies (infer from content)
12. Prioritize foundation features (features others depend on) first
13. Prioritize refactoring enablers (features that simplify future work)
14. Deprioritize independent features (can be parallelized later)

### Performance Requirements

| Metric | Target |
|--------|--------|
| PRD loading time (10 PRDs) | <100ms |
| Claude analysis latency | <30s (depends on API) |
| Total command execution | <45s |
| Memory usage | <50MB |

### Constraints

- Depends on Claude API via subagent invocation (Task tool)
- PRD directory must exist and contain at least one markdown file
- Requires `.claude/agents/prd-prioritizer.md` agent definition
- JSON output must be valid and parseable by standard tools
- Must handle malformed PRD frontmatter gracefully (skip invalid PRDs)

## Design

### Module Structure

```
internal/feature/
├── prioritizer.go    # Prioritizer type and Prioritize method
├── prd.go            # PRD loading and frontmatter parsing
└── types.go          # PriorityResult, Recommendation types

internal/cli/
└── next_feature.go   # choo next-feature command

.claude/agents/
└── prd-prioritizer.md  # Claude agent prompt
```

### Core Types

```go
// internal/feature/types.go

// Prioritizer analyzes PRDs and recommends implementation order
type Prioritizer struct {
    prdDir    string
    specsDir  string
    discovery *discovery.Discovery
}

// PriorityResult holds the analysis result
type PriorityResult struct {
    Recommendations []Recommendation `json:"recommendations"`
    DependencyGraph string           `json:"dependency_graph"`
    Analysis        string           `json:"analysis,omitempty"`
}

// Recommendation represents a single PRD recommendation
type Recommendation struct {
    PRDID      string   `json:"prd_id"`
    Title      string   `json:"title"`
    Priority   int      `json:"priority"` // 1 = highest
    Reasoning  string   `json:"reasoning"`
    DependsOn  []string `json:"depends_on"`
    EnablesFor []string `json:"enables_for"` // PRDs that depend on this
}

// PRD represents a loaded Product Requirements Document
type PRD struct {
    ID        string   // filename without extension
    Path      string   // absolute path to file
    Title     string   // extracted from first H1
    Content   string   // full markdown content
    DependsOn []string // from frontmatter (optional hints)
}

// PRDFrontmatter represents optional YAML frontmatter in PRDs
type PRDFrontmatter struct {
    Title     string   `yaml:"title"`
    DependsOn []string `yaml:"depends_on"`
    Status    string   `yaml:"status"` // draft, ready, in_progress, complete
    Priority  string   `yaml:"priority"` // hint: high, medium, low
}
```

```go
// internal/cli/next_feature.go

// NextFeatureOptions holds flags for the next-feature command
type NextFeatureOptions struct {
    PRDDir  string
    Explain bool
    TopN    int
    JSON    bool
}
```

### API Surface

```go
// internal/feature/prioritizer.go

// NewPrioritizer creates a new prioritizer for the given directories
func NewPrioritizer(prdDir, specsDir string) *Prioritizer

// Prioritize analyzes PRDs and returns ranked recommendations
func (p *Prioritizer) Prioritize(ctx context.Context, opts PrioritizeOptions) (*PriorityResult, error)

// PrioritizeOptions controls the prioritization
type PrioritizeOptions struct {
    TopN       int  // Return top N recommendations (default: 3)
    ShowReason bool // Include detailed reasoning
}
```

```go
// internal/feature/prd.go

// LoadPRDs reads all PRD files from the given directory
func LoadPRDs(prdDir string) ([]*PRD, error)

// ParsePRDFrontmatter extracts frontmatter from PRD content
func ParsePRDFrontmatter(content []byte) (*PRDFrontmatter, error)

// ExtractPRDTitle extracts the first H1 heading as title
func ExtractPRDTitle(content []byte) string
```

```go
// internal/cli/next_feature.go

// NewNextFeatureCmd creates the next-feature command
func NewNextFeatureCmd(app *App) *cobra.Command

// RunNextFeature executes the prioritization and displays results
func (a *App) RunNextFeature(ctx context.Context, opts NextFeatureOptions) error
```

### Command Structure

```
choo next-feature [prd-dir]

Analyze PRDs and recommend next feature to implement.

Arguments:
  prd-dir    Path to PRDs directory (default: docs/prds)

Flags:
  --explain         Show detailed reasoning for recommendation
  --top N           Show top N recommendations (default: 3)
  --json            Output as JSON

Examples:
  choo next-feature
  choo next-feature --explain --top 5
  choo next-feature docs/features --json
```

### Agent Prompt Structure

The prioritizer invokes Claude via the Task tool with a custom agent prompt stored in `.claude/agents/prd-prioritizer.md`:

```markdown
# PRD Prioritizer Agent

You analyze PRDs and recommend implementation order based on dependencies and refactoring impact.

## Input

You will receive:
1. A list of PRDs with their content
2. Existing specs showing what has been implemented
3. Current codebase state context

## Analysis Criteria

Evaluate PRDs using these criteria (in order of importance):

1. **Foundation features** - Features that other features depend on
   - Infrastructure, core types, shared utilities
   - Authentication/authorization systems
   - Data models and schemas

2. **Refactoring enablers** - Features that simplify future implementations
   - Abstractions that reduce duplication
   - Patterns that standardize future work
   - Technical debt that blocks progress

3. **Technical debt** - Features that fix or improve existing systems
   - Bug fixes that affect multiple features
   - Performance improvements
   - Security hardening

4. **Independent features** - Features with no dependencies
   - Can be parallelized
   - Lower priority for sequential work
   - Good candidates for delegation

## Dependency Analysis

For each PRD:
1. Check explicit `depends_on` hints in frontmatter (guidance, not authoritative)
2. Analyze content for implicit dependencies:
   - What systems does it require?
   - What APIs does it consume?
   - What data models does it need?
3. Determine what this feature enables for others

## Output Format

Provide your analysis in this exact JSON format:

```json
{
  "recommendations": [
    {
      "prd_id": "feature-name",
      "title": "Human-readable title",
      "priority": 1,
      "reasoning": "Brief explanation of why this is ranked here",
      "depends_on": ["other-feature"],
      "enables_for": ["downstream-feature"]
    }
  ],
  "dependency_graph": "ASCII or Mermaid diagram",
  "analysis": "Overall analysis summary (if --explain)"
}
```
```

### Prioritization Flow

```
Input: docs/prds/ directory path

1. Verify prdDir exists and contains .md files
2. Load all PRD files:
   a. Read file content
   b. Parse optional YAML frontmatter
   c. Extract title from first H1
   d. Build PRD struct
3. Load existing specs from specs/ directory
   a. Identify completed features
   b. Extract current codebase capabilities
4. Build Claude prompt:
   a. Include all PRD content
   b. Include existing spec summaries
   c. Include analysis criteria
5. Invoke Claude via Task tool with subagent_type: "general-purpose"
6. Parse Claude response as JSON
7. Validate response structure
8. Apply TopN limit if specified
9. Return PriorityResult

Output: *PriorityResult with ranked recommendations
```

### Output Formatting

Standard output (default):
```
═══════════════════════════════════════════════════════════════
Feature Prioritizer - Next Feature Recommendations
═══════════════════════════════════════════════════════════════

 #1  [auth] Authentication System
     Foundation feature required by dashboard, settings, notifications

 #2  [dashboard] User Dashboard
     Depends on: auth
     Enables: notifications, analytics

 #3  [settings] User Settings
     Depends on: auth, dashboard
     Independent feature, can be parallelized

───────────────────────────────────────────────────────────────
 Analyzed: 4 PRDs | Completed: 0 | Remaining: 4
═══════════════════════════════════════════════════════════════
```

With `--explain`:
```
═══════════════════════════════════════════════════════════════
Feature Prioritizer - Detailed Analysis
═══════════════════════════════════════════════════════════════

 #1  [auth] Authentication System
     Priority: FOUNDATION

     Reasoning:
     The authentication system is a prerequisite for all user-facing
     features. It establishes the identity model, session management,
     and permission framework that dashboard, settings, and notifications
     all require.

     Dependencies: none
     Enables: dashboard, settings, notifications

───────────────────────────────────────────────────────────────

 #2  [dashboard] User Dashboard
     Priority: ENABLER

     Reasoning:
     The dashboard provides the primary interface for users and
     establishes navigation patterns that other features will follow.
     Building it second allows notifications and analytics to integrate
     naturally.

     Dependencies: auth
     Enables: notifications, analytics

...
```

JSON output (`--json`):
```json
{
  "recommendations": [
    {
      "prd_id": "auth",
      "title": "Authentication System",
      "priority": 1,
      "reasoning": "Foundation feature required by all user-facing features",
      "depends_on": [],
      "enables_for": ["dashboard", "settings", "notifications"]
    }
  ],
  "dependency_graph": "auth -> dashboard -> [settings, notifications]",
  "analysis": "..."
}
```

## Implementation Notes

### PRD Loading

Load PRDs using filepath.Glob and parse frontmatter:

```go
func LoadPRDs(prdDir string) ([]*PRD, error) {
    pattern := filepath.Join(prdDir, "*.md")
    matches, err := filepath.Glob(pattern)
    if err != nil {
        return nil, fmt.Errorf("glob PRDs: %w", err)
    }

    if len(matches) == 0 {
        return nil, fmt.Errorf("no PRD files found in %s", prdDir)
    }

    var prds []*PRD
    for _, path := range matches {
        content, err := os.ReadFile(path)
        if err != nil {
            return nil, fmt.Errorf("read %s: %w", path, err)
        }

        prd := &PRD{
            ID:      strings.TrimSuffix(filepath.Base(path), ".md"),
            Path:    path,
            Content: string(content),
        }

        // Parse optional frontmatter
        if fm, err := ParsePRDFrontmatter(content); err == nil && fm != nil {
            prd.Title = fm.Title
            prd.DependsOn = fm.DependsOn
        }

        // Extract title from H1 if not in frontmatter
        if prd.Title == "" {
            prd.Title = ExtractPRDTitle(content)
        }

        prds = append(prds, prd)
    }

    return prds, nil
}
```

### Claude Invocation

The prioritizer uses the Task tool with the prd-prioritizer agent:

```go
func (p *Prioritizer) Prioritize(ctx context.Context, opts PrioritizeOptions) (*PriorityResult, error) {
    // Load PRDs
    prds, err := LoadPRDs(p.prdDir)
    if err != nil {
        return nil, fmt.Errorf("load PRDs: %w", err)
    }

    // Load existing specs for context
    specs, err := p.loadExistingSpecs()
    if err != nil {
        return nil, fmt.Errorf("load specs: %w", err)
    }

    // Build prompt
    prompt := p.buildPrompt(prds, specs, opts)

    // Invoke Claude via Task tool
    // In actual implementation, this uses the Task tool with subagent_type
    response, err := p.invokeAgent(ctx, prompt)
    if err != nil {
        return nil, fmt.Errorf("invoke agent: %w", err)
    }

    // Parse response
    result, err := p.parseResponse(response)
    if err != nil {
        return nil, fmt.Errorf("parse response: %w", err)
    }

    // Apply TopN limit
    if opts.TopN > 0 && len(result.Recommendations) > opts.TopN {
        result.Recommendations = result.Recommendations[:opts.TopN]
    }

    return result, nil
}
```

### CLI Command Implementation

```go
func NewNextFeatureCmd(app *App) *cobra.Command {
    opts := NextFeatureOptions{
        PRDDir:  "docs/prds",
        Explain: false,
        TopN:    3,
        JSON:    false,
    }

    cmd := &cobra.Command{
        Use:   "next-feature [prd-dir]",
        Short: "Recommend next feature to implement",
        Long: `Analyze PRDs and recommend implementation order based on
dependencies, refactoring impact, and codebase state.

The prioritizer considers:
- Explicit depends_on hints in PRD frontmatter
- Implicit dependencies from PRD content
- Which features enable others (foundation features first)
- Current codebase state from existing specs`,
        Args: cobra.MaximumNArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            if len(args) > 0 {
                opts.PRDDir = args[0]
            }

            ctx := context.Background()
            return app.RunNextFeature(ctx, opts)
        },
    }

    cmd.Flags().BoolVar(&opts.Explain, "explain", false,
        "Show detailed reasoning for recommendations")
    cmd.Flags().IntVar(&opts.TopN, "top", 3,
        "Number of recommendations to show")
    cmd.Flags().BoolVar(&opts.JSON, "json", false,
        "Output as JSON")

    return cmd
}
```

### Response Parsing

Parse Claude's JSON response with validation:

```go
func (p *Prioritizer) parseResponse(response string) (*PriorityResult, error) {
    // Extract JSON from response (may be wrapped in markdown code block)
    jsonStr := extractJSON(response)

    var result PriorityResult
    if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
        return nil, fmt.Errorf("unmarshal response: %w", err)
    }

    // Validate structure
    if len(result.Recommendations) == 0 {
        return nil, fmt.Errorf("no recommendations in response")
    }

    for i, rec := range result.Recommendations {
        if rec.PRDID == "" {
            return nil, fmt.Errorf("recommendation %d missing prd_id", i)
        }
        if rec.Priority <= 0 {
            return nil, fmt.Errorf("recommendation %d has invalid priority", i)
        }
    }

    return &result, nil
}

func extractJSON(s string) string {
    // Handle ```json ... ``` wrapper
    if idx := strings.Index(s, "```json"); idx >= 0 {
        start := idx + 7
        end := strings.Index(s[start:], "```")
        if end > 0 {
            return strings.TrimSpace(s[start : start+end])
        }
    }

    // Try raw JSON
    return strings.TrimSpace(s)
}
```

### Configuration Integration

Add prioritization settings to `.choo.yaml`:

```yaml
# .choo.yaml
prioritization:
  prd_dir: docs/prds
  specs_dir: specs
  default_top_n: 3
```

## Testing Strategy

### Unit Tests

```go
// internal/feature/prd_test.go

func TestLoadPRDs(t *testing.T) {
    tmpDir := t.TempDir()

    // Create test PRDs
    writeFile(t, filepath.Join(tmpDir, "auth.md"), `---
title: Authentication System
depends_on: []
---

# Authentication System

User authentication and authorization.
`)

    writeFile(t, filepath.Join(tmpDir, "dashboard.md"), `---
title: User Dashboard
depends_on: [auth]
---

# User Dashboard

Main user interface.
`)

    prds, err := LoadPRDs(tmpDir)
    if err != nil {
        t.Fatalf("LoadPRDs() error = %v", err)
    }

    if len(prds) != 2 {
        t.Fatalf("LoadPRDs() got %d PRDs, want 2", len(prds))
    }

    // Verify auth PRD
    auth := findPRD(prds, "auth")
    if auth == nil {
        t.Fatal("auth PRD not found")
    }
    if auth.Title != "Authentication System" {
        t.Errorf("auth.Title = %q, want %q", auth.Title, "Authentication System")
    }
    if len(auth.DependsOn) != 0 {
        t.Errorf("auth.DependsOn = %v, want []", auth.DependsOn)
    }

    // Verify dashboard PRD
    dashboard := findPRD(prds, "dashboard")
    if dashboard == nil {
        t.Fatal("dashboard PRD not found")
    }
    if len(dashboard.DependsOn) != 1 || dashboard.DependsOn[0] != "auth" {
        t.Errorf("dashboard.DependsOn = %v, want [auth]", dashboard.DependsOn)
    }
}

func TestLoadPRDs_EmptyDirectory(t *testing.T) {
    tmpDir := t.TempDir()

    _, err := LoadPRDs(tmpDir)
    if err == nil {
        t.Error("LoadPRDs() expected error for empty directory")
    }
}

func TestParsePRDFrontmatter(t *testing.T) {
    tests := []struct {
        name    string
        content string
        want    *PRDFrontmatter
        wantErr bool
    }{
        {
            name: "complete frontmatter",
            content: `---
title: Test Feature
depends_on: [auth, config]
status: ready
priority: high
---

# Content`,
            want: &PRDFrontmatter{
                Title:     "Test Feature",
                DependsOn: []string{"auth", "config"},
                Status:    "ready",
                Priority:  "high",
            },
        },
        {
            name: "no frontmatter",
            content: `# Just a Title

Some content.`,
            want: nil,
        },
        {
            name: "empty frontmatter",
            content: `---
---

# Title`,
            want: &PRDFrontmatter{},
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := ParsePRDFrontmatter([]byte(tt.content))
            if (err != nil) != tt.wantErr {
                t.Errorf("ParsePRDFrontmatter() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if !reflect.DeepEqual(got, tt.want) {
                t.Errorf("ParsePRDFrontmatter() = %+v, want %+v", got, tt.want)
            }
        })
    }
}

func TestExtractPRDTitle(t *testing.T) {
    tests := []struct {
        name    string
        content string
        want    string
    }{
        {
            name:    "simple title",
            content: "# My Feature\n\nContent here.",
            want:    "My Feature",
        },
        {
            name:    "title after frontmatter",
            content: "---\nkey: value\n---\n\n# Feature Title\n\nMore content.",
            want:    "Feature Title",
        },
        {
            name:    "no title",
            content: "Just some text without a heading.",
            want:    "",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := ExtractPRDTitle([]byte(tt.content))
            if got != tt.want {
                t.Errorf("ExtractPRDTitle() = %q, want %q", got, tt.want)
            }
        })
    }
}
```

```go
// internal/feature/prioritizer_test.go

func TestParseResponse(t *testing.T) {
    tests := []struct {
        name     string
        response string
        want     *PriorityResult
        wantErr  bool
    }{
        {
            name: "valid JSON",
            response: `{
  "recommendations": [
    {"prd_id": "auth", "title": "Auth", "priority": 1, "reasoning": "Foundation", "depends_on": [], "enables_for": ["dashboard"]}
  ],
  "dependency_graph": "auth -> dashboard",
  "analysis": "Auth is foundation"
}`,
            want: &PriorityResult{
                Recommendations: []Recommendation{
                    {
                        PRDID:      "auth",
                        Title:      "Auth",
                        Priority:   1,
                        Reasoning:  "Foundation",
                        DependsOn:  []string{},
                        EnablesFor: []string{"dashboard"},
                    },
                },
                DependencyGraph: "auth -> dashboard",
                Analysis:        "Auth is foundation",
            },
        },
        {
            name: "JSON in code block",
            response: "Here's my analysis:\n\n```json\n{\"recommendations\": [{\"prd_id\": \"test\", \"title\": \"Test\", \"priority\": 1, \"reasoning\": \"Test\", \"depends_on\": [], \"enables_for\": []}], \"dependency_graph\": \"test\"}\n```",
            want: &PriorityResult{
                Recommendations: []Recommendation{
                    {PRDID: "test", Title: "Test", Priority: 1, Reasoning: "Test", DependsOn: []string{}, EnablesFor: []string{}},
                },
                DependencyGraph: "test",
            },
        },
        {
            name:     "empty recommendations",
            response: `{"recommendations": [], "dependency_graph": ""}`,
            wantErr:  true,
        },
        {
            name:     "missing prd_id",
            response: `{"recommendations": [{"title": "Test", "priority": 1}]}`,
            wantErr:  true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            p := &Prioritizer{}
            got, err := p.parseResponse(tt.response)
            if (err != nil) != tt.wantErr {
                t.Errorf("parseResponse() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
                t.Errorf("parseResponse() = %+v, want %+v", got, tt.want)
            }
        })
    }
}
```

```go
// internal/cli/next_feature_test.go

func TestNextFeatureOptions_Defaults(t *testing.T) {
    cmd := NewNextFeatureCmd(&App{})

    // Execute with no args to check defaults
    opts := NextFeatureOptions{
        PRDDir:  "docs/prds",
        Explain: false,
        TopN:    3,
        JSON:    false,
    }

    if opts.PRDDir != "docs/prds" {
        t.Errorf("default PRDDir = %q, want %q", opts.PRDDir, "docs/prds")
    }
    if opts.TopN != 3 {
        t.Errorf("default TopN = %d, want %d", opts.TopN, 3)
    }
    if opts.Explain {
        t.Error("default Explain should be false")
    }
    if opts.JSON {
        t.Error("default JSON should be false")
    }
    _ = cmd // verify cmd was created successfully
}
```

### Integration Tests

| Scenario | Setup |
|----------|-------|
| Full prioritization flow | Fixture with 3 PRDs, mock Claude response, verify output |
| Empty PRD directory | Empty docs/prds/, verify helpful error message |
| Invalid PRD frontmatter | PRD with malformed YAML, verify graceful skip |
| JSON output mode | Run with --json, verify valid JSON output |
| TopN limiting | Request top 2 of 5 PRDs, verify only 2 returned |

### Manual Testing

- [ ] `choo next-feature` produces recommendations from docs/prds/
- [ ] `choo next-feature --explain` shows detailed reasoning
- [ ] `choo next-feature --top 5` shows 5 recommendations
- [ ] `choo next-feature --json` outputs valid JSON
- [ ] `choo next-feature custom/path` uses custom PRD directory
- [ ] Missing PRD directory produces clear error message
- [ ] PRDs without frontmatter are handled gracefully

## Design Decisions

### Why Use Claude for Prioritization?

Dependency analysis requires understanding PRD content semantically, not just parsing explicit declarations. Claude can:
1. Infer implicit dependencies from requirements text
2. Assess refactoring impact based on architectural patterns
3. Consider context from existing specs
4. Provide human-readable reasoning

Alternative considered: Rule-based analysis (faster but misses nuance and implicit dependencies).

### Why Optional depends_on in Frontmatter?

PRD authors may not know all dependencies upfront. Making `depends_on` optional:
1. Lowers the barrier to creating PRDs
2. Allows Claude to fill in missing dependencies
3. Treats hints as guidance, not constraints
4. Supports iterative PRD refinement

### Why Default to Top 3 Recommendations?

Three recommendations provide:
1. Clear primary recommendation for immediate action
2. Alternatives if primary is blocked
3. Context for planning without overwhelming output

Users needing full analysis can use `--top N` with a larger N.

### Why JSON Output Option?

JSON output enables:
1. CI/CD integration for automated workflows
2. Piping to other tools (jq, scripts)
3. Programmatic consumption by dashboards
4. Structured data for further analysis

## Future Enhancements

1. Caching of Claude analysis results to avoid repeated API calls
2. Interactive mode to explore dependency graph
3. Integration with GitHub issues to track PRD status
4. Cost estimation based on PRD complexity
5. Historical tracking of priority changes over time
6. Slack/webhook notifications for priority changes
7. Support for PRD templates with standardized sections

## References

- PRD Section 3.1: PRD Prioritizer Agent
- PRD Section 4.1: `choo next-feature` Command
- PRD Section 10 Phase 2: PRD Prioritizer Tasks
- [CLI Spec](specs/completed/CLI.md) - Command structure patterns
- [Discovery Spec](specs/completed/DISCOVERY.md) - Frontmatter parsing patterns
