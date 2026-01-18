# PROJECT-SETUP

## Dependencies

```yaml
spec_dependencies: []  # Foundational - no dependencies
type_imports: []
```

**This is a foundational spec.** All other specs depend on PROJECT-SETUP being complete.

---

## Overview

This spec covers the initial Tauri project scaffolding, directory structure, development environment configuration, build pipeline, and testing infrastructure for Koe. This is a prerequisite for all other implementation work.

## Requirements

### Functional Requirements
- Create a Tauri 2.x project with React frontend and Rust backend
- Configure Vite for fast development builds
- Set up Tailwind CSS for styling
- Initialize SQLite database with migrations system
- Configure testing frameworks for both frontend and backend
- Set up development tooling (linting, formatting, type checking)
- Configure build pipeline for multi-platform distribution

### Performance Requirements
- Dev server hot reload in < 500ms
- Production build completes in < 2 minutes
- Test suite runs in < 30s (unit tests), < 2min (integration tests)

### Constraints
- Must support macOS (primary), Windows, Linux (secondary)
- Rust stable toolchain only (no nightly features)
- Node.js 18+ and Rust 1.70+

---

## Initial Project Creation

### Prerequisites

```bash
# Install Rust (if not already installed)
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
rustup default stable

# Install Tauri CLI
cargo install tauri-cli

# Install Node.js 18+ (via nvm, homebrew, etc)
node --version  # Should be >= 18.0.0
```

### Scaffolding Commands

```bash
# Create new Tauri project
npm create tauri-app@latest

# Follow prompts:
# - App name: koe
# - Window title: Koe
# - UI recipe: React + TypeScript
# - UI flavor: TypeScript
# - Package manager: npm

cd koe

# Verify setup
npm run tauri dev
```

---

## Directory Structure

```
koe/
├── src/                          # Frontend source
│   ├── components/
│   │   ├── practice/
│   │   │   ├── CardDisplay.tsx
│   │   │   ├── RecordingControls.tsx
│   │   │   ├── TranscriptionResult.tsx
│   │   │   └── AudioPlayback.tsx
│   │   ├── deck/
│   │   │   ├── DeckList.tsx
│   │   │   ├── DeckGenerator.tsx
│   │   │   └── CardEditor.tsx
│   │   ├── drill/
│   │   │   ├── DrillSession.tsx
│   │   │   └── DrillResults.tsx
│   │   ├── settings/
│   │   │   ├── Settings.tsx
│   │   │   ├── APIKeyConfig.tsx
│   │   │   └── VoiceSelector.tsx
│   │   └── shared/
│   │       ├── Button.tsx
│   │       ├── Modal.tsx
│   │       └── ProgressBar.tsx
│   ├── services/
│   │   ├── tauri.ts              # Tauri command wrappers
│   │   ├── audio.ts              # Web Audio API utilities
│   │   ├── srs.ts                # Frontend SRS helpers
│   │   └── types.ts              # TypeScript type definitions
│   ├── hooks/
│   │   ├── useRecording.ts
│   │   ├── useKeyboard.ts
│   │   └── useDeck.ts
│   ├── pages/
│   │   ├── Practice.tsx
│   │   ├── Decks.tsx
│   │   ├── Drills.tsx
│   │   └── Settings.tsx
│   ├── styles/
│   │   └── globals.css
│   ├── App.tsx
│   ├── main.tsx
│   └── vite-env.d.ts
├── src-tauri/
│   ├── src/
│   │   ├── commands/             # Tauri command handlers
│   │   │   ├── mod.rs
│   │   │   ├── transcribe.rs
│   │   │   ├── tts.rs
│   │   │   ├── audio.rs
│   │   │   ├── db.rs
│   │   │   └── config.rs
│   │   ├── services/             # Core business logic
│   │   │   ├── mod.rs
│   │   │   ├── stt.rs            # Whisper integration
│   │   │   ├── tts.rs            # ElevenLabs client
│   │   │   ├── matching.rs       # Answer matching logic
│   │   │   ├── srs.rs            # SM-2 implementation
│   │   │   └── audio_cache.rs    # File system cache
│   │   ├── models/               # Data models
│   │   │   ├── mod.rs
│   │   │   ├── card.rs
│   │   │   ├── deck.rs
│   │   │   ├── recording.rs
│   │   │   └── drill.rs
│   │   ├── db/                   # Database layer
│   │   │   ├── mod.rs
│   │   │   ├── connection.rs
│   │   │   ├── migrations.rs
│   │   │   └── queries/
│   │   │       ├── cards.rs
│   │   │       ├── decks.rs
│   │   │       └── recordings.rs
│   │   ├── error.rs              # Error types
│   │   ├── state.rs              # Application state
│   │   ├── lib.rs
│   │   └── main.rs
│   ├── migrations/               # SQL migration files
│   │   ├── 001_init.sql
│   │   ├── 002_add_drills.sql
│   │   └── ...
│   ├── icons/                    # App icons
│   ├── Cargo.toml
│   ├── tauri.conf.json
│   └── build.rs
├── tests/                        # Integration tests
│   ├── practice_flow.spec.ts
│   ├── deck_generation.spec.ts
│   └── drill_session.spec.ts
├── public/                       # Static assets
├── index.html
├── package.json
├── tsconfig.json
├── tailwind.config.js
├── postcss.config.js
├── vite.config.ts
├── vitest.config.ts
├── playwright.config.ts
├── .gitignore
├── .prettierrc
├── .eslintrc.json
└── README.md
```

