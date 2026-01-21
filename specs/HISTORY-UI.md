# HISTORY-UI — Frontend JavaScript Module for Historical Runs Visualization

## Overview

The History UI module extends the choo web interface with a history view that enables users to browse and analyze past orchestration runs. It provides a complete audit trail of orchestration activity, scoped to the current repository.

The module consists of a JavaScript class (`HistoryView`) that handles fetching run data, rendering run lists, displaying run details with the existing D3 graph visualization, and showing event timelines with visual markers for stop/resume boundaries. This allows users to understand not just what happened during a run, but also how runs were interrupted and resumed over time.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Web Interface                                   │
├─────────────────────────────────────────────────────────────────────────────┤
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │  Navigation: [ Live ] [ History ]                                     │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │ History View                                                         │    │
│  │  ┌───────────────────────┐  ┌─────────────────────────────────────┐ │    │
│  │  │     Run List          │  │          Run Detail                  │ │    │
│  │  │  ┌─────────────────┐  │  │  ┌─────────────────────────────────┐│ │    │
│  │  │  │ Run 1 [complete]│  │  │  │        D3 Graph (reused)        ││ │    │
│  │  │  │ Run 2 [failed]  │  │  │  └─────────────────────────────────┘│ │    │
│  │  │  │ Run 3 [resumed] │  │  │  ┌─────────────────────────────────┐│ │    │
│  │  │  │ ...             │  │  │  │     Event Timeline              ││ │    │
│  │  │  └─────────────────┘  │  │  │  ○─●─●─▌STOP▐─●─●─●─▌STOP▐─●   ││ │    │
│  │  └───────────────────────┘  │  └─────────────────────────────────┘│ │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Requirements

### Functional Requirements

1. Display a navigation bar with Live/History tabs to switch between views
2. Fetch and display a paginated list of historical runs for the current repository
3. Show run metadata in the list: ID, status badge, unit counts, duration, resume count
4. Load and display full run details when a run is selected
5. Reuse the existing D3 graph visualization to display the dependency graph for historical runs
6. Display an event timeline that shows all events for a run in chronological order
7. Color-code events by type in the timeline (unit events, task events, orchestrator events)
8. Show visual markers for stop/resume boundaries in the timeline
9. Support filtering events by type and unit
10. Make the event timeline scrollable for runs with many events
11. Persist the selected view (Live/History) across page refreshes
12. Handle loading and error states with appropriate UI feedback

### Performance Requirements

| Metric | Target |
|--------|--------|
| Initial run list load | <500ms for first 20 runs |
| Run detail load | <300ms including graph data |
| Event timeline render | <100ms for up to 1000 events |
| View switch latency | <50ms (no network) |
| Pagination fetch | <200ms per page |

### Constraints

- Must reuse the existing D3 graph module (`graph.js`) without modification
- Must work with the existing API endpoints defined in the backend
- Must integrate with the existing `app.js` state management pattern
- Must maintain visual consistency with the existing Live view styling
- D3.js v7 is already loaded globally via CDN

## Design

### Shared Types

All shared types (Run, RunStatus, StoredEvent, GraphData, ListOptions, EventListOptions) are defined in [HISTORY-TYPES.md](./HISTORY-TYPES.md). The UI implements JavaScript equivalents of these types for client-side use.

### Module Structure

```
internal/web/static/
├── index.html      # MODIFY - Add navigation and history view containers
├── app.js          # MODIFY - Add view switching logic
├── graph.js        # EXISTING - Reused for history graph display
├── history.js      # CREATE - History view module
└── style.css       # MODIFY - Add history-specific styles
```

### UI Types (JavaScript)

These types are JavaScript equivalents of the types defined in [HISTORY-TYPES.md](./HISTORY-TYPES.md).

```javascript
// internal/web/static/history.js

/**
 * Represents a historical run summary for the run list.
 * Maps to Run type in HISTORY-TYPES.md.
 * @typedef {Object} RunSummary
 * @property {string} id - Unique run identifier
 * @property {string} status - Run status: "running" | "completed" | "failed" | "stopped"
 * @property {number} totalUnits - Total number of units in the run
 * @property {number} completedUnits - Number of completed units
 * @property {number} failedUnits - Number of failed units
 * @property {string} startedAt - ISO 8601 timestamp of run start
 * @property {string|null} completedAt - ISO 8601 timestamp of run end, null if still running
 * @property {number} duration - Run duration in milliseconds
 * @property {number} resumeCount - Number of times the run was resumed (derived from events)
 */

/**
 * Represents full run details including graph and events.
 * @typedef {Object} RunDetail
 * @property {string} id - Unique run identifier
 * @property {string} status - Run status per HISTORY-TYPES.md RunStatus
 * @property {string} repoPath - Repository path this run belongs to
 * @property {Object} graph - Graph data per HISTORY-TYPES.md GraphData
 * @property {Array<RunEvent>} events - All events for this run
 * @property {Object} summary - Unit counts by status
 */

/**
 * Represents a single event in the run timeline.
 * Maps to StoredEvent type in HISTORY-TYPES.md.
 * @typedef {Object} RunEvent
 * @property {number} seq - Event sequence number
 * @property {string} type - Event type per HISTORY-TYPES.md (e.g., "unit.started", "run.stopped")
 * @property {string} time - ISO 8601 timestamp
 * @property {string|null} unit - Unit ID if applicable
 * @property {number|null} task - Task number if applicable
 * @property {string|null} error - Error message if applicable
 * @property {Object|null} payload - Additional event-specific data
 */

/**
 * Configuration for event filtering.
 * Maps to EventListOptions in HISTORY-TYPES.md.
 * @typedef {Object} EventFilter
 * @property {string|null} type - Filter by event type prefix (e.g., "unit", "task")
 * @property {string|null} unit - Filter by specific unit ID
 */
```

### API Surface

