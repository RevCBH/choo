# Task 04: App Integration

```yaml
task: 04-app
unit: history-ui
depends_on: [03-history-js]
backpressure: "curl -s http://localhost:8099/app.js | grep -q 'initNavigation'"
```

## Objective

Integrate the history view navigation into the main app.js.

## Requirements

1. Modify `internal/web/static/app.js` to:

   ```javascript
   import { initNavigation, switchView } from './history.js';

   async function init() {
       // ... existing init code ...

       // Initialize navigation for Live/History view switching
       initNavigation();

       // Export showToast for history module to use
       window.chooApp = { showToast };
   }

   // Add to exports
   export { state, showToast, updateGraphStatus, handleEvent, switchView };
   ```

2. Ensure existing live view functionality still works:
   - SSE connection for real-time updates
   - Graph rendering and updates
   - Event log display
   - Detail panel interactions

3. Handle view switching:
   - Live view: continue SSE connection
   - History view: disconnect SSE (optional, or keep for notifications)
   - Clean up resources when switching views

4. Error handling:
   - History module dispatches `history:error` events
   - App.js listens and shows toast notifications

5. Expose necessary functions globally for history module:
   - `showToast(message, type)` for notifications
   - Any other shared utilities

## Acceptance Criteria

- [ ] Navigation tabs switch between views
- [ ] Live view continues to work as before
- [ ] History view loads and functions correctly
- [ ] Error toasts display from history module
- [ ] No console errors during normal operation
- [ ] View switch is smooth (no flicker)

## Files to Create/Modify

- `internal/web/static/app.js` (modify)