---

## Configuration Files

### Cargo.toml

```toml
[package]
name = "koe"
version = "0.1.0"
description = "Japanese speaking practice with speech recognition"
authors = ["Bennett"]
license = "MIT"
repository = "https://github.com/yourusername/koe"
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
    "vitest": "^3",
    "@playwright/test": "^1",
    "happy-dom": "^15",
    "@testing-library/react": "^16"
  }
}
```

### tauri.conf.json

Tauri 2 uses a simplified configuration structure. Capabilities (permissions) are now defined in separate JSON files.

```json
{
  "$schema": "https://schema.tauri.app/config/2",
  "productName": "Koe",
  "version": "0.1.0",
  "identifier": "com.koe.app",
  "build": {
    "beforeDevCommand": "npm run dev",
    "beforeBuildCommand": "npm run build",
    "devUrl": "http://localhost:5173",
    "frontendDist": "../dist"
  },
  "app": {
    "windows": [
      {
        "fullscreen": false,
        "resizable": true,
        "title": "Koe",
        "width": 1024,
        "height": 768,
        "minWidth": 800,
        "minHeight": 600
      }
    ],
    "security": {
      "csp": null
    }
  },
  "bundle": {
    "active": true,
    "icon": [
      "icons/32x32.png",
      "icons/128x128.png",
      "icons/128x128@2x.png",
      "icons/icon.icns",
      "icons/icon.ico"
    ],
    "category": "Education",
    "shortDescription": "Japanese speaking practice",
    "longDescription": "Practice Japanese pronunciation with speech recognition",
    "macOS": {
      "minimumSystemVersion": "10.13"
    }
  }
}
```

### Capabilities (src-tauri/capabilities/default.json)

Tauri 2 uses capability files instead of allowlist. Create `src-tauri/capabilities/default.json`:

```json
{
  "$schema": "https://schema.tauri.app/config/2",
  "identifier": "default",
  "description": "Default capabilities for Koe",
  "windows": ["main"],
  "permissions": [
    "core:default",
    "shell:allow-open",
    "dialog:default",
    "fs:default",
    {
      "identifier": "fs:allow-read",
      "allow": [
        { "path": "$APPDATA/**" },
        { "path": "$HOME/.koe/**" }
      ]
    },
    {
      "identifier": "fs:allow-write",
      "allow": [
        { "path": "$APPDATA/**" },
        { "path": "$HOME/.koe/**" }
      ]
    }
  ]
}
```

### vite.config.ts

```typescript
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  clearScreen: false,
  server: {
    port: 5173,
    strictPort: true,
    watch: {
      ignored: ['**/src-tauri/**']
    }
  },
  envPrefix: ['VITE_', 'TAURI_'],
  build: {
    target: 'esnext',
    minify: !process.env.TAURI_DEBUG ? 'esbuild' : false,
    sourcemap: !!process.env.TAURI_DEBUG
  }
});
```

### tsconfig.json

```json
{
  "compilerOptions": {
    "target": "ES2020",
    "useDefineForClassFields": true,
    "lib": ["ES2020", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "skipLibCheck": true,

    /* Bundler mode */
    "moduleResolution": "bundler",
    "allowImportingTsExtensions": true,
    "resolveJsonModule": true,
    "isolatedModules": true,
    "noEmit": true,
    "jsx": "react-jsx",

    /* Linting */
    "strict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noFallthroughCasesInSwitch": true,

    /* Paths */
    "baseUrl": ".",
    "paths": {
      "@/*": ["./src/*"]
    }
  },
  "include": ["src"],
  "references": [{ "path": "./tsconfig.node.json" }]
}
```

