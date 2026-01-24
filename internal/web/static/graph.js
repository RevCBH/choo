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

const PHASE_COLORS = {
    reviewing: "#A855F7",
    review_passed: "#22C55E",
    review_issues: "#F59E0B",
    review_fixing: "#F59E0B",
    review_fix_applied: "#F59E0B",
    review_failed: "#EF4444",
    pr_created: "#0EA5E9",
    feature_pr_opened: "#0EA5E9",
    pr_review: "#A855F7",
    merging: "#F97316",
    merge_conflict: "#EF4444",
    pr_merged: "#22C55E",
    pr_failed: "#EF4444"
};

const LAYOUT = {
    nodeWidth: 200,
    nodeHeight: 50,
    levelSpacing: 100,   // Vertical spacing between levels (top-down)
    nodeSpacing: 240,    // Horizontal spacing between nodes in same level
    padding: 80,         // Padding from edges
    // Progress bar settings
    progressWidth: 140,
    progressHeight: 6,
    progressMarginTop: 10
};

let svg, contentGroup, nodesGroup, edgesGroup;
let graphData = { nodes: [], edges: [], levels: [] };
let callbacks = {};

/**
 * Initialize the D3 graph visualization.
 * @param {HTMLElement} container - DOM element to render into
 * @param {Object} data - Initial graph structure {nodes, edges, levels}
 * @param {Object} eventCallbacks - Event callbacks {onClick, onHover}
 */
export async function initGraph(container, data, eventCallbacks) {
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

    svg.style("cursor", "grab");

    const zoomBehavior = d3.zoom()
        .scaleExtent([0.3, 2])
        .on("start", () => {
            svg.style("cursor", "grabbing");
        })
        .on("end", () => {
            svg.style("cursor", "grab");
        })
        .on("zoom", (event) => {
            contentGroup.attr("transform", event.transform);
        });

    svg.call(zoomBehavior);

    // Arrow marker for edges (pointing down for top-bottom layout)
    svg.append("defs")
        .append("marker")
        .attr("id", "arrowhead")
        .attr("viewBox", "-5 0 10 10")
        .attr("refX", 0)
        .attr("refY", 8)
        .attr("markerWidth", 6)
        .attr("markerHeight", 6)
        .attr("orient", "0")
        .append("path")
        .attr("d", "M-5,0L0,10L5,0")
        .attr("fill", "#6B7280");

    contentGroup = svg.append("g").attr("class", "graph-content");
    edgesGroup = contentGroup.append("g").attr("class", "edges");
    nodesGroup = contentGroup.append("g").attr("class", "nodes");

    await computeLayout(width, height);
    renderEdges();
    renderNodes();
}

