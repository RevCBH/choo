# Spec Writing Skill

**Description**: Create or edit technical specifications following the established style and structure for Ralph-driven projects.

**When to use**: When the user asks to create a new spec, update an existing spec, or write documentation for a technical system/component.

---

## Spec Style Guide

### Core Principles

1. **Clarity over cleverness**: Write for engineers who need to implement the spec
2. **Concrete over abstract**: Show actual code, not just concepts
3. **Complete but concise**: Include enough detail to implement, but stay focused
4. **Structured consistency**: Follow the template sections in order
5. **Latest stable versions**: Always use current stable versions of tools and libraries

### Versioning Policy

**Always use the latest stable version** of tools, libraries, and frameworks when writing specs.

- Check current stable versions before specifying dependencies (use web search if needed)
- Prefer `^major.minor` semver ranges over exact pinned versions
- Never copy old version numbers from other specs without verifying they're current
- When in doubt, omit version numbers and note "use latest stable"

This prevents technical debt from day one and ensures implementations use modern APIs.

### Writing Tone

- **Technical and precise**, but conversational
- Avoid marketing language ("amazing", "powerful", "revolutionary")
- Use active voice ("The system converts audio" not "Audio is converted")
- Be direct and unambiguous
- It's okay to say "we" when discussing design decisions

---

## Spec Template Structure

