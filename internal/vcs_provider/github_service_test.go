package vcs_provider

import (
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
	tests := []struct {
		name        string
		cfg         *api.Config
		expectError bool
	}{
		{
			name:        "valid config",
			cfg:         &api.Config{VcsApiKey: "test-token"},
			expectError: false,
		},
		{
			name: "enterprise config",
			cfg: &api.Config{
				VcsApiKey:    "test-token",
				VcsRemoteUrl: "https://github.example.com/api/v3",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, err := NewGitHubService(tt.cfg)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if svc == nil {
				t.Fatal("expected non-nil service")
			}
			if svc.client == nil {
				t.Error("expected client to be initialized")
			}
		})
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

	tests := []struct {
		name      string
		input     *api.InlineComment
		wantNil   bool
		wantPath  string
		wantLine  int
		wantSide  string
		wantStart *int
	}{
		{
			name:    "nil input",
			input:   nil,
			wantNil: true,
		},
		{
			name:    "nil position",
			input:   &api.InlineComment{Body: util.Ptr("test")},
			wantNil: true,
		},
		{
			name: "new line comment",
			input: &api.InlineComment{
				Body:     util.Ptr("test comment"),
				CommitID: util.Ptr("abc123"),
				Position: &api.InlineCommentPosition{
					NewPath: util.Ptr("file.go"),
					NewLine: util.Ptr(int64(42)),
				},
			},
			wantPath: "file.go",
			wantLine: 42,
			wantSide: "RIGHT",
		},
		{
			name: "old line comment",
			input: &api.InlineComment{
				Body: util.Ptr("deleted line comment"),
				Position: &api.InlineCommentPosition{
					OldPath: util.Ptr("old_file.go"),
					OldLine: util.Ptr(int64(10)),
				},
			},
			wantPath: "old_file.go",
			wantLine: 10,
			wantSide: "LEFT",
		},
		{
			name: "prefers new path over old path",
			input: &api.InlineComment{
				Body: util.Ptr("test"),
				Position: &api.InlineCommentPosition{
					NewPath: util.Ptr("new.go"),
					OldPath: util.Ptr("old.go"),
					NewLine: util.Ptr(int64(1)),
				},
			},
			wantPath: "new.go",
			wantLine: 1,
			wantSide: "RIGHT",
		},
		{
			name: "line range with new lines",
			input: &api.InlineComment{
				Body: util.Ptr("range comment"),
				Position: &api.InlineCommentPosition{
					NewPath: util.Ptr("file.go"),
					LineRange: &api.LineRangeOptions{
						Start: &api.LinePositionOptions{NewLine: util.Ptr(int64(15))},
						End:   &api.LinePositionOptions{NewLine: util.Ptr(int64(20))},
					},
				},
			},
			wantPath:  "file.go",
			wantLine:  20,
			wantSide:  "RIGHT",
			wantStart: util.Ptr(15),
		},
		{
			name: "line range with old lines",
			input: &api.InlineComment{
				Body: util.Ptr("range comment"),
				Position: &api.InlineCommentPosition{
					OldPath: util.Ptr("file.go"),
					LineRange: &api.LineRangeOptions{
						Start: &api.LinePositionOptions{OldLine: util.Ptr(int64(15))},
						End:   &api.LinePositionOptions{OldLine: util.Ptr(int64(20))},
					},
				},
			},
			wantPath:  "file.go",
			wantLine:  20,
			wantSide:  "LEFT",
			wantStart: util.Ptr(15),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := svc.convertApiComment(tt.input)

			if tt.wantNil {
				if result != nil {
					t.Error("expected nil for nil input")
				}
				return
			}

			if result == nil {
				t.Fatal("expected non-nil result")
			}
			if *result.Path != tt.wantPath {
				t.Errorf("path = %q, want %q", *result.Path, tt.wantPath)
			}
			if *result.Line != tt.wantLine {
				t.Errorf("line = %d, want %d", *result.Line, tt.wantLine)
			}
			if *result.Side != tt.wantSide {
				t.Errorf("side = %q, want %q", *result.Side, tt.wantSide)
			}
			if tt.wantStart != nil {
				if result.StartLine == nil || *result.StartLine != *tt.wantStart {
					t.Errorf("startLine = %v, want %d", result.StartLine, *tt.wantStart)
				}
			}
		})
	}
}

