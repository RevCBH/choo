# WEB-FRONTEND — Browser UI with D3.js Dependency Graph Visualization

## Overview

The WEB-FRONTEND provides a real-time browser interface for monitoring the orchestrator. It displays a dependency graph of units as an interactive D3.js visualization with live status updates via Server-Sent Events (SSE), shows task progress through color-coded nodes, and surfaces errors through multiple channels (toasts, detail panels, and event logs).

This component exists to give operators visibility into orchestrator progress without requiring terminal access. The visual graph representation makes it easy to understand unit dependencies at a glance, identify bottlenecks, and quickly locate failed or blocked units. Real-time updates via SSE ensure the display stays synchronized with orchestrator state.

```
┌─────────────────────────────────────────────────────────────────────┐
│                           Browser                                    │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│   ┌─────────────────┐  ┌─────────────────────────────────────────┐  │
│   │   Connection    │  │              Graph Area                  │  │
│   │    Status       │  │                                          │  │
│   └─────────────────┘  │    ┌───┐      ┌───┐      ┌───┐         │  │
│                        │    │ A │─────▶│ B │─────▶│ C │         │  │
│   ┌─────────────────┐  │    └───┘      └───┘      └───┘         │  │
│   │    Summary      │  │                                          │  │
│   │  Stats Panel    │  └─────────────────────────────────────────┘  │
│   └─────────────────┘                                                │
│                        ┌─────────────────────────────────────────┐  │
│   ┌─────────────────┐  │           Detail Panel                   │  │
│   │   Toast Area    │  │   (shows on node click)                 │  │
│   └─────────────────┘  └─────────────────────────────────────────┘  │
│                                                                      │
│   ┌─────────────────────────────────────────────────────────────┐   │
│   │                    Event Log                                  │   │
│   └─────────────────────────────────────────────────────────────┘   │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

## Requirements

### Functional Requirements

1. Display dependency graph with nodes representing units and edges representing dependencies
2. Update node colors in real-time based on unit status (pending, ready, in_progress, pr_open, in_review, merging, complete, failed, blocked)
3. Animate in-progress nodes with a pulsing effect
4. Show detail panel when user clicks a node, displaying task list and error messages
5. Highlight unit dependencies on node hover
6. Display connection status indicator showing SSE connection state
7. Show summary statistics panel with counts by status
8. Display toast notifications for failure events
9. Maintain scrollable event log showing recent events
10. Show "waiting" state when no orchestrator is connected
11. Preserve final state after orchestrator exits (no clearing)
12. Automatically reconnect SSE on disconnect with exponential backoff
13. Consume `/api/state` for initial state snapshot
14. Consume `/api/graph` for dependency graph structure
15. Consume `/api/events` SSE stream for real-time updates

### Performance Requirements

| Metric | Target |
|--------|--------|
| Initial render time | < 500ms |
| SSE event processing | < 50ms per event |
| Graph re-render on update | < 100ms |
| SSE reconnect delay | 1s initial, 30s max |
| Maximum nodes displayed | 100 nodes without performance degradation |

### Constraints

- Requires modern browser with ES6+ support and EventSource API
- Depends on D3.js v7 for graph visualization
- Backend must serve static files and provide API endpoints
- SSE stream must be same-origin or CORS-enabled
- No build step required - vanilla JavaScript with ES modules

## Design

### Module Structure

```
internal/web/static/
├── index.html    # Main HTML page with layout structure
├── style.css     # CSS styling for all components
├── app.js        # State management, SSE handling, UI coordination
└── graph.js      # D3.js DAG visualization and rendering
```

### Core Types

```javascript
// app.js - Application state object
const state = {
    connected: false,          // SSE connection status
    status: "waiting",         // "waiting" | "running" | "complete" | "failed"
    startedAt: null,           // ISO timestamp or null
    parallelism: 0,            // Max concurrent units
    units: [],                 // Array of UnitState
    summary: {                 // Aggregated counts
        total: 0,
        pending: 0,
        inProgress: 0,
        complete: 0,
        failed: 0,
        blocked: 0
    },
    graph: {                   // Graph structure from /api/graph
        nodes: [],             // Array of {id: string, level: number}
        edges: [],             // Array of {from: string, to: string}
        levels: []             // Array of arrays grouping nodes by level
    },
    events: [],                // Recent events for log display
    selectedUnit: null         // Currently selected unit ID or null
};

// UnitState - status of a single unit
const unitState = {
    id: "unit-name",           // Unique unit identifier
    status: "pending",         // pending|ready|in_progress|pr_open|in_review|merging|complete|failed|blocked
    currentTask: 0,            // Current task index (0-based)
    totalTasks: 0,             // Total number of tasks
    error: null                // Error message string or null
};

