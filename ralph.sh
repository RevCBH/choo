#!/usr/bin/env bash
#
# ralph.sh - Autonomous task execution loop
#
# Executes a ralph-prep generated workset, running through atomic tasks
# until completion or iteration limit. Status is tracked in YAML frontmatter
# of each task spec file.
#
# Usage:
#   ./ralph.sh <workset-dir>                    # Run until complete
#   ./ralph.sh <workset-dir> --max-iterations 5 # Limit to 5 iterations
#   ./ralph.sh <workset-dir> --dry-run          # Show what would be done
#   ./ralph.sh <workset-dir> --status           # Show current progress
#
# Examples:
#   ./ralph.sh specs/tasks/audio-pipeline/      # Single workset
#   ./ralph.sh specs/tasks/                     # All worksets (multi-workset mode)
#

set -euo pipefail

# ============================================================================
# Configuration
# ============================================================================

RALPH_BASE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RALPH_LOG_DIR="$RALPH_BASE_DIR/.ralph"
RALPH_LOG_FILE="$RALPH_LOG_DIR/ralph.log"
RALPH_CLAUDE_CMD="${RALPH_CLAUDE_CMD:-claude}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m'

mkdir -p "$RALPH_LOG_DIR"

# ============================================================================
# Helpers
# ============================================================================

log() {
    local level="$1"
    shift
    local timestamp
    timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    echo -e "[$timestamp] [$level] $*" | tee -a "$RALPH_LOG_FILE"
}

notify() {
    local title="$1"
    local message="$2"
    if [[ "$(uname)" == "Darwin" ]]; then
        local icon_path="$RALPH_BASE_DIR/ralph-icon.png"
        if command -v terminal-notifier &>/dev/null && [[ -f "$icon_path" ]]; then
            terminal-notifier -title "$title" -message "$message" -appIcon "$icon_path" 2>/dev/null || true
        else
            osascript -e "display notification \"$message\" with title \"$title\"" 2>/dev/null || true
        fi
    fi
}

info()    { log "INFO" "$*"; }
warn()    { log "${YELLOW}WARN${NC}" "$*"; }
error()   { log "${RED}ERROR${NC}" "$*"; }
success() { log "${GREEN}OK${NC}" "$*"; }

die() {
    error "$*"
    exit 1
}

# ============================================================================
# YAML Frontmatter Parsing
# ============================================================================

# Extract frontmatter from a markdown file (content between first --- markers only)
get_frontmatter() {
    local file="$1"
    # Use awk to extract only the FIRST frontmatter block
    # This avoids matching --- markers in code examples within the file
    awk '/^---$/ { if (++count == 2) exit; next } count == 1' "$file"
}

# Get a specific field from frontmatter
get_frontmatter_field() {
    local file="$1"
    local field="$2"
    local fm
    fm=$(get_frontmatter "$file")
    # Strip inline comments (# ...) and quotes
    echo "$fm" | grep "^${field}:" | sed "s/^${field}:[[:space:]]*//" | sed 's/#.*//' | tr -d '"' | xargs
}

# Get array field from frontmatter (e.g., depends_on: [1, 2])
get_frontmatter_array() {
    local file="$1"
    local field="$2"
    local value
    value=$(get_frontmatter_field "$file" "$field")
    # Extract numbers from array notation [1, 2, 3] or []
    echo "$value" | tr -d '[]' | tr ',' '\n' | xargs
}

# Update a field in frontmatter
set_frontmatter_field() {
    local file="$1"
    local field="$2"
    local value="$3"

    if [[ "$DRY_RUN" == "true" ]]; then
        echo "[DRY RUN] Would set $field: $value in $(basename "$file")"
        return 0
    fi

    # Use sed to replace the field value in frontmatter
    if [[ "$(uname)" == "Darwin" ]]; then
        sed -i '' "s/^${field}:.*$/${field}: ${value}/" "$file"
    else
        sed -i "s/^${field}:.*$/${field}: ${value}/" "$file"
    fi
}

# ============================================================================
# Task Management
# ============================================================================

