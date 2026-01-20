---
task: 4
status: complete
backpressure: "test -f internal/web/static/app.js && grep -q 'export.*state' internal/web/static/app.js && grep -q 'SSEClient' internal/web/static/app.js"
depends_on: [1, 2, 3]
---

# Main App Module

**Parent spec**: `/specs/WEB-FRONTEND.md`
**Task**: #4 of 4

## Objective

Create the main application module with state management, SSE client for real-time updates, event handlers, UI rendering functions, and coordination between graph and UI components.

## Dependencies

### Task Dependencies
- Task #1 (HTML structure with all container elements)
- Task #2 (CSS styling for UI components)
- Task #3 (graph.js module for visualization)

### External Dependencies
- D3.js v7 (loaded from CDN in HTML)

## Deliverables

### Files to Create

```
internal/web/static/
└── app.js    # CREATE: Main application module
```

### JavaScript Implementation

```javascript
// app.js - Main application

import { initGraph, updateNodeStatuses, highlightDependencies } from './graph.js';

// Application state
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

// SSE Client class with exponential backoff reconnection
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
            this.reconnectDelay = 1000;
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

// Event handlers by event type
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
        renderConnectionStatus();
        addEventLog(event);
    },

    "orch.completed": (event) => {
        state.status = "complete";
        renderConnectionStatus();
        addEventLog(event);
    },

    "orch.failed": (event) => {
        state.status = "failed";
        showToast("Orchestration failed", "error");
        renderConnectionStatus();
        addEventLog(event);
    }
};

function handleEvent(event) {
    const handler = eventHandlers[event.type];
    if (handler) {
        handler(event);
    }
}

// Initialize application
async function init() {
    try {
        // Fetch initial state
        const [stateResponse, graphResponse] = await Promise.all([
            fetch('/api/state'),
            fetch('/api/graph')
        ]);

        if (stateResponse.ok) {
            const stateData = await stateResponse.json();
            Object.assign(state, stateData);
        }

        if (graphResponse.ok) {
            const graphData = await graphResponse.json();
            state.graph = graphData;
        }

        // Initialize graph visualization
        const container = document.getElementById('graph-container');
        if (container && state.graph.nodes.length > 0) {
            initGraph(container, state.graph, {
                onClick: handleNodeClick,
                onHover: handleNodeHover
            });
        }

        // Render initial UI
        renderConnectionStatus();
        renderSummary();

        // Start SSE connection
        connectSSE();

        // Bind event handlers
        document.getElementById('detail-close')?.addEventListener('click', hideDetailPanel);

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

    if (!panel) return;

    title.textContent = unit.id;
    status.textContent = unit.status;
    status.style.backgroundColor = getStatusColor(unit.status);
    progress.textContent = `Task ${(unit.currentTask || 0) + 1} of ${unit.totalTasks || 0}`;

    if (unit.error) {
        errorDiv.textContent = unit.error;
        errorDiv.classList.remove('hidden');
    } else {
        errorDiv.classList.add('hidden');
    }

    panel.classList.remove('hidden');
}

function hideDetailPanel() {
    document.getElementById('detail-panel')?.classList.add('hidden');
    state.selectedUnit = null;
}

function renderConnectionStatus() {
    const indicator = document.querySelector('.status-indicator');
    const text = document.querySelector('.status-text');

    if (!indicator || !text) return;

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
    if (!list) return;

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
    if (!container) return;

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

// Start application when DOM is ready
document.addEventListener('DOMContentLoaded', init);

// Export for testing and external access
export { state, showToast, updateGraphStatus, handleEvent };
```

## Backpressure

### Validation Command

```bash
test -f internal/web/static/app.js && \
grep -q 'export.*state' internal/web/static/app.js && \
grep -q 'SSEClient' internal/web/static/app.js && \
grep -q 'handleEvent' internal/web/static/app.js && \
grep -q "import.*from.*graph.js" internal/web/static/app.js
```

### Must Pass
- File exists at `internal/web/static/app.js`
- Exports `state` object
- Contains `SSEClient` class with reconnection logic
- Contains `handleEvent` function for SSE events
- Imports from `graph.js`
- Contains event handlers for all event types
- Contains UI rendering functions (renderConnectionStatus, renderSummary, etc.)
- Contains `showToast` function
- Initializes on DOMContentLoaded

### CI Compatibility
- [x] No external API keys required
- [x] No network access required for validation (static file check only)
- [x] Runs in <60 seconds

## NOT In Scope
- Server-side API implementation (WEB spec)
- Actual orchestrator integration
- Unit tests (separate testing task)
- Browser compatibility polyfills
