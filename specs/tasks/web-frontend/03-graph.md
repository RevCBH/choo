---
task: 3
status: pending
backpressure: "test -f internal/web/static/graph.js && grep -q 'export.*initGraph' internal/web/static/graph.js && grep -q 'export.*updateNodeStatuses' internal/web/static/graph.js"
depends_on: [1]
---

# D3 Graph Module

**Parent spec**: `/specs/WEB-FRONTEND.md`
**Task**: #3 of 4

## Objective

Create the D3.js graph visualization module that renders the dependency graph with level-based layout, handles node/edge rendering, supports status color updates, and provides hover highlighting.

## Dependencies

### Task Dependencies
- Task #1 (HTML must provide graph-container element)

### External Dependencies
- D3.js v7 (loaded from CDN in HTML)

## Deliverables

### Files to Create

```
internal/web/static/
└── graph.js    # CREATE: D3 graph visualization module
```

### JavaScript Implementation

```javascript
// graph.js - D3.js DAG visualization

const STATUS_COLORS = {
    pending: "#9CA3AF",
    ready: "#FBBF24",
    in_progress: "#3B82F6",
    pr_open: "#A855F7",
    in_review: "#A855F7",
    merging: "#A855F7",
    complete: "#22C55E",
    failed: "#EF4444",
    blocked: "#F97316"
};

const LAYOUT = {
    nodeRadius: 30,
    levelSpacing: 150,
    nodeSpacing: 80,
    padding: 50
};

let svg, nodesGroup, edgesGroup;
let graphData = { nodes: [], edges: [], levels: [] };
let callbacks = {};

/**
 * Initialize the D3 graph visualization.
 * @param {HTMLElement} container - DOM element to render into
 * @param {Object} data - Initial graph structure {nodes, edges, levels}
 * @param {Object} eventCallbacks - Event callbacks {onClick, onHover}
 */
export function initGraph(container, data, eventCallbacks) {
    graphData = data;
    callbacks = eventCallbacks || {};

    const width = container.clientWidth;
    const height = container.clientHeight;

    // Clear any existing SVG
    d3.select(container).select("svg").remove();

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

    // Remove old nodes
    nodeSelection.exit().remove();

    // Add new nodes
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

    // Update existing nodes
    const nodeMerge = nodeSelection.merge(nodeEnter);

    nodeMerge.select("circle")
        .attr("fill", d => STATUS_COLORS[d.status] || STATUS_COLORS.pending)
        .classed("pulse", d => d.status === "in_progress");
}

function renderEdges() {
    const edgeSelection = edgesGroup.selectAll(".edge")
        .data(graphData.edges, d => `${d.from}-${d.to}`);

    // Remove old edges
    edgeSelection.exit().remove();

    // Add new edges
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
    return `M ${source.x + LAYOUT.nodeRadius} ${source.y} C ${midX} ${source.y}, ${midX} ${target.y}, ${target.x - LAYOUT.nodeRadius - 10} ${target.y}`;
}

function truncateLabel(text, maxLength) {
    if (text.length <= maxLength) return text;
    return text.substring(0, maxLength - 1) + "...";
}

/**
 * Update node statuses without re-rendering layout.
 * @param {Map<string, string>} statusMap - Unit ID to status mapping
 */
export function updateNodeStatuses(statusMap) {
    graphData.nodes.forEach(node => {
        if (statusMap.has(node.id)) {
            node.status = statusMap.get(node.id);
        }
    });
    renderNodes();
}

/**
 * Highlight a node and its dependencies.
 * @param {string|null} nodeId - Node to highlight, or null to clear
 */
export function highlightDependencies(nodeId) {
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

export { STATUS_COLORS };
```

## Backpressure

### Validation Command

```bash
test -f internal/web/static/graph.js && \
grep -q 'export.*initGraph' internal/web/static/graph.js && \
grep -q 'export.*updateNodeStatuses' internal/web/static/graph.js && \
grep -q 'export.*highlightDependencies' internal/web/static/graph.js && \
grep -q 'STATUS_COLORS' internal/web/static/graph.js
```

### Must Pass
- File exists at `internal/web/static/graph.js`
- Exports `initGraph` function
- Exports `updateNodeStatuses` function
- Exports `highlightDependencies` function
- Exports `STATUS_COLORS` constant
- Contains D3 SVG setup code
- Contains level-based layout computation
- Contains bezier path calculation for edges

### CI Compatibility
- [x] No external API keys required
- [x] No network access required for validation
- [x] Runs in <60 seconds

## NOT In Scope
- SSE connection handling (task #4)
- State management (task #4)
- Toast notifications (task #4)
- Event log rendering (task #4)