### tailwind.config.js

```javascript
/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        correct: '#10b981',
        close: '#f59e0b',
        incorrect: '#ef4444',
      },
      keyframes: {
        'pulse-record': {
          '0%, 100%': { opacity: 1 },
          '50%': { opacity: 0.5 },
        }
      },
      animation: {
        'pulse-record': 'pulse-record 1.5s ease-in-out infinite',
      }
    },
  },
  plugins: [],
}
```

---

## Testing Infrastructure

### Backend Testing (Rust)

#### Unit Tests

```rust
// src-tauri/src/services/matching.rs

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_normalize_hiragana() {
        assert_eq!(
            normalize_for_comparison("たべる"),
            "たべる"
        );
    }

    #[test]
    fn test_normalize_katakana_to_hiragana() {
        assert_eq!(
            normalize_for_comparison("タベル"),
            "たべる"
        );
    }

    #[test]
    fn test_exact_match() {
        let targets = vec!["たべる".to_string()];
        assert!(matches!(
            check_match("たべる", &targets),
            MatchResult::Correct
        ));
    }

    #[test]
    fn test_close_match() {
        let targets = vec!["たべる".to_string()];
        assert!(matches!(
            check_match("たべり", &targets),
            MatchResult::Close { .. }
        ));
    }
}
```

Run with:
```bash
cd src-tauri
cargo test
```

#### Integration Tests

```rust
// src-tauri/tests/db_integration.rs

use koe::db::Database;
use koe::models::Card;
use tempfile::tempdir;

#[test]
fn test_card_crud() {
    let dir = tempdir().unwrap();
    let db_path = dir.path().join("test.db");
    let db = Database::new(db_path.to_str().unwrap()).unwrap();

    // Create
    let card = Card::new(
        "deck-1",
        CardType::Translation,
        "eat",
        vec!["たべる".to_string()],
    );
    db.insert_card(&card).unwrap();

    // Read
    let retrieved = db.get_card(&card.id).unwrap().unwrap();
    assert_eq!(retrieved.prompt, "eat");

    // Update
    // ...

    // Delete
    db.delete_card(&card.id).unwrap();
    assert!(db.get_card(&card.id).unwrap().is_none());
}
```

Run with:
```bash
cargo test --test db_integration
```

#### Benchmarks

```rust
// src-tauri/benches/stt_benchmark.rs

use criterion::{black_box, criterion_group, criterion_main, Criterion};
use koe::services::SpeechToText;

fn benchmark_transcribe(c: &mut Criterion) {
    let stt = SpeechToText::new("path/to/model").unwrap();
    let audio = vec![0.0f32; 16000 * 3]; // 3 seconds of silence

    c.bench_function("transcribe_3s_audio", |b| {
        b.iter(|| {
            stt.transcribe(black_box(&audio)).unwrap()
        })
    });
}

criterion_group!(benches, benchmark_transcribe);
criterion_main!(benches);
```

Run with:
```bash
cargo bench
```

### Frontend Testing

#### vitest.config.ts

```typescript
import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';
import path from 'path';

export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'happy-dom',
    globals: true,
    setupFiles: './src/test/setup.ts',
  },
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
});
```

#### Unit Tests (Components)

```typescript
// src/components/practice/RecordingControls.test.tsx

import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { RecordingControls } from './RecordingControls';

describe('RecordingControls', () => {
  it('renders record button', () => {
    render(<RecordingControls onRecord={vi.fn()} isRecording={false} />);
    expect(screen.getByRole('button', { name: /record/i })).toBeInTheDocument();
  });

  it('calls onRecord when spacebar pressed', () => {
    const onRecord = vi.fn();
    render(<RecordingControls onRecord={onRecord} isRecording={false} />);

    fireEvent.keyDown(window, { key: ' ' });
    expect(onRecord).toHaveBeenCalledWith(true);

    fireEvent.keyUp(window, { key: ' ' });
    expect(onRecord).toHaveBeenCalledWith(false);
  });

  it('shows recording indicator when active', () => {
    render(<RecordingControls onRecord={vi.fn()} isRecording={true} />);
    expect(screen.getByTestId('recording-indicator')).toBeInTheDocument();
  });
});
```

#### Integration Tests (Service Layer)

