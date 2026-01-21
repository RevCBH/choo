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
    nodeWidth: 200,
    nodeHeight: 50,
    levelSpacing: 100,   // Vertical spacing between levels (top-down)
    nodeSpacing: 240,    // Horizontal spacing between nodes in same level
    padding: 80,         // Padding from edges
    // Progress block settings
    blockSize: 8,
    blockGap: 3,
    blockMarginTop: 4
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

    // Arrow marker for edges (pointing down for top-bottom layout)
    svg.append("defs")
        .append("marker")
        .attr("id", "arrowhead")
        .attr("viewBox", "-5 0 10 10")
        .attr("refX", 0)
        .attr("refY", 8)
        .attr("markerWidth", 6)
        .attr("markerHeight", 6)
        .attr("orient", "auto")
        .append("path")
        .attr("d", "M-5,0L0,10L5,0")
        .attr("fill", "#6B7280");

    edgesGroup = svg.append("g").attr("class", "edges");
    nodesGroup = svg.append("g").attr("class", "nodes");

    computeLayout(width, height);
    renderEdges();
    renderNodes();
}

function computeLayout(width, height) {
    const levels = graphData.levels || [];

    // Build adjacency maps for fast lookup
    const nodeById = new Map(graphData.nodes.map(n => [n.id, n]));
    const incomingEdges = new Map(); // nodeId -> [sourceNodeIds]
    const outgoingEdges = new Map(); // nodeId -> [targetNodeIds]

    graphData.nodes.forEach(n => {
        incomingEdges.set(n.id, []);
        outgoingEdges.set(n.id, []);
    });

    graphData.edges.forEach(e => {
        // e.from = dependent (below), e.to = dependency (above)
        incomingEdges.get(e.from)?.push(e.to);
        outgoingEdges.get(e.to)?.push(e.from);
    });

    // First pass: assign Y positions (levels) and initial centered X positions
    levels.forEach((levelNodes, levelIndex) => {
        const y = LAYOUT.padding + levelIndex * LAYOUT.levelSpacing;
        const levelWidth = levelNodes.length * LAYOUT.nodeSpacing;
        const startX = (width - levelWidth) / 2 + LAYOUT.nodeSpacing / 2;

        levelNodes.forEach((nodeId, nodeIndex) => {
            const node = nodeById.get(nodeId);
            if (node) {
                node.y = y;
                node.x = startX + nodeIndex * LAYOUT.nodeSpacing;
            }
        });
    });

    // Barycentric positioning: iteratively adjust X based on connected neighbors
    const iterations = 4;
    for (let iter = 0; iter < iterations; iter++) {
        // Forward pass: adjust based on incoming edges (dependencies)
        for (let levelIndex = 1; levelIndex < levels.length; levelIndex++) {
            const levelNodes = levels[levelIndex];
            const barycenters = [];

            levelNodes.forEach(nodeId => {
                const node = nodeById.get(nodeId);
                const incoming = incomingEdges.get(nodeId) || [];
                if (incoming.length > 0) {
                    const avgX = incoming.reduce((sum, depId) => {
                        const dep = nodeById.get(depId);
                        return sum + (dep ? dep.x : 0);
                    }, 0) / incoming.length;
                    barycenters.push({ nodeId, x: avgX });
                } else {
                    barycenters.push({ nodeId, x: node.x });
                }
            });

            // Sort by barycenter and reassign positions
            barycenters.sort((a, b) => a.x - b.x);
            const levelWidth = levelNodes.length * LAYOUT.nodeSpacing;
            const startX = (width - levelWidth) / 2 + LAYOUT.nodeSpacing / 2;

            barycenters.forEach((item, i) => {
                const node = nodeById.get(item.nodeId);
                if (node) {
                    node.x = startX + i * LAYOUT.nodeSpacing;
                }
            });
        }

        // Backward pass: adjust based on outgoing edges (dependents)
        for (let levelIndex = levels.length - 2; levelIndex >= 0; levelIndex--) {
            const levelNodes = levels[levelIndex];
            const barycenters = [];

            levelNodes.forEach(nodeId => {
                const node = nodeById.get(nodeId);
                const outgoing = outgoingEdges.get(nodeId) || [];
                if (outgoing.length > 0) {
                    const avgX = outgoing.reduce((sum, depId) => {
                        const dep = nodeById.get(depId);
                        return sum + (dep ? dep.x : 0);
                    }, 0) / outgoing.length;
                    barycenters.push({ nodeId, x: avgX });
                } else {
                    barycenters.push({ nodeId, x: node.x });
                }
            });

            // Sort by barycenter and reassign positions
            barycenters.sort((a, b) => a.x - b.x);
            const levelWidth = levelNodes.length * LAYOUT.nodeSpacing;
            const startX = (width - levelWidth) / 2 + LAYOUT.nodeSpacing / 2;

            barycenters.forEach((item, i) => {
                const node = nodeById.get(item.nodeId);
                if (node) {
                    node.x = startX + i * LAYOUT.nodeSpacing;
                }
            });
        }
    }
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

    // Node rectangle
    nodeEnter.append("rect")
        .attr("x", -LAYOUT.nodeWidth / 2)
        .attr("y", -LAYOUT.nodeHeight / 2)
        .attr("width", LAYOUT.nodeWidth)
        .attr("height", LAYOUT.nodeHeight)
        .attr("rx", 6)
        .attr("ry", 6)
        .attr("fill", d => STATUS_COLORS[d.status] || STATUS_COLORS.pending)
        .attr("stroke", "#374151")
        .attr("stroke-width", 2);

    // Node label (positioned higher to make room for progress blocks)
    nodeEnter.append("text")
        .attr("class", "node-label")
        .attr("text-anchor", "middle")
        .attr("dy", "-0.2em")
        .attr("fill", "white")
        .attr("font-size", "13px")
        .attr("font-weight", "500")
        .text(d => truncateLabel(d.id, 26));

    // Progress blocks container
    nodeEnter.append("g")
        .attr("class", "progress-blocks");

    // Update existing nodes
    const nodeMerge = nodeSelection.merge(nodeEnter);

    nodeMerge.select("rect")
        .attr("fill", d => STATUS_COLORS[d.status] || STATUS_COLORS.pending)
        .classed("pulse", d => d.status === "in_progress" && (!d.tasks || d.tasks === 0));

    // Update progress blocks
    nodeMerge.each(function(d) {
        renderProgressBlocks(d3.select(this).select(".progress-blocks"), d);
    });
}