```javascript
// internal/web/static/history.js

/**
 * HistoryView manages the historical runs interface.
 */
export class HistoryView {
    /**
     * Create a new HistoryView instance.
     * @param {HTMLElement} container - The container element for the history view
     */
    constructor(container)

    /**
     * Fetch paginated runs for a repository.
     * Uses limit/offset pagination per HISTORY-TYPES.md.
     * @param {string} repoPath - Repository path to filter runs
     * @param {number} page - Page number (1-indexed, converted to offset internally)
     * @returns {Promise<{runs: RunSummary[], total: number, hasMore: boolean}>}
     */
    async loadRuns(repoPath, page = 1)

    /**
     * Load full details for a specific run.
     * @param {string} runId - The run ID to load
     * @returns {Promise<RunDetail>}
     */
    async selectRun(runId)

    /**
     * Render the run detail view with graph and timeline.
     * Called automatically after selectRun().
     */
    renderRunDetail()

    /**
     * Render visual markers for stop/resume boundaries in the timeline.
     * Called automatically by renderRunDetail().
     */
    renderResumeMarkers()

    /**
     * Apply filters to the event timeline.
     * @param {EventFilter} filter - Filter configuration
     */
    filterEvents(filter)

    /**
     * Clean up resources when view is hidden.
     */
    destroy()
}

/**
 * Switch between Live and History views.
 * @param {string} view - "live" or "history"
 */
export function switchView(view)

/**
 * Initialize view switching and restore last selected view.
 */
export function initNavigation()
```

### HTML Structure

```html
<!-- internal/web/static/index.html -->
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
            <!-- Navigation tabs -->
            <nav class="main-nav">
                <button id="nav-live" class="nav-btn active">Live</button>
                <button id="nav-history" class="nav-btn">History</button>
            </nav>

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
            <!-- Live View (existing) -->
            <div id="live-view" class="view-panel">
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
            </div>

            <!-- History View (new) -->
            <div id="history-view" class="view-panel hidden">
                <div class="history-layout">
                    <!-- Run List Panel -->
                    <div class="history-list">
                        <div class="history-list-header">
                            <h3>Historical Runs</h3>
                            <div class="history-pagination">
                                <button id="history-prev" class="pagination-btn" disabled>&larr;</button>
                                <span id="history-page-info">Page 1</span>
                                <button id="history-next" class="pagination-btn">&rarr;</button>
                            </div>
                        </div>
                        <div id="history-run-list" class="run-list">
                            <!-- Run items rendered here -->
                        </div>
                        <div id="history-loading" class="loading-indicator hidden">
                            Loading runs...
                        </div>
                        <div id="history-empty" class="empty-state hidden">
                            No historical runs found for this repository.
                        </div>
                    </div>

                    <!-- Run Detail Panel -->
                    <div class="history-detail">
                        <div id="history-detail-placeholder" class="detail-placeholder">
                            Select a run to view details
                        </div>
                        <div id="history-detail-content" class="detail-content hidden">
                            <div class="history-detail-header">
                                <h3 id="history-run-title">Run ID</h3>
                                <span id="history-run-status" class="status-badge">complete</span>
                            </div>
                            <div class="history-detail-meta">
                                <span id="history-run-duration"></span>
                                <span id="history-run-units"></span>
                                <span id="history-run-resumes"></span>
                            </div>

                            <!-- Graph visualization -->
                            <div id="history-graph-container" class="history-graph"></div>

                            <!-- Event timeline -->
                            <div class="history-timeline-section">
                                <div class="timeline-header">
                                    <h4>Event Timeline</h4>
                                    <div class="timeline-filters">
                                        <select id="history-event-type-filter">
                                            <option value="">All Events</option>
                                            <option value="unit">Unit Events</option>
                                            <option value="task">Task Events</option>
                                            <option value="orch">Orchestrator Events</option>
                                            <option value="run">Run Events</option>
                                        </select>
                                        <select id="history-event-unit-filter">
                                            <option value="">All Units</option>
                                            <!-- Unit options populated dynamically -->
                                        </select>
                                    </div>
                                </div>
                                <div id="history-timeline" class="event-timeline">
                                    <!-- Timeline events rendered here -->
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </main>
    </div>

    <script type="module" src="app.js"></script>
</body>
</html>
```

### Complete JavaScript Implementation