```typescript
// src/services/tauri.test.ts

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { TauriSTT } from './tauri';
import { invoke } from '@tauri-apps/api/tauri';

vi.mock('@tauri-apps/api/tauri');

describe('TauriSTT', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('transcribes audio', async () => {
    const mockInvoke = vi.mocked(invoke);
    mockInvoke.mockResolvedValue('たべる');

    const stt = new TauriSTT();
    const audio = new Float32Array([0, 0.1, 0.2]);
    const result = await stt.transcribe(audio);

    expect(result).toBe('たべる');
    expect(mockInvoke).toHaveBeenCalledWith('transcribe', {
      audio: Array.from(audio)
    });
  });

  it('handles transcription errors', async () => {
    const mockInvoke = vi.mocked(invoke);
    mockInvoke.mockRejectedValue(new Error('Model not loaded'));

    const stt = new TauriSTT();
    const audio = new Float32Array([0]);

    await expect(stt.transcribe(audio)).rejects.toThrow('Model not loaded');
  });
});
```

Run with:
```bash
npm test
```

### E2E Testing

#### playwright.config.ts

```typescript
import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './tests',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: 'html',
  use: {
    trace: 'on-first-retry',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
  webServer: {
    command: 'npm run tauri:dev',
    url: 'http://localhost:5173',
    reuseExistingServer: !process.env.CI,
    timeout: 120000,
  },
});
```

#### E2E Tests

```typescript
// tests/practice_flow.spec.ts

import { test, expect } from '@playwright/test';

test.describe('Card Practice Flow', () => {
  test('complete practice session', async ({ page }) => {
    await page.goto('http://localhost:5173');

    // Navigate to practice
    await page.click('[data-testid="practice-button"]');

    // Wait for card to load
    await expect(page.locator('[data-testid="card-prompt"]')).toBeVisible();

    // Simulate recording (mock)
    await page.keyboard.down(' ');
    await page.waitForTimeout(1000);
    await page.keyboard.up(' ');

    // Check transcription result appears
    await expect(page.locator('[data-testid="transcription-result"]')).toBeVisible();

    // Confirm and advance
    await page.keyboard.press('Enter');

    // Next card should appear
    await expect(page.locator('[data-testid="card-prompt"]')).toBeVisible();
  });

  test('play reference audio', async ({ page }) => {
    await page.goto('http://localhost:5173/practice');

    // Press P for reference audio
    await page.keyboard.press('p');

    // Audio element should be playing
    const audioPlaying = await page.evaluate(() => {
      const audio = document.querySelector('audio');
      return audio && !audio.paused;
    });

    expect(audioPlaying).toBe(true);
  });
});
```

Run with:
```bash
npm run test:e2e
```

---

## Development Tooling

### .eslintrc.json

```json
{
  "extends": [
    "eslint:recommended",
    "plugin:@typescript-eslint/recommended",
    "plugin:react-hooks/recommended"
  ],
  "parser": "@typescript-eslint/parser",
  "plugins": ["react-refresh"],
  "rules": {
    "react-refresh/only-export-components": "warn",
    "@typescript-eslint/no-unused-vars": ["error", { "argsIgnorePattern": "^_" }]
  }
}
```

### .prettierrc

```json
{
  "semi": true,
  "trailingComma": "es5",
  "singleQuote": true,
  "printWidth": 100,
  "tabWidth": 2
}
```

### .gitignore

```
# Dependencies
node_modules/
/target

# Build output
/dist
/dist-ssr
/src-tauri/target

# Logs
*.log
npm-debug.log*
yarn-debug.log*
yarn-error.log*
pnpm-debug.log*

# Editor directories
.vscode/*
!.vscode/extensions.json
.idea
.DS_Store
*.suo
*.ntvs*
*.njsproj
*.sln
*.sw?

# Environment
.env
.env.local

# Testing
coverage/
.nyc_output/

# OS
Thumbs.db

# Tauri
src-tauri/target/
WixTools/

# Local data directory (for development)
.koe/
```

---

## Database Initialization

### migrations/001_init.sql