// GraphNode - D3 node data
const graphNode = {
    id: "unit-name",           // Unit identifier
    level: 0,                  // Dependency level (0 = no deps)
    x: 0,                      // Computed X position
    y: 0,                      // Computed Y position
    status: "pending"          // Current status for coloring
};

// SSE Event - incoming event from server
const sseEvent = {
    type: "unit.started",      // Event type
    unit: "unit-name",         // Unit ID (optional, depends on type)
    time: "2024-01-15T10:00:00Z", // ISO timestamp
    error: null                // Error message (for failure events)
};
```

### Status Color Mapping

```javascript
// graph.js - Node color constants
const STATUS_COLORS = {
    pending: "#9CA3AF",        // gray-400
    ready: "#FBBF24",          // yellow-400
    in_progress: "#3B82F6",    // blue-500 (animated pulse)
    pr_open: "#A855F7",        // purple-500
    in_review: "#A855F7",      // purple-500
    merging: "#A855F7",        // purple-500
    complete: "#22C55E",       // green-500
    failed: "#EF4444",         // red-500
    blocked: "#F97316"         // orange-500
};
```

### API Surface

```javascript
// app.js - Public functions

/**
 * Initialize the application. Fetches initial state and starts SSE.
 */
async function init() { ... }

/**
 * Update application state and re-render affected components.
 * @param {Partial<State>} updates - Partial state updates to merge
 */
function updateState(updates) { ... }

/**
 * Process an incoming SSE event and update state accordingly.
 * @param {SSEEvent} event - The parsed event data
 */
function handleEvent(event) { ... }

/**
 * Show the detail panel for a specific unit.
 * @param {string} unitId - The unit ID to display
 */
function showUnitDetail(unitId) { ... }

/**
 * Display a toast notification.
 * @param {string} message - Toast message text
 * @param {string} type - "error" | "info" | "success"
 */
function showToast(message, type) { ... }

// graph.js - Public functions

/**
 * Initialize the D3 graph visualization.
 * @param {HTMLElement} container - DOM element to render into
 * @param {GraphData} data - Initial graph structure
 * @param {Object} callbacks - Event callbacks {onClick, onHover}
 */
function initGraph(container, data, callbacks) { ... }

/**
 * Update node statuses without re-rendering layout.
 * @param {Map<string, string>} statusMap - Unit ID to status mapping
 */
function updateNodeStatuses(statusMap) { ... }

/**
 * Highlight a node and its dependencies.
 * @param {string|null} nodeId - Node to highlight, or null to clear
 */
function highlightDependencies(nodeId) { ... }
```

### SSE Client Implementation

```javascript
// app.js - SSE connection management

class SSEClient {
    constructor(url, handlers) {
        this.url = url;
        this.handlers = handlers;
        this.eventSource = null;
        this.reconnectDelay = 1000;
        this.maxReconnectDelay = 30000;
    }

    connect() {
        this.eventSource = new EventSource(this.url);

        this.eventSource.onopen = () => {
            this.reconnectDelay = 1000; // Reset on successful connect
            this.handlers.onConnect();
        };

        this.eventSource.onmessage = (e) => {
            const event = JSON.parse(e.data);
            this.handlers.onEvent(event);
        };

        this.eventSource.onerror = () => {
            this.eventSource.close();
            this.handlers.onDisconnect();
            this.scheduleReconnect();
        };

        // Listen for specific event types
        const eventTypes = [
            'unit.started', 'unit.completed', 'unit.failed',
            'task.started', 'task.completed',
            'orch.started', 'orch.completed', 'orch.failed'
        ];

        eventTypes.forEach(type => {
            this.eventSource.addEventListener(type, (e) => {
                const event = JSON.parse(e.data);
                this.handlers.onEvent(event);
            });
        });
    }

    scheduleReconnect() {
        setTimeout(() => {
            this.connect();
        }, this.reconnectDelay);

        // Exponential backoff with max
        this.reconnectDelay = Math.min(
            this.reconnectDelay * 2,
            this.maxReconnectDelay
        );
    }

    disconnect() {
        if (this.eventSource) {
            this.eventSource.close();
            this.eventSource = null;
        }
    }
}
```

### Graph Rendering Implementation

```javascript
// graph.js - D3 DAG visualization

const LAYOUT = {
    nodeRadius: 30,
    levelSpacing: 150,    // Horizontal spacing between levels
    nodeSpacing: 80,      // Vertical spacing within levels
    padding: 50
};

let svg, nodesGroup, edgesGroup, simulation;
let graphData = { nodes: [], edges: [] };
let callbacks = {};

