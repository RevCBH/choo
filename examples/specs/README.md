# Koe Specifications

Design documentation for Koe (声), a Japanese speaking practice tool built with Tauri.

## Versioning Policy

**Always use the latest stable version** of tools, libraries, and frameworks when writing specs. Do not pin to old versions unless there's a specific compatibility reason documented.

- Check current stable versions before specifying dependencies
- Prefer `^major.minor` semver ranges over exact versions
- When a spec references outdated versions, update them during implementation

This keeps the codebase modern and avoids accumulating technical debt from day one.

---

## Architecture

| Spec | Code | Purpose |
|------|------|---------|
| [PROJECT-SETUP.md](./PROJECT-SETUP.md) | [src-tauri/](../src-tauri/) | Tauri scaffolding, dev environment, testing infra |

## User Interface

| Spec | Code | Purpose |
|------|------|---------|
| [APP-SHELL.md](./APP-SHELL.md) | `src/components/layout/` | Navigation, routing, main layout |
| [STARTUP-FLOW.md](./STARTUP-FLOW.md) | `src/pages/SetupPage.tsx` | First-run experience, model download check |
| [DECK-LIST.md](./DECK-LIST.md) | `src/pages/DeckListPage.tsx` | Home page, deck grid, create deck |
| [DECK-VIEW.md](./DECK-VIEW.md) | `src/pages/DeckViewPage.tsx` | Single deck view, card list, filters |
| [CARD-EDITOR.md](./CARD-EDITOR.md) | `src/components/cards/` | Create/edit card modal and form |
| [DRILL-SELECTION.md](./DRILL-SELECTION.md) | `src/pages/DrillSelectionPage.tsx` | Browse drills by difficulty |

## Audio & Speech

| Spec | Code | Purpose |
|------|------|---------|
| [AUDIO-PIPELINE.md](./AUDIO-PIPELINE.md) | `src-tauri/src/audio/` | Recording, playback, format conversion, caching |
| [STT-ENGINE.md](./STT-ENGINE.md) | `src-tauri/src/stt/` | Whisper integration for speech-to-text |
| [TTS-SERVICE.md](./TTS-SERVICE.md) | `src-tauri/src/tts/` | ElevenLabs integration for reference audio |
| [MATCHING.md](./MATCHING.md) | `src-tauri/src/matching/` | Japanese text normalization and answer matching |

## Data Layer

| Spec | Code | Purpose |
|------|------|---------|
| [DATABASE.md](./DATABASE.md) | `src-tauri/src/db/` | SQLite schema, migrations, repositories |
| [SRS-ALGORITHM.md](./SRS-ALGORITHM.md) | `src-tauri/src/srs/` | SM-2 spaced repetition scheduling |

## Practice Modes

| Spec | Code | Purpose |
|------|------|---------|
| [CARD-PRACTICE.md](./CARD-PRACTICE.md) | `src/pages/Practice.tsx` | Translation and reading card review flow |
| [JSL-DRILLS.md](./JSL-DRILLS.md) | `src/pages/Drills.tsx` | Rapid-fire conversational drill patterns |

## Content & Configuration

| Spec | Code | Purpose |
|------|------|---------|
| [DECK-GENERATION.md](./DECK-GENERATION.md) | `src-tauri/src/generation/` | Claude API integration for card generation |
| [CONFIG.md](./CONFIG.md) | `src-tauri/src/config/` | User settings, API keys, preferences |
| [MODEL-DOWNLOAD.md](./MODEL-DOWNLOAD.md) | `src-tauri/src/model/` | Whisper model acquisition and verification |

---

## Architecture Overview

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
│  │    STT      │  │  (rusqlite) │  │  Audio  │ │
│  └─────────────┘  └─────────────┘  └─────────┘ │
├─────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐              │
│  │ ElevenLabs  │  │   Claude    │              │
│  │    TTS      │  │     API     │              │
│  └─────────────┘  └─────────────┘              │
└─────────────────────────────────────────────────┘
```

## Key Design Decisions

### Local-First STT

Speech recognition runs entirely on-device via whisper-rs. This provides:
- No per-request API costs
- Low latency (<2s for short phrases)
- Privacy (audio never leaves the device)
- Offline capability

**Trade-off:** Requires ~3GB model download on first launch.

### Intelligibility Over Accent

Core thesis: if Whisper can transcribe what you said, your pronunciation is communicatively effective. We don't grade pitch accent or native-like prosody—we verify that you would be understood.

### SRS for Production

Unlike Anki which tests recognition, Koe tests production. Can you *say* it, not just recognize it?

---

## Status Legend

| Status | Meaning |
|--------|---------|
| Draft | Initial spec, subject to change |
| Review | Spec complete, awaiting review |
| Approved | Finalized, ready for implementation |
| Implemented | Code complete, links to source added |

All specs are currently **Draft** status.

---

## Dependency Graph

Specs must be implemented in dependency order. Each spec declares its dependencies in a `## Dependencies` YAML block.

