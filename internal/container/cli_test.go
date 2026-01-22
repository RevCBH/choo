package container

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

func TestCLIManager_ImplementsManagerInterface(t *testing.T) {
	var _ Manager = (*CLIManager)(nil)
}

func TestCLIManager_NewCLIManager(t *testing.T) {
	mgr := NewCLIManager("docker")
	if mgr == nil {
		t.Fatal("NewCLIManager returned nil")
	}
	if mgr.runtime != "docker" {
		t.Errorf("expected runtime docker, got %s", mgr.runtime)
	}
}

func TestCLIManager_FullLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	runtime, err := DetectRuntime()
	if err != nil {
		t.Skip("no container runtime available")
	}

	mgr := NewCLIManager(runtime)
	ctx := context.Background()

	cfg := ContainerConfig{
		Image: "alpine:latest",
		Name:  fmt.Sprintf("test-%d", time.Now().UnixNano()),
		Cmd:   []string{"sh", "-c", "echo hello && exit 42"},
	}

	// Create
	id, err := mgr.Create(ctx, cfg)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	t.Cleanup(func() {
		mgr.Remove(context.Background(), id)
	})

	if id == "" {
		t.Error("Create returned empty container ID")
	}

	// Start
	if err := mgr.Start(ctx, id); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Wait
	exitCode, err := mgr.Wait(ctx, id)
	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}
	if exitCode != 42 {
		t.Errorf("expected exit code 42, got %d", exitCode)
	}
}

func TestCLIManager_LogStreaming(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	runtime, err := DetectRuntime()
	if err != nil {
		t.Skip("no container runtime available")
	}

	mgr := NewCLIManager(runtime)
	ctx := context.Background()

	cfg := ContainerConfig{
		Image: "alpine:latest",
		Name:  fmt.Sprintf("test-logs-%d", time.Now().UnixNano()),
		Cmd:   []string{"sh", "-c", "echo line1 && echo line2 && echo line3"},
	}

	id, err := mgr.Create(ctx, cfg)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	t.Cleanup(func() {
		mgr.Remove(context.Background(), id)
	})

	if err := mgr.Start(ctx, id); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Wait for container to finish first
	mgr.Wait(ctx, id)

	// Now get logs
	logs, err := mgr.Logs(ctx, id)
	if err != nil {
		t.Fatalf("Logs failed: %v", err)
	}
	defer logs.Close()

	output, err := io.ReadAll(logs)
	if err != nil {
		t.Fatalf("failed to read logs: %v", err)
	}

	if !strings.Contains(string(output), "line1") {
		t.Error("logs missing expected output 'line1'")
	}
	if !strings.Contains(string(output), "line2") {
		t.Error("logs missing expected output 'line2'")
	}
	if !strings.Contains(string(output), "line3") {
		t.Error("logs missing expected output 'line3'")
	}
}

func TestCLIManager_CreateWithEnvAndWorkDir(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	runtime, err := DetectRuntime()
	if err != nil {
		t.Skip("no container runtime available")
	}

	mgr := NewCLIManager(runtime)
	ctx := context.Background()

	cfg := ContainerConfig{
		Image:   "alpine:latest",
		Name:    fmt.Sprintf("test-env-%d", time.Now().UnixNano()),
		Env:     map[string]string{"TEST_VAR": "test_value"},
		WorkDir: "/tmp",
		Cmd:     []string{"sh", "-c", "echo $TEST_VAR && pwd"},
	}

	id, err := mgr.Create(ctx, cfg)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	t.Cleanup(func() {
		mgr.Remove(context.Background(), id)
	})

	if err := mgr.Start(ctx, id); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	mgr.Wait(ctx, id)

	logs, err := mgr.Logs(ctx, id)
	if err != nil {
		t.Fatalf("Logs failed: %v", err)
	}
	defer logs.Close()

	output, _ := io.ReadAll(logs)
	outputStr := string(output)

	if !strings.Contains(outputStr, "test_value") {
		t.Error("environment variable not set correctly")
	}
	if !strings.Contains(outputStr, "/tmp") {
		t.Error("working directory not set correctly")
	}
}

func TestCLIManager_StopContainer(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	runtime, err := DetectRuntime()
	if err != nil {
		t.Skip("no container runtime available")
	}

	mgr := NewCLIManager(runtime)
	ctx := context.Background()

	cfg := ContainerConfig{
		Image: "alpine:latest",
		Name:  fmt.Sprintf("test-stop-%d", time.Now().UnixNano()),
		Cmd:   []string{"sleep", "300"},
	}

	id, err := mgr.Create(ctx, cfg)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	t.Cleanup(func() {
		mgr.Remove(context.Background(), id)
	})

	if err := mgr.Start(ctx, id); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Stop with short timeout
	if err := mgr.Stop(ctx, id, 1*time.Second); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Container should now be stopped, Remove should work
	if err := mgr.Remove(ctx, id); err != nil {
		t.Errorf("Remove after stop failed: %v", err)
	}
}
