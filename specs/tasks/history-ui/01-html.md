# Task 01: HTML Structure

```yaml
task: 01-html
unit: history-ui
depends_on: []
backpressure: "curl -s http://localhost:8099/ | grep -q 'history-view'"
```

## Objective

Modify the index.html to add navigation tabs and history view containers.

## Requirements

1. Add navigation tabs to sidebar in `internal/web/static/index.html`:

   ```html
   <nav class="main-nav">
       <button id="nav-live" class="nav-btn active">Live</button>
       <button id="nav-history" class="nav-btn">History</button>
   </nav>
   ```

2. Wrap existing content in a "live-view" panel:

   ```html
   <div id="live-view" class="view-panel">
       <!-- existing graph, detail panel, event log -->
   </div>
   ```

3. Add history view panel (initially hidden):

   ```html
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
               <div id="history-run-list" class="run-list"></div>
               <div id="history-loading" class="loading-indicator hidden">Loading...</div>
               <div id="history-empty" class="empty-state hidden">No runs found.</div>
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
                   <div id="history-graph-container" class="history-graph"></div>
                   <div class="history-timeline-section">
                       <div class="timeline-header">
                           <h4>Event Timeline</h4>
                           <div class="timeline-filters">
                               <select id="history-event-type-filter">
                                   <option value="">All Events</option>
                                   <option value="unit">Unit Events</option>
                                   <option value="task">Task Events</option>
                                   <option value="run">Run Events</option>
                               </select>
                               <select id="history-event-unit-filter">
                                   <option value="">All Units</option>
                               </select>
                           </div>
                       </div>
                       <div id="history-timeline" class="event-timeline"></div>
                   </div>
               </div>
           </div>
       </div>
   </div>
   ```

4. Add script import for history.js:

   ```html
   <script type="module" src="history.js"></script>
   ```

## Acceptance Criteria

- [ ] Navigation tabs visible in sidebar
- [ ] Live view contains existing content
- [ ] History view container exists (hidden by default)
- [ ] All required IDs are present for JavaScript binding
- [ ] Page loads without errors

## Files to Create/Modify

- `internal/web/static/index.html` (modify)