```
                         ┌───────────────┐
                         │ PROJECT-SETUP │  Layer 0: Foundation
                         └───────┬───────┘
                                 │
          ┌──────────────────────┼──────────────────────┐
          │                      │                      │
          ▼                      ▼                      ▼
    ┌──────────┐          ┌──────────┐          ┌──────────────┐
    │  CONFIG  │          │ DATABASE │          │MODEL-DOWNLOAD│  Layer 1
    └────┬─────┘          └────┬─────┘          └──────┬───────┘
         │                     │                       │
         │    ┌────────────────┼───────────────┐      │
         │    │                │               │      │
         ▼    ▼                │               │      ▼
    ┌────────────────┐         │               │ ┌──────────────┐
    │ AUDIO-PIPELINE │         │               │ │  STT-ENGINE  │  Layer 2
    └───────┬────────┘         │               │ └──────┬───────┘
            │                  │               │        │
            └──────────────────┼───────────────┼────────┘
                               │               │
            ┌──────────────────┼───────────────┤
            │                  │               │
            ▼                  ▼               ▼
      ┌──────────┐       ┌─────────────┐ ┌──────────────┐
      │ MATCHING │       │TTS-SERVICE  │ │SRS-ALGORITHM │  Layer 3
      └────┬─────┘       └──────┬──────┘ └──────┬───────┘
           │                    │               │
           └────────────────────┼───────────────┘
                                │
             ┌──────────────────┼──────────────────┐
             │                  │                  │
             ▼                  ▼                  ▼
       ┌───────────┐     ┌───────────┐     ┌────────────────┐
       │CARD-PRACT.│     │JSL-DRILLS │     │DECK-GENERATION │  Layer 4: Backend
       └─────┬─────┘     └─────┬─────┘     └────────────────┘
             │                 │
             └────────┬────────┘
                      │
                      ▼
       ┌─────────────────────────────────────────────────┐
       │                    APP-SHELL                     │  Layer 5: UI Shell
       └─────────────────────────┬───────────────────────┘
                                 │
          ┌──────────────────────┼──────────────────────┐
          │                      │                      │
          ▼                      ▼                      ▼
    ┌───────────┐         ┌───────────┐         ┌──────────────┐
    │STARTUP-   │         │ DECK-LIST │         │DRILL-        │  Layer 6: Pages
    │FLOW       │         │           │         │SELECTION     │
    └───────────┘         └─────┬─────┘         └──────────────┘
                                │
                                ▼
                         ┌───────────┐
                         │ DECK-VIEW │
                         └─────┬─────┘
                               │
                               ▼
                         ┌───────────┐
                         │CARD-EDITOR│  Layer 7: Leaf UI
                         └───────────┘
```

### Implementation Order

| Layer | Specs | Can Parallelize |
|-------|-------|-----------------|
| 0 | PROJECT-SETUP | — |
| 1 | CONFIG, DATABASE, MODEL-DOWNLOAD | Yes |
| 2 | AUDIO-PIPELINE, STT-ENGINE | Partial (STT needs MODEL-DOWNLOAD) |
| 3 | MATCHING, TTS-SERVICE, SRS-ALGORITHM | Yes |
| 4 | CARD-PRACTICE, JSL-DRILLS, DECK-GENERATION | Yes |
| 5 | APP-SHELL | — |
| 6 | STARTUP-FLOW, DECK-LIST, DRILL-SELECTION | Yes |
| 7 | DECK-VIEW, CARD-EDITOR | Yes (DECK-VIEW first) |

### Dependency Types

Each spec's `## Dependencies` section contains:

```yaml
spec_dependencies:     # Specs that must be implemented first
type_imports:          # Types consumed from other specs
type_exports:          # Types this spec produces
```

---

## Ralph Workflow

Specs in this directory are **design specs**—comprehensive documents covering entire subsystems. For Ralph-style autonomous implementation:

1. Design specs live here in `/specs/`
2. Check the dependency graph above for implementation order
3. Use the `ralph-prep` skill to decompose into atomic task specs
4. Task specs go in `/specs/tasks/{component}/`
5. `IMPLEMENTATION_PLAN.md` orders the atomic tasks
6. Run `./ralph.sh specs/tasks/{component}/` to execute

See `docs/HOW-TO-RALPH.md` for the full workflow.
