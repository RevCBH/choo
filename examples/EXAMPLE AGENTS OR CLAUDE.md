# Koe Agent Guidelines

## Specifications

**IMPORTANT:** Before implementing any feature, consult the specifications in `specs/README.md`.

- **Assume NOT implemented.** Many specs describe planned features that may not yet exist in the codebase.
- **Check the codebase first.** Before concluding something is or isn't implemented, search the actual code. Specs describe intent; code describes reality.
- **Use specs as guidance.** When implementing a feature, follow the design patterns, types, and architecture defined in the relevant spec.
- **Spec index:** `specs/README.md` lists all specifications organized by category.

## Ralph Workflow

This project uses a Ralph-style autonomous workflow. See `docs/HOW-TO-RALPH.md` for details.

### Key Principles

1. **Single-concern specs** — Each spec should have ONE clear purpose (no "and")
2. **Atomic tasks** — Implementation plan breaks specs into tasks completable in one iteration
3. **Backpressure** — Tests and checks that must pass before a task is complete
4. **Fresh context** — Each iteration starts clean; read the plan, pick one task, implement, validate, commit

### Running Ralph

```bash
# Execute a workset until completion
./ralph.sh specs/tasks/audio-pipeline/

# Limit to 5 iterations
./ralph.sh specs/tasks/audio-pipeline/ --max-iterations 5

# See current progress
./ralph.sh specs/tasks/audio-pipeline/ --status

# Dry run (show what would happen)
./ralph.sh specs/tasks/audio-pipeline/ --dry-run
```

**Environment variables:**
- `RALPH_AGENT_CMD` — Agent command (default: `claude`)

**State:**
- Progress tracked in `.ralph/{workset}.state`
- Logs written to `.ralph/ralph.log`
- Task logs in `.ralph/task_{N}.log`

### Skills

Custom skills are defined in `.claude/skills/`:

| Skill | Purpose |
|-------|---------|
| `spec.md` | Write technical specifications following project conventions |
| `ralph-prep.md` | Decompose design specs into atomic, Ralph-executable task specs |

## Commands

### Building (Tauri + Cargo)

- **Dev mode:** `cargo tauri dev`
- **Build release:** `cargo tauri build`
- **Build backend only:** `cargo build -p koe`
- **Check backend:** `cargo check -p koe`

### Testing

- **All tests:** `cargo test --workspace`
- **Single crate:** `cargo test -p koe`
- **Specific test:** `cargo test -p koe test_name`
- **With STT (requires model):** `cargo test --features=stt-tests`

### Linting & Formatting

- **Lint:** `cargo clippy --workspace -- -D warnings`
- **Format Rust:** `cargo fmt --all`
- **Format check:** `cargo fmt --all -- --check`

### Frontend

- **Dev server:** `cd src && pnpm dev` (runs via Tauri)
- **Type check:** `pnpm typecheck`
- **Lint:** `pnpm lint`
- **Test:** `pnpm test`

### Full Check (Before Commit)

```bash
cargo fmt --all && cargo clippy --workspace -- -D warnings && cargo test --workspace
```

## Architecture

### Tech Stack

| Layer | Technology |
|-------|------------|
| Desktop Framework | Tauri 2.x |
| Backend | Rust |
| Frontend | React + Tailwind + Vite |
| Database | SQLite (rusqlite) |
| STT | whisper-rs (local, offline) |
| TTS | ElevenLabs API |
| LLM | Claude API (optional, for deck generation) |

### Directory Structure

```
koe/
├── src-tauri/
│   └── src/
│       ├── main.rs           # Tauri entry point
│       ├── commands/         # Tauri command handlers
│       ├── audio/            # Audio pipeline
│       ├── stt/              # Speech-to-text (whisper-rs)
│       ├── tts/              # Text-to-speech (ElevenLabs)
│       ├── db/               # Database layer
│       ├── srs/              # Spaced repetition algorithm
│       └── matching/         # Japanese text matching
├── src/
│   ├── components/           # React components
│   ├── services/             # Tauri invoke wrappers
│   ├── stores/               # State management
│   └── pages/                # Route components
├── specs/
│   ├── design/               # Comprehensive design specs
│   └── tasks/                # Atomic task specs (Ralph-ready)
└── docs/
    └── HOW-TO-RALPH.md       # Ralph workflow documentation
```

### Data Flow

```
┌─────────────────────────────────────────────────────────────┐
│                      Frontend (React)                        │
│  User speaks → MediaRecorder → PCM samples → invoke()       │
└─────────────────────────┬───────────────────────────────────┘
                          │
┌─────────────────────────▼───────────────────────────────────┐
│                      Backend (Rust)                          │
│  Audio → Whisper STT → Match → SRS Update → Response        │
└─────────────────────────────────────────────────────────────┘
```

## Database

### Location

Database file: `~/.koe/koe.db`

### Migrations

Migrations are in `src-tauri/migrations/` as numbered SQL files.

- **Convention:** `NNN_description.sql` (e.g., `001_init.sql`)
- Migrations run automatically on app startup
- Check existing migrations for the next available number

