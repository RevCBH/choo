---
task: 1
status: complete
depends_on: []
backpressure: "cd koe && pnpm install && cd src-tauri && cargo check"
---

# Core Configuration Files

**Parent spec**: `/specs/PROJECT-SETUP.md` **Task**: #1 of 9 in implementation
plan

## Objective

Create the foundational configuration files that define the Rust and Node.js
project: Cargo.toml, package.json, and tauri.conf.json.

## Dependencies

### External Specs (must be implemented)

- None — this is the foundational task

### Task Dependencies (within this workset)

- None — this is the first task

### Prerequisites

- User has run `npm create tauri-app@latest` with React + TypeScript template
- Project directory exists with basic Tauri scaffold

## Deliverables

### Files to Create/Modify

```
koe/
├── src-tauri/
│   ├── Cargo.toml        # MODIFY: Replace with full dependency list
│   └── tauri.conf.json   # MODIFY: Configure allowlist and bundle settings
└── package.json          # MODIFY: Add all scripts and dependencies
```

### Cargo.toml

```toml
[package]
name = "koe"
version = "0.1.0"
description = "Japanese speaking practice with speech recognition"
authors = ["Bennett"]
license = "MIT"
edition = "2021"

[build-dependencies]
tauri-build = { version = "2", features = [] }

[dependencies]
tauri = { version = "2", features = [] }
serde = { version = "1.0", features = ["derive"] }
serde_json = "1.0"
tokio = { version = "1", features = ["full"] }
rusqlite = { version = "0.32", features = ["bundled"] }
chrono = { version = "0.4", features = ["serde"] }
uuid = { version = "1", features = ["v4", "serde"] }
anyhow = "1.0"
thiserror = "2"

# Audio & ML
whisper-rs = "0.12"

# HTTP & APIs
reqwest = { version = "0.12", features = ["json", "stream"] }

# Japanese text processing
wana-kana = "5"
unicode-normalization = "0.1"

# Utilities
sha2 = "0.10"
hex = "0.4"
tracing = "0.1"
tracing-subscriber = "0.3"

[dev-dependencies]
criterion = "0.5"
mockito = "1"
tempfile = "3"

[[bench]]
name = "stt_benchmark"
harness = false

[features]
default = ["custom-protocol"]
custom-protocol = ["tauri/custom-protocol"]
```

### package.json

```json
{
  "name": "koe",
  "version": "0.1.0",
  "description": "Japanese speaking practice with speech recognition",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc && vite build",
    "preview": "vite preview",
    "tauri": "tauri",
    "tauri:dev": "tauri dev",
    "tauri:build": "tauri build",
    "test": "vitest",
    "test:ui": "vitest --ui",
    "test:e2e": "playwright test",
    "lint": "eslint src --ext ts,tsx --report-unused-disable-directives --max-warnings 0",
    "format": "prettier --write \"src/**/*.{ts,tsx,css}\"",
    "type-check": "tsc --noEmit"
  },
  "dependencies": {
    "@tauri-apps/api": "^2",
    "react": "^19",
    "react-dom": "^19",
    "react-router-dom": "^7",
    "clsx": "^2"
  },
  "devDependencies": {
    "@tauri-apps/cli": "^2",
    "@types/react": "^19",
    "@types/react-dom": "^19",
    "@typescript-eslint/eslint-plugin": "^8",
    "@typescript-eslint/parser": "^8",
    "@vitejs/plugin-react": "^4",
    "@vitest/ui": "^3",
    "autoprefixer": "^10",
    "eslint": "^9",
    "eslint-plugin-react-hooks": "^5",
    "eslint-plugin-react-refresh": "^0.4",
    "postcss": "^8",
    "prettier": "^3",
    "tailwindcss": "^3",
    "typescript": "^5",
    "vite": "^7",
    "@playwright/test": "^1",
    "happy-dom": "^15",
    "@testing-library/react": "^16",
    "vitest": "^3"
  }
}
```

### tauri.conf.json

Tauri 2 uses a new configuration structure. See parent spec for full content.
Key sections:

- `identifier`: `com.koe.app`
- `app.windows[0]`: 1024x768, min 800x600
- Capabilities are now in separate files under `src-tauri/capabilities/`

## Backpressure

### Validation Command

```bash
cd koe && pnpm install && cd src-tauri && cargo check
```

### Must Pass

| Test                    | Assertion                                      |
| ----------------------- | ---------------------------------------------- |
| `pnpm_install_succeeds` | `pnpm install` exits with code 0               |
| `cargo_check_succeeds`  | `cargo check` exits with code 0                |
| `package_json_valid`    | `node -e "require('./package.json')"` succeeds |
| `cargo_toml_valid`      | `cargo metadata --format-version 1` succeeds   |

### Test Fixtures

None required for this task.

### CI Compatibility

- [x] No external API keys required
- [x] No network access required (after npm install)
- [x] No large files (>10MB) required
- [x] Runs in <60 seconds

## Implementation Notes

- The Tauri 2 scaffold from `npm create tauri-app@latest` provides minimal
  Cargo.toml — we expand it
- `whisper-rs` requires system libraries on some platforms (handled in later CI
  task)
- `rusqlite` with `bundled` feature compiles SQLite from source (no system
  dependency)
- Tauri 2 uses capabilities files instead of allowlist — see parent spec for
  `default.json`

## NOT In Scope

- Creating directory structure (Task #2)
- Frontend configuration files (Task #3)
- Writing any Rust or TypeScript code (later tasks)
- Database migrations (Task #8)
