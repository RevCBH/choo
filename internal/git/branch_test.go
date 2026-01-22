package git

import (
	"context"
	"strings"
	"testing"
)

func TestGetWorkingDirStatus_Clean(t *testing.T) {
	fake := newFakeRunner()
	fake.stub("status --porcelain", "", nil)
	SetDefaultRunner(fake)
	defer SetDefaultRunner(nil)

	status, err := GetWorkingDirStatus(context.Background(), "/test/repo")
	if err != nil {
		t.Fatalf("GetWorkingDirStatus() returned error: %v", err)
	}

	if status.HasChanges {
		t.Error("GetWorkingDirStatus() HasChanges = true, want false for clean repo")
	}
	if len(status.ChangedFiles) != 0 {
		t.Errorf("GetWorkingDirStatus() ChangedFiles = %v, want empty", status.ChangedFiles)
	}
}

func TestGetWorkingDirStatus_WithChanges(t *testing.T) {
	fake := newFakeRunner()
	// Simulate modified and untracked files
	fake.stub("status --porcelain", " M internal/cli/run.go\n?? newfile.txt\n", nil)
	SetDefaultRunner(fake)
	defer SetDefaultRunner(nil)

	status, err := GetWorkingDirStatus(context.Background(), "/test/repo")
	if err != nil {
		t.Fatalf("GetWorkingDirStatus() returned error: %v", err)
	}

	if !status.HasChanges {
		t.Error("GetWorkingDirStatus() HasChanges = false, want true")
	}
	if len(status.ChangedFiles) != 2 {
		t.Errorf("GetWorkingDirStatus() ChangedFiles length = %d, want 2", len(status.ChangedFiles))
	}
}

func TestGetWorkingDirStatus_WatchPaths(t *testing.T) {
	fake := newFakeRunner()
	// Simulate changes in specs/ directory
	fake.stub("status --porcelain", " M specs/tasks/unit1/task.md\n M internal/cli/run.go\n", nil)
	SetDefaultRunner(fake)
	defer SetDefaultRunner(nil)

	status, err := GetWorkingDirStatus(context.Background(), "/test/repo", "specs/", "docs/")
	if err != nil {
		t.Fatalf("GetWorkingDirStatus() returned error: %v", err)
	}

	if !status.HasChanges {
		t.Error("GetWorkingDirStatus() HasChanges = false, want true")
	}
	if !status.PathChanges["specs/"] {
		t.Error("GetWorkingDirStatus() PathChanges[\"specs/\"] = false, want true")
	}
	if status.PathChanges["docs/"] {
		t.Error("GetWorkingDirStatus() PathChanges[\"docs/\"] = true, want false")
	}
}

func TestGetWorkingDirStatus_RenamedFiles(t *testing.T) {
	fake := newFakeRunner()
	// Simulate renamed file
	fake.stub("status --porcelain", "R  oldname.go -> newname.go\n", nil)
	SetDefaultRunner(fake)
	defer SetDefaultRunner(nil)

	status, err := GetWorkingDirStatus(context.Background(), "/test/repo")
	if err != nil {
		t.Fatalf("GetWorkingDirStatus() returned error: %v", err)
	}

	if !status.HasChanges {
		t.Error("GetWorkingDirStatus() HasChanges = false, want true")
	}
	if len(status.ChangedFiles) != 1 {
		t.Errorf("GetWorkingDirStatus() ChangedFiles length = %d, want 1", len(status.ChangedFiles))
	}
	// Should contain the new name
	if status.ChangedFiles[0] != "newname.go" {
		t.Errorf("GetWorkingDirStatus() ChangedFiles[0] = %q, want %q", status.ChangedFiles[0], "newname.go")
	}
}

// mockClaudeClient implements ClaudeClient for testing
type mockClaudeClient struct {
	response    string
	err         error
	resolveFunc func(ctx context.Context, opts InvokeOptions) (string, error)
	callCount   int
}

func (m *mockClaudeClient) Invoke(ctx context.Context, opts InvokeOptions) (string, error) {
	m.callCount++
	if m.resolveFunc != nil {
		return m.resolveFunc(ctx, opts)
	}
	return m.response, m.err
}

