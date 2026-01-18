package vcs_provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/eridan-ltu/gitex/api"
	"github.com/eridan-ltu/gitex/internal/util"
	"github.com/google/go-github/v81/github"
)

func TestNewGitHubService(t *testing.T) {
	cfg := &api.Config{VcsApiKey: "test-token"}
	svc, err := NewGitHubService(cfg)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
	if svc.client == nil {
		t.Error("expected client to be initialized")
	}
}

func TestNewGitHubService_Enterprise(t *testing.T) {
	cfg := &api.Config{
		VcsApiKey:    "test-token",
		VcsRemoteUrl: "https://github.example.com/api/v3",
	}
	svc, err := NewGitHubService(cfg)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}

func TestGitHubService_parseWebUrl(t *testing.T) {
	svc := &GitHubService{}

	tests := []struct {
		name        string
		url         string
		wantOwner   string
		wantRepo    string
		wantNumber  int
		expectError bool
	}{
		{
			name:       "valid PR URL",
			url:        "https://github.com/owner/repo/pull/123",
			wantOwner:  "owner",
			wantRepo:   "repo",
			wantNumber: 123,
		},
		{
			name:       "valid PR URL with trailing slash",
			url:        "https://github.com/owner/repo/pull/456/",
			wantOwner:  "owner",
			wantRepo:   "repo",
			wantNumber: 456,
		},
		{
			name:       "PR URL with files tab",
			url:        "https://github.com/owner/repo/pull/789/files",
			wantOwner:  "owner",
			wantRepo:   "repo",
			wantNumber: 789,
		},
		{
			name:       "PR URL with commits tab",
			url:        "https://github.com/owner/repo/pull/101/commits",
			wantOwner:  "owner",
			wantRepo:   "repo",
			wantNumber: 101,
		},
		{
			name:       "PR URL with checks tab",
			url:        "https://github.com/owner/repo/pull/202/checks",
			wantOwner:  "owner",
			wantRepo:   "repo",
			wantNumber: 202,
		},
		{
			name:        "invalid URL - not a PR",
			url:         "https://github.com/owner/repo",
			expectError: true,
		},
		{
			name:        "invalid URL - issues instead of pull",
			url:         "https://github.com/owner/repo/issues/123",
			expectError: true,
		},
		{
			name:        "invalid URL - non-numeric PR number",
			url:         "https://github.com/owner/repo/pull/abc",
			expectError: true,
		},
		{
			name:        "invalid URL - malformed",
			url:         "://invalid",
			expectError: true,
		},
		{
			name:        "invalid URL - too few parts",
			url:         "https://github.com/owner",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo, number, err := svc.parseWebUrl(tt.url)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if owner != tt.wantOwner {
				t.Errorf("owner = %q, want %q", owner, tt.wantOwner)
			}
			if repo != tt.wantRepo {
				t.Errorf("repo = %q, want %q", repo, tt.wantRepo)
			}
			if number != tt.wantNumber {
				t.Errorf("number = %d, want %d", number, tt.wantNumber)
			}
		})
	}
}

