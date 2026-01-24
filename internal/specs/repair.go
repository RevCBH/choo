package specs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/RevCBH/choo/internal/config"
	"github.com/RevCBH/choo/internal/discovery"
	"github.com/RevCBH/choo/internal/provider"
	"gopkg.in/yaml.v3"
)

var (
	errRepairOutputNotJSON       = errors.New("repair output is not valid JSON")
	errRepairMissingBackpressure = errors.New("repair JSON missing backpressure")
)

// RepairBatchOptions configures concurrent repair processing.
type RepairBatchOptions struct {
	RepoRoot    string
	Config      config.SpecRepairConfig
	Parallelism int
	Verbose     bool
	Output      io.Writer
}

// RepairInvoker abstracts LLM invocation for spec repair.
type RepairInvoker interface {
	Invoke(ctx context.Context, prompt string, workdir string) (string, error)
}

// ProviderInvoker adapts a provider.Provider to RepairInvoker.
type ProviderInvoker struct {
	Provider provider.Provider
	Verbose  bool
	Output   io.Writer
}

// Invoke runs the provider and returns stdout as a string.
func (p ProviderInvoker) Invoke(ctx context.Context, prompt string, workdir string) (string, error) {
	if p.Verbose {
		p.printInvocation(prompt)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := p.Provider.Invoke(ctx, prompt, workdir, &stdout, &stderr); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return "", fmt.Errorf("%w: %s", err, detail)
		}
		return "", err
	}
	return stdout.String(), nil
}

func (p ProviderInvoker) printInvocation(prompt string) {
	out := p.Output
	if out == nil {
		out = os.Stderr
	}

	cmd, args := providerInvocation(p.Provider)
	if cmd == "" {
		fmt.Fprintf(out, "spec repair invoke: <unknown-provider> (prompt_bytes=%d)\n", len(prompt))
		return
	}
	if len(args) == 0 {
		fmt.Fprintf(out, "spec repair invoke: %s (prompt_bytes=%d)\n", cmd, len(prompt))
		return
	}
	fmt.Fprintf(out, "spec repair invoke: %s %s (prompt_bytes=%d)\n", cmd, strings.Join(args, " "), len(prompt))
}

func providerInvocation(prov provider.Provider) (string, []string) {
	switch p := prov.(type) {
	case *provider.ClaudeProvider:
		args := []string{"--dangerously-skip-permissions"}
		if model := p.Model(); model != "" {
			args = append(args, "--model", model)
		}
		args = append(args, "-p", "<prompt>")
		return p.Command(), args
	case *provider.CodexProvider:
		args := []string{"exec", "--yolo"}
		if model := p.Model(); model != "" {
			args = append(args, "--model", model)
		}
		args = append(args, "<prompt>")
		return p.Command(), args
	default:
		return "", nil
	}
}

// Repairer performs LLM-based metadata repair.
type Repairer struct {
	Invoker RepairInvoker
	Timeout time.Duration
	Verbose bool
	Output  io.Writer
}

// NewRepairer constructs a Repairer from configuration.
func NewRepairer(cfg config.SpecRepairConfig) (*Repairer, error) {
	providerType := cfg.Provider
	if providerType == "" {
		providerType = config.DefaultProviderType
	}

	cmd := cfg.Command
	if cmd == "" {
		switch providerType {
		case config.ProviderClaude:
			cmd = config.DefaultClaudeCommand
		case config.ProviderCodex:
			cmd = config.DefaultCodexCommand
		}
	}

	var prov provider.Provider
	switch providerType {
	case config.ProviderClaude:
		prov = provider.NewClaude(cmd)
	case config.ProviderCodex:
		prov = provider.NewCodex(cmd)
	default:
		return nil, fmt.Errorf("unsupported repair provider: %s", providerType)
	}

	if cfg.Model != "" {
		switch p := prov.(type) {
		case *provider.ClaudeProvider:
			p.SetModel(cfg.Model)
		case *provider.CodexProvider:
			p.SetModel(cfg.Model)
		}
	}

	timeout := cfg.TimeoutDuration()

	return &Repairer{
		Invoker: ProviderInvoker{Provider: prov},
		Timeout: timeout,
	}, nil
}