```javascript
// internal/web/static/history.js

import { initGraph, updateNodeStatuses, STATUS_COLORS } from './graph.js';

/**
 * Event type color mapping for timeline visualization.
 */
const EVENT_COLORS = {
    'unit.started': '#3B82F6',      // Blue
    'unit.completed': '#22C55E',    // Green
    'unit.failed': '#EF4444',       // Red
    'task.started': '#60A5FA',      // Light blue
    'task.completed': '#4ADE80',    // Light green
    'orch.started': '#A855F7',      // Purple
    'orch.completed': '#A855F7',    // Purple
    'orch.failed': '#EF4444',       // Red
    'run.stopped': '#F97316',       // Orange
    'run.resumed': '#FBBF24',       // Yellow
    'default': '#9CA3AF'            // Gray
};

/**
 * HistoryView manages the historical runs interface.
 */
export class HistoryView {
    /**
     * Create a new HistoryView instance.
     * @param {HTMLElement} container - The container element for the history view
     */
    constructor(container) {
        this.container = container;
        this.currentRun = null;
        this.runs = [];
        this.currentPage = 1;
        this.totalPages = 1;
        this.pageSize = 20;
        this.repoPath = null;
        this.eventFilter = { type: null, unit: null };
        this.graphInitialized = false;

        this.bindElements();
        this.bindEventListeners();
    }

    /**
     * Cache DOM element references.
     */
    bindElements() {
        this.elements = {
            runList: this.container.querySelector('#history-run-list'),
            loading: this.container.querySelector('#history-loading'),
            empty: this.container.querySelector('#history-empty'),
            prevBtn: this.container.querySelector('#history-prev'),
            nextBtn: this.container.querySelector('#history-next'),
            pageInfo: this.container.querySelector('#history-page-info'),
            placeholder: this.container.querySelector('#history-detail-placeholder'),
            detailContent: this.container.querySelector('#history-detail-content'),
            runTitle: this.container.querySelector('#history-run-title'),
            runStatus: this.container.querySelector('#history-run-status'),
            runDuration: this.container.querySelector('#history-run-duration'),
            runUnits: this.container.querySelector('#history-run-units'),
            runResumes: this.container.querySelector('#history-run-resumes'),
            graphContainer: this.container.querySelector('#history-graph-container'),
            timeline: this.container.querySelector('#history-timeline'),
            typeFilter: this.container.querySelector('#history-event-type-filter'),
            unitFilter: this.container.querySelector('#history-event-unit-filter')
        };
    }

    /**
     * Set up event listeners for user interactions.
     */
    bindEventListeners() {
        this.elements.prevBtn?.addEventListener('click', () => this.goToPage(this.currentPage - 1));
        this.elements.nextBtn?.addEventListener('click', () => this.goToPage(this.currentPage + 1));
        this.elements.typeFilter?.addEventListener('change', (e) => {
            this.eventFilter.type = e.target.value || null;
            this.renderTimeline();
        });
        this.elements.unitFilter?.addEventListener('change', (e) => {
            this.eventFilter.unit = e.target.value || null;
            this.renderTimeline();
        });
    }

    /**
     * Fetch paginated runs for a repository.
     * Uses limit/offset pagination per HISTORY-TYPES.md.
     * @param {string} repoPath - Repository path to filter runs
     * @param {number} page - Page number (1-indexed, converted to offset internally)
     * @returns {Promise<{runs: Array, total: number, hasMore: boolean}>}
     */
    async loadRuns(repoPath, page = 1) {
        this.repoPath = repoPath;
        this.currentPage = page;

        this.showLoading(true);
        this.showEmpty(false);

        try {
            // Convert page to limit/offset per HISTORY-TYPES.md
            const limit = this.pageSize;
            const offset = (page - 1) * this.pageSize;
            const url = `/api/history/runs?repo=${encodeURIComponent(repoPath)}&limit=${limit}&offset=${offset}`;
            const response = await fetch(url);

            if (!response.ok) {
                throw new Error(`Failed to fetch runs: ${response.status}`);
            }

            const data = await response.json();
            this.runs = data.runs || [];
            this.totalPages = Math.ceil((data.total || 0) / this.pageSize);

            this.renderRunList();
            this.updatePagination();

            if (this.runs.length === 0) {
                this.showEmpty(true);
            }

            return data;
        } catch (error) {
            console.error('Failed to load runs:', error);
            this.showError('Failed to load historical runs');
            throw error;
        } finally {
            this.showLoading(false);
        }
    }

    /**
     * Load full details for a specific run.
     * @param {string} runId - The run ID to load
     * @returns {Promise<Object>}
     */
    async selectRun(runId) {
        try {
            // Fetch run details, graph, and events in parallel
            const [detailResponse, graphResponse, eventsResponse] = await Promise.all([
                fetch(`/api/history/runs/${runId}`),
                fetch(`/api/history/runs/${runId}/graph`),
                fetch(`/api/history/runs/${runId}/events`)
            ]);

            if (!detailResponse.ok || !graphResponse.ok || !eventsResponse.ok) {
                throw new Error('Failed to fetch run details');
            }

            const [detail, graph, events] = await Promise.all([
                detailResponse.json(),
                graphResponse.json(),
                eventsResponse.json()
            ]);

            this.currentRun = {
                ...detail,
                graph: graph,
                events: events.events || []
            };

            this.renderRunDetail();
            return this.currentRun;
        } catch (error) {
            console.error('Failed to load run details:', error);
            this.showError('Failed to load run details');
            throw error;
        }
    }

    /**
     * Render the list of runs.
     */
    renderRunList() {
        if (!this.elements.runList) return;

        const html = this.runs.map(run => this.renderRunListItem(run)).join('');
        this.elements.runList.innerHTML = html;

        // Bind click handlers
        this.elements.runList.querySelectorAll('.run-item').forEach(item => {
            item.addEventListener('click', () => {
                const runId = item.dataset.runId;
                this.selectRun(runId);

                // Update selected state
                this.elements.runList.querySelectorAll('.run-item').forEach(el => {
                    el.classList.remove('selected');
                });
                item.classList.add('selected');
            });
        });
    }

    /**
     * Render a single run list item.
     * @param {Object} run - Run summary data
     * @returns {string} HTML string
     */
    renderRunListItem(run) {
        const duration = this.formatDuration(run.duration);
        const startTime = new Date(run.startedAt).toLocaleString();
        const statusClass = this.getStatusClass(run.status);

        return `
            <div class="run-item" data-run-id="${run.id}">
                <div class="run-item-header">
                    <span class="run-id">${this.truncateId(run.id)}</span>
                    <span class="status-badge ${statusClass}">${run.status}</span>
                </div>
                <div class="run-item-meta">
                    <span class="run-time">${startTime}</span>
                    <span class="run-duration">${duration}</span>
                </div>
                <div class="run-item-stats">
                    <span class="unit-count">${run.completeCount}/${run.unitCount} units</span>
                    ${run.resumeCount > 0 ? `<span class="resume-count">${run.resumeCount} resumes</span>` : ''}
                    ${run.failedCount > 0 ? `<span class="failed-count">${run.failedCount} failed</span>` : ''}
                </div>
            </div>
        `;
    }

    /**
     * Render the run detail view with graph and timeline.
     */
    renderRunDetail() {
        if (!this.currentRun) return;

        const run = this.currentRun;

        // Show detail content, hide placeholder
        this.elements.placeholder?.classList.add('hidden');
        this.elements.detailContent?.classList.remove('hidden');

        // Update header info
        if (this.elements.runTitle) {
            this.elements.runTitle.textContent = `Run: ${this.truncateId(run.id)}`;
        }
        if (this.elements.runStatus) {
            this.elements.runStatus.textContent = run.status;
            this.elements.runStatus.className = `status-badge ${this.getStatusClass(run.status)}`;
        }
        if (this.elements.runDuration) {
            this.elements.runDuration.textContent = `Duration: ${this.formatDuration(run.duration)}`;
        }
        if (this.elements.runUnits) {
            const summary = run.summary || {};
            this.elements.runUnits.textContent = `Units: ${summary.complete || 0}/${summary.total || 0} complete`;
        }
        if (this.elements.runResumes) {
            const resumeCount = this.countResumes(run.events);
            this.elements.runResumes.textContent = resumeCount > 0 ? `Resumed ${resumeCount}x` : '';
        }

        // Initialize graph
        this.initializeGraph();

        // Populate unit filter dropdown
        this.populateUnitFilter();

        // Render timeline
        this.renderTimeline();
    }

    /**
     * Initialize or update the D3 graph visualization.
     */
    initializeGraph() {
        if (!this.elements.graphContainer || !this.currentRun?.graph) return;

        const graphData = this.currentRun.graph;

        // Apply status from events to nodes
        const statusMap = this.computeNodeStatuses();
        graphData.nodes.forEach(node => {
            if (statusMap.has(node.id)) {
                node.status = statusMap.get(node.id);
            }
        });

        // Initialize graph with click handler
        initGraph(this.elements.graphContainer, graphData, {
            onClick: (unitId) => this.scrollToUnitEvents(unitId),
            onHover: null
        });

        this.graphInitialized = true;
    }

    /**
     * Compute final node statuses from events.
     * @returns {Map<string, string>}
     */
    computeNodeStatuses() {
        const statusMap = new Map();

        if (!this.currentRun?.events) return statusMap;

        // Process events in order to determine final status
        for (const event of this.currentRun.events) {
            if (event.unit) {
                if (event.type === 'unit.completed') {
                    statusMap.set(event.unit, 'complete');
                } else if (event.type === 'unit.failed') {
                    statusMap.set(event.unit, 'failed');
                } else if (event.type === 'unit.started') {
                    // Only set in_progress if not already complete/failed
                    if (!statusMap.has(event.unit)) {
                        statusMap.set(event.unit, 'in_progress');
                    }
                }
            }
        }

        return statusMap;
    }

    /**
     * Populate the unit filter dropdown with units from the current run.
     */
    populateUnitFilter() {
        if (!this.elements.unitFilter || !this.currentRun?.graph?.nodes) return;

        const units = this.currentRun.graph.nodes.map(n => n.id).sort();

        const options = ['<option value="">All Units</option>'];
        units.forEach(unit => {
            options.push(`<option value="${unit}">${unit}</option>`);
        });

        this.elements.unitFilter.innerHTML = options.join('');
    }

    /**
     * Render the event timeline with stop/resume markers.
     */
    renderTimeline() {
        if (!this.elements.timeline || !this.currentRun?.events) return;

        let events = this.currentRun.events;

        // Apply filters
        if (this.eventFilter.type) {
            events = events.filter(e => e.type.startsWith(this.eventFilter.type));
        }
        if (this.eventFilter.unit) {
            events = events.filter(e => e.unit === this.eventFilter.unit);
        }

        const html = events.map(event => this.renderTimelineEvent(event)).join('');
        this.elements.timeline.innerHTML = html;

        // Add resume markers
        this.renderResumeMarkers();
    }

    /**
     * Render a single timeline event.
     * @param {Object} event - Event data
     * @returns {string} HTML string
     */
    renderTimelineEvent(event) {
        const time = new Date(event.time).toLocaleTimeString();
        const color = EVENT_COLORS[event.type] || EVENT_COLORS.default;
        const isStopResume = event.type === 'run.stopped' || event.type === 'run.resumed';
        const markerClass = isStopResume ? 'resume-marker' : '';
        const eventClass = event.type.includes('failed') ? 'error' : '';

        return `
            <div class="timeline-event ${markerClass} ${eventClass}" data-seq="${event.seq}" data-type="${event.type}">
                <div class="timeline-marker" style="background-color: ${color}"></div>
                <div class="timeline-content">
                    <div class="timeline-header">
                        <span class="timeline-time">${time}</span>
                        <span class="timeline-type">${event.type}</span>
                        ${event.unit ? `<span class="timeline-unit">${event.unit}</span>` : ''}
                    </div>
                    ${event.task !== null && event.task !== undefined ? `<div class="timeline-task">Task #${event.task}</div>` : ''}
                    ${event.error ? `<div class="timeline-error">${event.error}</div>` : ''}
                    ${event.type === 'run.stopped' && event.data?.reason ? `<div class="timeline-reason">Reason: ${event.data.reason}</div>` : ''}
                </div>
            </div>
        `;
    }

    /**
     * Render visual markers for stop/resume boundaries in the timeline.
     */
    renderResumeMarkers() {
        if (!this.elements.timeline) return;

        // Find all stop/resume events and add special styling
        const stopEvents = this.elements.timeline.querySelectorAll('[data-type="run.stopped"]');
        const resumeEvents = this.elements.timeline.querySelectorAll('[data-type="run.resumed"]');

        stopEvents.forEach(el => {
            el.classList.add('stop-boundary');
        });

        resumeEvents.forEach(el => {
            el.classList.add('resume-boundary');
        });
    }

    /**
     * Scroll the timeline to events for a specific unit.
     * @param {string} unitId - Unit ID to scroll to
     */
    scrollToUnitEvents(unitId) {
        if (!this.elements.timeline) return;

        // Clear unit filter and find first event for this unit
        this.elements.unitFilter.value = unitId;
        this.eventFilter.unit = unitId;
        this.renderTimeline();

        // Scroll to top of timeline
        this.elements.timeline.scrollTop = 0;
    }

    /**
     * Apply filters to the event timeline.
     * @param {Object} filter - Filter configuration {type, unit}
     */
    filterEvents(filter) {
        this.eventFilter = { ...this.eventFilter, ...filter };
        this.renderTimeline();
    }

    /**
     * Navigate to a specific page of runs.
     * @param {number} page - Page number
     */
    goToPage(page) {
        if (page < 1 || page > this.totalPages) return;
        this.loadRuns(this.repoPath, page);
    }

    /**
     * Update pagination controls.
     */
    updatePagination() {
        if (this.elements.prevBtn) {
            this.elements.prevBtn.disabled = this.currentPage <= 1;
        }
        if (this.elements.nextBtn) {
            this.elements.nextBtn.disabled = this.currentPage >= this.totalPages;
        }
        if (this.elements.pageInfo) {
            this.elements.pageInfo.textContent = `Page ${this.currentPage} of ${this.totalPages}`;
        }
    }

    /**
     * Show or hide the loading indicator.
     * @param {boolean} show
     */
    showLoading(show) {
        this.elements.loading?.classList.toggle('hidden', !show);
    }

    /**
     * Show or hide the empty state.
     * @param {boolean} show
     */
    showEmpty(show) {
        this.elements.empty?.classList.toggle('hidden', !show);
    }

    /**
     * Display an error message.
     * @param {string} message
     */
    showError(message) {
        // Dispatch custom event for app.js to handle via toast
        const event = new CustomEvent('history:error', { detail: { message } });
        document.dispatchEvent(event);
    }

    /**
     * Count resume events in the event list.
     * @param {Array} events
     * @returns {number}
     */
    countResumes(events) {
        if (!events) return 0;
        return events.filter(e => e.type === 'run.resumed').length;
    }

    /**
     * Format duration in milliseconds to human-readable string.
     * @param {number} ms - Duration in milliseconds
     * @returns {string}
     */
    formatDuration(ms) {
        if (!ms || ms < 0) return '--';

        const seconds = Math.floor(ms / 1000);
        const minutes = Math.floor(seconds / 60);
        const hours = Math.floor(minutes / 60);

        if (hours > 0) {
            return `${hours}h ${minutes % 60}m`;
        } else if (minutes > 0) {
            return `${minutes}m ${seconds % 60}s`;
        } else {
            return `${seconds}s`;
        }
    }

    /**
     * Truncate a run ID for display.
     * @param {string} id
     * @returns {string}
     */
    truncateId(id) {
        if (!id) return '';
        if (id.length <= 12) return id;
        return id.substring(0, 8) + '...';
    }

    /**
     * Get CSS class for a status.
     * @param {string} status
     * @returns {string}
     */
    getStatusClass(status) {
        const classes = {
            complete: 'status-complete',
            failed: 'status-failed',
            stopped: 'status-stopped',
            running: 'status-running',
            pending: 'status-pending'
        };
        return classes[status] || 'status-pending';
    }

    /**
     * Clean up resources when view is hidden.
     */
    destroy() {
        this.currentRun = null;
        this.runs = [];
        this.graphInitialized = false;

        // Clear graph container
        if (this.elements.graphContainer) {
            this.elements.graphContainer.innerHTML = '';
        }
    }
}

// View switching state
let currentView = 'live';
let historyView = null;

/**
 * Switch between Live and History views.
 * @param {string} view - "live" or "history"
 */
export function switchView(view) {
    if (view === currentView) return;

    const livePanel = document.getElementById('live-view');
    const historyPanel = document.getElementById('history-view');
    const liveBtn = document.getElementById('nav-live');
    const historyBtn = document.getElementById('nav-history');

    if (view === 'live') {
        livePanel?.classList.remove('hidden');
        historyPanel?.classList.add('hidden');
        liveBtn?.classList.add('active');
        historyBtn?.classList.remove('active');

        // Clean up history view
        historyView?.destroy();
    } else if (view === 'history') {
        livePanel?.classList.add('hidden');
        historyPanel?.classList.remove('hidden');
        liveBtn?.classList.remove('active');
        historyBtn?.classList.add('active');

        // Initialize history view if needed
        if (!historyView) {
            const container = document.getElementById('history-view');
            if (container) {
                historyView = new HistoryView(container);
            }
        }

        // Load runs for current repo (repo path obtained from server state)
        fetch('/api/state')
            .then(r => r.json())
            .then(state => {
                if (state.repoPath) {
                    historyView?.loadRuns(state.repoPath, 1);
                }
            })
            .catch(err => console.error('Failed to get repo path:', err));
    }

    currentView = view;

    // Persist view preference
    try {
        localStorage.setItem('choo:view', view);
    } catch (e) {
        // localStorage may be unavailable
    }
}

/**
 * Initialize view switching and restore last selected view.
 */
export function initNavigation() {
    const liveBtn = document.getElementById('nav-live');
    const historyBtn = document.getElementById('nav-history');

    liveBtn?.addEventListener('click', () => switchView('live'));
    historyBtn?.addEventListener('click', () => switchView('history'));

    // Listen for history errors to show toasts
    document.addEventListener('history:error', (e) => {
        const { showToast } = window.chooApp || {};
        if (showToast) {
            showToast(e.detail.message, 'error');
        }
    });

    // Restore last view
    try {
        const savedView = localStorage.getItem('choo:view');
        if (savedView === 'history') {
            switchView('history');
        }
    } catch (e) {
        // localStorage may be unavailable
    }
}

export { HistoryView as default };
```