func TestSanitizeBranchName_Spaces(t *testing.T) {
	got := SanitizeBranchName("hello world")
	want := "hello-world"
	if got != want {
		t.Errorf("SanitizeBranchName(%q) = %q, want %q", "hello world", got, want)
	}
}

func TestSanitizeBranchName_Case(t *testing.T) {
	got := SanitizeBranchName("Hello World")
	want := "hello-world"
	if got != want {
		t.Errorf("SanitizeBranchName(%q) = %q, want %q", "Hello World", got, want)
	}
}

func TestSanitizeBranchName_Slashes(t *testing.T) {
	got := SanitizeBranchName("foo/bar")
	want := "foo-bar"
	if got != want {
		t.Errorf("SanitizeBranchName(%q) = %q, want %q", "foo/bar", got, want)
	}
}

func TestSanitizeBranchName_Dots(t *testing.T) {
	got := SanitizeBranchName("foo..bar")
	want := "foo-bar"
	if got != want {
		t.Errorf("SanitizeBranchName(%q) = %q, want %q", "foo..bar", got, want)
	}
}

func TestSanitizeBranchName_Special(t *testing.T) {
	got := SanitizeBranchName("special@#chars!")
	want := "special-chars"
	if got != want {
		t.Errorf("SanitizeBranchName(%q) = %q, want %q", "special@#chars!", got, want)
	}
}

func TestValidateBranchName_Valid(t *testing.T) {
	validNames := []string{
		"ralph/app-shell-sunset",
		"feature/add-login",
		"main",
		"develop",
		"bugfix/fix-123",
	}

	for _, name := range validNames {
		t.Run(name, func(t *testing.T) {
			err := ValidateBranchName(name)
			if err != nil {
				t.Errorf("ValidateBranchName(%q) returned error: %v", name, err)
			}
		})
	}
}

func TestValidateBranchName_Empty(t *testing.T) {
	err := ValidateBranchName("")
	if err == nil {
		t.Error("ValidateBranchName(\"\") should return error for empty name")
	}
}

func TestValidateBranchName_Refs(t *testing.T) {
	err := ValidateBranchName("refs/heads/main")
	if err == nil {
		t.Error("ValidateBranchName(\"refs/heads/main\") should return error for name starting with refs/")
	}
}

func TestValidateBranchName_DoubleDot(t *testing.T) {
	err := ValidateBranchName("branch..name")
	if err == nil {
		t.Error("ValidateBranchName(\"branch..name\") should return error for name containing ..")
	}
}

func TestValidateBranchName_Spaces(t *testing.T) {
	err := ValidateBranchName("branch name")
	if err == nil {
		t.Error("ValidateBranchName(\"branch name\") should return error for name containing spaces")
	}
}

func TestBranchNamer_GenerateName(t *testing.T) {
	mock := &mockClaudeClient{
		response: "sunset-harbor",
		err:      nil,
	}

	namer := NewBranchNamer(mock)
	ctx := context.Background()

	branchName, err := namer.GenerateName(ctx, "app-shell")
	if err != nil {
		t.Fatalf("GenerateName() returned error: %v", err)
	}

	// Should have the prefix
	if !strings.HasPrefix(branchName, "ralph/") {
		t.Errorf("GenerateName() = %q, should start with 'ralph/'", branchName)
	}

	// Should contain the unit ID
	if !strings.Contains(branchName, "app-shell") {
		t.Errorf("GenerateName() = %q, should contain 'app-shell'", branchName)
	}

	// Should be a valid branch name
	if err := ValidateBranchName(branchName); err != nil {
		t.Errorf("GenerateName() produced invalid branch name: %v", err)
	}
}

