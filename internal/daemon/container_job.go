package daemon

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/RevCBH/choo/internal/container"
	"github.com/RevCBH/choo/internal/daemon/db"
	"github.com/RevCBH/choo/internal/events"
	"github.com/oklog/ulid/v2"
)

// ManagedContainerJob tracks a running container job.
type ManagedContainerJob struct {
	ID        string
	Config    ContainerJobConfig
	State     *ContainerJobState
	Events    *events.Bus
	Streamer  *LogStreamer
	StartedAt time.Time
}

// StartContainerJob creates and starts a job in a container.
// It returns the job ID and starts container execution in the background.
func (jm *jobManagerImpl) StartContainerJob(ctx context.Context, cfg ContainerJobConfig) (string, error) {
	// 1. Generate job ID if not provided
	jobID := cfg.JobID
	if jobID == "" {
		jobID = ulid.Make().String()
	}

	// 2. Build container configuration
	containerCfg := container.ContainerConfig{
		Image: jm.cfg.ContainerImage,
		Name:  fmt.Sprintf("choo-%s", jobID),
		Env:   buildContainerEnv(cfg),
		Cmd: []string{
			"choo", "run",
			"--clone-url", cfg.GitURL,
			"--tasks", cfg.TasksDir,
			"--json-events",
		},
	}

	if cfg.Unit != "" {
		containerCfg.Cmd = append(containerCfg.Cmd, "--unit", cfg.Unit)
	}

	// 3. Record job in database
	now := time.Now()
	run := &db.Run{
		ID:            jobID,
		FeatureBranch: jobID,
		RepoPath:      cfg.RepoPath,
		TargetBranch:  cfg.TargetBranch,
		TasksDir:      cfg.TasksDir,
		Parallelism:   0,
		Status:        db.RunStatusRunning,
		DaemonVersion: "",
		StartedAt:     &now,
	}
	if err := jm.db.CreateRun(run); err != nil {
		return "", fmt.Errorf("failed to create run record: %w", err)
	}

	// 4. Create container
	containerID, err := jm.runtime.Create(ctx, containerCfg)
	if err != nil {
		_ = jm.markJobFailed(ctx, jobID, fmt.Sprintf("container create failed: %v", err))
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	// 5. Start container
	if err := jm.runtime.Start(ctx, containerID); err != nil {
		_ = jm.markJobFailed(ctx, jobID, fmt.Sprintf("container start failed: %v", err))
		return "", fmt.Errorf("failed to start container: %w", err)
	}

	// 6. Create event bus for this job
	jobEvents := events.NewBus(1000)
	startTime := time.Now()

	state := &ContainerJobState{
		ContainerID:   string(containerID),
		ContainerName: containerCfg.Name,
		Status:        ContainerStatusRunning,
		StartedAt:     &startTime,
	}

	// 7. Create and store managed job
	managed := &ManagedContainerJob{
		ID:        jobID,
		Config:    cfg,
		State:     state,
		Events:    jobEvents,
		StartedAt: startTime,
	}

	jm.mu.Lock()
	jm.containerJobs[jobID] = managed
	jm.mu.Unlock()

	// 8. Start log streamer in background
	streamer := NewLogStreamer(string(containerID), jm.runtime, jobEvents)
	managed.Streamer = streamer

	go func() {
		defer jm.cleanupContainerJob(jobID)

		// Start streaming logs
		if err := streamer.Start(ctx); err != nil && ctx.Err() == nil {
			log.Printf("Log streamer error for job %s: %v", jobID, err)
		}

		// Wait for container to exit
		exitCode, err := jm.runtime.Wait(ctx, containerID)
		if err != nil {
			jm.handleContainerFailure(ctx, jobID, string(containerID), err)
		} else if exitCode != 0 {
			jm.handleContainerFailure(ctx, jobID, string(containerID),
				fmt.Errorf("container exited with code %d", exitCode))
		} else {
			jm.markContainerJobComplete(ctx, jobID)
		}
	}()

	return jobID, nil
}

// GetContainerState returns the container state for a job.
func (jm *jobManagerImpl) GetContainerState(jobID string) (*ContainerJobState, error) {
	jm.mu.RLock()
	defer jm.mu.RUnlock()

	managed, ok := jm.containerJobs[jobID]
	if !ok {
		return nil, fmt.Errorf("container job not found: %s", jobID)
	}

	return managed.State, nil
}

// buildContainerEnv builds environment variables to pass to the container.
func buildContainerEnv(cfg ContainerJobConfig) map[string]string {
	env := map[string]string{
		"GIT_URL": cfg.GitURL,
	}

	// Pass through credential environment variables
	credVars := []string{
		"GITHUB_TOKEN",
		"ANTHROPIC_API_KEY",
		"OPENAI_API_KEY",
		"SSH_AUTH_SOCK", // For SSH agent forwarding
	}

	for _, v := range credVars {
		if val := os.Getenv(v); val != "" {
			env[v] = val
		}
	}

	return env
}

// handleContainerFailure captures container logs and marks the job as failed.
func (jm *jobManagerImpl) handleContainerFailure(ctx context.Context, jobID string, containerID string, err error) {
	// Get container logs for debugging
	logsReader, logsErr := jm.runtime.Logs(ctx, container.ContainerID(containerID))
	var logContent string
	if logsErr == nil {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, logsReader)
		_ = logsReader.Close()
		// Take last 100 lines
		logContent = tailLines(buf.String(), 100)
	}

	// Update job state
	jm.mu.Lock()
	if managed, ok := jm.containerJobs[jobID]; ok {
		managed.State.Status = ContainerStatusFailed
		managed.State.Error = err.Error()
		now := time.Now()
		managed.State.StoppedAt = &now
	}
	jm.mu.Unlock()

	// Update database
	if markErr := jm.markJobFailed(ctx, jobID, err.Error()); markErr != nil {
		log.Printf("failed to update run status: %v", markErr)
	}

	// Publish failure event
	jm.eventBus.Emit(events.NewEvent(events.OrchFailed, "").
		WithPayload(map[string]string{
			"job_id":  jobID,
			"error":   err.Error(),
			"details": logContent,
		}))
}