func TestGitHubService_SendInlineComments(t *testing.T) {
	tests := []struct {
		name        string
		comments    []*api.InlineComment
		prInfo      *api.PullRequestInfo
		setupMock   func(mux *http.ServeMux, requestCount *int)
		expectError bool
		wantCalls   int
	}{
		{
			name: "successful comment",
			comments: []*api.InlineComment{
				{
					Body:     util.Ptr("test comment"),
					CommitID: util.Ptr("abc123"),
					Position: &api.InlineCommentPosition{
						NewPath: util.Ptr("file.go"),
						NewLine: util.Ptr(int64(10)),
					},
				},
			},
			prInfo: &api.PullRequestInfo{Owner: "owner", ProjectName: "repo", PullRequestId: 1},
			setupMock: func(mux *http.ServeMux, requestCount *int) {
				mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
					if strings.Contains(r.URL.Path, "/pulls/") && strings.Contains(r.URL.Path, "/comments") {
						*requestCount++
						w.WriteHeader(http.StatusCreated)
						_ = json.NewEncoder(w).Encode(github.PullRequestComment{ID: github.Ptr(int64(1))})
						return
					}
					w.WriteHeader(http.StatusNotFound)
				})
			},
			wantCalls: 1,
		},
		{
			name: "client error 422 - no retry",
			comments: []*api.InlineComment{
				{
					Body:     util.Ptr("test"),
					CommitID: util.Ptr("abc"),
					Position: &api.InlineCommentPosition{
						NewPath: util.Ptr("file.go"),
						NewLine: util.Ptr(int64(999)),
					},
				},
			},
			prInfo: &api.PullRequestInfo{Owner: "owner", ProjectName: "repo", PullRequestId: 1},
			setupMock: func(mux *http.ServeMux, requestCount *int) {
				mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
					if strings.Contains(r.URL.Path, "/pulls/") && strings.Contains(r.URL.Path, "/comments") {
						*requestCount++
						w.WriteHeader(http.StatusUnprocessableEntity)
						_ = json.NewEncoder(w).Encode(map[string]interface{}{
							"message": "Validation Failed",
							"errors": []map[string]string{
								{"resource": "PullRequestReviewComment", "field": "line", "code": "invalid"},
							},
						})
						return
					}
					w.WriteHeader(http.StatusNotFound)
				})
			},
			expectError: true,
			wantCalls:   1,
		},
		{
			name: "server error 500 - retries",
			comments: []*api.InlineComment{
				{
					Body:     util.Ptr("test"),
					CommitID: util.Ptr("abc"),
					Position: &api.InlineCommentPosition{
						NewPath: util.Ptr("file.go"),
						NewLine: util.Ptr(int64(10)),
					},
				},
			},
			prInfo: &api.PullRequestInfo{Owner: "owner", ProjectName: "repo", PullRequestId: 1},
			setupMock: func(mux *http.ServeMux, requestCount *int) {
				mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
					if strings.Contains(r.URL.Path, "/pulls/") && strings.Contains(r.URL.Path, "/comments") {
						*requestCount++
						if *requestCount < 3 {
							w.WriteHeader(http.StatusInternalServerError)
							return
						}
						w.WriteHeader(http.StatusCreated)
						_ = json.NewEncoder(w).Encode(github.PullRequestComment{ID: github.Ptr(int64(1))})
						return
					}
					w.WriteHeader(http.StatusNotFound)
				})
			},
			wantCalls: 3,
		},
		{
			name: "skips nil and invalid comments",
			comments: []*api.InlineComment{
				nil,
				{Body: util.Ptr("no position")},
				{
					Body:     util.Ptr("valid"),
					CommitID: util.Ptr("abc"),
					Position: &api.InlineCommentPosition{
						NewPath: util.Ptr("file.go"),
						NewLine: util.Ptr(int64(10)),
					},
				},
			},
			prInfo: &api.PullRequestInfo{Owner: "owner", ProjectName: "repo", PullRequestId: 1},
			setupMock: func(mux *http.ServeMux, requestCount *int) {
				mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
					if strings.Contains(r.URL.Path, "/pulls/") && strings.Contains(r.URL.Path, "/comments") {
						*requestCount++
						w.WriteHeader(http.StatusCreated)
						_ = json.NewEncoder(w).Encode(github.PullRequestComment{ID: github.Ptr(int64(1))})
						return
					}
					w.WriteHeader(http.StatusNotFound)
				})
			},
			wantCalls: 1,
		},
		{
			name: "multiple comments with partial failure",
			comments: []*api.InlineComment{
				{Body: util.Ptr("c1"), CommitID: util.Ptr("abc"), Position: &api.InlineCommentPosition{NewPath: util.Ptr("a.go"), NewLine: util.Ptr(int64(1))}},
				{Body: util.Ptr("c2"), CommitID: util.Ptr("abc"), Position: &api.InlineCommentPosition{NewPath: util.Ptr("b.go"), NewLine: util.Ptr(int64(2))}},
				{Body: util.Ptr("c3"), CommitID: util.Ptr("abc"), Position: &api.InlineCommentPosition{NewPath: util.Ptr("c.go"), NewLine: util.Ptr(int64(3))}},
			},
			prInfo: &api.PullRequestInfo{Owner: "owner", ProjectName: "repo", PullRequestId: 1},
			setupMock: func(mux *http.ServeMux, requestCount *int) {
				mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
					if strings.Contains(r.URL.Path, "/pulls/") && strings.Contains(r.URL.Path, "/comments") {
						*requestCount++
						if *requestCount == 2 {
							w.WriteHeader(http.StatusUnprocessableEntity)
							_ = json.NewEncoder(w).Encode(map[string]string{"message": "Validation Failed"})
							return
						}
						w.WriteHeader(http.StatusCreated)
						_ = json.NewEncoder(w).Encode(github.PullRequestComment{ID: github.Ptr(int64(1))})
						return
					}
					w.WriteHeader(http.StatusNotFound)
				})
			},
			expectError: true,
			wantCalls:   3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			server := httptest.NewServer(mux)
			defer server.Close()

			var requestCount int
			tt.setupMock(mux, &requestCount)

			cfg := &api.Config{VcsApiKey: "test-token", VcsRemoteUrl: server.URL}
			svc, _ := NewGitHubService(cfg)

			err := svc.SendInlineComments(tt.comments, tt.prInfo)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}

			if tt.wantCalls > 0 && requestCount < tt.wantCalls {
				t.Errorf("request count = %d, want at least %d", requestCount, tt.wantCalls)
			}
		})
	}
}

func TestGitHubService_SendInlineComments_ErrorMessage(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/pulls/") && strings.Contains(r.URL.Path, "/comments") {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "Validation Failed"})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	cfg := &api.Config{VcsApiKey: "test-token", VcsRemoteUrl: server.URL}
	svc, _ := NewGitHubService(cfg)

	comments := []*api.InlineComment{
		{Body: util.Ptr("c1"), CommitID: util.Ptr("abc"), Position: &api.InlineCommentPosition{NewPath: util.Ptr("a.go"), NewLine: util.Ptr(int64(1))}},
		{Body: util.Ptr("c2"), CommitID: util.Ptr("abc"), Position: &api.InlineCommentPosition{NewPath: util.Ptr("b.go"), NewLine: util.Ptr(int64(2))}},
	}
	prInfo := &api.PullRequestInfo{Owner: "owner", ProjectName: "repo", PullRequestId: 1}

	err := svc.SendInlineComments(comments, prInfo)

	if err == nil {
		t.Fatal("expected error for partial failure")
	}
	if !strings.Contains(err.Error(), "2") {
		t.Errorf("error should mention failed count: %v", err)
	}
}