function initGraph(container, data, eventCallbacks) {
    graphData = data;
    callbacks = eventCallbacks;

    const width = container.clientWidth;
    const height = container.clientHeight;

    svg = d3.select(container)
        .append("svg")
        .attr("width", width)
        .attr("height", height);

    // Arrow marker for edges
    svg.append("defs")
        .append("marker")
        .attr("id", "arrowhead")
        .attr("viewBox", "0 -5 10 10")
        .attr("refX", 8)
        .attr("refY", 0)
        .attr("markerWidth", 6)
        .attr("markerHeight", 6)
        .attr("orient", "auto")
        .append("path")
        .attr("d", "M0,-5L10,0L0,5")
        .attr("fill", "#6B7280");

    edgesGroup = svg.append("g").attr("class", "edges");
    nodesGroup = svg.append("g").attr("class", "nodes");

    computeLayout(width, height);
    renderEdges();
    renderNodes();
}

function computeLayout(width, height) {
    // Position nodes by level (X) and distribute within level (Y)
    const levels = graphData.levels || [];

    levels.forEach((levelNodes, levelIndex) => {
        const x = LAYOUT.padding + levelIndex * LAYOUT.levelSpacing;
        const levelHeight = levelNodes.length * LAYOUT.nodeSpacing;
        const startY = (height - levelHeight) / 2 + LAYOUT.nodeSpacing / 2;

        levelNodes.forEach((nodeId, nodeIndex) => {
            const node = graphData.nodes.find(n => n.id === nodeId);
            if (node) {
                node.x = x;
                node.y = startY + nodeIndex * LAYOUT.nodeSpacing;
            }
        });
    });
}

function renderNodes() {
    const nodeSelection = nodesGroup.selectAll(".node")
        .data(graphData.nodes, d => d.id);

    const nodeEnter = nodeSelection.enter()
        .append("g")
        .attr("class", "node")
        .attr("transform", d => `translate(${d.x}, ${d.y})`)
        .on("click", (event, d) => callbacks.onClick?.(d.id))
        .on("mouseenter", (event, d) => callbacks.onHover?.(d.id))
        .on("mouseleave", () => callbacks.onHover?.(null));

    // Node circle
    nodeEnter.append("circle")
        .attr("r", LAYOUT.nodeRadius)
        .attr("fill", d => STATUS_COLORS[d.status] || STATUS_COLORS.pending)
        .attr("stroke", "#374151")
        .attr("stroke-width", 2);

    // Node label
    nodeEnter.append("text")
        .attr("text-anchor", "middle")
        .attr("dy", "0.35em")
        .attr("fill", "white")
        .attr("font-size", "12px")
        .attr("font-weight", "500")
        .text(d => truncateLabel(d.id, 8));

    // Merge enter and update
    nodeSelection.merge(nodeEnter)
        .select("circle")
        .attr("fill", d => STATUS_COLORS[d.status] || STATUS_COLORS.pending)
        .classed("pulse", d => d.status === "in_progress");
}

function renderEdges() {
    const edgeSelection = edgesGroup.selectAll(".edge")
        .data(graphData.edges, d => `${d.from}-${d.to}`);

    edgeSelection.enter()
        .append("path")
        .attr("class", "edge")
        .attr("d", d => {
            const source = graphData.nodes.find(n => n.id === d.from);
            const target = graphData.nodes.find(n => n.id === d.to);
            if (!source || !target) return "";
            return bezierPath(source, target);
        })
        .attr("fill", "none")
        .attr("stroke", "#6B7280")
        .attr("stroke-width", 2)
        .attr("marker-end", "url(#arrowhead)");
}

function bezierPath(source, target) {
    const midX = (source.x + target.x) / 2;
    return `M ${source.x + LAYOUT.nodeRadius} ${source.y}
            C ${midX} ${source.y}, ${midX} ${target.y},
              ${target.x - LAYOUT.nodeRadius - 10} ${target.y}`;
}

function updateNodeStatuses(statusMap) {
    graphData.nodes.forEach(node => {
        if (statusMap.has(node.id)) {
            node.status = statusMap.get(node.id);
        }
    });
    renderNodes();
}

function highlightDependencies(nodeId) {
    if (!nodeId) {
        nodesGroup.selectAll(".node").classed("dimmed", false);
        edgesGroup.selectAll(".edge").classed("highlighted", false);
        return;
    }

    // Find all connected nodes (upstream and downstream)
    const connectedNodes = new Set([nodeId]);
    graphData.edges.forEach(edge => {
        if (edge.to === nodeId) connectedNodes.add(edge.from);
        if (edge.from === nodeId) connectedNodes.add(edge.to);
    });

    nodesGroup.selectAll(".node")
        .classed("dimmed", d => !connectedNodes.has(d.id));

    edgesGroup.selectAll(".edge")
        .classed("highlighted", d => d.from === nodeId || d.to === nodeId);
}