```sql
-- Core schema
CREATE TABLE IF NOT EXISTS decks (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    last_studied_at TEXT
);

CREATE TABLE IF NOT EXISTS cards (
    id TEXT PRIMARY KEY,
    deck_id TEXT NOT NULL REFERENCES decks(id) ON DELETE CASCADE,
    type TEXT NOT NULL CHECK (type IN ('translation', 'reading')),
    prompt TEXT NOT NULL,
    targets TEXT NOT NULL, -- JSON array
    meaning TEXT,
    example TEXT,
    tags TEXT, -- JSON array

    -- SRS fields
    interval_days INTEGER NOT NULL DEFAULT 0,
    ease_factor REAL NOT NULL DEFAULT 2.5,
    due_date TEXT NOT NULL DEFAULT (datetime('now')),
    review_count INTEGER NOT NULL DEFAULT 0,
    lapse_count INTEGER NOT NULL DEFAULT 0,

    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS recordings (
    id TEXT PRIMARY KEY,
    card_id TEXT NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
    file_path TEXT NOT NULL,
    transcription TEXT,
    was_correct INTEGER,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS drill_sessions (
    id TEXT PRIMARY KEY,
    drill_type TEXT NOT NULL,
    started_at TEXT NOT NULL DEFAULT (datetime('now')),
    completed_at TEXT,
    results TEXT -- JSON
);

CREATE INDEX idx_cards_due ON cards(deck_id, due_date);
CREATE INDEX idx_recordings_card ON recordings(card_id, created_at DESC);
```

### Migration System

```rust
// src-tauri/src/db/migrations.rs

use rusqlite::Connection;
use anyhow::Result;

const MIGRATIONS: &[&str] = &[
    include_str!("../../migrations/001_init.sql"),
    // include_str!("../../migrations/002_add_drills.sql"),
];

pub fn run_migrations(conn: &Connection) -> Result<()> {
    // Create migrations table
    conn.execute(
        "CREATE TABLE IF NOT EXISTS schema_migrations (
            version INTEGER PRIMARY KEY,
            applied_at TEXT NOT NULL DEFAULT (datetime('now'))
        )",
        [],
    )?;

    // Get current version
    let current_version: i32 = conn
        .query_row(
            "SELECT COALESCE(MAX(version), 0) FROM schema_migrations",
            [],
            |row| row.get(0),
        )
        .unwrap_or(0);

    // Run pending migrations
    for (idx, migration) in MIGRATIONS.iter().enumerate() {
        let version = (idx + 1) as i32;
        if version > current_version {
            tracing::info!("Running migration {}", version);
            conn.execute_batch(migration)?;
            conn.execute(
                "INSERT INTO schema_migrations (version) VALUES (?1)",
                [version],
            )?;
        }
    }

    Ok(())
}
```

---

## Build Pipeline

### Development Build

```bash
# Start dev server with hot reload
npm run tauri:dev

# Watch mode for Rust (in separate terminal)
cd src-tauri
cargo watch -x build
```

### Production Build

```bash
# Build for current platform
npm run tauri:build

# macOS universal binary (Intel + Apple Silicon)
npm run tauri:build -- --target universal-apple-darwin

# Windows
npm run tauri:build -- --target x86_64-pc-windows-msvc

# Linux
npm run tauri:build -- --target x86_64-unknown-linux-gnu
```

### CI/CD Configuration (.github/workflows/build.yml)

```yaml
name: Build

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: 18
      - uses: actions-rust-lang/setup-rust-toolchain@v1

      - name: Install dependencies
        run: npm ci

      - name: Lint
        run: npm run lint

      - name: Type check
        run: npm run type-check

      - name: Test frontend
        run: npm test

      - name: Test backend
        run: cd src-tauri && cargo test

  build:
    needs: test
    strategy:
      matrix:
        platform: [macos-latest, ubuntu-latest, windows-latest]
    runs-on: ${{ matrix.platform }}

    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: 18
      - uses: actions-rust-lang/setup-rust-toolchain@v1

      - name: Install dependencies (Ubuntu)
        if: matrix.platform == 'ubuntu-latest'
        run: |
          sudo apt-get update
          sudo apt-get install -y libgtk-3-dev libwebkit2gtk-4.0-dev \
            libappindicator3-dev librsvg2-dev patchelf

      - name: Install frontend dependencies
        run: npm ci

      - name: Build
        run: npm run tauri:build

      - name: Upload artifacts
        uses: actions/upload-artifact@v3
        with:
          name: koe-${{ matrix.platform }}
          path: src-tauri/target/release/bundle/
```

---

## Initial Setup Checklist

After creating the project, complete these setup tasks:

- [ ] Run `npm create tauri-app@latest` and select React + TypeScript
- [ ] Install all dependencies: `npm install` and verify Rust toolchain
- [ ] Copy all configuration files (Cargo.toml, package.json, tauri.conf.json, etc.)
- [ ] Create directory structure as outlined above
- [ ] Set up Tailwind CSS: `npx tailwindcss init -p`
- [ ] Initialize Git: `git init` and create `.gitignore`
- [ ] Set up ESLint and Prettier configs
- [ ] Create migration files in `src-tauri/migrations/`
- [ ] Verify dev server runs: `npm run tauri:dev`
- [ ] Run initial tests: `npm test` and `cd src-tauri && cargo test`
- [ ] Generate app icons and place in `src-tauri/icons/`
- [ ] Configure VS Code settings (optional, see below)