// RepairIssues repairs the provided file issues concurrently.
func RepairIssues(ctx context.Context, opts RepairBatchOptions, issues []FileIssue) ([]RepairResult, error) {
	if len(issues) == 0 {
		return nil, nil
	}

	seen := make(map[string]FileIssue, len(issues))
	var unique []FileIssue
	for _, issue := range issues {
		key := issue.Path
		if opts.RepoRoot != "" && !filepath.IsAbs(key) {
			key = normalizePath(filepath.Join(opts.RepoRoot, key), opts.RepoRoot)
		} else {
			key = normalizePath(key, opts.RepoRoot)
		}
		if _, ok := seen[key]; ok {
			continue
		}
		issue.Path = key
		seen[key] = issue
		unique = append(unique, issue)
	}

	parallelism := opts.Parallelism
	if parallelism <= 0 {
		parallelism = runtime.GOMAXPROCS(0)
	}
	if parallelism < 1 {
		parallelism = 1
	}
	if parallelism > len(unique) {
		parallelism = len(unique)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobs := make(chan FileIssue)
	results := make(chan RepairResult, len(unique))
	errCh := make(chan error, 1)

	sendErr := func(err error) {
		select {
		case errCh <- err:
		default:
		}
	}

	var wg sync.WaitGroup
	for i := 0; i < parallelism; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			repairer, err := NewRepairer(opts.Config)
			if err != nil {
				sendErr(err)
				cancel()
				return
			}
			repairer.SetVerbose(opts.Verbose, opts.Output)

			for issue := range jobs {
				if ctx.Err() != nil {
					return
				}
				path := issue.Path
				if opts.RepoRoot != "" && !filepath.IsAbs(path) {
					path = filepath.Join(opts.RepoRoot, path)
				}
				if _, err := repairer.RepairFile(ctx, path, issue.Kind); err != nil {
					sendErr(fmt.Errorf("%s: %w", issue.Path, err))
					cancel()
					return
				}
				results <- RepairResult{Path: issue.Path, Kind: issue.Kind}
			}
		}()
	}

	go func() {
		defer close(jobs)
		for _, issue := range unique {
			select {
			case <-ctx.Done():
				return
			case jobs <- issue:
			}
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	var repaired []RepairResult
	for res := range results {
		repaired = append(repaired, res)
	}

	select {
	case err := <-errCh:
		return repaired, err
	default:
	}

	sort.Slice(repaired, func(i, j int) bool {
		return repaired[i].Path < repaired[j].Path
	})

	return repaired, nil
}

// SetVerbose enables or disables invocation logging when supported.
func (r *Repairer) SetVerbose(enabled bool, out io.Writer) {
	if r == nil || r.Invoker == nil {
		return
	}
	if inv, ok := r.Invoker.(ProviderInvoker); ok {
		inv.Verbose = enabled
		if out != nil {
			inv.Output = out
		}
		r.Invoker = inv
	}
	r.Verbose = enabled
	if out != nil {
		r.Output = out
	}
}

func (r *Repairer) logDebug(prompt string, output string) {
	if r == nil || !r.Verbose {
		return
	}
	out := r.Output
	if out == nil {
		out = os.Stderr
	}

	fmt.Fprintln(out, "spec repair prompt:\n"+prompt)
	fmt.Fprintln(out, "spec repair response:\n"+strings.TrimSpace(output))
}

// RepairResult captures a repair outcome.
type RepairResult struct {
	Path string
	Kind FileKind
}

// RepairFile repairs a single spec file by rewriting metadata as frontmatter.
func (r *Repairer) RepairFile(ctx context.Context, path string, kind FileKind) (*RepairResult, error) {
	if r.Invoker == nil {
		return nil, fmt.Errorf("repair invoker is required")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	prompt := buildRepairPrompt(kind, path)

	invokeCtx := ctx
	if r.Timeout > 0 {
		var cancel context.CancelFunc
		invokeCtx, cancel = context.WithTimeout(ctx, r.Timeout)
		defer cancel()
	}

	output, err := r.Invoker.Invoke(invokeCtx, prompt, filepathDir(path))
	if err != nil {
		return nil, fmt.Errorf("repair invocation failed: %w", err)
	}

	switch kind {
	case FileKindTask:
		meta, err := parseTaskRepair(output)
		if err != nil {
			if r.retryable(err) {
				if meta, err = r.retryTaskRepair(invokeCtx, path, content); err == nil {
					break
				}
			}
		}
		if err != nil {
			r.logDebug(prompt, output)
			return nil, err
		}
		if err := writeRepairedFile(path, content, meta); err != nil {
			return nil, err
		}
	case FileKindUnit:
		meta, err := parseUnitRepair(output)
		if err != nil {
			if r.retryable(err) {
				if meta, err = r.retryUnitRepair(invokeCtx, path, content); err == nil {
					break
				}
			}
		}
		if err != nil {
			r.logDebug(prompt, output)
			return nil, err
		}
		if err := writeRepairedFile(path, content, meta); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unknown file kind: %s", kind)
	}

	return &RepairResult{Path: path, Kind: kind}, nil
}

func (r *Repairer) retryable(err error) bool {
	return errors.Is(err, errRepairOutputNotJSON) || errors.Is(err, errRepairMissingBackpressure)
}

func (r *Repairer) retryTaskRepair(ctx context.Context, path string, content []byte) (*taskRepairMeta, error) {
	prompt := buildRepairPromptWithPreview(FileKindTask, path, content)
	output, err := r.Invoker.Invoke(ctx, prompt, filepathDir(path))
	if err != nil {
		return nil, fmt.Errorf("repair invocation failed: %w", err)
	}
	meta, err := parseTaskRepair(output)
	if err != nil {
		r.logDebug(prompt, output)
		return nil, err
	}
	return meta, nil
}

func (r *Repairer) retryUnitRepair(ctx context.Context, path string, content []byte) (*unitRepairMeta, error) {
	prompt := buildRepairPromptWithPreview(FileKindUnit, path, content)
	output, err := r.Invoker.Invoke(ctx, prompt, filepathDir(path))
	if err != nil {
		return nil, fmt.Errorf("repair invocation failed: %w", err)
	}
	meta, err := parseUnitRepair(output)
	if err != nil {
		r.logDebug(prompt, output)
		return nil, err
	}
	return meta, nil
}

type taskRepairMeta struct {
	Task         int    `json:"task" yaml:"task"`
	Status       string `json:"status,omitempty" yaml:"status,omitempty"`
	Backpressure string `json:"backpressure" yaml:"backpressure"`
	DependsOn    []int  `json:"depends_on,omitempty" yaml:"depends_on,omitempty"`
}

type unitRepairMeta struct {
	Unit      string   `json:"unit" yaml:"unit"`
	DependsOn []string `json:"depends_on,omitempty" yaml:"depends_on,omitempty"`
}

func parseTaskRepair(output string) (*taskRepairMeta, error) {
	trimmed, err := extractJSONOutput(output)
	if err != nil {
		return nil, err
	}

	var meta taskRepairMeta
	if err := json.Unmarshal([]byte(trimmed), &meta); err != nil {
		return nil, fmt.Errorf("repair JSON invalid: %w", err)
	}

	if meta.Task < 1 {
		return nil, fmt.Errorf("repair JSON missing task")
	}
	if strings.TrimSpace(meta.Backpressure) == "" {
		return nil, errRepairMissingBackpressure
	}
	if meta.Status != "" {
		if err := validateTaskStatus(meta.Status); err != nil {
			return nil, err
		}
	}

	return &meta, nil
}

func parseUnitRepair(output string) (*unitRepairMeta, error) {
	trimmed, err := extractJSONOutput(output)
	if err != nil {
		return nil, err
	}

	var meta unitRepairMeta
	if err := json.Unmarshal([]byte(trimmed), &meta); err != nil {
		return nil, fmt.Errorf("repair JSON invalid: %w", err)
	}

	if strings.TrimSpace(meta.Unit) == "" {
		return nil, fmt.Errorf("repair JSON missing unit")
	}

	return &meta, nil
}

func writeRepairedFile(path string, original []byte, meta any) error {
	body := stripMetadata(original)

	yamlBytes, err := yaml.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshal repaired metadata: %w", err)
	}
	frontmatter := buildFrontmatter(yamlBytes)

	updated := append(frontmatter, body...)
	if err := os.WriteFile(path, updated, 0644); err != nil {
		return fmt.Errorf("write repaired file: %w", err)
	}
	return nil
}

func extractJSONOutput(output string) (string, error) {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return "", errRepairOutputNotJSON
	}
	if strings.HasPrefix(trimmed, "```") {
		fenceEnd := strings.Index(trimmed, "\n")
		if fenceEnd != -1 {
			rest := trimmed[fenceEnd+1:]
			closeIdx := strings.Index(rest, "```")
			if closeIdx != -1 {
				trimmed = strings.TrimSpace(rest[:closeIdx])
			}
		}
	}
	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start == -1 || end <= start {
		return "", errRepairOutputNotJSON
	}
	return strings.TrimSpace(trimmed[start : end+1]), nil
}