func TestGitHubService_convertApiComment(t *testing.T) {
	svc := &GitHubService{}

	t.Run("nil input", func(t *testing.T) {
		result := svc.convertApiComment(nil)
		if result != nil {
			t.Error("expected nil for nil input")
		}
	})

	t.Run("nil position", func(t *testing.T) {
		result := svc.convertApiComment(&api.InlineComment{Body: util.Ptr("test")})
		if result != nil {
			t.Error("expected nil for nil position")
		}
	})

	t.Run("new line comment", func(t *testing.T) {
		comment := &api.InlineComment{
			Body:     util.Ptr("test comment"),
			CommitID: util.Ptr("abc123"),
			Position: &api.InlineCommentPosition{
				NewPath: util.Ptr("file.go"),
				NewLine: util.Ptr(int64(42)),
			},
		}

		result := svc.convertApiComment(comment)

		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if *result.Body != "test comment" {
			t.Errorf("body = %q, want %q", *result.Body, "test comment")
		}
		if *result.CommitID != "abc123" {
			t.Errorf("commitID = %q, want %q", *result.CommitID, "abc123")
		}
		if *result.Path != "file.go" {
			t.Errorf("path = %q, want %q", *result.Path, "file.go")
		}
		if *result.Line != 42 {
			t.Errorf("line = %d, want %d", *result.Line, 42)
		}
		if *result.Side != "RIGHT" {
			t.Errorf("side = %q, want %q", *result.Side, "RIGHT")
		}
	})

	t.Run("old line comment", func(t *testing.T) {
		comment := &api.InlineComment{
			Body: util.Ptr("deleted line comment"),
			Position: &api.InlineCommentPosition{
				OldPath: util.Ptr("old_file.go"),
				OldLine: util.Ptr(int64(10)),
			},
		}

		result := svc.convertApiComment(comment)

		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if *result.Path != "old_file.go" {
			t.Errorf("path = %q, want %q", *result.Path, "old_file.go")
		}
		if *result.Line != 10 {
			t.Errorf("line = %d, want %d", *result.Line, 10)
		}
		if *result.Side != "LEFT" {
			t.Errorf("side = %q, want %q", *result.Side, "LEFT")
		}
	})

	t.Run("prefers new path over old path", func(t *testing.T) {
		comment := &api.InlineComment{
			Body: util.Ptr("test"),
			Position: &api.InlineCommentPosition{
				NewPath: util.Ptr("new.go"),
				OldPath: util.Ptr("old.go"),
				NewLine: util.Ptr(int64(1)),
			},
		}

		result := svc.convertApiComment(comment)

		if *result.Path != "new.go" {
			t.Errorf("path = %q, want %q", *result.Path, "new.go")
		}
	})

	t.Run("line range with new lines", func(t *testing.T) {
		comment := &api.InlineComment{
			Body: util.Ptr("range comment"),
			Position: &api.InlineCommentPosition{
				NewPath: util.Ptr("file.go"),
				NewLine: util.Ptr(int64(20)),
				LineRange: &api.LineRangeOptions{
					Start: &api.LinePositionOptions{
						NewLine: util.Ptr(int64(15)),
					},
				},
			},
		}

		result := svc.convertApiComment(comment)

		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if *result.StartLine != 15 {
			t.Errorf("startLine = %d, want %d", *result.StartLine, 15)
		}
		if *result.StartSide != "RIGHT" {
			t.Errorf("startSide = %q, want %q", *result.StartSide, "RIGHT")
		}
	})

	t.Run("line range with old lines", func(t *testing.T) {
		comment := &api.InlineComment{
			Body: util.Ptr("range comment"),
			Position: &api.InlineCommentPosition{
				OldPath: util.Ptr("file.go"),
				OldLine: util.Ptr(int64(20)),
				LineRange: &api.LineRangeOptions{
					Start: &api.LinePositionOptions{
						OldLine: util.Ptr(int64(15)),
					},
				},
			},
		}

		result := svc.convertApiComment(comment)

		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if *result.StartLine != 15 {
			t.Errorf("startLine = %d, want %d", *result.StartLine, 15)
		}
		if *result.StartSide != "LEFT" {
			t.Errorf("startSide = %q, want %q", *result.StartSide, "LEFT")
		}
	})
}

