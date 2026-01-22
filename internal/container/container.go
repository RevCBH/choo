package container

// ContainerID is a unique identifier for a container.
// This is the full container ID returned by `docker create`, not the short form.
type ContainerID string

// ContainerConfig specifies container creation parameters.
type ContainerConfig struct {
	// Image is the container image (e.g., "choo:latest")
	Image string

	// Name is the container name (e.g., "choo-job-abc123")
	Name string

	// Env contains environment variables to set in the container
	Env map[string]string

	// Cmd is the command and arguments to run
	Cmd []string

	// WorkDir is the working directory inside the container
	WorkDir string
}