func stripMetadata(content []byte) []byte {
	// Remove frontmatter if present and valid.
	if bytes.HasPrefix(content, []byte("---\n")) {
		_, body, err := discovery.ParseFrontmatter(content)
		if err == nil {
			content = body
		}
	}

	// Remove metadata block if valid.
	block, err := discovery.FindMetadataBlock(content)
	if err == nil && block != nil {
		content = discovery.RemoveMetadataBlock(content, block)
	}

	return content
}

func buildRepairPrompt(kind FileKind, path string) string {
	schema := ""
	switch kind {
	case FileKindTask:
		schema = `{"task":1,"status":"pending","backpressure":"go test ./...","depends_on":[1]}`
	case FileKindUnit:
		schema = `{"unit":"unit-name","depends_on":["core"]}`
	default:
		schema = `{}`
	}

	return fmt.Sprintf(`Read the markdown spec at this path and extract metadata. Return ONLY valid JSON matching this schema example:
%s

Rules:
- Output JSON only, no surrounding text
- Do not wrap output in code fences or markdown
- Required fields must be present
- Do not invent backpressure commands; infer or leave empty (invalid) only if absent

Spec path:
%s

Return only the JSON object. No markdown. No code fences.`, schema, path)
}

func buildRepairPromptWithPreview(kind FileKind, path string, content []byte) string {
	schema := ""
	switch kind {
	case FileKindTask:
		schema = `{"task":1,"status":"pending","backpressure":"go test ./...","depends_on":[1]}`
	case FileKindUnit:
		schema = `{"unit":"unit-name","depends_on":["core"]}`
	default:
		schema = `{}`
	}

	lines := strings.Split(string(content), "\n")
	if len(lines) > 60 {
		lines = lines[:60]
	}
	preview := strings.Join(lines, "\n")

	return fmt.Sprintf(`Read the markdown spec at this path and extract metadata. If you cannot access the file, use the preview below. Return ONLY valid JSON matching this schema example:
%s

Rules:
- Output JSON only, no surrounding text
- Do not wrap output in code fences or markdown
- Required fields must be present
- Do not invent backpressure commands; infer or leave empty (invalid) only if absent

Spec path:
%s

Preview (if needed):
%s

Return only the JSON object. No markdown. No code fences.`, schema, path, preview)
}

func filepathDir(path string) string {
	if path == "" {
		return ""
	}
	return filepath.Dir(path)
}