function truncateLabel(text, maxLength) {
    if (text.length <= maxLength) return text;
    return text.substring(0, maxLength - 1) + "...";
}
```

### Event Handling Implementation

```javascript
// app.js - Event type handlers

const eventHandlers = {
    "unit.started": (event) => {
        const unit = state.units.find(u => u.id === event.unit);
        if (unit) {
            unit.status = "in_progress";
            updateSummary();
            updateGraphStatus(event.unit, "in_progress");
        }
        addEventLog(event);
    },

    "unit.completed": (event) => {
        const unit = state.units.find(u => u.id === event.unit);
        if (unit) {
            unit.status = "complete";
            updateSummary();
            updateGraphStatus(event.unit, "complete");
        }
        addEventLog(event);
    },

    "unit.failed": (event) => {
        const unit = state.units.find(u => u.id === event.unit);
        if (unit) {
            unit.status = "failed";
            unit.error = event.error;
            updateSummary();
            updateGraphStatus(event.unit, "failed");
        }
        showToast(`Unit "${event.unit}" failed: ${event.error}`, "error");
        addEventLog(event);
    },

    "task.started": (event) => {
        const unit = state.units.find(u => u.id === event.unit);
        if (unit) {
            unit.currentTask = event.task;
        }
        addEventLog(event);
    },

    "task.completed": (event) => {
        const unit = state.units.find(u => u.id === event.unit);
        if (unit) {
            unit.currentTask = event.task + 1;
        }
        addEventLog(event);
    },

    "orch.started": (event) => {
        state.status = "running";
        state.startedAt = event.time;
        updateConnectionStatus();
        addEventLog(event);
    },

    "orch.completed": (event) => {
        state.status = "complete";
        updateConnectionStatus();
        addEventLog(event);
    },

    "orch.failed": (event) => {
        state.status = "failed";
        showToast("Orchestration failed", "error");
        updateConnectionStatus();
        addEventLog(event);
    }
};

function handleEvent(event) {
    const handler = eventHandlers[event.type];
    if (handler) {
        handler(event);
    }
}
```

### HTML Structure

```html
<!-- index.html -->
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

### CSS Styling