### CSS Styles

```css
/* Add to internal/web/static/style.css */

/* Navigation */
.main-nav {
    display: flex;
    gap: 4px;
    padding: 4px;
    background-color: var(--bg-tertiary);
    border-radius: 8px;
    margin-bottom: 16px;
}

.nav-btn {
    flex: 1;
    padding: 10px 16px;
    background: transparent;
    border: none;
    border-radius: 6px;
    color: var(--text-secondary);
    font-size: 14px;
    font-weight: 500;
    cursor: pointer;
    transition: all 0.2s;
}

.nav-btn:hover {
    color: var(--text-primary);
    background-color: rgba(255, 255, 255, 0.05);
}

.nav-btn.active {
    background-color: var(--bg-primary);
    color: var(--text-primary);
}

/* View panels */
.view-panel {
    height: 100%;
    display: flex;
    flex-direction: column;
}

.view-panel.hidden {
    display: none;
}

/* History Layout */
.history-layout {
    display: flex;
    height: 100%;
    gap: 1px;
    background-color: var(--border-color);
}

.history-list {
    width: 320px;
    background-color: var(--bg-secondary);
    display: flex;
    flex-direction: column;
}

.history-list-header {
    padding: 16px;
    border-bottom: 1px solid var(--border-color);
    display: flex;
    justify-content: space-between;
    align-items: center;
}

.history-list-header h3 {
    font-size: 14px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-secondary);
}

.history-pagination {
    display: flex;
    align-items: center;
    gap: 8px;
}

.pagination-btn {
    width: 28px;
    height: 28px;
    border: 1px solid var(--border-color);
    border-radius: 4px;
    background: var(--bg-tertiary);
    color: var(--text-secondary);
    cursor: pointer;
    font-size: 14px;
}

.pagination-btn:hover:not(:disabled) {
    background: var(--bg-primary);
    color: var(--text-primary);
}

.pagination-btn:disabled {
    opacity: 0.4;
    cursor: not-allowed;
}

#history-page-info {
    font-size: 12px;
    color: var(--text-secondary);
}

/* Run List */
.run-list {
    flex: 1;
    overflow-y: auto;
    padding: 8px;
}

.run-item {
    padding: 12px;
    background-color: var(--bg-tertiary);
    border-radius: 6px;
    margin-bottom: 8px;
    cursor: pointer;
    transition: all 0.2s;
    border: 1px solid transparent;
}

.run-item:hover {
    border-color: var(--border-color);
}

.run-item.selected {
    border-color: var(--status-in-progress);
    background-color: rgba(59, 130, 246, 0.1);
}

.run-item-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 6px;
}

.run-id {
    font-family: 'Monaco', 'Menlo', monospace;
    font-size: 13px;
    color: var(--text-primary);
}

.run-item-meta {
    display: flex;
    gap: 12px;
    font-size: 12px;
    color: var(--text-secondary);
    margin-bottom: 6px;
}

.run-item-stats {
    display: flex;
    gap: 12px;
    font-size: 12px;
}

.unit-count {
    color: var(--text-secondary);
}

.resume-count {
    color: var(--status-ready);
}

.failed-count {
    color: var(--status-failed);
}

/* Status Badges */
.status-badge {
    display: inline-block;
    padding: 2px 8px;
    border-radius: 4px;
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.03em;
}

.status-complete {
    background-color: rgba(34, 197, 94, 0.2);
    color: var(--status-complete);
}

.status-failed {
    background-color: rgba(239, 68, 68, 0.2);
    color: var(--status-failed);
}

.status-stopped {
    background-color: rgba(249, 115, 22, 0.2);
    color: var(--status-blocked);
}

.status-running {
    background-color: rgba(59, 130, 246, 0.2);
    color: var(--status-in-progress);
}

.status-pending {
    background-color: rgba(156, 163, 175, 0.2);
    color: var(--status-pending);
}

/* Loading and Empty States */
.loading-indicator,
.empty-state {
    padding: 32px;
    text-align: center;
    color: var(--text-secondary);
    font-size: 14px;
}

.loading-indicator.hidden,
.empty-state.hidden {
    display: none;
}

/* History Detail Panel */
.history-detail {
    flex: 1;
    background-color: var(--bg-primary);
    display: flex;
    flex-direction: column;
    overflow: hidden;
}

.detail-placeholder {
    display: flex;
    align-items: center;
    justify-content: center;
    height: 100%;
    color: var(--text-secondary);
    font-size: 14px;
}

.detail-placeholder.hidden {
    display: none;
}

.detail-content {
    display: flex;
    flex-direction: column;
    height: 100%;
    overflow: hidden;
}

.detail-content.hidden {
    display: none;
}

.history-detail-header {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 16px 20px;
    border-bottom: 1px solid var(--border-color);
}

.history-detail-header h3 {
    font-size: 16px;
    font-weight: 600;
}

.history-detail-meta {
    display: flex;
    gap: 20px;
    padding: 12px 20px;
    background-color: var(--bg-secondary);
    font-size: 13px;
    color: var(--text-secondary);
}

/* History Graph */
.history-graph {
    height: 300px;
    min-height: 200px;
    border-bottom: 1px solid var(--border-color);
}

/* Event Timeline */
.history-timeline-section {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-height: 0;
    overflow: hidden;
}

.timeline-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 12px 20px;
    border-bottom: 1px solid var(--border-color);
    background-color: var(--bg-secondary);
}

.timeline-header h4 {
    font-size: 12px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-secondary);
}

.timeline-filters {
    display: flex;
    gap: 8px;
}

.timeline-filters select {
    padding: 6px 10px;
    background-color: var(--bg-tertiary);
    border: 1px solid var(--border-color);
    border-radius: 4px;
    color: var(--text-primary);
    font-size: 12px;
    cursor: pointer;
}

.timeline-filters select:focus {
    outline: none;
    border-color: var(--status-in-progress);
}

.event-timeline {
    flex: 1;
    overflow-y: auto;
    padding: 16px 20px;
}

/* Timeline Events */
.timeline-event {
    display: flex;
    gap: 12px;
    padding: 10px 0;
    border-bottom: 1px solid var(--bg-tertiary);
}

.timeline-event:last-child {
    border-bottom: none;
}

.timeline-marker {
    width: 12px;
    height: 12px;
    border-radius: 50%;
    flex-shrink: 0;
    margin-top: 2px;
}

.timeline-content {
    flex: 1;
    min-width: 0;
}

.timeline-header {
    display: flex;
    gap: 8px;
    align-items: center;
    flex-wrap: wrap;
    padding: 0;
    border: none;
    background: none;
}

.timeline-time {
    font-family: 'Monaco', 'Menlo', monospace;
    font-size: 11px;
    color: var(--text-secondary);
}

.timeline-type {
    font-size: 12px;
    color: var(--status-in-progress);
    font-weight: 500;
}

.timeline-unit {
    font-size: 12px;
    color: var(--text-primary);
    background-color: var(--bg-tertiary);
    padding: 1px 6px;
    border-radius: 3px;
}

.timeline-task {
    font-size: 12px;
    color: var(--text-secondary);
    margin-top: 4px;
}

.timeline-error {
    font-size: 12px;
    color: var(--status-failed);
    margin-top: 6px;
    padding: 8px;
    background-color: rgba(239, 68, 68, 0.1);
    border-radius: 4px;
    white-space: pre-wrap;
    word-break: break-word;
}

.timeline-reason {
    font-size: 12px;
    color: var(--status-blocked);
    margin-top: 4px;
    font-style: italic;
}

/* Stop/Resume Boundary Markers */
.timeline-event.stop-boundary {
    background-color: rgba(249, 115, 22, 0.1);
    margin: 8px -20px;
    padding: 12px 20px;
    border-top: 2px solid var(--status-blocked);
    border-bottom: none;
}

.timeline-event.stop-boundary .timeline-marker {
    width: 16px;
    height: 16px;
    border: 2px solid var(--status-blocked);
    background-color: var(--bg-primary);
}

.timeline-event.resume-boundary {
    background-color: rgba(251, 191, 36, 0.1);
    margin: 8px -20px;
    padding: 12px 20px;
    border-top: 2px solid var(--status-ready);
    border-bottom: none;
}

.timeline-event.resume-boundary .timeline-marker {
    width: 16px;
    height: 16px;
    border: 2px solid var(--status-ready);
    background-color: var(--bg-primary);
}

.timeline-event.error .timeline-type {
    color: var(--status-failed);
}
```