func TestBranchNamer_Fallback(t *testing.T) {
	// Mock that returns an error
	mock := &mockClaudeClient{
		response: "",
		err:      context.DeadlineExceeded,
	}

	namer := NewBranchNamer(mock)
	ctx := context.Background()

	branchName, err := namer.GenerateName(ctx, "app-shell")
	if err != nil {
		t.Fatalf("GenerateName() should not return error on Claude failure (should fallback): %v", err)
	}

	// Should have the prefix
	if !strings.HasPrefix(branchName, "ralph/") {
		t.Errorf("GenerateName() = %q, should start with 'ralph/'", branchName)
	}

	// Should contain the unit ID
	if !strings.Contains(branchName, "app-shell") {
		t.Errorf("GenerateName() = %q, should contain 'app-shell'", branchName)
	}

	// Should be a valid branch name
	if err := ValidateBranchName(branchName); err != nil {
		t.Errorf("GenerateName() with fallback produced invalid branch name: %v", err)
	}
}

func TestBranchNamer_NilClaude(t *testing.T) {
	// Test with nil Claude client (should use random suffix)
	namer := NewBranchNamer(nil)
	ctx := context.Background()

	branchName, err := namer.GenerateName(ctx, "test-unit")
	if err != nil {
		t.Fatalf("GenerateName() with nil Claude should not return error: %v", err)
	}

	// Should have the prefix
	if !strings.HasPrefix(branchName, "ralph/") {
		t.Errorf("GenerateName() = %q, should start with 'ralph/'", branchName)
	}

	// Should contain the unit ID
	if !strings.Contains(branchName, "test-unit") {
		t.Errorf("GenerateName() = %q, should contain 'test-unit'", branchName)
	}

	// Should be a valid branch name
	if err := ValidateBranchName(branchName); err != nil {
		t.Errorf("GenerateName() with nil Claude produced invalid branch name: %v", err)
	}
}

func TestRandomSuffix(t *testing.T) {
	suffix := randomSuffix()

	// Should be 6 characters
	if len(suffix) != 6 {
		t.Errorf("randomSuffix() returned %q (length %d), want length 6", suffix, len(suffix))
	}

	// Should only contain alphanumeric characters
	for _, r := range suffix {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')) {
			t.Errorf("randomSuffix() returned %q, contains invalid character %q", suffix, r)
		}
	}
}

func TestRandomSuffix_Uniqueness(t *testing.T) {
	// Generate multiple suffixes and check they're not all the same
	suffixes := make(map[string]bool)
	for i := 0; i < 100; i++ {
		suffix := randomSuffix()
		suffixes[suffix] = true
	}

	// With 100 random 6-character suffixes, we should have many unique ones
	// (probability of collision is very low)
	if len(suffixes) < 90 {
		t.Errorf("randomSuffix() generated only %d unique suffixes out of 100, expected more variety", len(suffixes))
	}
}

func TestSanitizeBranchName_Comprehensive(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello world", "hello-world"},
		{"Hello World", "hello-world"},
		{"foo/bar", "foo-bar"},
		{"foo..bar", "foo-bar"},
		{"special@#chars!", "special-chars"},
		{"  spaces  ", "spaces"},
		{"Multiple   Spaces", "multiple-spaces"},
		{"CamelCase", "camelcase"},
		{"with.dot", "with-dot"},
		{"multiple...dots", "multiple-dots"},
		{"trailing-", "trailing"},
		{"-leading", "leading"},
		{"---multiple---hyphens---", "multiple-hyphens"},
		{"under_score", "under-score"},
		{"mixed/chars@test#123", "mixed-chars-test-123"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := SanitizeBranchName(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeBranchName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidateBranchName_Comprehensive(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"ralph/app-shell-sunset", false},
		{"feature/add-login", false},
		{"main", false},
		{"", true},                  // Empty
		{"refs/heads/main", true},   // Starts with refs/
		{"branch..name", true},      // Contains ..
		{"branch name", true},       // Contains spaces
		{"-leading-hyphen", true},   // Starts with -
		{"trailing.dot.", true},     // Ends with .
		{"name.lock", true},         // Ends with .lock
		{"valid-branch-123", false}, // Valid with numbers
		{"v1.2.3", false},           // Valid version tag style
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBranchName(tt.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateBranchName(%q) error = %v, wantErr %v",
					tt.name, err, tt.wantErr)
			}
		})
	}
}
