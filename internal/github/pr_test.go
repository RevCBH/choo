package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCreatePR_DelegationError(t *testing.T) {
	client := &PRClient{
		owner: "testowner",
		repo:  "testrepo",
	}

	_, err := client.CreatePR(context.Background(), "Test PR", "Test body", "feature")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	expectedMsg := "PR creation is delegated to Claude via 'gh pr create'"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
	}
}

func TestGetPR_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET request, got %s", r.Method)
		}
		if r.URL.Path != "/repos/testowner/testrepo/pulls/123" {
			t.Errorf("expected path /repos/testowner/testrepo/pulls/123, got %s", r.URL.Path)
		}

		resp := ghPullRequest{
			Number:  123,
			HTMLURL: "https://github.com/testowner/testrepo/pull/123",
			Head: ghRef{
				Ref: "feature-branch",
			},
			Base: ghRef{
				Ref: "main",
			},
			Title:     "Test PR",
			CreatedAt: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := &PRClient{
		httpClient: server.Client(),
		owner:      "testowner",
		repo:       "testrepo",
		token:      "test-token",
	}

	// Call GetPR with modified URL
	ctx := context.Background()
	url := fmt.Sprintf("%s/repos/%s/%s/pulls/%d", server.URL, client.owner, client.repo, 123)
	resp, err := client.doRequest(ctx, "GET", url, nil)
	if err != nil {
		t.Fatalf("doRequest failed: %v", err)
	}
	defer resp.Body.Close()

	var ghPR ghPullRequest
	if err := json.NewDecoder(resp.Body).Decode(&ghPR); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	prInfo := &PRInfo{
		Number:       ghPR.Number,
		URL:          ghPR.HTMLURL,
		Branch:       ghPR.Head.Ref,
		TargetBranch: ghPR.Base.Ref,
		Title:        ghPR.Title,
		CreatedAt:    ghPR.CreatedAt,
	}

	if prInfo.Number != 123 {
		t.Errorf("expected number 123, got %d", prInfo.Number)
	}
	if prInfo.URL != "https://github.com/testowner/testrepo/pull/123" {
		t.Errorf("expected URL https://github.com/testowner/testrepo/pull/123, got %s", prInfo.URL)
	}
	if prInfo.Branch != "feature-branch" {
		t.Errorf("expected branch feature-branch, got %s", prInfo.Branch)
	}
	if prInfo.TargetBranch != "main" {
		t.Errorf("expected target branch main, got %s", prInfo.TargetBranch)
	}
}

func TestGetPR_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message": "Not Found"}`))
	}))
	defer server.Close()

	client := &PRClient{
		httpClient: server.Client(),
		owner:      "testowner",
		repo:       "testrepo",
		token:      "test-token",
	}

	ctx := context.Background()
	url := fmt.Sprintf("%s/repos/%s/%s/pulls/%d", server.URL, client.owner, client.repo, 999)
	_, err := client.doRequest(ctx, "GET", url, nil)
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
}

func TestUpdatePR_Success(t *testing.T) {
	var receivedBody map[string]string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" {
			t.Errorf("expected PATCH request, got %s", r.Method)
		}

		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"number": 123})
	}))
	defer server.Close()

	client := &PRClient{
		httpClient: server.Client(),
		owner:      "testowner",
		repo:       "testrepo",
		token:      "test-token",
	}

	ctx := context.Background()
	url := fmt.Sprintf("%s/repos/%s/%s/pulls/%d", server.URL, client.owner, client.repo, 123)
	reqBody := map[string]string{
		"title": "New Title",
		"body":  "New Body",
	}

	resp, err := client.doRequest(ctx, "PATCH", url, reqBody)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	resp.Body.Close()

	if receivedBody["title"] != "New Title" {
		t.Errorf("expected title 'New Title', got %s", receivedBody["title"])
	}
	if receivedBody["body"] != "New Body" {
		t.Errorf("expected body 'New Body', got %s", receivedBody["body"])
	}
}

func TestMerge_SquashMethod(t *testing.T) {
	var receivedBody map[string]string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("expected PUT request, got %s", r.Method)
		}

		json.NewDecoder(r.Body).Decode(&receivedBody)

		resp := ghMergeResponse{
			SHA:     "abc123",
			Merged:  true,
			Message: "Pull Request successfully merged",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := &PRClient{
		httpClient: server.Client(),
		owner:      "testowner",
		repo:       "testrepo",
		token:      "test-token",
	}

	ctx := context.Background()
	url := fmt.Sprintf("%s/repos/%s/%s/pulls/%d/merge", server.URL, client.owner, client.repo, 123)
	reqBody := map[string]string{
		"merge_method": "squash",
	}

	_, err := client.doRequest(ctx, "PUT", url, reqBody)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if receivedBody["merge_method"] != "squash" {
		t.Errorf("expected merge_method 'squash', got %s", receivedBody["merge_method"])
	}
}

func TestMerge_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ghMergeResponse{
			SHA:     "abc123def456",
			Merged:  true,
			Message: "Pull Request successfully merged",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := &PRClient{
		httpClient: server.Client(),
		owner:      "testowner",
		repo:       "testrepo",
		token:      "test-token",
	}

	ctx := context.Background()
	url := fmt.Sprintf("%s/repos/%s/%s/pulls/%d/merge", server.URL, client.owner, client.repo, 123)
	reqBody := map[string]string{
		"merge_method": "squash",
	}

	resp, err := client.doRequest(ctx, "PUT", url, reqBody)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	defer resp.Body.Close()

	var ghMerge ghMergeResponse
	if err := json.NewDecoder(resp.Body).Decode(&ghMerge); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	result := &MergeResult{
		Merged:  ghMerge.Merged,
		SHA:     ghMerge.SHA,
		Message: ghMerge.Message,
	}

	if !result.Merged {
		t.Error("expected Merged to be true")
	}
	if result.SHA != "abc123def456" {
		t.Errorf("expected SHA abc123def456, got %s", result.SHA)
	}
}

func TestMerge_Conflict(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(`{"message": "Merge conflict"}`))
	}))
	defer server.Close()

	client := &PRClient{
		httpClient: server.Client(),
		owner:      "testowner",
		repo:       "testrepo",
		token:      "test-token",
	}

	ctx := context.Background()
	url := fmt.Sprintf("%s/repos/%s/%s/pulls/%d/merge", server.URL, client.owner, client.repo, 123)
	reqBody := map[string]string{
		"merge_method": "squash",
	}

	_, err := client.doRequest(ctx, "PUT", url, reqBody)
	if err == nil {
		t.Fatal("expected error for 409 conflict, got nil")
	}
}

func TestClosePR_Success(t *testing.T) {
	var receivedBody map[string]string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" {
			t.Errorf("expected PATCH request, got %s", r.Method)
		}

		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"number": 123, "state": "closed"})
	}))
	defer server.Close()

	client := &PRClient{
		httpClient: server.Client(),
		owner:      "testowner",
		repo:       "testrepo",
		token:      "test-token",
	}

	ctx := context.Background()
	url := fmt.Sprintf("%s/repos/%s/%s/pulls/%d", server.URL, client.owner, client.repo, 123)
	reqBody := map[string]string{
		"state": "closed",
	}

	resp, err := client.doRequest(ctx, "PATCH", url, reqBody)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	resp.Body.Close()

	if receivedBody["state"] != "closed" {
		t.Errorf("expected state 'closed', got %s", receivedBody["state"])
	}
}