### App.js Modifications

```javascript
// Add to internal/web/static/app.js

import { initNavigation, switchView } from './history.js';

// Add to init() function, after existing initialization:
async function init() {
    // ... existing init code ...

    // Initialize navigation for Live/History view switching
    initNavigation();

    // Export showToast for history module to use
    window.chooApp = { showToast };
}

// Existing exports plus new ones
export { state, showToast, updateGraphStatus, handleEvent, switchView };
```

## Implementation Notes

### D3 Graph Reuse

The existing `graph.js` module exports three key functions that the history view uses directly:

1. `initGraph(container, data, callbacks)` - Initializes a new graph in a container
2. `updateNodeStatuses(statusMap)` - Updates node colors based on status
3. `STATUS_COLORS` - Color mapping for status values

The history view creates a separate graph instance in `#history-graph-container`. Since D3 selections are scoped to their container element, multiple graph instances can coexist without interference.

### Event Timeline Performance

For runs with many events (1000+), the timeline uses DOM recycling patterns:

1. Only visible events are fully rendered with event listeners
2. The timeline container uses `overflow-y: auto` for native scrolling
3. Event filtering happens in JavaScript before DOM updates
4. Batch DOM updates using innerHTML assignment rather than individual appends

### Resume Boundary Detection

The history module identifies stop/resume boundaries by looking for specific event types:

```javascript
// Events indicating a run was stopped
'run.stopped' // Contains data.reason explaining why

// Events indicating a run was resumed
'run.resumed' // Marks continuation of execution
```

These events maintain the same `run_id`, allowing the timeline to show a continuous history with visual boundaries.

### Local Storage Persistence

The view preference is stored in localStorage under the key `choo:view`. The code gracefully handles localStorage unavailability (e.g., in private browsing mode) by catching exceptions and defaulting to the live view.

### Error Handling

The history module uses custom events to communicate errors to the main app:

```javascript
document.dispatchEvent(new CustomEvent('history:error', {
    detail: { message: 'Failed to load runs' }
}));
```

This allows the existing toast notification system in `app.js` to display errors consistently across both views.

## Testing Strategy

### Unit Tests

```javascript
// internal/web/static/history.test.js

import { HistoryView } from './history.js';

describe('HistoryView', () => {
    let container;
    let view;

    beforeEach(() => {
        container = document.createElement('div');
        container.innerHTML = `
            <div id="history-run-list"></div>
            <div id="history-loading" class="hidden"></div>
            <div id="history-empty" class="hidden"></div>
            <button id="history-prev"></button>
            <button id="history-next"></button>
            <span id="history-page-info"></span>
            <div id="history-detail-placeholder"></div>
            <div id="history-detail-content" class="hidden"></div>
            <span id="history-run-title"></span>
            <span id="history-run-status"></span>
            <span id="history-run-duration"></span>
            <span id="history-run-units"></span>
            <span id="history-run-resumes"></span>
            <div id="history-graph-container"></div>
            <div id="history-timeline"></div>
            <select id="history-event-type-filter"></select>
            <select id="history-event-unit-filter"></select>
        `;
        view = new HistoryView(container);
    });

    describe('formatDuration', () => {
        it('formats seconds correctly', () => {
            expect(view.formatDuration(5000)).toBe('5s');
            expect(view.formatDuration(45000)).toBe('45s');
        });

        it('formats minutes correctly', () => {
            expect(view.formatDuration(60000)).toBe('1m 0s');
            expect(view.formatDuration(125000)).toBe('2m 5s');
        });

        it('formats hours correctly', () => {
            expect(view.formatDuration(3600000)).toBe('1h 0m');
            expect(view.formatDuration(5400000)).toBe('1h 30m');
        });

        it('handles null and negative values', () => {
            expect(view.formatDuration(null)).toBe('--');
            expect(view.formatDuration(-1000)).toBe('--');
        });
    });

    describe('truncateId', () => {
        it('returns short IDs unchanged', () => {
            expect(view.truncateId('abc123')).toBe('abc123');
        });

        it('truncates long IDs with ellipsis', () => {
            expect(view.truncateId('abc123def456ghi789')).toBe('abc123de...');
        });

        it('handles empty strings', () => {
            expect(view.truncateId('')).toBe('');
            expect(view.truncateId(null)).toBe('');
        });
    });

    describe('countResumes', () => {
        it('counts resume events correctly', () => {
            const events = [
                { type: 'unit.started' },
                { type: 'run.stopped' },
                { type: 'run.resumed' },
                { type: 'unit.completed' },
                { type: 'run.stopped' },
                { type: 'run.resumed' }
            ];
            expect(view.countResumes(events)).toBe(2);
        });

        it('returns 0 for no resumes', () => {
            const events = [
                { type: 'unit.started' },
                { type: 'unit.completed' }
            ];
            expect(view.countResumes(events)).toBe(0);
        });

        it('handles empty/null events', () => {
            expect(view.countResumes([])).toBe(0);
            expect(view.countResumes(null)).toBe(0);
        });
    });

    describe('getStatusClass', () => {
        it('returns correct classes for known statuses', () => {
            expect(view.getStatusClass('complete')).toBe('status-complete');
            expect(view.getStatusClass('failed')).toBe('status-failed');
            expect(view.getStatusClass('stopped')).toBe('status-stopped');
            expect(view.getStatusClass('running')).toBe('status-running');
        });

        it('returns pending class for unknown statuses', () => {
            expect(view.getStatusClass('unknown')).toBe('status-pending');
            expect(view.getStatusClass('')).toBe('status-pending');
        });
    });

    describe('computeNodeStatuses', () => {
        it('computes final statuses from events', () => {
            view.currentRun = {
                events: [
                    { type: 'unit.started', unit: 'a' },
                    { type: 'unit.started', unit: 'b' },
                    { type: 'unit.completed', unit: 'a' },
                    { type: 'unit.failed', unit: 'b' }
                ]
            };

            const statuses = view.computeNodeStatuses();

            expect(statuses.get('a')).toBe('complete');
            expect(statuses.get('b')).toBe('failed');
        });

        it('handles in-progress units', () => {
            view.currentRun = {
                events: [
                    { type: 'unit.started', unit: 'a' }
                ]
            };

            const statuses = view.computeNodeStatuses();

            expect(statuses.get('a')).toBe('in_progress');
        });
    });
});

describe('Event filtering', () => {
    let view;

    beforeEach(() => {
        const container = document.createElement('div');
        container.innerHTML = `
            <div id="history-timeline"></div>
            <select id="history-event-type-filter"></select>
            <select id="history-event-unit-filter"></select>
        `;
        view = new HistoryView(container);
        view.currentRun = {
            events: [
                { seq: 1, type: 'unit.started', unit: 'a', time: '2024-01-01T10:00:00Z' },
                { seq: 2, type: 'task.started', unit: 'a', task: 1, time: '2024-01-01T10:01:00Z' },
                { seq: 3, type: 'unit.started', unit: 'b', time: '2024-01-01T10:02:00Z' },
                { seq: 4, type: 'unit.completed', unit: 'a', time: '2024-01-01T10:03:00Z' }
            ]
        };
    });

    it('filters by event type', () => {
        view.filterEvents({ type: 'task' });
        view.renderTimeline();

        const events = view.elements.timeline.querySelectorAll('.timeline-event');
        expect(events.length).toBe(1);
        expect(events[0].dataset.type).toBe('task.started');
    });

    it('filters by unit', () => {
        view.filterEvents({ unit: 'b' });
        view.renderTimeline();

        const events = view.elements.timeline.querySelectorAll('.timeline-event');
        expect(events.length).toBe(1);
        expect(events[0].dataset.type).toBe('unit.started');
    });

    it('combines type and unit filters', () => {
        view.filterEvents({ type: 'unit', unit: 'a' });
        view.renderTimeline();

        const events = view.elements.timeline.querySelectorAll('.timeline-event');
        expect(events.length).toBe(2);
    });
});
```