async function computeLayout(width, height) {
    if (typeof window !== "undefined" && window.ELK) {
        const success = await computeElkLayout(width, height);
        if (success) return;
    }

    const levels = graphData.levels || [];

    // Build adjacency maps for fast lookup
    const nodeById = new Map(graphData.nodes.map(n => [n.id, n]));
    const incomingEdges = new Map(); // nodeId -> [dependencyNodeIds]
    const outgoingEdges = new Map(); // nodeId -> [dependentNodeIds]

    graphData.nodes.forEach(n => {
        incomingEdges.set(n.id, []);
        outgoingEdges.set(n.id, []);
    });

    graphData.edges.forEach(e => {
        // e.from = dependency (above), e.to = dependent (below)
        incomingEdges.get(e.to)?.push(e.from);
        outgoingEdges.get(e.from)?.push(e.to);
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

    // Tree-biased positioning: assign a primary parent and anchor children near it,
    // then lightly smooth with barycentric passes.
    assignPrimaryParents(nodeById, incomingEdges);

    const iterations = 3;
    for (let iter = 0; iter < iterations; iter++) {
        // Forward pass: adjust based on incoming edges (dependencies)
        for (let levelIndex = 1; levelIndex < levels.length; levelIndex++) {
            const levelNodes = levels[levelIndex];
            const barycenters = [];

            levelNodes.forEach(nodeId => {
                const node = nodeById.get(nodeId);
                const incoming = incomingEdges.get(nodeId) || [];
                if (incoming.length > 0 && node.primaryParentId) {
                    const parent = nodeById.get(node.primaryParentId);
                    const parentX = parent ? parent.x : node.x;
                    barycenters.push({ nodeId, x: parentX });
                } else {
                    barycenters.push({ nodeId, x: node.x });
                }
            });

            applyBarycentricPositions(nodeById, barycenters, width);
        }

        // Backward pass: adjust based on outgoing edges (dependents)
        for (let levelIndex = levels.length - 2; levelIndex >= 1; levelIndex--) {
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

            applyBarycentricPositions(nodeById, barycenters, width);
        }
    }
}

async function computeElkLayout(width, height) {
    try {
        const elk = new window.ELK();
        const nodeWidth = LAYOUT.nodeWidth;
        const nodeHeight = LAYOUT.nodeHeight;

        const elkGraph = {
            id: "root",
            layoutOptions: {
                "elk.algorithm": "layered",
                "elk.direction": "DOWN",
                "elk.edgeRouting": "ORTHOGONAL",
                "elk.layered.edgeRouting": "ORTHOGONAL",
                "elk.layered.spacing.nodeNodeBetweenLayers": String(LAYOUT.levelSpacing),
                "elk.spacing.nodeNode": String(LAYOUT.nodeSpacing),
                "elk.layered.crossingMinimization.strategy": "LAYER_SWEEP",
                "elk.layered.nodePlacement.strategy": "NETWORK_SIMPLEX"
            },
            children: graphData.nodes.map(node => ({
                id: node.id,
                width: nodeWidth,
                height: nodeHeight
            })),
            edges: graphData.edges.map(edge => ({
                id: `${edge.from}->${edge.to}`,
                sources: [edge.from],
                targets: [edge.to]
            }))
        };

        const layout = await elk.layout(elkGraph);
        if (!layout || !layout.children) return false;

        let minX = Infinity;
        let minY = Infinity;
        let maxX = -Infinity;
        let maxY = -Infinity;

        layout.children.forEach(child => {
            minX = Math.min(minX, child.x);
            minY = Math.min(minY, child.y);
            maxX = Math.max(maxX, child.x + child.width);
            maxY = Math.max(maxY, child.y + child.height);
        });

        if (!isFinite(minX) || !isFinite(minY)) return false;

        const graphWidth = maxX - minX;
        const graphHeight = maxY - minY;

        const offsetX = (width - graphWidth) / 2 - minX;
        const offsetY = LAYOUT.padding - minY;

        const nodeById = new Map(graphData.nodes.map(n => [n.id, n]));
        layout.children.forEach(child => {
            const node = nodeById.get(child.id);
            if (!node) return;
            node.x = child.x + child.width / 2 + offsetX;
            node.y = child.y + child.height / 2 + offsetY;
        });

        const edgeById = new Map(graphData.edges.map(e => [`${e.from}->${e.to}`, e]));
        layout.edges?.forEach(edge => {
            const existing = edgeById.get(edge.id);
            if (!existing) return;

            const points = [];
            edge.sections?.forEach(section => {
                if (section.startPoint) {
                    points.push({
                        x: section.startPoint.x + offsetX,
                        y: section.startPoint.y + offsetY
                    });
                }
                section.bendPoints?.forEach(p => {
                    points.push({ x: p.x + offsetX, y: p.y + offsetY });
                });
                if (section.endPoint) {
                    points.push({
                        x: section.endPoint.x + offsetX,
                        y: section.endPoint.y + offsetY
                    });
                }
            });

            existing.points = trimPathEnd(points, 2);
        });

        return true;
    } catch (err) {
        console.warn("ELK layout failed, falling back to manual layout", err);
        return false;
    }
}

function assignPrimaryParents(nodeById, incomingEdges) {
    const childrenByParent = new Map();

    nodeById.forEach((node, nodeId) => {
        let primary = node.primary_parent || node.primaryParent || null;
        if (!primary) {
            const incoming = incomingEdges.get(nodeId) || [];
            if (incoming.length > 0) {
                primary = incoming
                    .map(depId => nodeById.get(depId))
                    .filter(Boolean)
                    .sort((a, b) => {
                        if (a.level !== b.level) return b.level - a.level;
                        return (b.x || 0) - (a.x || 0);
                    })[0]?.id;
            }
        }
        node.primaryParentId = primary;
        if (primary) {
            if (!childrenByParent.has(primary)) {
                childrenByParent.set(primary, []);
            }
            childrenByParent.get(primary).push(nodeId);
        }
    });

    // Order children left-to-right based on current X to keep layout stable.
    childrenByParent.forEach((children, parentId) => {
        const parent = nodeById.get(parentId);
        if (!parent) return;

        const parentX = parent.x;
        const sorted = children
            .map(id => nodeById.get(id))
            .filter(Boolean)
            .sort((a, b) => a.x - b.x);

        const count = sorted.length;
        if (count === 0) return;

        const spread = Math.min(LAYOUT.nodeSpacing * 0.9, LAYOUT.nodeSpacing * (count - 1));
        const startX = parentX - spread / 2;
        const step = count > 1 ? spread / (count - 1) : 0;

        sorted.forEach((child, idx) => {
            child.x = startX + idx * step;
        });
    });
}

function applyBarycentricPositions(nodeById, barycenters, width) {
    if (barycenters.length === 0) return;

    // Sort by desired barycenter to preserve ordering.
    const sorted = barycenters.slice().sort((a, b) => a.x - b.x);

    const minX = LAYOUT.padding + LAYOUT.nodeWidth / 2;
    const maxX = width - LAYOUT.padding - LAYOUT.nodeWidth / 2;
    const spacing = LAYOUT.nodeSpacing;

    const placed = [];
    const count = sorted.length;
    if (count === 1) {
        const desired = sorted[0].x;
        const x = Math.min(Math.max(desired, minX), maxX);
        placed.push({ nodeId: sorted[0].nodeId, x });
    } else {
        const clusterWidth = spacing * (count - 1);
        const avgDesired = sorted.reduce((sum, item) => sum + item.x, 0) / count;
        const minStart = minX;
        const maxStart = maxX - clusterWidth;
        const startX = Math.min(Math.max(avgDesired - clusterWidth / 2, minStart), maxStart);

        sorted.forEach((item, i) => {
            placed.push({ nodeId: item.nodeId, x: startX + i * spacing });
        });
    }

    placed.forEach(item => {
        const node = nodeById.get(item.nodeId);
        if (node) {
            node.x = item.x;
        }
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

    // Node rectangle
    nodeEnter.append("rect")
        .attr("x", -LAYOUT.nodeWidth / 2)
        .attr("y", -LAYOUT.nodeHeight / 2)
        .attr("width", LAYOUT.nodeWidth)
        .attr("height", LAYOUT.nodeHeight)
        .attr("rx", 6)
        .attr("ry", 6)
        .attr("fill", d => STATUS_COLORS[d.status] || STATUS_COLORS.pending)
        .attr("stroke", d => PHASE_COLORS[d.phase] || "#374151")
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

    // Progress bar container
    nodeEnter.append("g")
        .attr("class", "progress-bar");

    // Update existing nodes
    const nodeMerge = nodeSelection.merge(nodeEnter);

    nodeMerge.select("rect")
        .attr("fill", d => STATUS_COLORS[d.status] || STATUS_COLORS.pending)
        .attr("stroke", d => PHASE_COLORS[d.phase] || "#374151")
        .attr("stroke-width", d => (d.phase ? 3 : 2))
        .classed("pulse", d => d.status === "in_progress" && (!d.tasks || d.tasks === 0));

    // Update progress bar
    nodeMerge.each(function(d) {
        renderProgressBar(d3.select(this).select(".progress-bar"), d);
    });
}

/**
 * Render progress bar for a node.
 * @param {d3.Selection} container - The progress bar group
 * @param {Object} node - Node data with tasks, currentTask, completedTasks
 */
function renderProgressBar(container, node) {
    const totalTasks = node.tasks || 0;
    if (totalTasks === 0) {
        container.selectAll("*").remove();
        return;
    }

    const currentTask = node.currentTask ?? -1; // 0-indexed, -1 means none started
    const completedTasks = node.completedTasks ?? 0;

    const barWidth = LAYOUT.progressWidth;
    const barHeight = LAYOUT.progressHeight;
    const x = -barWidth / 2;
    const y = LAYOUT.progressMarginTop;

    const safeTotal = Math.max(1, totalTasks);
    const sliceWidth = barWidth / safeTotal;
    const clampedCompleted = Math.min(Math.max(completedTasks, 0), totalTasks);
    const completedWidth = clampedCompleted * sliceWidth;
    const showCurrent = node.status === "in_progress" && currentTask >= 0 && currentTask < totalTasks;
    const currentX = x + currentTask * sliceWidth;

    container.selectAll("*").remove();

    container.append("rect")
        .attr("class", "progress-track")
        .attr("x", x)
        .attr("y", y)
        .attr("width", barWidth)
        .attr("height", barHeight)
        .attr("rx", 3)
        .attr("ry", 3);

    if (completedWidth > 0) {
        container.append("rect")
            .attr("class", "progress-complete")
            .attr("x", x)
            .attr("y", y)
            .attr("width", completedWidth)
            .attr("height", barHeight)
            .attr("rx", 3)
            .attr("ry", 3);
    }

    if (showCurrent) {
        container.append("rect")
            .attr("class", "progress-current pulse")
            .attr("x", currentX)
            .attr("y", y)
            .attr("width", sliceWidth)
            .attr("height", barHeight)
            .attr("rx", 3)
            .attr("ry", 3);
    }
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
            if (d.points && d.points.length > 1) {
                return pathFromPoints(d.points);
            }
            // Edge "from" is the dependency (top), "to" is the dependent (bottom)
            const source = graphData.nodes.find(n => n.id === d.from);
            const target = graphData.nodes.find(n => n.id === d.to);
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

    // Group edges by source (top side) and target (bottom side) nodes
    const sourceEdges = new Map(); // nodeId -> edges leaving from bottom side (dependency nodes)
    const targetEdges = new Map(); // nodeId -> edges entering from top side (dependent nodes)

    graphData.edges.forEach(e => {
        // e.from = source (dependency, top node), e.to = target (dependent, bottom node)
        if (!sourceEdges.has(e.from)) sourceEdges.set(e.from, []);
        if (!targetEdges.has(e.to)) targetEdges.set(e.to, []);
        sourceEdges.get(e.from).push(e);
        targetEdges.get(e.to).push(e);
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
            const targetA = graphData.nodes.find(n => n.id === a.to);
            const targetB = graphData.nodes.find(n => n.id === b.to);
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
            const sourceA = graphData.nodes.find(n => n.id === a.from);
            const sourceB = graphData.nodes.find(n => n.id === b.from);
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

    // Step path: bias horizontal segment closer to the source to avoid crossing mid-level nodes
    let midY = (sourceY + targetY) / 2;
    if (targetY > sourceY) {
        const preferred = 40;
        const maxOffset = (targetY - sourceY) / 2;
        const offset = Math.min(preferred, maxOffset);
        midY = sourceY + Math.max(12, offset);
        if (midY >= targetY - 8) {
            midY = (sourceY + targetY) / 2;
        }
    }
    return `M ${sourceX} ${sourceY} L ${sourceX} ${midY} L ${targetX} ${midY} L ${targetX} ${targetY}`;
}

function pathFromPoints(points) {
    if (!points || points.length === 0) return "";
    let d = `M ${points[0].x} ${points[0].y}`;
    for (let i = 1; i < points.length; i++) {
        d += ` L ${points[i].x} ${points[i].y}`;
    }
    return d;
}

function trimPathEnd(points, trim) {
    if (!points || points.length < 2) return points;
    const trimmed = points.map(p => ({ x: p.x, y: p.y }));
    const last = trimmed[trimmed.length - 1];
    const prev = trimmed[trimmed.length - 2];
    const dx = last.x - prev.x;
    const dy = last.y - prev.y;
    const len = Math.sqrt(dx * dx + dy * dy);
    if (len <= 0) return trimmed;
    const ratio = Math.max((len - trim) / len, 0);
    last.x = prev.x + dx * ratio;
    last.y = prev.y + dy * ratio;
    return trimmed;
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

export function updateNodePhase(nodeId, phase) {
    const node = graphData.nodes.find(n => n.id === nodeId);
    if (node) {
        node.phase = phase;
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