```css
/* style.css */

:root {
    --bg-primary: #111827;
    --bg-secondary: #1F2937;
    --bg-tertiary: #374151;
    --text-primary: #F9FAFB;
    --text-secondary: #9CA3AF;
    --border-color: #4B5563;
    --status-pending: #9CA3AF;
    --status-ready: #FBBF24;
    --status-in-progress: #3B82F6;
    --status-pr: #A855F7;
    --status-complete: #22C55E;
    --status-failed: #EF4444;
    --status-blocked: #F97316;
}

* {
    box-sizing: border-box;
    margin: 0;
    padding: 0;
}

body {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    background-color: var(--bg-primary);
    color: var(--text-primary);
    height: 100vh;
    overflow: hidden;
}

.container {
    display: flex;
    height: 100%;
}

/* Sidebar */
.sidebar {
    width: 280px;
    background-color: var(--bg-secondary);
    padding: 20px;
    display: flex;
    flex-direction: column;
    gap: 20px;
    border-right: 1px solid var(--border-color);
}

.status-card {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 16px;
    background-color: var(--bg-tertiary);
    border-radius: 8px;
}

.status-indicator {
    width: 12px;
    height: 12px;
    border-radius: 50%;
    background-color: var(--status-pending);
}

.status-indicator.connected {
    background-color: var(--status-complete);
}

.status-indicator.disconnected {
    background-color: var(--status-failed);
}

.summary-card {
    padding: 16px;
    background-color: var(--bg-tertiary);
    border-radius: 8px;
}

.summary-card h3 {
    margin-bottom: 16px;
    font-size: 14px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-secondary);
}

.summary-grid {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 12px;
}

.stat {
    text-align: center;
}

.stat-value {
    display: block;
    font-size: 24px;
    font-weight: 600;
}

.stat-label {
    font-size: 12px;
    color: var(--text-secondary);
}

.stat[data-status="complete"] .stat-value { color: var(--status-complete); }
.stat[data-status="failed"] .stat-value { color: var(--status-failed); }
.stat[data-status="inProgress"] .stat-value { color: var(--status-in-progress); }
.stat[data-status="blocked"] .stat-value { color: var(--status-blocked); }

/* Toast notifications */
#toast-container {
    display: flex;
    flex-direction: column;
    gap: 8px;
}

.toast {
    padding: 12px 16px;
    border-radius: 8px;
    font-size: 14px;
    animation: slideIn 0.3s ease-out;
}

.toast.error {
    background-color: rgba(239, 68, 68, 0.2);
    border: 1px solid var(--status-failed);
}

.toast.success {
    background-color: rgba(34, 197, 94, 0.2);
    border: 1px solid var(--status-complete);
}

@keyframes slideIn {
    from {
        opacity: 0;
        transform: translateX(-20px);
    }
    to {
        opacity: 1;
        transform: translateX(0);
    }
}

/* Main content */
.main-content {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow: hidden;
}

.graph-area {
    flex: 1;
    min-height: 0;
    position: relative;
}

.graph-area svg {
    width: 100%;
    height: 100%;
}

/* Node styling */
.node {
    cursor: pointer;
    transition: opacity 0.2s;
}

.node.dimmed {
    opacity: 0.3;
}

.node circle {
    transition: fill 0.3s;
}

.node circle.pulse {
    animation: pulse 2s infinite;
}

@keyframes pulse {
    0%, 100% {
        opacity: 1;
    }
    50% {
        opacity: 0.6;
    }
}

.edge {
    transition: stroke 0.2s, stroke-width 0.2s;
}

.edge.highlighted {
    stroke: var(--status-in-progress);
    stroke-width: 3;
}

/* Detail panel */
.detail-panel {
    position: absolute;
    right: 20px;
    top: 20px;
    width: 320px;
    background-color: var(--bg-secondary);
    border: 1px solid var(--border-color);
    border-radius: 8px;
    box-shadow: 0 4px 20px rgba(0, 0, 0, 0.3);
}

.detail-panel.hidden {
    display: none;
}

.detail-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 16px;
    border-bottom: 1px solid var(--border-color);
}

.detail-header h3 {
    font-size: 16px;
    font-weight: 600;
}

.close-btn {
    background: none;
    border: none;
    color: var(--text-secondary);
    font-size: 24px;
    cursor: pointer;
    line-height: 1;
}

.close-btn:hover {
    color: var(--text-primary);
}

.detail-body {
    padding: 16px;
}

.detail-status {
    display: inline-block;
    padding: 4px 12px;
    border-radius: 4px;
    font-size: 12px;
    font-weight: 500;
    text-transform: uppercase;
    margin-bottom: 12px;
}

.detail-progress {
    font-size: 14px;
    color: var(--text-secondary);
    margin-bottom: 12px;
}

.detail-error {
    padding: 12px;
    background-color: rgba(239, 68, 68, 0.1);
    border: 1px solid var(--status-failed);
    border-radius: 4px;
    color: var(--status-failed);
    font-size: 13px;
    margin-bottom: 12px;
    white-space: pre-wrap;
    word-break: break-word;
}

.detail-error.hidden {
    display: none;
}

/* Event log */
.event-log {
    height: 200px;
    background-color: var(--bg-secondary);
    border-top: 1px solid var(--border-color);
    display: flex;
    flex-direction: column;
}

.event-log h4 {
    padding: 12px 16px;
    font-size: 12px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-secondary);
    border-bottom: 1px solid var(--border-color);
}

.event-list {
    flex: 1;
    overflow-y: auto;
    padding: 8px 16px;
    font-family: 'Monaco', 'Menlo', monospace;
    font-size: 12px;
}

.event-item {
    padding: 4px 0;
    color: var(--text-secondary);
}

.event-item .time {
    color: var(--text-secondary);
    margin-right: 8px;
}

.event-item .type {
    color: var(--status-in-progress);
    margin-right: 8px;
}

.event-item.error .type {
    color: var(--status-failed);
}
```

### Main Application Entry Point