# Check if a directory is a multi-workset target (contains workset subdirs)
is_multi_workset() {
    local dir="$1"
    # Multi-workset if directory contains subdirs with task specs
    # but doesn't have task specs directly
    local direct_specs subdir_specs
    direct_specs=$(find "$dir" -maxdepth 1 -name '[0-9][0-9]-*.md' -type f 2>/dev/null | wc -l | xargs)
    subdir_specs=$(find "$dir" -mindepth 2 -maxdepth 2 -name '[0-9][0-9]-*.md' -type f 2>/dev/null | wc -l | xargs)

    [[ "$direct_specs" -eq 0 && "$subdir_specs" -gt 0 ]]
}

# Find all workset directories within a multi-workset target
find_worksets() {
    local parent_dir="$1"
    for subdir in "$parent_dir"/*/; do
        if [[ -d "$subdir" ]] && find "$subdir" -maxdepth 1 -name '[0-9][0-9]-*.md' -type f | grep -q .; then
            echo "${subdir%/}"
        fi
    done | sort
}

# Find all task spec files in workset directory (or all worksets if multi-workset)
find_task_specs() {
    local workset_dir="$1"

    if is_multi_workset "$workset_dir"; then
        # Multi-workset: find specs in all subdirectories
        find "$workset_dir" -mindepth 2 -maxdepth 2 -name '[0-9][0-9]-*.md' -type f | sort
    else
        # Single workset: find specs directly
        find "$workset_dir" -maxdepth 1 -name '[0-9][0-9]-*.md' -type f | sort
    fi
}

# Get the workset directory for a given spec file
get_spec_workset() {
    local spec_file="$1"
    dirname "$spec_file"
}

# Check if task's dependencies are met
check_task_dependencies() {
    local workset_dir="$1"  # May be multi-workset parent, but we use spec's own dir
    local spec_file="$2"

    local deps
    deps=$(get_frontmatter_array "$spec_file" "depends_on")

    # No dependencies
    if [[ -z "$deps" ]]; then
        return 0
    fi

    # Get the actual workset dir for this spec (for resolving local deps)
    local spec_workset
    spec_workset=$(get_spec_workset "$spec_file")

    for dep_num in $deps; do
        # Find the spec file for this dependency (in same workset)
        local dep_file
        dep_file=$(find "$spec_workset" -maxdepth 1 -name "$(printf '%02d' "$dep_num")-*.md" -type f | head -1)

        if [[ -z "$dep_file" ]]; then
            warn "Dependency task #$dep_num not found in $(basename "$spec_workset")"
            return 1
        fi

        local dep_status
        dep_status=$(get_frontmatter_field "$dep_file" "status")

        if [[ "$dep_status" != "complete" ]]; then
            return 1
        fi
    done

    return 0
}

# Find next task to execute
find_next_task() {
    local workset_dir="$1"

    while read -r spec_file; do
        local status
        status=$(get_frontmatter_field "$spec_file" "status")

        # Skip completed and failed
        if [[ "$status" == "complete" || "$status" == "failed" ]]; then
            continue
        fi

        # Check dependencies
        if check_task_dependencies "$workset_dir" "$spec_file"; then
            echo "$spec_file"
            return 0
        fi
    done < <(find_task_specs "$workset_dir")
}

# ============================================================================
# Task Execution
# ============================================================================

build_agent_prompt() {
    local spec_file="$1"
    local task_desc="$2"

    [[ -f "$spec_file" ]] || die "Task spec not found: $spec_file"

    cat <<EOF
You are executing a Ralph task. Follow these instructions exactly.

## Task
$task_desc

## Task Spec File
$spec_file

## Task Spec Content
$(cat "$spec_file")

## Instructions
1. Read the task spec completely
2. Implement ONLY what is specified - nothing more, nothing less
3. Run the backpressure validation command from the frontmatter
4. Also run baseline checks: \`go fmt ./...\` and \`go vet ./...\`
5. If any validation fails, fix the issues and re-run until all pass
6. When ALL checks pass, UPDATE THE FRONTMATTER STATUS to complete:
   - Edit the spec file ($spec_file)
   - Change \`status: in_progress\` to \`status: complete\`
7. Do NOT move on to other tasks

## Critical
- You MUST update the spec file's frontmatter status to \`complete\` when done
- The backpressure command AND baseline checks (go fmt, go vet) MUST pass before marking complete
- Stay focused on this single task
- Do not refactor unrelated code
- Do not add features not in the spec
- NEVER run tests in watch mode. Always use flags to run tests once and exit (e.g., \`--watch=false\`, \`--run\`, \`--watchAll=false\`, \`CI=true\`). Watch mode will block the execution loop indefinitely.
EOF
}

run_backpressure() {
    local backpressure="$1"

    if [[ -z "$backpressure" || "$backpressure" == "-" || "$backpressure" == "None" ]]; then
        warn "No backpressure defined, skipping validation"
        return 0
    fi

    info "Running backpressure: $backpressure"
    # Run in subshell to prevent cd from affecting parent process
    (eval "$backpressure")
}

# Baseline checks that all tasks must pass (fmt, vet)
# This ensures commits won't fail pre-commit hooks
run_baseline_checks() {
    info "Running baseline checks (go fmt, go vet)..."

    # Check if we're in a Go project
    if [[ -f "go.mod" ]]; then
        info "Checking Go formatting..."
        local unformatted
        unformatted=$(gofmt -l . 2>/dev/null | grep -v vendor || true)
        if [[ -n "$unformatted" ]]; then
            error "Go formatting check failed. Unformatted files:"
            echo "$unformatted"
            error "Run: go fmt ./..."
            return 1
        fi

        info "Running go vet..."
        if ! go vet ./...; then
            error "go vet check failed"
            return 1
        fi
    fi

    success "Baseline checks passed"
    return 0
}

commit_task() {
    local task_num="$1"
    local task_desc="$2"
    local workset_name="${3:-}"

    local commit_prefix="ralph: task #$task_num"
    if [[ -n "$workset_name" ]]; then
        commit_prefix="ralph: $workset_name/#$task_num"
    fi

    if [[ "$DRY_RUN" == "true" ]]; then
        echo "[DRY RUN] Would commit: $commit_prefix - $task_desc"
        return 0
    fi

    if git diff --quiet && git diff --cached --quiet; then
        info "No changes to commit"
        return 0
    fi

    info "Committing changes..."
    git add -A
    # Skip pre-commit hooks - backpressure already validated fmt/clippy
    git commit --no-verify -m "$commit_prefix - $task_desc

Automated commit by ralph.sh
"
}

execute_task() {
    local workset_dir="$1"
    local spec_file="$2"

    local task_num backpressure task_desc spec_workset workset_name
    task_num=$(get_frontmatter_field "$spec_file" "task")
    backpressure=$(get_frontmatter_field "$spec_file" "backpressure")
    task_desc=$(grep -m1 '^# ' "$spec_file" | sed 's/^# //')
    spec_workset=$(get_spec_workset "$spec_file")
    workset_name=$(basename "$spec_workset")

    info "═══════════════════════════════════════════════════════════════"
    info "Workset: $workset_name"
    info "Task #$task_num: $task_desc"
    info "Spec: $(basename "$spec_file")"
    info "Backpressure: $backpressure"
    info "═══════════════════════════════════════════════════════════════"

    # Set status to in_progress
    set_frontmatter_field "$spec_file" "status" "in_progress"

    local prompt
    prompt=$(build_agent_prompt "$spec_file" "$task_desc")

    local task_log="$RALPH_LOG_DIR/task_${task_num}.log"

    if [[ "$DRY_RUN" == "true" ]]; then
        echo "[DRY RUN] Would execute agent with spec: $(basename "$spec_file")"
        echo "[DRY RUN] Backpressure: $backpressure"
        return 0
    fi

    # Run agent once per iteration - main loop handles retries
    local prompt_file
    prompt_file=$(mktemp)
    printf '%s' "$prompt" > "$prompt_file"

    info "Invoking agent..."
    "$RALPH_CLAUDE_CMD" --dangerously-skip-permissions -p "$(cat "$prompt_file")" 2>&1 | tee "$task_log"
    local exit_code=${PIPESTATUS[0]}

    rm -f "$prompt_file"

    if [[ $exit_code -ne 0 ]]; then
        error "Agent exited with code $exit_code"
        return 1
    fi

    # Check if agent updated the status to complete
    local new_status
    new_status=$(get_frontmatter_field "$spec_file" "status")

    if [[ "$new_status" == "complete" ]]; then
        # Verify backpressure and baseline checks pass
        if run_backpressure "$backpressure" && run_baseline_checks; then
            success "Task #$task_num completed successfully"
            return 0
        else
            error "Agent marked complete but validation failed - reverting status"
            set_frontmatter_field "$spec_file" "status" "in_progress"
            return 1
        fi
    else
        info "Task #$task_num status is '$new_status' (not complete yet)"
        return 1
    fi
}

# ============================================================================
# Status Display
# ============================================================================

show_status() {
    local workset_dir="$1"

    echo ""
    echo "═══════════════════════════════════════════════════════════════"
    echo "Ralph Status: $workset_dir"
    echo "═══════════════════════════════════════════════════════════════"
    echo ""

    local total=0 complete=0 pending=0 in_progress=0 failed=0

    if is_multi_workset "$workset_dir"; then
        # Multi-workset: show status per workset
        while read -r ws_dir; do
            local workset_name
            workset_name=$(basename "$ws_dir")
            echo -e " ${BLUE}[$workset_name]${NC}"

            while read -r spec_file; do
                ((total++)) || true

                local task_num status icon
                task_num=$(get_frontmatter_field "$spec_file" "task")
                status=$(get_frontmatter_field "$spec_file" "status")

                case "$status" in
                    complete)    icon="${GREEN}✓${NC}"; ((complete++)) || true ;;
                    in_progress) icon="${BLUE}●${NC}"; ((in_progress++)) || true ;;
                    failed)      icon="${RED}✗${NC}"; ((failed++)) || true ;;
                    pending)     icon="${YELLOW}○${NC}"; ((pending++)) || true ;;
                    *)           icon="?" ;;
                esac

                printf "   %b #%-2s %-30s %s\n" "$icon" "$task_num" "$(basename "$spec_file")" "($status)"
            done < <(find "$ws_dir" -maxdepth 1 -name '[0-9][0-9]-*.md' -type f | sort)

            echo ""
        done < <(find_worksets "$workset_dir")
    else
        # Single workset
        while read -r spec_file; do
            ((total++)) || true

            local task_num status task_desc icon
            task_num=$(get_frontmatter_field "$spec_file" "task")
            status=$(get_frontmatter_field "$spec_file" "status")
            task_desc=$(grep -m1 '^# ' "$spec_file" | sed 's/^# //')

            case "$status" in
                complete)    icon="${GREEN}✓${NC}"; ((complete++)) || true ;;
                in_progress) icon="${BLUE}●${NC}"; ((in_progress++)) || true ;;
                failed)      icon="${RED}✗${NC}"; ((failed++)) || true ;;
                pending)     icon="${YELLOW}○${NC}"; ((pending++)) || true ;;
                *)           icon="?" ;;
            esac

            printf " %b #%-2s %-30s %s\n" "$icon" "$task_num" "$(basename "$spec_file")" "($status)"
        done < <(find_task_specs "$workset_dir")
    fi

    echo "───────────────────────────────────────────────────────────────"
    echo -e " Total: $total | Complete: ${GREEN}$complete${NC} | Pending: $pending | In Progress: ${BLUE}$in_progress${NC} | Failed: ${RED}$failed${NC}"
    echo ""
}

# ============================================================================
# Main
# ============================================================================

main() {
    local workset_dir=""
    local max_iterations=0
    local show_status_only=false
    DRY_RUN=false

    while [[ $# -gt 0 ]]; do
        case "$1" in
            --max-iterations)
                max_iterations="$2"
                shift 2
                ;;
            --dry-run)
                DRY_RUN=true
                shift
                ;;
            --status)
                show_status_only=true
                shift
                ;;
            --help|-h)
                echo "Usage: $0 <workset-dir> [options]"
                echo ""
                echo "Options:"
                echo "  --max-iterations N   Limit to N agent invocations (default: unlimited)"
                echo "  --dry-run            Show what would be done without executing"
                echo "  --status             Show current progress and exit"
                echo "  --help               Show this help"
                echo ""
                echo "Environment:"
                echo "  RALPH_CLAUDE_CMD      Agent command (default: claude)"
                echo ""
                echo "Each iteration invokes the agent once. Tasks are retried until"
                echo "the agent updates the spec's frontmatter status to 'complete'."
                echo ""
                echo "Status is tracked in YAML frontmatter of each task spec:"
                echo "  pending     - Not started"
                echo "  in_progress - Currently being worked on"
                echo "  complete    - Done, backpressure passed"
                exit 0
                ;;
            -*)
                die "Unknown option: $1"
                ;;
            *)
                workset_dir="$1"
                shift
                ;;
        esac
    done

    [[ -n "$workset_dir" ]] || die "Usage: $0 <workset-dir> [--max-iterations N] [--dry-run] [--status]"
    [[ -d "$workset_dir" ]] || die "Workset directory not found: $workset_dir"

    workset_dir="${workset_dir%/}"

    # Verify at least one task spec exists
    local spec_count
    spec_count=$(find_task_specs "$workset_dir" | wc -l | xargs)
    [[ "$spec_count" -gt 0 ]] || die "No task specs found in $workset_dir (expected files like 01-*.md)"

    if [[ "$show_status_only" == "true" ]]; then
        show_status "$workset_dir"
        exit 0
    fi

    echo "" >> "$RALPH_LOG_FILE"
    info "════════════════════════════════════════════════════════════════"
    info "Ralph starting: $workset_dir"
    if is_multi_workset "$workset_dir"; then
        local workset_count
        workset_count=$(find_worksets "$workset_dir" | wc -l | xargs)
        info "Mode: multi-workset ($workset_count worksets)"
    else
        info "Mode: single workset"
    fi
    info "Max iterations: ${max_iterations:-unlimited}"
    info "Agent: $RALPH_CLAUDE_CMD"
    info "Found $spec_count task specs"
    info "════════════════════════════════════════════════════════════════"

    local iteration=0

    while true; do
        ((iteration++)) || true

        if [[ $max_iterations -gt 0 && $iteration -gt $max_iterations ]]; then
            info "Reached maximum iterations ($max_iterations)"
            notify "Ralph Paused" "Reached $max_iterations iterations"
            break
        fi

        info "────────────────────────────────────────────────────────────────"
        info "Iteration $iteration"
        info "────────────────────────────────────────────────────────────────"

        local next_spec
        next_spec=$(find_next_task "$workset_dir")

        if [[ -z "$next_spec" ]]; then
            local complete_count
            complete_count=$(find_task_specs "$workset_dir" | while read -r f; do
                [[ "$(get_frontmatter_field "$f" "status")" == "complete" ]] && echo 1
            done | wc -l | xargs)

            if [[ "$complete_count" -eq "$spec_count" ]]; then
                success "All $spec_count tasks completed!"
                notify "Ralph Complete ✓" "All $spec_count tasks finished successfully"
            else
                warn "No actionable tasks found (some may be blocked or failed)"
            fi
            break
        fi

        local task_num spec_workset workset_name
        task_num=$(get_frontmatter_field "$next_spec" "task")
        spec_workset=$(get_spec_workset "$next_spec")
        workset_name=$(basename "$spec_workset")
        info "Next task: $workset_name/#$task_num - $(basename "$next_spec")"

        if execute_task "$workset_dir" "$next_spec"; then
            local task_desc
            task_desc=$(grep -m1 '^# ' "$next_spec" | sed 's/^# //')
            commit_task "$task_num" "$task_desc" "$workset_name"
        else
            # Task not complete yet - will retry on next iteration
            info "Task $workset_name/#$task_num not complete, will retry..."
        fi

        sleep 2
    done

    show_status "$workset_dir"
}

main "$@"
