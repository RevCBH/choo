package worker

import (
	"strings"
	"testing"

	"github.com/RevCBH/choo/internal/discovery"
)

func TestBuildTaskPrompt_SingleTask(t *testing.T) {
	tasks := []*discovery.Task{
		{Number: 1, Title: "Nav Types", FilePath: "01-nav-types.md", Backpressure: "pnpm typecheck"},
	}

	prompt := BuildTaskPrompt(tasks)

	if !strings.Contains(prompt.Content, "Task #1: Nav Types") {
		t.Error("prompt should contain task title")
	}
	if !strings.Contains(prompt.Content, "pnpm typecheck") {
		t.Error("prompt should contain backpressure command")
	}
	if len(prompt.ReadyTasks) != 1 {
		t.Errorf("expected 1 ready task, got %d", len(prompt.ReadyTasks))
	}
}

func TestBuildTaskPrompt_MultipleTasks(t *testing.T) {
	tasks := []*discovery.Task{
		{Number: 1, Title: "Task A", FilePath: "01-a.md", Backpressure: "cmd-a"},
		{Number: 2, Title: "Task B", FilePath: "02-b.md", Backpressure: "cmd-b"},
		{Number: 3, Title: "Task C", FilePath: "03-c.md", Backpressure: "cmd-c"},
	}

	prompt := BuildTaskPrompt(tasks)

	for _, task := range tasks {
		if !strings.Contains(prompt.Content, task.Title) {
			t.Errorf("prompt should contain task %q", task.Title)
		}
	}
	if !strings.Contains(prompt.Content, "Choose ONE") {
		t.Error("prompt should instruct to choose one task")
	}
}

func TestBuildTaskPrompt_EmptyTasks(t *testing.T) {
	prompt := BuildTaskPrompt([]*discovery.Task{})

	if prompt.Content == "" {
		t.Error("should still have instruction content")
	}
	if len(prompt.ReadyTasks) != 0 {
		t.Error("should have empty ready tasks")
	}
}

func TestBuildBaselineFixPrompt(t *testing.T) {
	output := "fmt: main.go has incorrect formatting"
	commands := "go fmt ./..."

	prompt := BuildBaselineFixPrompt(output, commands)

	if !strings.Contains(prompt, output) {
		t.Error("prompt should contain check output")
	}
	if !strings.Contains(prompt, commands) {
		t.Error("prompt should contain baseline commands")
	}
	if !strings.Contains(prompt, "Do NOT commit") {
		t.Error("prompt should instruct not to commit")
	}
}