```javascript
// app.js - Main application

import { initGraph, updateNodeStatuses, highlightDependencies } from './graph.js';

// State
const state = {
    connected: false,
    status: "waiting",
    startedAt: null,
    parallelism: 0,
    units: [],
    summary: { total: 0, pending: 0, inProgress: 0, complete: 0, failed: 0, blocked: 0 },
    graph: { nodes: [], edges: [], levels: [] },
    events: [],
    selectedUnit: null
};

let sseClient = null;

// Initialize application
async function init() {
    try {
        // Fetch initial state
        const [stateResponse, graphResponse] = await Promise.all([
            fetch('/api/state'),
            fetch('/api/graph')
        ]);

        const stateData = await stateResponse.json();
        const graphData = await graphResponse.json();

        // Update state
        Object.assign(state, stateData);
        state.graph = graphData;

        // Initialize graph visualization
        const container = document.getElementById('graph-container');
        initGraph(container, graphData, {
            onClick: handleNodeClick,
            onHover: handleNodeHover
        });

        // Render initial UI
        renderConnectionStatus();
        renderSummary();

        // Start SSE connection
        connectSSE();

        // Bind event handlers
        document.getElementById('detail-close').addEventListener('click', hideDetailPanel);

    } catch (error) {
        console.error('Failed to initialize:', error);
        showToast('Failed to connect to server', 'error');
    }
}

function connectSSE() {
    sseClient = new SSEClient('/api/events', {
        onConnect: () => {
            state.connected = true;
            renderConnectionStatus();
        },
        onDisconnect: () => {
            state.connected = false;
            renderConnectionStatus();
        },
        onEvent: handleEvent
    });
    sseClient.connect();
}

function handleNodeClick(unitId) {
    state.selectedUnit = unitId;
    showDetailPanel(unitId);
}

function handleNodeHover(unitId) {
    highlightDependencies(unitId);
}

function showDetailPanel(unitId) {
    const unit = state.units.find(u => u.id === unitId);
    if (!unit) return;

    const panel = document.getElementById('detail-panel');
    const title = document.getElementById('detail-title');
    const status = document.getElementById('detail-status');
    const progress = document.getElementById('detail-progress');
    const errorDiv = document.getElementById('detail-error');

    title.textContent = unit.id;
    status.textContent = unit.status;
    status.style.backgroundColor = getStatusColor(unit.status);
    progress.textContent = `Task ${unit.currentTask + 1} of ${unit.totalTasks}`;

    if (unit.error) {
        errorDiv.textContent = unit.error;
        errorDiv.classList.remove('hidden');
    } else {
        errorDiv.classList.add('hidden');
    }

    panel.classList.remove('hidden');
}

function hideDetailPanel() {
    document.getElementById('detail-panel').classList.add('hidden');
    state.selectedUnit = null;
}

function renderConnectionStatus() {
    const indicator = document.querySelector('.status-indicator');
    const text = document.querySelector('.status-text');

    indicator.classList.toggle('connected', state.connected);
    indicator.classList.toggle('disconnected', !state.connected);

    if (!state.connected) {
        text.textContent = 'Disconnected';
    } else if (state.status === 'waiting') {
        text.textContent = 'Waiting for orchestrator';
    } else if (state.status === 'running') {
        text.textContent = 'Running';
    } else if (state.status === 'complete') {
        text.textContent = 'Complete';
    } else if (state.status === 'failed') {
        text.textContent = 'Failed';
    }
}

function renderSummary() {
    Object.entries(state.summary).forEach(([key, value]) => {
        const el = document.querySelector(`.stat[data-status="${key}"] .stat-value`);
        if (el) el.textContent = value;
    });
}

function updateSummary() {
    const summary = { total: 0, pending: 0, inProgress: 0, complete: 0, failed: 0, blocked: 0 };

    state.units.forEach(unit => {
        summary.total++;
        if (unit.status === 'pending' || unit.status === 'ready') summary.pending++;
        else if (unit.status === 'in_progress' || unit.status === 'pr_open' ||
                 unit.status === 'in_review' || unit.status === 'merging') summary.inProgress++;
        else if (unit.status === 'complete') summary.complete++;
        else if (unit.status === 'failed') summary.failed++;
        else if (unit.status === 'blocked') summary.blocked++;
    });

    state.summary = summary;
    renderSummary();
}

function updateGraphStatus(unitId, status) {
    const statusMap = new Map([[unitId, status]]);
    updateNodeStatuses(statusMap);
}

function addEventLog(event) {
    state.events.unshift(event);
    if (state.events.length > 100) state.events.pop();
    renderEventLog();
}

function renderEventLog() {
    const list = document.getElementById('event-list');
    const html = state.events.slice(0, 50).map(event => {
        const time = new Date(event.time).toLocaleTimeString();
        const isError = event.type.includes('failed');
        return `<div class="event-item ${isError ? 'error' : ''}">
            <span class="time">${time}</span>
            <span class="type">${event.type}</span>
            ${event.unit ? `<span class="unit">${event.unit}</span>` : ''}
        </div>`;
    }).join('');
    list.innerHTML = html;
}

function showToast(message, type = 'info') {
    const container = document.getElementById('toast-container');
    const toast = document.createElement('div');
    toast.className = `toast ${type}`;
    toast.textContent = message;
    container.appendChild(toast);

    setTimeout(() => {
        toast.remove();
    }, 5000);
}

function getStatusColor(status) {
    const colors = {
        pending: '#9CA3AF',
        ready: '#FBBF24',
        in_progress: '#3B82F6',
        pr_open: '#A855F7',
        in_review: '#A855F7',
        merging: '#A855F7',
        complete: '#22C55E',
        failed: '#EF4444',
        blocked: '#F97316'
    };
    return colors[status] || colors.pending;
}

// Start application
document.addEventListener('DOMContentLoaded', init);

export { state, showToast, updateGraphStatus };
```

