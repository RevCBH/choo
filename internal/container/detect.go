package container

import (
	"errors"
	"os/exec"
)

// ErrNoRuntime is returned when no container runtime is found.
var ErrNoRuntime = errors.New("no container runtime found (need docker or podman)")

// DetectRuntime finds an available container runtime.
// Checks docker first, then podman. Verifies the binary actually works
// by running `<runtime> version`.
func DetectRuntime() (string, error) {
	for _, bin := range []string{"docker", "podman"} {
		if _, err := exec.LookPath(bin); err != nil {
			continue
		}
		cmd := exec.Command(bin, "version")
		if err := cmd.Run(); err != nil {
			continue
		}
		return bin, nil
	}
	return "", ErrNoRuntime
}