/**
 * Render progress blocks for a node.
 * @param {d3.Selection} container - The progress-blocks group
 * @param {Object} node - Node data with tasks, currentTask, completedTasks
 */
function renderProgressBlocks(container, node) {
    const totalTasks = node.tasks || 0;
    if (totalTasks === 0) {
        container.selectAll("*").remove();
        return;
    }

    const currentTask = node.currentTask ?? -1; // 0-indexed, -1 means none started
    const completedTasks = node.completedTasks ?? 0;

    // Calculate block positions (centered)
    const totalWidth = totalTasks * LAYOUT.blockSize + (totalTasks - 1) * LAYOUT.blockGap;
    const startX = -totalWidth / 2;
    const y = LAYOUT.blockMarginTop + 4; // Position below the label

    // Create data for blocks
    const blockData = Array.from({ length: totalTasks }, (_, i) => ({
        index: i,
        isComplete: i < completedTasks,
        isCurrent: i === currentTask,
        isPending: i > currentTask && i >= completedTasks
    }));

    // Bind data
    const blocks = container.selectAll(".progress-block")
        .data(blockData, d => d.index);

    // Remove old blocks
    blocks.exit().remove();

    // Add new blocks
    const blocksEnter = blocks.enter()
        .append("rect")
        .attr("class", "progress-block")
        .attr("width", LAYOUT.blockSize)
        .attr("height", LAYOUT.blockSize)
        .attr("rx", 1)
        .attr("ry", 1);

    // Update all blocks
    blocks.merge(blocksEnter)
        .attr("x", d => startX + d.index * (LAYOUT.blockSize + LAYOUT.blockGap))
        .attr("y", y)
        .attr("fill", d => {
            if (d.isComplete) return "#22C55E"; // Green for complete
            if (d.isCurrent) return "#FBBF24"; // Yellow for current
            return "rgba(255,255,255,0.2)"; // Dim for pending
        })
        .attr("stroke", d => {
            if (d.isComplete) return "#16A34A";
            if (d.isCurrent) return "#D97706";
            return "rgba(255,255,255,0.3)";
        })
        .attr("stroke-width", 1)
        .classed("current-task", d => d.isCurrent);
}

function renderEdges() {
    // Compute port offsets for edges connecting to the same node side
    const portOffsets = computePortOffsets();

    const edgeSelection = edgesGroup.selectAll(".edge")
        .data(graphData.edges, d => `${d.from}-${d.to}`);

    // Remove old edges
    edgeSelection.exit().remove();

    // Add new edges
    edgeSelection.enter()
        .append("path")
        .attr("class", "edge")
        .attr("d", d => {
            // Edge "from" is the dependent, "to" is the dependency
            // Draw arrow from dependency (to) to dependent (from) so it flows top-to-bottom
            const source = graphData.nodes.find(n => n.id === d.to);   // dependency (top)
            const target = graphData.nodes.find(n => n.id === d.from); // dependent (bottom)
            if (!source || !target) return "";
            const offsets = portOffsets.get(`${d.from}-${d.to}`) || { sourceOffset: 0, targetOffset: 0 };
            return stepPath(source, target, offsets.sourceOffset, offsets.targetOffset);
        })
        .attr("fill", "none")
        .attr("stroke", "#6B7280")
        .attr("stroke-width", 2)
        .attr("marker-end", "url(#arrowhead)");
}