func TestGithubRetryPolicy(t *testing.T) {
	tests := []struct {
		name        string
		statusCode  int
		headers     map[string]string
		err         error
		wantRetry   bool
		wantErr     bool
		ctxCanceled bool
	}{
		{
			name:       "200 OK - no retry",
			statusCode: 200,
			wantRetry:  false,
		},
		{
			name:       "201 Created - no retry",
			statusCode: 201,
			wantRetry:  false,
		},
		{
			name:       "400 Bad Request - no retry",
			statusCode: 400,
			wantRetry:  false,
		},
		{
			name:       "401 Unauthorized - no retry",
			statusCode: 401,
			wantRetry:  false,
		},
		{
			name:       "403 Forbidden without rate limit - no retry",
			statusCode: 403,
			headers:    map[string]string{"X-RateLimit-Remaining": "100"},
			wantRetry:  false,
		},
		{
			name:       "403 Forbidden with rate limit exhausted - retry",
			statusCode: 403,
			headers:    map[string]string{"X-RateLimit-Remaining": "0"},
			wantRetry:  true,
		},
		{
			name:       "404 Not Found - no retry",
			statusCode: 404,
			wantRetry:  false,
		},
		{
			name:       "422 Unprocessable Entity - no retry",
			statusCode: 422,
			wantRetry:  false,
		},
		{
			name:       "429 Too Many Requests - retry",
			statusCode: 429,
			wantRetry:  true,
		},
		{
			name:       "500 Internal Server Error - retry",
			statusCode: 500,
			wantRetry:  true,
		},
		{
			name:       "501 Not Implemented - no retry",
			statusCode: 501,
			wantRetry:  false,
		},
		{
			name:       "502 Bad Gateway - retry",
			statusCode: 502,
			wantRetry:  true,
		},
		{
			name:       "503 Service Unavailable - retry",
			statusCode: 503,
			wantRetry:  true,
		},
		{
			name:      "connection error - retry",
			err:       context.DeadlineExceeded,
			wantRetry: true,
		},
		{
			name:        "context canceled - no retry, return error",
			ctxCanceled: true,
			wantRetry:   false,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.ctxCanceled {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}

			var resp *http.Response
			if tt.err == nil && !tt.ctxCanceled {
				resp = &http.Response{
					StatusCode: tt.statusCode,
					Header:     make(http.Header),
				}
				for k, v := range tt.headers {
					resp.Header.Set(k, v)
				}
			}

			retry, err := githubRetryPolicy(ctx, resp, tt.err)

			if tt.wantErr && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if retry != tt.wantRetry {
				t.Errorf("retry = %v, want %v", retry, tt.wantRetry)
			}
		})
	}
}

