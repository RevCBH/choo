# Task 03: History JavaScript Module

```yaml
task: 03-history-js
unit: history-ui
depends_on: [01-html]
backpressure: "curl -s http://localhost:8099/history.js | grep -q 'HistoryView'"
```

## Objective

Implement the HistoryView class for browsing and viewing historical runs.

## Requirements

1. Create `internal/web/static/history.js` with:

   ```javascript
   import { initGraph, updateNodeStatuses, STATUS_COLORS } from './graph.js';

   const EVENT_COLORS = {
       'unit.started': '#3B82F6',
       'unit.completed': '#22C55E',
       'unit.failed': '#EF4444',
       // ... etc
   };

   export class HistoryView {
       constructor(container) { /* bind elements, setup listeners */ }

       // Data loading
       async loadRuns(repoPath, page = 1) { /* fetch runs with limit/offset */ }
       async selectRun(runId) { /* fetch run detail, graph, events */ }

       // Rendering
       renderRunList() { /* render list of run items */ }
       renderRunDetail() { /* render detail view with graph and timeline */ }
       renderTimeline() { /* render event timeline */ }
       renderResumeMarkers() { /* add stop/resume visual markers */ }

       // Filtering
       filterEvents(filter) { /* apply type/unit filter */ }
       populateUnitFilter() { /* populate unit dropdown from graph nodes */ }

       // Pagination
       goToPage(page) { /* navigate to page */ }
       updatePagination() { /* update prev/next buttons */ }

       // Graph integration
       initializeGraph() { /* create D3 graph in history container */ }
       computeNodeStatuses() { /* derive statuses from events */ }

       // Utilities
       formatDuration(ms) { /* format milliseconds to human string */ }
       truncateId(id) { /* shorten long IDs for display */ }
       getStatusClass(status) { /* map status to CSS class */ }
       countResumes(events) { /* count run.resumed events */ }

       // Cleanup
       destroy() { /* cleanup when switching away */ }
   }

   export function switchView(view) { /* switch between live/history */ }
   export function initNavigation() { /* setup tab click handlers */ }
   ```

2. API integration:
   - Use fetch to call `/api/history/runs`, `/api/history/runs/{id}`, etc.
   - Use `limit/offset` pagination per HISTORY-TYPES.md
   - Handle errors gracefully with user feedback

3. Graph reuse:
   - Import existing `graph.js` module
   - Create separate graph instance in history container
   - Apply status colors from computed event states

4. Event timeline features:
   - Scrollable list of events
   - Color-coded markers by event type
   - Filter dropdowns for type and unit
   - Visual markers for stop/resume boundaries

5. State management:
   - Track current page, selected run
   - Persist view preference to localStorage
   - Clean up on view switch

## Acceptance Criteria

- [ ] HistoryView class exports correctly
- [ ] Run list loads and displays
- [ ] Pagination works with prev/next buttons
- [ ] Selecting a run shows detail view
- [ ] Graph renders with correct node statuses
- [ ] Event timeline displays all events
- [ ] Filters work correctly
- [ ] Stop/resume boundaries are marked
- [ ] View preference persists across refresh

## Files to Create/Modify

- `internal/web/static/history.js` (create)