## Implementation Notes

### Browser Compatibility

Target modern browsers (Chrome 90+, Firefox 88+, Safari 14+, Edge 90+). Required features:
- ES6 modules (`type="module"`)
- EventSource API for SSE
- CSS custom properties
- CSS Grid and Flexbox
- Fetch API with async/await

No transpilation or bundling required. D3.js loaded from CDN for simplicity.

### SSE Reconnection Strategy

The SSE client uses exponential backoff starting at 1 second, doubling each retry up to 30 seconds maximum. On successful connection, the delay resets to 1 second. This prevents overwhelming the server during outages while ensuring quick recovery.

When reconnecting, the client should fetch the current state from `/api/state` to synchronize any missed events. The `handleEvent` function is idempotent - processing the same event twice does not corrupt state.

### State Preservation After Orchestrator Exit

When the orchestrator completes or fails, the web server continues running and serving the final state. The SSE stream may close (triggering reconnection attempts), but the UI preserves all displayed data. The server should keep the last known state in memory and continue serving it via `/api/state`.

### Graph Layout Performance

For graphs with many nodes (50+), the level-based layout computation is O(n) where n is the number of nodes. D3 selection updates use key functions to minimize DOM operations. The `updateNodeStatuses` function only updates fill colors without re-computing layout.

For very large graphs (100+ nodes), consider:
- Viewport culling (only render visible nodes)
- Level of detail (collapse distant nodes)
- Pan/zoom controls for navigation

### Memory Management

The event log is capped at 100 events to prevent unbounded memory growth. Toast notifications auto-remove after 5 seconds. The graph maintains references to all nodes but this is bounded by the number of units in the orchestration.

### Edge Cases

1. **Empty graph**: Display "No units" message in graph area
2. **Single node**: Center node without edges
3. **Long unit names**: Truncate with ellipsis in nodes, show full name in detail panel
4. **Many events**: Event log scrolls, oldest events removed
5. **Rapid updates**: D3 transitions handle batched updates smoothly

## Testing Strategy

### Unit Tests

```javascript
// app.test.js - Unit tests with a testing framework like Vitest

import { describe, it, expect, vi, beforeEach } from 'vitest';

describe('SSEClient', () => {
    let client;
    let mockHandlers;

    beforeEach(() => {
        mockHandlers = {
            onConnect: vi.fn(),
            onDisconnect: vi.fn(),
            onEvent: vi.fn()
        };
        client = new SSEClient('/api/events', mockHandlers);
    });

    it('calls onConnect when EventSource opens', () => {
        const mockEventSource = {
            onopen: null,
            onmessage: null,
            onerror: null,
            addEventListener: vi.fn(),
            close: vi.fn()
        };
        global.EventSource = vi.fn(() => mockEventSource);

        client.connect();
        mockEventSource.onopen();

        expect(mockHandlers.onConnect).toHaveBeenCalled();
    });

    it('resets reconnect delay on successful connection', () => {
        client.reconnectDelay = 16000; // Previously backed off

        const mockEventSource = {
            onopen: null,
            addEventListener: vi.fn(),
            close: vi.fn()
        };
        global.EventSource = vi.fn(() => mockEventSource);

        client.connect();
        mockEventSource.onopen();

        expect(client.reconnectDelay).toBe(1000);
    });

    it('doubles reconnect delay on error up to max', () => {
        client.reconnectDelay = 1000;

        client.scheduleReconnect();
        expect(client.reconnectDelay).toBe(2000);

        client.reconnectDelay = 16000;
        client.scheduleReconnect();
        expect(client.reconnectDelay).toBe(30000); // Capped at max
    });
});

describe('Event Handlers', () => {
    it('updates unit status on unit.started', () => {
        const state = {
            units: [{ id: 'test-unit', status: 'pending' }],
            summary: {}
        };

        eventHandlers['unit.started']({ type: 'unit.started', unit: 'test-unit', time: new Date().toISOString() });

        expect(state.units[0].status).toBe('in_progress');
    });

    it('shows toast on unit.failed', () => {
        const showToast = vi.fn();

        eventHandlers['unit.failed']({
            type: 'unit.failed',
            unit: 'test-unit',
            error: 'Build failed',
            time: new Date().toISOString()
        });

        expect(showToast).toHaveBeenCalledWith(
            expect.stringContaining('test-unit'),
            'error'
        );
    });
});

describe('Summary Calculation', () => {
    it('correctly counts statuses', () => {
        const units = [
            { id: 'a', status: 'pending' },
            { id: 'b', status: 'in_progress' },
            { id: 'c', status: 'complete' },
            { id: 'd', status: 'complete' },
            { id: 'e', status: 'failed' }
        ];

        const summary = calculateSummary(units);

        expect(summary).toEqual({
            total: 5,
            pending: 1,
            inProgress: 1,
            complete: 2,
            failed: 1,
            blocked: 0
        });
    });

    it('groups PR statuses under inProgress', () => {
        const units = [
            { id: 'a', status: 'pr_open' },
            { id: 'b', status: 'in_review' },
            { id: 'c', status: 'merging' }
        ];

        const summary = calculateSummary(units);

        expect(summary.inProgress).toBe(3);
    });
});
```