func TestGitHubService_SendInlineComments(t *testing.T) {
	t.Run("successful comment", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "/pulls/") && strings.Contains(r.URL.Path, "/comments") {
				w.WriteHeader(http.StatusCreated)
				_ = json.NewEncoder(w).Encode(github.PullRequestComment{ID: github.Ptr(int64(1))})
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		cfg := &api.Config{VcsApiKey: "test-token", VcsRemoteUrl: server.URL}
		svc, _ := NewGitHubService(cfg)

		comments := []*api.InlineComment{
			{
				Body:     util.Ptr("test comment"),
				CommitID: util.Ptr("abc123"),
				Position: &api.InlineCommentPosition{
					NewPath: util.Ptr("file.go"),
					NewLine: util.Ptr(int64(10)),
				},
			},
		}
		prInfo := &api.PullRequestInfo{
			Owner:         "owner",
			ProjectName:   "repo",
			PullRequestId: 1,
		}

		err := svc.SendInlineComments(comments, prInfo)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("client error 422 - no retry, logs error", func(t *testing.T) {
		requestCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCount++
			w.WriteHeader(http.StatusUnprocessableEntity)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"message": "Validation Failed",
				"errors": []map[string]string{
					{"resource": "PullRequestReviewComment", "field": "line", "code": "invalid", "message": "line must be part of the diff"},
				},
			})
		}))
		defer server.Close()

		cfg := &api.Config{VcsApiKey: "test-token", VcsRemoteUrl: server.URL}
		svc, _ := NewGitHubService(cfg)

		comments := []*api.InlineComment{
			{
				Body:     util.Ptr("test"),
				CommitID: util.Ptr("abc"),
				Position: &api.InlineCommentPosition{
					NewPath: util.Ptr("file.go"),
					NewLine: util.Ptr(int64(999)),
				},
			},
		}
		prInfo := &api.PullRequestInfo{Owner: "owner", ProjectName: "repo", PullRequestId: 1}

		err := svc.SendInlineComments(comments, prInfo)
		if err == nil {
			t.Error("expected error for failed comment")
		}
		// Should only be called once (no retry for 422)
		if requestCount != 1 {
			t.Errorf("expected 1 request (no retry), got %d", requestCount)
		}
	})

	t.Run("server error 500 - retries", func(t *testing.T) {
		requestCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCount++
			if requestCount < 3 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(github.PullRequestComment{ID: github.Ptr(int64(1))})
		}))
		defer server.Close()

		cfg := &api.Config{VcsApiKey: "test-token", VcsRemoteUrl: server.URL}
		svc, _ := NewGitHubService(cfg)

		comments := []*api.InlineComment{
			{
				Body:     util.Ptr("test"),
				CommitID: util.Ptr("abc"),
				Position: &api.InlineCommentPosition{
					NewPath: util.Ptr("file.go"),
					NewLine: util.Ptr(int64(10)),
				},
			},
		}
		prInfo := &api.PullRequestInfo{Owner: "owner", ProjectName: "repo", PullRequestId: 1}

		err := svc.SendInlineComments(comments, prInfo)
		if err != nil {
			t.Errorf("unexpected error after retry: %v", err)
		}
		if requestCount < 2 {
			t.Errorf("expected retries, got %d requests", requestCount)
		}
	})

	t.Run("skips nil comments", func(t *testing.T) {
		requestCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCount++
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(github.PullRequestComment{ID: github.Ptr(int64(1))})
		}))
		defer server.Close()

		cfg := &api.Config{VcsApiKey: "test-token", VcsRemoteUrl: server.URL}
		svc, _ := NewGitHubService(cfg)

		comments := []*api.InlineComment{
			nil,
			{Body: util.Ptr("no position")}, // nil position -> skipped
			{
				Body:     util.Ptr("valid"),
				CommitID: util.Ptr("abc"),
				Position: &api.InlineCommentPosition{
					NewPath: util.Ptr("file.go"),
					NewLine: util.Ptr(int64(10)),
				},
			},
		}
		prInfo := &api.PullRequestInfo{Owner: "owner", ProjectName: "repo", PullRequestId: 1}

		err := svc.SendInlineComments(comments, prInfo)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if requestCount != 1 {
			t.Errorf("expected 1 request (only valid comment), got %d", requestCount)
		}
	})

	t.Run("multiple comments with partial failure", func(t *testing.T) {
		requestCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCount++
			if requestCount == 2 {
				w.WriteHeader(http.StatusUnprocessableEntity)
				_ = json.NewEncoder(w).Encode(map[string]string{"message": "Validation Failed"})
				return
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(github.PullRequestComment{ID: github.Ptr(int64(1))})
		}))
		defer server.Close()

		cfg := &api.Config{VcsApiKey: "test-token", VcsRemoteUrl: server.URL}
		svc, _ := NewGitHubService(cfg)

		comments := []*api.InlineComment{
			{Body: util.Ptr("c1"), CommitID: util.Ptr("abc"), Position: &api.InlineCommentPosition{NewPath: util.Ptr("a.go"), NewLine: util.Ptr(int64(1))}},
			{Body: util.Ptr("c2"), CommitID: util.Ptr("abc"), Position: &api.InlineCommentPosition{NewPath: util.Ptr("b.go"), NewLine: util.Ptr(int64(2))}},
			{Body: util.Ptr("c3"), CommitID: util.Ptr("abc"), Position: &api.InlineCommentPosition{NewPath: util.Ptr("c.go"), NewLine: util.Ptr(int64(3))}},
		}
		prInfo := &api.PullRequestInfo{Owner: "owner", ProjectName: "repo", PullRequestId: 1}

		err := svc.SendInlineComments(comments, prInfo)
		if err == nil {
			t.Error("expected error for partial failure")
		}
		if !strings.Contains(err.Error(), "1 comments") {
			t.Errorf("error should mention 1 failed comment: %v", err)
		}
	})
}
