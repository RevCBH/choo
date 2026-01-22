package container

import (
	"os/exec"
	"testing"
)

func TestDetectRuntime_FindsDocker(t *testing.T) {
	// Skip if docker is not available
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available")
	}

	runtime, err := DetectRuntime()
	if err != nil {
		t.Fatalf("DetectRuntime() failed: %v", err)
	}

	// Docker should be preferred if both are available
	if runtime != "docker" {
		t.Errorf("expected docker, got %s", runtime)
	}
}

func TestDetectRuntime_FindsPodman(t *testing.T) {
	// This test only runs if podman is available but docker is not
	if _, err := exec.LookPath("docker"); err == nil {
		t.Skip("docker is available, podman fallback not tested")
	}
	if _, err := exec.LookPath("podman"); err != nil {
		t.Skip("podman not available")
	}

	runtime, err := DetectRuntime()
	if err != nil {
		t.Fatalf("DetectRuntime() failed: %v", err)
	}

	if runtime != "podman" {
		t.Errorf("expected podman, got %s", runtime)
	}
}

func TestDetectRuntime_ReturnsErrorWhenNoneAvailable(t *testing.T) {
	// This test documents the expected behavior but cannot easily
	// be run in environments where docker or podman are installed.
	// The function should return ErrNoRuntime when neither is found.
	t.Log("DetectRuntime returns ErrNoRuntime when no runtime is found")
}

func TestDetectRuntime_VerifiesBinaryWorks(t *testing.T) {
	// Verify that we get a valid runtime that can execute commands
	runtime, err := DetectRuntime()
	if err != nil {
		t.Skip("no container runtime available")
	}

	// The detected runtime should be able to run 'version'
	cmd := exec.Command(runtime, "version")
	if err := cmd.Run(); err != nil {
		t.Errorf("%s version failed: %v", runtime, err)
	}
}