// markContainerJobComplete marks a container job as successfully completed.
func (jm *jobManagerImpl) markContainerJobComplete(ctx context.Context, jobID string) {
	jm.mu.Lock()
	if managed, ok := jm.containerJobs[jobID]; ok {
		managed.State.Status = ContainerStatusStopped
		exitCode := 0
		managed.State.ExitCode = &exitCode
		now := time.Now()
		managed.State.StoppedAt = &now
	}
	jm.mu.Unlock()

	if err := jm.markJobComplete(ctx, jobID); err != nil {
		log.Printf("failed to update run status: %v", err)
	}
}

// markJobComplete updates a job's database status to completed.
func (jm *jobManagerImpl) markJobComplete(ctx context.Context, runID string) error {
	return jm.db.UpdateRunStatus(runID, db.RunStatusCompleted, nil)
}

// cleanupContainerJob removes the container and cleans up resources.
func (jm *jobManagerImpl) cleanupContainerJob(jobID string) {
	jm.mu.Lock()
	managed, ok := jm.containerJobs[jobID]
	if ok {
		delete(jm.containerJobs, jobID)
	}
	jm.mu.Unlock()

	if ok && managed.State != nil {
		// Best-effort container removal
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = jm.runtime.Remove(ctx, container.ContainerID(managed.State.ContainerID))
	}
}

// tailLines returns the last n lines of a string.
func tailLines(s string, n int) string {
	lines := bytes.Split([]byte(s), []byte("\n"))
	if len(lines) <= n {
		return s
	}
	return string(bytes.Join(lines[len(lines)-n:], []byte("\n")))
}