### Integration Tests

| Scenario | Setup | Verification |
|----------|-------|--------------|
| View switching | Click Live/History tabs | Correct panel visible, state preserved |
| Run list loading | Mock API with 30 runs | Pagination works, 20 runs displayed per page |
| Run detail display | Select a run from list | Graph renders, timeline populates, metadata correct |
| Resume markers | Select run with stop/resume events | Markers visible at correct positions |
| Event filtering | Select type and unit filters | Timeline shows only matching events |
| Error handling | API returns 500 | Toast notification appears, UI remains usable |
| Empty state | API returns 0 runs | Empty state message displayed |

### Manual Testing

- [ ] Navigation tabs switch between Live and History views
- [ ] Run list displays runs with correct status badges
- [ ] Pagination controls enable/disable correctly at boundaries
- [ ] Clicking a run loads and displays its details
- [ ] D3 graph shows with correct node colors based on final status
- [ ] Event timeline is scrollable for long event lists
- [ ] Stop/resume boundaries are visually distinct
- [ ] Type filter narrows events to selected category
- [ ] Unit filter narrows events to selected unit
- [ ] View preference persists across page refresh
- [ ] Works correctly when no runs exist (empty state)
- [ ] Error toast appears when API requests fail

## Design Decisions

### Why Reuse Existing D3 Graph?