### Integration Tests

- Load page and verify graph renders with mock data
- Verify SSE events update node colors
- Click node and verify detail panel opens with correct data
- Hover node and verify dependency highlighting
- Simulate disconnect and verify reconnection
- Verify event log scrolls and limits entries
- Test with varying numbers of nodes (1, 10, 50, 100)

### Manual Testing

- [ ] Page loads and displays "Waiting for orchestrator" state
- [ ] Graph renders nodes at correct positions by level
- [ ] Edges connect dependent nodes with arrows
- [ ] Node colors match status (pending=gray, complete=green, etc.)
- [ ] In-progress nodes pulse with animation
- [ ] Clicking node opens detail panel
- [ ] Detail panel shows task progress
- [ ] Failed node detail panel shows error message
- [ ] Hovering node highlights its dependencies
- [ ] Summary panel shows correct counts
- [ ] Toast appears on failure event
- [ ] Toast auto-dismisses after 5 seconds
- [ ] Event log shows recent events
- [ ] Event log scrolls for many events
- [ ] SSE reconnects after network interruption
- [ ] Connection indicator reflects actual state
- [ ] Final state preserved after orchestrator exits
- [ ] UI responsive at various window sizes

## Design Decisions

### Why D3.js for Graph Visualization?

D3.js provides precise control over SVG rendering needed for custom node shapes, edge curves, and animations. Alternatives considered:

- **vis.js**: Easier setup but limited customization for status-based styling
- **Cytoscape.js**: Feature-rich but heavier, overkill for static layout
- **Canvas rendering**: Better for 1000+ nodes but loses SVG interactivity

D3.js strikes the right balance: flexible enough for custom styling, performant for 100 nodes, and well-documented.

### Why Level-Based Layout Instead of Force-Directed?

Force-directed layouts look organic but have downsides:
- Non-deterministic: graph looks different each load
- Requires settling time before stable
- Can produce layouts where dependencies flow "backward"

Level-based layout ensures:
- Dependencies always flow left-to-right
- Consistent, predictable positioning
- Instant rendering without settling

### Why SSE Instead of WebSocket?

SSE is unidirectional server-to-client, which matches our use case (server pushes updates, client only observes). Benefits:

- Simpler server implementation (HTTP response with streaming)
- Automatic reconnection built into EventSource API
- Works through HTTP proxies without special configuration
- No need for ping/pong heartbeats

WebSocket would be needed if the client sent frequent messages, but clicking nodes doesn't require server roundtrips.

### Why No Build Step?

Using vanilla JavaScript with ES modules and CDN-hosted D3.js:

- Zero build configuration to maintain
- Faster development iteration
- Easier debugging (no source maps needed)
- Works in any modern browser directly

For a monitoring dashboard that's not performance-critical, the simplicity outweighs the benefits of bundling and minification.

## Future Enhancements

1. **Pan and zoom** - Allow navigating large graphs with mouse wheel zoom and drag pan
2. **Search/filter** - Text input to highlight matching units
3. **Time travel** - Slider to replay orchestration events
4. **Export** - Download graph as SVG or PNG
5. **Dark/light themes** - Toggle between color schemes
6. **Mobile layout** - Responsive design for smaller screens
7. **Keyboard navigation** - Arrow keys to move between nodes
8. **Sound alerts** - Audio notification on failure (optional)
9. **Multiple orchestrations** - Tabs to view different runs
10. **Metrics overlay** - Show duration, memory usage per unit

## References

- WEB spec for server-side implementation
- ORCHESTRATOR spec for event types and state machine
- [D3.js documentation](https://d3js.org/)
- [Server-Sent Events specification](https://html.spec.whatwg.org/multipage/server-sent-events.html)
- [CSS Custom Properties](https://developer.mozilla.org/en-US/docs/Web/CSS/Using_CSS_custom_properties)
