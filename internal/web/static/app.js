// app.js - Main application

import { initGraph, updateNodeStatuses, highlightDependencies, updateTaskProgress } from './graph.js';

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
            'orch.started', 'orch.completed', 'orch.failed',
            'orch.dryrun.started', 'orch.dryrun.completed'
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
            // Extract task info from payload if available
            if (event.payload) {
                unit.totalTasks = event.payload.total_tasks || unit.totalTasks || 0;
                unit.currentTask = event.payload.completed_tasks || 0;
            }
            updateSummary();
            updateGraphStatus(event.unit, "in_progress");
            // Update graph progress blocks
            updateTaskProgress(event.unit, unit.currentTask, unit.currentTask);
        }
        addEventLog(event);
    },

    "unit.completed": (event) => {
        const unit = state.units.find(u => u.id === event.unit);
        if (unit) {
            unit.status = "complete";
            // Mark all tasks as complete
            unit.completedTasks = unit.totalTasks || 0;
            unit.currentTask = -1; // No current task
            updateSummary();
            updateGraphStatus(event.unit, "complete");
            // Update graph progress blocks (all complete, none current)
            updateTaskProgress(event.unit, -1, unit.completedTasks);
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
        if (unit && event.task != null) {
            // Convert 1-indexed task number to 0-indexed for display
            unit.currentTask = event.task - 1;
            // Update graph progress blocks (currentTask is the one being worked on)
            updateTaskProgress(event.unit, unit.currentTask, unit.completedTasks || 0);
        }
        addEventLog(event);
    },

    "task.completed": (event) => {
        const unit = state.units.find(u => u.id === event.unit);
        if (unit && event.task != null) {
            // Track completed tasks (1-indexed task number means tasks 1..N are done)
            unit.completedTasks = event.task;
            // Update graph progress blocks
            updateTaskProgress(event.unit, unit.currentTask, unit.completedTasks);
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
    },

    "orch.dryrun.started": (event) => {
        state.status = "running";
        state.startedAt = event.time;
        renderConnectionStatus();
        addEventLog(event);
    },

    "orch.dryrun.completed": (event) => {
        state.status = "complete";
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
            // Sync initial statuses and task progress from units to graph nodes
            state.units.forEach(unit => {
                const node = state.graph.nodes.find(n => n.id === unit.id);
                if (node) {
                    node.status = unit.status;
                    // Sync task counts - node.tasks comes from graph, unit has currentTask/totalTasks
                    if (unit.totalTasks) {
                        node.tasks = unit.totalTasks;
                    }
                    // For completed units, show all tasks as complete
                    if (unit.status === 'complete') {
                        node.completedTasks = node.tasks || 0;
                        node.currentTask = -1;
                    } else if (unit.status === 'in_progress') {
                        node.currentTask = unit.currentTask ?? 0;
                        node.completedTasks = unit.currentTask ?? 0;
                    } else {
                        node.currentTask = -1;
                        node.completedTasks = 0;
                    }
                }
            });

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