The existing `graph.js` module provides a well-tested, visually consistent graph visualization. Reusing it ensures:

1. Visual consistency between Live and History views
2. No duplicate code for graph rendering
3. Users see familiar interaction patterns
4. Future graph improvements benefit both views

The module's design with container-scoped D3 selections allows multiple instances, making reuse straightforward.

### Why Separate CSS Classes for Status Badges?

Using semantic class names like `.status-complete` instead of inline styles provides:

1. Consistent styling across all status displays
2. Easy theming and customization
3. Better debugging in browser dev tools
4. Reusability across different components

### Why Custom Events for Error Communication?

The history module dispatches `history:error` events rather than directly calling toast functions to:

1. Maintain loose coupling between modules
2. Allow `app.js` to own the toast UI
3. Enable future error handling customization
4. Support testing without mocking imports

### Why localStorage for View Persistence?

localStorage provides simple, synchronous persistence without:

1. Server round-trips
2. Cookie overhead
3. Complex state management

The graceful fallback ensures the feature works even when localStorage is unavailable.

## Future Enhancements

1. **Run comparison**: Side-by-side view of two runs to see differences
2. **Event search**: Full-text search across event data
3. **Export functionality**: Download run data as JSON or CSV
4. **Run annotations**: Add notes to runs for debugging context
5. **Timeline zoom**: Visual zoom for dense event sequences
6. **Keyboard navigation**: Arrow keys to navigate run list and timeline
7. **Real-time updates**: WebSocket connection to see completed runs appear

## References

- [HISTORY-TYPES.md](./HISTORY-TYPES.md) - Canonical shared type definitions
- [HISTORY-API.md](./HISTORY-API.md) - HTTP API this UI consumes
- [Historical Runs PRD](/docs/HISTORICAL-RUNS-PRD.md) - Product requirements
- [D3.js v7 Documentation](https://d3js.org/) - Graph visualization library
- Existing Graph Module: `internal/web/static/graph.js`
- Existing App Module: `internal/web/static/app.js`