---

## VS Code Configuration

### .vscode/extensions.json

```json
{
  "recommendations": [
    "rust-lang.rust-analyzer",
    "tauri-apps.tauri-vscode",
    "dbaeumer.vscode-eslint",
    "esbenp.prettier-vscode",
    "bradlc.vscode-tailwindcss"
  ]
}
```

### .vscode/settings.json

```json
{
  "editor.formatOnSave": true,
  "editor.defaultFormatter": "esbenp.prettier-vscode",
  "[rust]": {
    "editor.defaultFormatter": "rust-lang.rust-analyzer"
  },
  "rust-analyzer.checkOnSave.command": "clippy",
  "tailwindCSS.experimental.classRegex": [
    ["clsx\\(([^)]*)\\)", "(?:'|\"|`)([^']*)(?:'|\"|`)"]
  ]
}
```

---

## Performance Benchmarks (Target)

| Metric | Target | Measurement |
|--------|--------|-------------|
| Dev server start | < 3s | `time npm run dev` |
| Hot reload | < 500ms | Time from save to render |
| Production build | < 2min | `time npm run tauri:build` |
| Unit test suite | < 10s | `time npm test` |
| Integration tests | < 30s | `time cargo test` |
| E2E test suite | < 2min | `time npm run test:e2e` |
| Bundle size (app) | < 20MB | `ls -lh src-tauri/target/release/bundle/` |

---

## Troubleshooting

### Common Issues

**Issue:** `cargo build` fails with linker errors on macOS
```bash
# Solution: Install Xcode command line tools
xcode-select --install
```

**Issue:** Tauri dev command hangs on "Waiting for your frontend dev server to start"
```bash
# Solution: Check that port 5173 is not in use
lsof -i :5173
# Kill the process or change vite.config.ts port
```

**Issue:** Whisper model fails to load
```bash
# Solution: Verify model file exists and has correct permissions
ls -lh ~/.koe/models/ggml-large-v3.bin
# Should be ~3.1GB, readable
```

**Issue:** Tests fail with "Cannot find module @tauri-apps/api"
```bash
# Solution: Ensure dev dependencies are installed
npm install
# And that mock is properly set up in vitest.config.ts
```

---

## Implementation Notes

### Initialization Order

When the app starts, follow this sequence:

1. Initialize logging (tracing)
2. Load or create config.json
3. Run database migrations
4. Check for Whisper model, prompt download if missing
5. Initialize application state (STT, TTS clients, DB connection)
6. Register Tauri commands
7. Launch window

```rust
// src-tauri/src/main.rs

use tauri::Manager;

fn main() {
    // Initialize tracing
    tracing_subscriber::fmt::init();

    tauri::Builder::default()
        .setup(|app| {
            let app_handle = app.handle();

            // Initialize app state
            tauri::async_runtime::block_on(async move {
                let state = AppState::new(&app_handle).await?;
                app_handle.manage(state);
                Ok(())
            })
        })
        .invoke_handler(tauri::generate_handler![
            commands::transcribe::transcribe,
            commands::tts::generate_tts,
            commands::audio::save_recording,
            commands::db::get_due_cards,
            // ... other commands
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
```

### Testing Strategy Summary

- **Unit tests:** Test individual functions in isolation (matching logic, normalization, SRS calculations)
- **Integration tests:** Test database operations, API clients, file system operations
- **Component tests:** Test React components with mocked Tauri commands
- **E2E tests:** Test critical user flows (practice session, deck creation, settings)
- **Benchmarks:** Measure STT performance, database query speed, audio processing

Run full test suite:
```bash
npm run lint && \
npm run type-check && \
npm test && \
cd src-tauri && cargo test && cargo clippy
```

---

## References

- [Tauri Getting Started](https://tauri.app/v1/guides/getting-started/setup/)
- [Vite Configuration](https://vitejs.dev/config/)
- [Vitest Documentation](https://vitest.dev/)
- [Playwright Testing](https://playwright.dev/)
- [Rust Testing Guide](https://doc.rust-lang.org/book/ch11-00-testing.html)
- [Criterion Benchmarking](https://github.com/bheisler/criterion.rs)
