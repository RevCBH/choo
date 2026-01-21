# Task 02: CSS Styles

```yaml
task: 02-css
unit: history-ui
depends_on: [01-html]
backpressure: "curl -s http://localhost:8099/style.css | grep -q 'history-layout'"
```

## Objective

Add CSS styles for the history view components.

## Requirements

1. Add navigation styles to `internal/web/static/style.css`:

   ```css
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

   .nav-btn:hover { /* hover styles */ }
   .nav-btn.active { /* active styles */ }
   ```

2. Add view panel styles:

   ```css
   .view-panel { height: 100%; display: flex; flex-direction: column; }
   .view-panel.hidden { display: none; }
   ```

3. Add history layout styles:

   ```css
   .history-layout {
       display: flex;
       height: 100%;
       gap: 1px;
       background-color: var(--border-color);
   }

   .history-list { /* sidebar list styles */ }
   .history-detail { /* main detail panel styles */ }
   ```

4. Add run list item styles:

   ```css
   .run-item { /* card styles */ }
   .run-item:hover { /* hover state */ }
   .run-item.selected { /* selected state */ }
   .run-item-header { /* header with ID and status */ }
   .run-item-meta { /* timestamp and duration */ }
   .run-item-stats { /* unit counts */ }
   ```

5. Add status badge styles:

   ```css
   .status-badge { /* base badge */ }
   .status-complete { /* green */ }
   .status-failed { /* red */ }
   .status-stopped { /* orange */ }
   .status-running { /* blue */ }
   ```

6. Add timeline styles:

   ```css
   .event-timeline { /* scrollable container */ }
   .timeline-event { /* single event row */ }
   .timeline-marker { /* colored dot */ }
   .timeline-content { /* event details */ }
   .stop-boundary { /* stop event marker */ }
   .resume-boundary { /* resume event marker */ }
   ```

7. Add responsive behavior for smaller screens.

## Acceptance Criteria

- [ ] Navigation tabs styled correctly
- [ ] History layout has two-column design
- [ ] Run list items have proper states (hover, selected)
- [ ] Status badges are color-coded
- [ ] Timeline events display correctly
- [ ] Stop/resume markers are visually distinct
- [ ] Styles match existing app aesthetic

## Files to Create/Modify

- `internal/web/static/style.css` (modify)
