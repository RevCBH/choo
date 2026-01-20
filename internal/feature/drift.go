package feature

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// PRD represents a Product Requirements Document
type PRD struct {
	Body  string
	Units []Unit
}

// Unit represents a work unit in the PRD
type Unit struct {
	Name   string
	Status string
}

// ClaudeClient defines the interface for Claude AI client
type ClaudeClient interface {
	AssessDrift(ctx context.Context, req AssessDriftRequest) (*DriftAssessment, error)
}

// DriftDetector monitors PRD changes and assesses impact
type DriftDetector struct {
	prd          *PRD
	lastBodyHash string
	lastBody     string
	claude       ClaudeClient
}

// DriftResult contains the assessment of PRD changes
type DriftResult struct {
	HasDrift       bool
	Significant    bool
	Changes        string   // Diff summary
	AffectedUnits  []string
	Recommendation string
}

// AssessDriftRequest is the request to Claude for drift assessment
type AssessDriftRequest struct {
	OriginalBody    string
	NewBody         string
	Diff            string
	InProgressUnits []string
}

// DriftAssessment is the response from Claude's drift assessment
type DriftAssessment struct {
	Significant    bool
	AffectedUnits  []string
	Recommendation string
}

// NewDriftDetector creates a drift detector for the given PRD
func NewDriftDetector(prd *PRD, claudeClient ClaudeClient) *DriftDetector {
	currentBody := prd.Body
	return &DriftDetector{
		prd:          prd,
		lastBodyHash: hashBody(currentBody),
		lastBody:     currentBody,
		claude:       claudeClient,
	}
}

// CheckDrift compares current PRD body to last known state
func (d *DriftDetector) CheckDrift(ctx context.Context) (*DriftResult, error) {
	currentHash := hashBody(d.prd.Body)
	if currentHash == d.lastBodyHash {
		return &DriftResult{HasDrift: false}, nil
	}

	// Compute diff
	diff := computeDiff(d.lastBody, d.prd.Body)

	// Invoke Claude to assess impact
	assessment, err := d.claude.AssessDrift(ctx, AssessDriftRequest{
		OriginalBody:    d.lastBody,
		NewBody:         d.prd.Body,
		Diff:            diff,
		InProgressUnits: d.getInProgressUnits(),
	})
	if err != nil {
		return nil, fmt.Errorf("drift assessment failed: %w", err)
	}

	return &DriftResult{
		HasDrift:       true,
		Significant:    assessment.Significant,
		Changes:        diff,
		AffectedUnits:  assessment.AffectedUnits,
		Recommendation: assessment.Recommendation,
	}, nil
}

// UpdateBaseline sets the current PRD body as the new baseline
func (d *DriftDetector) UpdateBaseline() {
	d.lastBody = d.prd.Body
	d.lastBodyHash = hashBody(d.prd.Body)
}

// getInProgressUnits returns list of units currently in progress
func (d *DriftDetector) getInProgressUnits() []string {
	var inProgress []string
	for _, unit := range d.prd.Units {
		if unit.Status == "in_progress" {
			inProgress = append(inProgress, unit.Name)
		}
	}
	return inProgress
}

// hashBody computes SHA256 hash of PRD body for fast comparison
func hashBody(body string) string {
	hash := sha256.Sum256([]byte(body))
	return hex.EncodeToString(hash[:])
}

// computeDiff generates a diff summary between old and new body
func computeDiff(oldBody, newBody string) string {
	if oldBody == "" && newBody == "" {
		return "No changes"
	}
	if oldBody == "" {
		return fmt.Sprintf("Added: %d characters", len(newBody))
	}
	if newBody == "" {
		return fmt.Sprintf("Removed: %d characters", len(oldBody))
	}

	// Simple line-based diff
	oldLines := strings.Split(oldBody, "\n")
	newLines := strings.Split(newBody, "\n")

	var changes []string
	if len(newLines) > len(oldLines) {
		changes = append(changes, fmt.Sprintf("+%d lines", len(newLines)-len(oldLines)))
	} else if len(newLines) < len(oldLines) {
		changes = append(changes, fmt.Sprintf("-%d lines", len(oldLines)-len(newLines)))
	}

	// Character diff
	charDiff := len(newBody) - len(oldBody)
	if charDiff > 0 {
		changes = append(changes, fmt.Sprintf("+%d chars", charDiff))
	} else if charDiff < 0 {
		changes = append(changes, fmt.Sprintf("%d chars", charDiff))
	}

	if len(changes) == 0 {
		return "Content modified (same length)"
	}

	return strings.Join(changes, ", ")
}