### Key Tables

| Table | Purpose |
|-------|---------|
| `decks` | Card deck metadata |
| `cards` | Flashcards with SRS state |
| `recordings` | Recording metadata (files on disk) |
| `study_sessions` | Study session tracking |
| `reviews` | Individual card reviews |
| `drill_sequences` | JSL-style drill definitions |
| `drill_sessions` | Drill session history |

## Whisper Model

The app requires a Whisper model file (`ggml-large-v3.bin`, ~3.1GB).

- **Location:** `~/.koe/models/ggml-large-v3.bin`
- **Download:** See `MODEL-DOWNLOAD.md` spec
- **First run:** App prompts user to download if missing

### Model Selection Rationale

Large-v3 is chosen for accuracy over speed. For speaking practice, users need to trust that a "miss" is their pronunciation, not model error.

## Japanese Text Handling

### Normalization

All text comparison uses normalized form:
- Katakana → Hiragana
- Punctuation stripped
- Unicode NFC normalized

### Matching Strategy

1. **Exact match** (after normalization) → Correct
2. **Close match** (Levenshtein distance ≤ 2) → Close
3. **No match** → Incorrect

### Multiple Targets

Cards can have multiple acceptable answers:
- Kanji and kana variants: `["食べる", "たべる"]`
- Formal/casual: `["食べます", "食べる"]`
- Particle variations: `["学校へ", "学校え"]`

## Code Style

### Rust

- **Formatting:** Use `rustfmt` defaults
- **Errors:** Use `thiserror` for error enums
- **Async:** Tokio runtime (via Tauri)
- **Naming:** snake_case functions, PascalCase types
- **Imports:** Group std, external, then internal modules

### TypeScript/React

- **Formatting:** Prettier defaults
- **State:** React hooks + context for global state
- **Tauri calls:** Wrap in services under `src/services/`
- **Types:** Define interfaces matching Rust structs

### Comments

Prefer self-documenting code. Add comments only when:
- Logic is non-obvious
- There's a gotcha or platform quirk
- Explaining WHY, not WHAT

## Testing

### Rust Tests

```rust
#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_specific_behavior() {
        // Arrange
        let input = ...;
        // Act
        let result = function(input);
        // Assert
        assert_eq!(result, expected);
    }
}
```

### Property-Based Tests

Use `proptest` for invariants:

```rust
use proptest::prelude::*;

proptest! {
    #[test]
    fn normalized_text_is_idempotent(s in "\\PC+") {
        let once = normalize(&s);
        let twice = normalize(&once);
        prop_assert_eq!(once, twice);
    }
}
```

### Frontend Tests

- Component tests with React Testing Library
- E2E tests for critical flows

## Environment Variables

| Variable | Purpose | Default |
|----------|---------|---------|
| `ELEVENLABS_API_KEY` | TTS API key | Required for TTS |
| `ANTHROPIC_API_KEY` | Claude API key | Optional (deck gen) |
| `KOE_MODEL_PATH` | Override model location | `~/.koe/models/` |
| `KOE_DB_PATH` | Override database location | `~/.koe/koe.db` |

## Tauri Commands

Commands are the API between frontend and backend.

### Convention

```rust
#[tauri::command]
pub async fn verb_noun(
    param: Type,
    state: State<'_, AppState>,
) -> Result<ReturnType, String> {
    // Implementation
}
```

### Registration

Commands must be registered in `main.rs`:

```rust
.invoke_handler(tauri::generate_handler![
    commands::audio::process_recording,
    commands::db::list_decks,
    // ... all commands
])
```

### Argument Naming (camelCase)

**Important:** Tauri 2 automatically converts argument names from camelCase (frontend) to snake_case (Rust). Always use camelCase when calling `invoke()`:

```typescript
// CORRECT - use camelCase in frontend
invoke('list_cards', { deckId: id });
invoke('record_review', { cardId: card.id, matchResult: result });

// WRONG - snake_case will fail
invoke('list_cards', { deck_id: id });  // Error: missing required key deckId
```

Rust commands use snake_case as normal:

```rust
#[tauri::command]
pub fn list_cards(deck_id: String, db: State<'_, Database>) -> Result<Vec<Card>, String>
```

## Common Issues

### "Model not found"

Whisper model not downloaded. Run the model download flow or manually place `ggml-large-v3.bin` in `~/.koe/models/`.

### "Microphone permission denied"

macOS: Check System Preferences → Security & Privacy → Microphone.
App must be granted permission.

### "TTS failed"

Check `ELEVENLABS_API_KEY` is set and valid. API has rate limits; check quota.

### "Database locked"

Only one instance of the app should run. Check for zombie processes.

## Before Submitting Code

1. **Format:** `cargo fmt --all`
2. **Lint:** `cargo clippy --workspace -- -D warnings`
3. **Test:** `cargo test --workspace`
4. **Frontend:** `pnpm typecheck && pnpm lint`
5. **Specs match:** Implementation follows relevant spec