/**
 * Compute X offsets for edge connection points to avoid overlap.
 * When multiple edges connect to the same side of a node, stagger them horizontally.
 */
function computePortOffsets() {
    const offsets = new Map(); // edgeKey -> {sourceOffset, targetOffset}
    const maxPortOffset = LAYOUT.nodeWidth / 3; // Max offset from center

    // Group edges by source (bottom side) and target (top side) nodes
    const sourceEdges = new Map(); // nodeId -> edges leaving from bottom side
    const targetEdges = new Map(); // nodeId -> edges entering from top side

    graphData.edges.forEach(e => {
        // e.to = source (dependency, top node), e.from = target (dependent, bottom node)
        if (!sourceEdges.has(e.to)) sourceEdges.set(e.to, []);
        if (!targetEdges.has(e.from)) targetEdges.set(e.from, []);
        sourceEdges.get(e.to).push(e);
        targetEdges.get(e.from).push(e);
    });

    // Compute offsets for edges leaving each source node (bottom side)
    sourceEdges.forEach((edges, nodeId) => {
        if (edges.length <= 1) {
            edges.forEach(e => {
                const key = `${e.from}-${e.to}`;
                if (!offsets.has(key)) offsets.set(key, { sourceOffset: 0, targetOffset: 0 });
                offsets.get(key).sourceOffset = 0;
            });
            return;
        }

        // Sort edges by target node's X position for consistent ordering
        edges.sort((a, b) => {
            const targetA = graphData.nodes.find(n => n.id === a.from);
            const targetB = graphData.nodes.find(n => n.id === b.from);
            return (targetA?.x || 0) - (targetB?.x || 0);
        });

        const step = (2 * maxPortOffset) / (edges.length - 1 || 1);
        edges.forEach((e, i) => {
            const key = `${e.from}-${e.to}`;
            if (!offsets.has(key)) offsets.set(key, { sourceOffset: 0, targetOffset: 0 });
            offsets.get(key).sourceOffset = -maxPortOffset + i * step;
        });
    });

    // Compute offsets for edges entering each target node (top side)
    targetEdges.forEach((edges, nodeId) => {
        if (edges.length <= 1) {
            edges.forEach(e => {
                const key = `${e.from}-${e.to}`;
                if (!offsets.has(key)) offsets.set(key, { sourceOffset: 0, targetOffset: 0 });
                offsets.get(key).targetOffset = 0;
            });
            return;
        }

        // Sort edges by source node's X position for consistent ordering
        edges.sort((a, b) => {
            const sourceA = graphData.nodes.find(n => n.id === a.to);
            const sourceB = graphData.nodes.find(n => n.id === b.to);
            return (sourceA?.x || 0) - (sourceB?.x || 0);
        });

        const step = (2 * maxPortOffset) / (edges.length - 1 || 1);
        edges.forEach((e, i) => {
            const key = `${e.from}-${e.to}`;
            if (!offsets.has(key)) offsets.set(key, { sourceOffset: 0, targetOffset: 0 });
            offsets.get(key).targetOffset = -maxPortOffset + i * step;
        });
    });

    return offsets;
}

function stepPath(source, target, sourceXOffset = 0, targetXOffset = 0) {
    // For top-down layout: edges go from bottom of source to top of target
    const sourceX = source.x + sourceXOffset;
    const sourceY = source.y + LAYOUT.nodeHeight / 2;
    const targetX = target.x + targetXOffset;
    // Arrowhead marker has refY=8 with 10-unit path, so tip extends 2 units past path end
    // End path 2 units before node edge so arrowhead tip touches the edge
    const targetY = target.y - LAYOUT.nodeHeight / 2 - 2;

    // Step path: go down halfway, then horizontal, then down to target
    const midY = (sourceY + targetY) / 2;
    return `M ${sourceX} ${sourceY} L ${sourceX} ${midY} L ${targetX} ${midY} L ${targetX} ${targetY}`;
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
 * Update task progress for a node.
 * @param {string} nodeId - Unit ID
 * @param {number} currentTask - Current task index (0-indexed)
 * @param {number} completedTasks - Number of completed tasks
 */
export function updateTaskProgress(nodeId, currentTask, completedTasks) {
    const node = graphData.nodes.find(n => n.id === nodeId);
    if (node) {
        node.currentTask = currentTask;
        node.completedTasks = completedTasks;
        renderNodes();
    }
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
