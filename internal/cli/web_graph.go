package cli

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/RevCBH/choo/internal/discovery"
	"github.com/RevCBH/choo/internal/git"
	"github.com/RevCBH/choo/internal/scheduler"
	"github.com/RevCBH/choo/internal/web"
)

func buildWebState(tasksDir, repoRoot string) (*web.GraphData, []*web.UnitState, error) {
	units, err := discovery.Discover(tasksDir)
	if err != nil {
		return nil, nil, fmt.Errorf("discover units: %w", err)
	}
	if len(units) == 0 {
		graph := &web.GraphData{
			Nodes:  []web.GraphNode{},
			Edges:  []web.GraphEdge{},
			Levels: [][]string{},
		}
		return graph, []*web.UnitState{}, nil
	}

	if repoRoot != "" {
		refreshUnitsFromWorktrees(context.Background(), repoRoot, units)
	}

	graph, err := scheduler.NewGraph(units)
	if err != nil {
		return nil, nil, fmt.Errorf("build graph: %w", err)
	}

	return buildWebGraphData(units, graph.GetLevels()), buildWebUnitStates(units), nil
}

func refreshUnitsFromWorktrees(ctx context.Context, repoRoot string, units []*discovery.Unit) {
	wtManager := git.NewWorktreeManager(repoRoot, nil)

	for _, unit := range units {
		wt, err := wtManager.GetWorktree(ctx, unit.ID)
		if err != nil || wt == nil {
			continue
		}

		unitPath := unit.Path
		if filepath.IsAbs(unitPath) {
			relPath, err := filepath.Rel(repoRoot, unitPath)
			if err != nil {
				continue
			}
			unitPath = relPath
		}

		wtUnitDir := filepath.Join(wt.Path, unitPath)
		wtUnit, err := discovery.DiscoverUnit(wtUnitDir)
		if err != nil || wtUnit == nil {
			continue
		}

		unit.Status = wtUnit.Status
		unit.Tasks = wtUnit.Tasks
		unit.DependsOn = wtUnit.DependsOn
		unit.Provider = wtUnit.Provider
		unit.StartedAt = wtUnit.StartedAt
		unit.CompletedAt = wtUnit.CompletedAt
	}
}

func buildWebGraphData(units []*discovery.Unit, levels [][]string) *web.GraphData {
	levelMap := make(map[string]int)
	unitMap := make(map[string]*discovery.Unit, len(units))
	for i, level := range levels {
		for _, unitID := range level {
			levelMap[unitID] = i
		}
	}
	for _, unit := range units {
		unitMap[unit.ID] = unit
	}

	nodes := make([]web.GraphNode, 0, len(units))
	for _, unit := range units {
		completedTasks := 0
		for _, task := range unit.Tasks {
			if task != nil && task.Status == discovery.TaskStatusComplete {
				completedTasks++
			}
		}

		status := "pending"
		if unit.Status != "" {
			status = string(unit.Status)
		}

		primaryParent := ""
		for _, dep := range unit.DependsOn {
			if _, ok := unitMap[dep]; ok {
				primaryParent = dep
				break
			}
		}

		nodes = append(nodes, web.GraphNode{
			ID:             unit.ID,
			Level:          levelMap[unit.ID],
			Tasks:          len(unit.Tasks),
			Status:         status,
			CompletedTasks: completedTasks,
			PrimaryParent:  primaryParent,
		})
	}

	depMap := make(map[string][]string)
	for _, unit := range units {
		depMap[unit.ID] = unit.DependsOn
	}

	var edges []web.GraphEdge
	for _, unit := range units {
		directDeps := unit.DependsOn
		if len(directDeps) <= 1 {
			for _, dep := range directDeps {
				edges = append(edges, web.GraphEdge{
					From: dep,
					To:   unit.ID,
				})
			}
			continue
		}

		for _, dep := range directDeps {
			isTransitive := false
			for _, otherDep := range directDeps {
				if otherDep == dep {
					continue
				}
				if isReachable(otherDep, dep, depMap) {
					isTransitive = true
					break
				}
			}
			if !isTransitive {
				edges = append(edges, web.GraphEdge{
					From: dep,
					To:   unit.ID,
				})
			}
		}
	}

	levelsData := make([][]string, len(levels))
	for i, level := range levels {
		levelsData[i] = make([]string, len(level))
		copy(levelsData[i], level)
	}

	return &web.GraphData{
		Nodes:  nodes,
		Edges:  edges,
		Levels: levelsData,
	}
}

func buildWebUnitStates(units []*discovery.Unit) []*web.UnitState {
	states := make([]*web.UnitState, 0, len(units))
	for _, unit := range units {
		totalTasks := len(unit.Tasks)
		completedTasks := 0
		currentTask := -1

		for _, task := range unit.Tasks {
			switch task.Status {
			case discovery.TaskStatusComplete:
				completedTasks++
			case discovery.TaskStatusInProgress:
				if currentTask == -1 {
					currentTask = task.Number - 1
				}
			}
		}

		status := string(unit.Status)
		if status == "" {
			status = "pending"
		}

		if status == string(discovery.UnitStatusComplete) && totalTasks > 0 {
			currentTask = totalTasks - 1
		}

		states = append(states, &web.UnitState{
			ID:             unit.ID,
			Status:         status,
			CurrentTask:    currentTask,
			TotalTasks:     totalTasks,
			CompletedTasks: completedTasks,
		})
	}

	return states
}

func isReachable(start, target string, depMap map[string][]string) bool {
	visited := make(map[string]bool)
	queue := []string{start}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if visited[current] {
			continue
		}
		visited[current] = true

		for _, dep := range depMap[current] {
			if dep == target {
				return true
			}
			if !visited[dep] {
				queue = append(queue, dep)
			}
		}
	}

	return false
}
