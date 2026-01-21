---
task: 1
status: complete
backpressure: "test -f internal/web/static/index.html && grep -q 'DOCTYPE html' internal/web/static/index.html && grep -q 'graph-container' internal/web/static/index.html"
depends_on: []
---

# HTML Structure

**Parent spec**: `/specs/WEB-FRONTEND.md`
**Task**: #1 of 4

## Objective

Create the main HTML page with complete layout structure for the Choo orchestrator web UI, including sidebar, graph area, detail panel, and event log containers.

## Dependencies

### Task Dependencies
- None

### External Dependencies
- D3.js v7 (loaded from CDN)

## Deliverables

### Files to Create

```
internal/web/static/
└── index.html    # CREATE: Main HTML page
```

### HTML Structure

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Choo Orchestrator</title>
    <link rel="stylesheet" href="style.css">
    <script src="https://d3js.org/d3.v7.min.js"></script>
</head>
<body>
    <div class="container">
        <aside class="sidebar">
            <div id="connection-status" class="status-card">
                <div class="status-indicator"></div>
                <span class="status-text">Connecting...</span>
            </div>

            <div id="summary-panel" class="summary-card">
                <h3>Summary</h3>
                <div class="summary-grid">
                    <div class="stat" data-status="total">
                        <span class="stat-value">0</span>
                        <span class="stat-label">Total</span>
                    </div>
                    <div class="stat" data-status="pending">
                        <span class="stat-value">0</span>
                        <span class="stat-label">Pending</span>
                    </div>
                    <div class="stat" data-status="inProgress">
                        <span class="stat-value">0</span>
                        <span class="stat-label">In Progress</span>
                    </div>
                    <div class="stat" data-status="complete">
                        <span class="stat-value">0</span>
                        <span class="stat-label">Complete</span>
                    </div>
                    <div class="stat" data-status="failed">
                        <span class="stat-value">0</span>
                        <span class="stat-label">Failed</span>
                    </div>
                    <div class="stat" data-status="blocked">
                        <span class="stat-value">0</span>
                        <span class="stat-label">Blocked</span>
                    </div>
                </div>
            </div>

            <div id="toast-container"></div>
        </aside>

        <main class="main-content">
            <div id="graph-container" class="graph-area"></div>

            <div id="detail-panel" class="detail-panel hidden">
                <div class="detail-header">
                    <h3 id="detail-title">Unit Name</h3>
                    <button id="detail-close" class="close-btn">&times;</button>
                </div>
                <div class="detail-body">
                    <div id="detail-status" class="detail-status"></div>
                    <div id="detail-progress" class="detail-progress"></div>
                    <div id="detail-error" class="detail-error hidden"></div>
                    <div id="detail-tasks" class="detail-tasks"></div>
                </div>
            </div>

            <div id="event-log" class="event-log">
                <h4>Event Log</h4>
                <div id="event-list" class="event-list"></div>
            </div>
        </main>
    </div>

    <script type="module" src="app.js"></script>
</body>
</html>
```

## Backpressure

### Validation Command

```bash
test -f internal/web/static/index.html && \
grep -q 'DOCTYPE html' internal/web/static/index.html && \
grep -q 'graph-container' internal/web/static/index.html && \
grep -q 'detail-panel' internal/web/static/index.html && \
grep -q 'event-log' internal/web/static/index.html
```

### Must Pass
- File exists at `internal/web/static/index.html`
- Contains valid HTML5 doctype
- Contains required container IDs: `graph-container`, `detail-panel`, `event-log`
- Contains summary panel with stat elements
- Contains connection status indicator
- Loads D3.js from CDN
- Loads app.js as ES module

### CI Compatibility
- [x] No external API keys required
- [x] No network access required for validation
- [x] Runs in <60 seconds

## NOT In Scope
- CSS styling (task #2)
- JavaScript logic (tasks #3, #4)
- Actual data rendering