Every spec should follow this structure (omit sections that don't apply):

```markdown
# COMPONENT-NAME — Brief Description

## Overview

[2-3 paragraphs explaining WHAT this component does and WHY it exists]

[Include an ASCII architecture diagram if relevant]

## Requirements

### Functional Requirements

- Numbered list of features this component must provide
- Each should be testable/verifiable

### Performance Requirements

| Metric | Target |
|--------|--------|
| [Measurement] | [Specific value] |

### Constraints

- Platform limitations
- Dependencies on other systems
- Technical restrictions

## Design

### Module Structure / Crate Structure / File Structure

```
[Show directory tree or module organization]
```

### Core Types

```go
// For Go projects: Show the key structs and interfaces
// Include field-level comments explaining non-obvious choices
```

```rust
// For Rust projects: Show the key structs, enums, traits
// Include field-level comments explaining non-obvious choices
```

### API Surface

```go
// For Go: Public functions and methods
// Show signatures and brief descriptions
```

```rust
// For Rust: Public functions and Tauri commands
// Show signatures and brief descriptions
```

### [Custom sections as needed]

- Data flow diagrams
- Protocol mappings
- State machines
- Algorithms

## Implementation Notes

### [Subsection for each gotcha/consideration]

- Platform-specific issues
- Performance considerations
- Edge cases
- Security concerns
- Memory management

## Testing Strategy

### Unit Tests

```go
// For Go projects
func TestSpecificBehavior(t *testing.T) {
    // Show example test structure
}
```

```rust
// For Rust projects
#[cfg(test)]
mod tests {
    // Show example test structure
}
```

### Integration Tests

[List key integration scenarios]

### Manual Testing

- [ ] Checklist of things to verify manually

## Design Decisions

### Why [Decision]?

[Explain rationale for non-obvious choices]
[Include trade-offs considered]

### Why [Alternative] Not Chosen?

[When alternatives existed, explain why this path was chosen]

## Future Enhancements

1. [Feature/improvement not in initial scope]
2. [Potential extension points]

## References

- [Link to relevant docs]
- [Related specs]
- [External resources]
```

---

## Code Examples Guidelines

### Go Code

```go
// ✅ GOOD: Complete, contextual, runnable
type Worker struct {
    ID       string
    Unit     *Unit
    Worktree string
    Events   *EventBus
}

func NewWorker(unit *Unit, worktreeBase string, events *EventBus) (*Worker, error) {
    worktree := filepath.Join(worktreeBase, unit.ID)
    return &Worker{
        ID:       fmt.Sprintf("worker-%s", unit.ID),
        Unit:     unit,
        Worktree: worktree,
        Events:   events,
    }, nil
}

// Execute runs the Ralph loop for this unit
func (w *Worker) Execute(ctx context.Context) error {
    w.Events.Emit(Event{Type: UnitStarted, Unit: w.Unit.ID})
    // ... implementation
}
```

```go
// ❌ BAD: Abstract, incomplete, pseudocode
type Worker struct {
    // ... fields
}

func (w *Worker) Execute() {
    // ... does work
}
```

### Rust Code

```rust
// ✅ GOOD: Complete, contextual, runnable
pub struct AudioCache {
    base_dir: PathBuf,
    retention_count: usize,
}

impl AudioCache {
    pub fn new(base_dir: PathBuf, retention_count: usize) -> Self {
        Self { base_dir, retention_count }
    }

    /// Get path for card's reference audio
    pub fn reference_path(&self, card_id: &str) -> PathBuf {
        self.base_dir.join(card_id).join("reference.mp3")
    }
}
```

```rust
// ❌ BAD: Abstract, incomplete, pseudocode
pub struct Cache {
    // ... fields
}

impl Cache {
    // ... methods for getting paths
}
```

### TypeScript/Frontend Code

```typescript
// ✅ GOOD: Shows actual integration
import { invoke } from '@tauri-apps/api/tauri';

export class AudioRecorder {
  async processRecording(cardId: string, samples: Float32Array): Promise<AudioResult> {
    const input: AudioInput = {
      samples: Array.from(samples),
      sample_rate: 16000,
      channels: 1,
    };

    return invoke('process_recording', { cardId, audio: input });
  }
}
```

### SQL Schema

```sql
-- ✅ GOOD: Complete with constraints and indexes
CREATE TABLE IF NOT EXISTS recordings (
    id TEXT PRIMARY KEY,
    card_id TEXT NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
    file_path TEXT NOT NULL,
    transcription TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_recordings_card ON recordings(card_id, created_at DESC);
```

---

## ASCII Diagrams

Use ASCII art for:
- System architecture
- Data flow
- State machines
- Request/response sequences

### Architecture Diagram Style

```
┌─────────────────────────────────────────────────┐
│                   Frontend                       │
│            React + Tailwind + Vite              │
└─────────────────────┬───────────────────────────┘
                      │ invoke() / events
┌─────────────────────▼───────────────────────────┐
│                 Tauri Backend                    │
│                    (Rust)                        │
├─────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────┐ │
│  │ whisper-rs  │  │   SQLite    │  │   FS    │ │
│  └─────────────┘  └─────────────┘  └─────────┘ │
└─────────────────────────────────────────────────┘
```

### State Machine Style

```
                    ┌─────────┐
                    │  Idle   │
                    └────┬────┘
                         │ event
                         ▼
                    ┌─────────┐
          ┌────────│ Active  │────────┐
          │        └────┬────┘        │
          │             │             │
          │    success  │  error      │
          │             ▼             │
          │        ┌─────────┐        │
          └───────▶│Complete │◀───────┘
                   └─────────┘
```

### Sequence Diagram Style

```
Client              Server              Database
  │                    │                    │
  │ ─── request ──────►│                    │
  │                    │ ── query ─────────►│
  │                    │◄── result ─────────│
  │◄─── response ──────│                    │
```

---

## Tables

Use tables for:
- Feature comparisons
- Protocol/API mappings
- Performance targets
- Status tracking

### Comparison Table

| Feature | Option A | Option B |
|---------|----------|----------|
| Cost | Low | High |
| Speed | Fast | Faster |
| **Choice** | ✓ | |

### Mapping Table

| ACP Method | Loom Action |
|------------|-------------|
| `initialize` | Return agent info and capabilities |
| `session/new` | Create new Thread |

### Performance Table

| Metric | Target | Actual |
|--------|--------|--------|
| Latency | <100ms | 45ms ✓ |
| Throughput | 1000 req/s | 850 req/s |

---

## Section-Specific Guidelines

### Overview Section

- Start with 1-2 sentence summary of what the component is
- Explain the problem it solves
- Show how it fits into the larger system
- Include architecture diagram if helpful

**Example**:
```markdown
## Overview

The audio pipeline handles all audio capture, processing, and playback in Koe.
It bridges the frontend Web Audio API with the Rust backend, managing recording
sessions, format conversion, and file I/O.

[ASCII diagram showing flow]
```

### Requirements Section

**Functional Requirements**: List capabilities as numbered items
- Use active verbs: "Capture audio", "Convert formats", "Play recordings"
- Each should be verifiable in testing

**Performance Requirements**: Use table format with specific metrics
- Include units (ms, MB, requests/sec)
- Make targets realistic but measurable

**Constraints**: List technical or architectural limitations
- Platform issues
- Dependencies
- External system requirements

### Design Section

**Start with structure**: Show file/module organization first

**Then types**: Show the core data structures
- Include all fields with types
- Add comments explaining non-obvious choices
- Use Rust syntax even if not Rust (shows precision)

**Then API**: Show public functions/commands
- Include signatures
- Brief description of each
- Show actual usage patterns

**Custom subsections**: Add as needed:
- "Protocol Mapping" for network protocols
- "State Machine" for stateful components
- "Data Flow" for pipelines
- "Evaluation Strategy" for algorithms

### Implementation Notes

This is where you explain the gotchas:
- Platform-specific behavior
- Performance considerations
- Edge cases to watch for
- Security concerns
- Memory management strategies

Use subsections with clear titles:
```markdown
### Platform Considerations

**macOS**: Requires permission in Info.plist...
**Windows**: Handle device enumeration...
**Linux**: Requires PulseAudio...

### Memory Management

- Large buffers should be released promptly
- Consider streaming for recordings >30s
```

### Design Decisions

Explain **why** choices were made, especially when:
- Multiple valid approaches existed
- The choice has significant trade-offs
- It's not obvious from the code
- Future maintainers might question it

Format as Q&A:
```markdown
### Why Local Tool Execution?

Tools execute locally rather than through ACP callbacks because:
- Simpler implementation
- Better performance (no round-trip)
- Consistent with REPL mode
- Loom always has filesystem access

If sandboxing becomes needed, we can refactor to abstract FS layer.
```

### Testing Strategy

Show **concrete examples**, not just "write tests"

**Unit tests**: Show example test structure
```rust
#[test]
fn test_specific_behavior() {
    // Arrange
    let input = ...;
    // Act
    let result = ...;
    // Assert
    assert_eq!(result, expected);
}
```

**Integration tests**: List key scenarios as prose

**Manual testing**: Checklist format
```markdown
- [ ] Permission prompt appears
- [ ] Hold-to-record responds immediately
- [ ] Audio plays correctly
```

---

## Anti-Patterns to Avoid

### ❌ Too Abstract

```markdown
## Design

The system uses a modular architecture with clear separation of concerns.
Components communicate through well-defined interfaces.
```

**Problem**: No concrete information. What modules? What interfaces?

### ✅ Concrete Instead

```markdown
## Design

### Module Structure

```
src-tauri/src/
├── audio/
│   ├── recorder.rs    # Recording state management
│   └── cache.rs       # File storage and pruning
```

### API Surface

```rust
#[tauri::command]
pub async fn process_recording(
    card_id: String,
    audio: AudioInput,
) -> Result<AudioResult, String>
```
```

---

### ❌ Missing Context

```rust
pub fn convert(input: &[f32]) -> Vec<f32> {
    // implementation
}
```

**Problem**: What format is input? What format is output? Why convert?

### ✅ Context Included

```rust
/// Convert arbitrary audio input to Whisper-compatible format (16kHz mono)
pub fn convert_for_whisper(input: AudioInput) -> WhisperAudio {
    let samples = if input.channels > 1 {
        mix_to_mono(&input.samples, input.channels as usize)
    } else {
        input.samples
    };
    // ...
}
```

---

### ❌ Vague Requirements

```markdown
- The system should be fast
- Error handling should be robust
```

**Problem**: Not measurable or actionable.

### ✅ Specific Requirements

```markdown
- Transcription completes in <2s for 5-second audio clips
- All errors return structured error types with context
- Network errors trigger automatic retry with exponential backoff
```

---

## Checklist Before Finalizing

Use this checklist when writing or reviewing a spec:

**Structure**
- [ ] Follows template sections in order
- [ ] Each section has substantive content (or is omitted if not applicable)
- [ ] Headers use consistent levels (## for main, ### for sub)

**Content**
- [ ] Overview explains what, why, and how it fits
- [ ] Requirements are specific and testable
- [ ] Code examples are complete and realistic
- [ ] ASCII diagrams are clear and properly formatted
- [ ] Design decisions are explained with rationale

**Code Quality**
- [ ] Rust code uses proper syntax (even if incomplete implementation)
- [ ] All types have clear field names and comments
- [ ] API surface shows actual function signatures
- [ ] Examples show end-to-end usage, not fragments

**Clarity**
- [ ] Technical terms are used correctly
- [ ] No marketing language or vague adjectives
- [ ] Tables are formatted consistently
- [ ] Links work and point to relevant resources

**Completeness**
- [ ] Platform-specific considerations noted
- [ ] Edge cases and gotchas documented
- [ ] Testing approach defined
- [ ] Future enhancements listed if relevant

---

## Examples of Excellent Specs

### Excellent Spec Structure

**What makes it good:**
- Clear "Core principle" statement upfront
- Architecture explained with code
- Complete state machine diagrams
- Comprehensive "Open Design Decisions" section
- Concrete user experience flow

### Excellent Code Examples

**What makes them good:**
- Complete type implementations
- Realistic test cases with assertions
- Integration shown end-to-end
- Platform-specific notes in Implementation section

### Excellent Architecture

**What makes it good:**
- Stack diagram shows all layers
- Data schema with constraints
- Acceptance criteria for requirements
- MVP scope broken into phases
- Dependencies listed with versions

---

## Special Case: Updates to Existing Specs

When modifying an existing spec:

1. **Read the entire spec first** to understand context
2. **Match the existing tone and style** - don't introduce inconsistencies
3. **Update all affected sections** - if you change Requirements, check if Design needs updates too
4. **Preserve good examples** - don't delete working code samples
5. **Mark status changes** - if spec was "Planned" and you implemented it, mark "Implemented"

---

## Interaction Protocol

When the user asks you to create or edit a spec:

1. **Clarify scope**: "Should this be a standalone spec or part of [existing spec]?"
2. **Check for references**: Read related specs in the project to match style
3. **Ask about depth**: "Should this be high-level (like README) or detailed (like JSL-DRILLS)?"
4. **Present outline first** if the spec is large: "Here's the structure I'm planning..."
5. **Write in one pass**: Complete the full spec, don't do it section by section
6. **Offer to update README**: "Should I add this to the spec index in README.md?"

---

## Quick Reference: Section Templates

### Data Structure Template (Go)

```go
// TypeName represents [brief description]
type TypeName struct {
    // FieldName is [field description - when it's not obvious]
    FieldName FieldType `json:"field_name"`

    // AnotherField [explain design choice if non-standard]
    AnotherField ComplexType `json:"another_field,omitempty"`
}
```

### Data Structure Template (Rust)

```rust
/// [Brief description of what this represents]
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TypeName {
    /// [Field description - when it's not obvious]
    pub field_name: FieldType,

    /// [Explain design choice if non-standard]
    pub another_field: ComplexType,
}
```

### API Function Template (Go)

```go
// FunctionName [what this function does]
//
// Parameters:
//   - param: [description if non-obvious]
//
// Returns [what gets returned and why] or error [when this function fails]
func FunctionName(param Type) (ReturnType, error) {
    // Implementation
}
```

### API Function Template (Rust)

```rust
/// [What this function does]
///
/// # Arguments
/// * `param` - [Description if non-obvious]
///
/// # Returns
/// [What gets returned and why]
///
/// # Errors
/// [When this function fails]
pub fn function_name(param: Type) -> Result<ReturnType, Error> {
    // Implementation
}
```

### Table Template

| Column A | Column B | Column C |
|----------|----------|----------|
| Value 1  | Value 2  | Value 3  |

### Diagram Template

```
┌──────────┐         ┌──────────┐
│ Component│────────►│Component │
│    A     │  label  │    B     │
└──────────┘         └──────────┘
```

---

## Final Note

**When in doubt, look at existing specs in the project.** The best guide for style is the established pattern. This skill codifies those patterns, but the actual specs in the project's `specs/` directory are the source of truth.
