package vcs_provider

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/eridan-ltu/gitex/api"
	"github.com/eridan-ltu/gitex/internal/util"
	gitlab "gitlab.com/gitlab-org/api/client-go"
)

func TestNewGitLabService(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *api.Config
		expectError bool
	}{
		{
			name: "valid config",
			cfg: &api.Config{
				VcsApiKey:    "test-token",
				VcsRemoteUrl: "https://gitlab.com",
			},
			expectError: false,
		},
		{
			name: "valid config with custom instance",
			cfg: &api.Config{
				VcsApiKey:    "test-token",
				VcsRemoteUrl: "https://gitlab.example.com",
			},
			expectError: false,
		},
		{
			name: "empty api key",
			cfg: &api.Config{
				VcsApiKey:    "",
				VcsRemoteUrl: "https://gitlab.com",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, err := NewGitLabService(tt.cfg)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				if svc != nil {
					t.Error("expected nil service on error")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if svc == nil {
					t.Error("expected non-nil service")
				}
				if svc != nil && svc.client == nil {
					t.Error("expected non-nil client")
				}
			}
		})
	}
}

func TestGitLabService_parseWebUrl(t *testing.T) {
	svc := &GitLabService{}

	tests := []struct {
		name            string
		url             string
		expectedProject string
		expectedMRId    int
		expectError     bool
	}{
		{
			name:            "valid gitlab.com MR URL",
			url:             "https://gitlab.com/user/project/-/merge_requests/123",
			expectedProject: "user/project",
			expectedMRId:    123,
		},
		{
			name:            "valid nested project path",
			url:             "https://gitlab.com/group/subgroup/project/-/merge_requests/456",
			expectedProject: "group/subgroup/project",
			expectedMRId:    456,
		},
		{
			name:            "valid self-hosted gitlab",
			url:             "https://gitlab.example.com/team/repo/-/merge_requests/789",
			expectedProject: "team/repo",
			expectedMRId:    789,
		},
		{
			name:            "deeply nested project",
			url:             "https://gitlab.com/org/team/subteam/project/-/merge_requests/1",
			expectedProject: "org/team/subteam/project",
			expectedMRId:    1,
		},
		{
			name:            "MR URL with diffs tab",
			url:             "https://gitlab.com/user/project/-/merge_requests/123/diffs",
			expectedProject: "user/project",
			expectedMRId:    123,
		},
		{
			name:            "MR URL with commits tab",
			url:             "https://gitlab.com/user/project/-/merge_requests/123/commits",
			expectedProject: "user/project",
			expectedMRId:    123,
		},
		{
			name:            "MR URL with pipelines tab",
			url:             "https://gitlab.com/user/project/-/merge_requests/123/pipelines",
			expectedProject: "user/project",
			expectedMRId:    123,
		},
		{
			name:        "invalid URL format - missing merge_requests",
			url:         "https://gitlab.com/user/project/-/issues/123",
			expectError: true,
		},
		{
			name:        "invalid MR ID - not a number",
			url:         "https://gitlab.com/user/project/-/merge_requests/abc",
			expectError: true,
		},
		{
			name:        "malformed URL",
			url:         "://invalid-url",
			expectError: true,
		},
		{
			name:        "empty URL",
			url:         "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectPath, mrId, err := svc.parseWebUrl(tt.url)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if projectPath != tt.expectedProject {
				t.Errorf("expected project path %q, got %q", tt.expectedProject, projectPath)
			}
			if mrId != tt.expectedMRId {
				t.Errorf("expected MR ID %d, got %d", tt.expectedMRId, mrId)
			}
		})
	}
}

func TestConvertApiComment(t *testing.T) {
	tests := []struct {
		name       string
		input      *api.InlineComment
		wantNil    bool
		wantBody   string
		wantCommit string
	}{
		{
			name:    "nil comment",
			input:   nil,
			wantNil: true,
		},
		{
			name: "comment with all fields",
			input: &api.InlineComment{
				Body:      util.Ptr("Test comment"),
				CommitID:  util.Ptr("abc123"),
				CreatedAt: &time.Time{},
				Position: &api.InlineCommentPosition{
					BaseSha:      util.Ptr("base123"),
					HeadSha:      util.Ptr("head123"),
					StartSha:     util.Ptr("start123"),
					NewPath:      util.Ptr("file.go"),
					OldPath:      util.Ptr("file.go"),
					PositionType: util.Ptr("text"),
					NewLine:      util.Ptr(int64(10)),
					OldLine:      util.Ptr(int64(5)),
				},
			},
			wantBody:   "Test comment",
			wantCommit: "abc123",
		},
		{
			name: "comment with nil position",
			input: &api.InlineComment{
				Body:     util.Ptr("Simple comment"),
				Position: nil,
			},
			wantBody: "Simple comment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertApiComment(tt.input)

			if tt.wantNil {
				if result != nil {
					t.Error("expected nil result")
				}
				return
			}

			if result == nil {
				t.Fatal("expected non-nil result")
			}
			if result.Body != nil && *result.Body != tt.wantBody {
				t.Errorf("body = %q, want %q", *result.Body, tt.wantBody)
			}
		})
	}
}

func TestConvertInlineCommentPosition(t *testing.T) {
	tests := []struct {
		name    string
		input   *api.InlineCommentPosition
		wantNil bool
	}{
		{
			name:    "nil position",
			input:   nil,
			wantNil: true,
		},
		{
			name: "complete position",
			input: &api.InlineCommentPosition{
				BaseSha:      util.Ptr("base"),
				HeadSha:      util.Ptr("head"),
				StartSha:     util.Ptr("start"),
				NewPath:      util.Ptr("new.go"),
				OldPath:      util.Ptr("old.go"),
				PositionType: util.Ptr("text"),
				NewLine:      util.Ptr(int64(20)),
				OldLine:      util.Ptr(int64(15)),
			},
		},
		{
			name: "position with line range",
			input: &api.InlineCommentPosition{
				BaseSha:      util.Ptr("base"),
				HeadSha:      util.Ptr("head"),
				StartSha:     util.Ptr("start"),
				NewPath:      util.Ptr("file.go"),
				PositionType: util.Ptr("text"),
				LineRange: &api.LineRangeOptions{
					Start: &api.LinePositionOptions{
						LineCode: util.Ptr("code1"),
						Type:     util.Ptr("new"),
						NewLine:  util.Ptr(int64(10)),
					},
					End: &api.LinePositionOptions{
						LineCode: util.Ptr("code2"),
						Type:     util.Ptr("new"),
						NewLine:  util.Ptr(int64(15)),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertInlineCommentPosition(tt.input)

			if tt.wantNil {
				if result != nil {
					t.Error("expected nil result")
				}
				return
			}

			if result == nil {
				t.Error("expected non-nil result")
			}
		})
	}
}

func TestConvertInlineLineRange(t *testing.T) {
	tests := []struct {
		name    string
		input   *api.LineRangeOptions
		wantNil bool
	}{
		{
			name:    "nil line range",
			input:   nil,
			wantNil: true,
		},
		{
			name: "complete line range",
			input: &api.LineRangeOptions{
				Start: &api.LinePositionOptions{
					LineCode: util.Ptr("start_code"),
					Type:     util.Ptr("new"),
					NewLine:  util.Ptr(int64(5)),
				},
				End: &api.LinePositionOptions{
					LineCode: util.Ptr("end_code"),
					Type:     util.Ptr("new"),
					NewLine:  util.Ptr(int64(10)),
				},
			},
		},
		{
			name: "line range with nil start",
			input: &api.LineRangeOptions{
				Start: nil,
				End: &api.LinePositionOptions{
					LineCode: util.Ptr("end_code"),
					Type:     util.Ptr("new"),
					NewLine:  util.Ptr(int64(10)),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertInlineLineRange(tt.input)

			if tt.wantNil {
				if result != nil {
					t.Error("expected nil result")
				}
				return
			}

			if result == nil {
				t.Error("expected non-nil result")
			}
		})
	}
}

func TestConvertInlineLinePosition(t *testing.T) {
	tests := []struct {
		name    string
		input   *api.LinePositionOptions
		wantNil bool
	}{
		{
			name:    "nil position",
			input:   nil,
			wantNil: true,
		},
		{
			name: "complete position",
			input: &api.LinePositionOptions{
				LineCode: util.Ptr("code123"),
				Type:     util.Ptr("new"),
				OldLine:  util.Ptr(int64(5)),
				NewLine:  util.Ptr(int64(10)),
			},
		},
		{
			name: "position with only new line",
			input: &api.LinePositionOptions{
				LineCode: util.Ptr("code456"),
				Type:     util.Ptr("new"),
				NewLine:  util.Ptr(int64(15)),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertInlineLinePosition(tt.input)

			if tt.wantNil {
				if result != nil {
					t.Error("expected nil result")
				}
				return
			}

			if result == nil {
				t.Error("expected non-nil result")
			}
		})
	}
}

func TestGitLabService_GetPullRequestInfo(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		setupMock   func(mux *http.ServeMux)
		expectError bool
		validate    func(*testing.T, *api.PullRequestInfo)
	}{
		{
			name: "successful fetch",
			url:  "https://gitlab.com/test/project/-/merge_requests/1",
			setupMock: func(mux *http.ServeMux) {
				mux.HandleFunc("/api/v4/projects/test%2Fproject/merge_requests/1", func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					_, _ = fmt.Fprint(w, `{
						"id": 1,
						"iid": 1,
						"project_id": 123,
						"source_branch": "feature",
						"target_branch": "main",
						"diff_refs": {
							"base_sha": "base123",
							"head_sha": "head123",
							"start_sha": "start123"
						}
					}`)
				})
				mux.HandleFunc("/api/v4/projects/123", func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					_, _ = fmt.Fprint(w, `{
						"id": 123,
						"name": "project",
						"path_with_namespace": "test/project",
						"http_url_to_repo": "https://gitlab.com/test/project.git"
					}`)
				})
			},
			validate: func(t *testing.T, info *api.PullRequestInfo) {
				if info.HeadSha != "head123" {
					t.Errorf("HeadSha = %q, want %q", info.HeadSha, "head123")
				}
				if info.BaseSha != "base123" {
					t.Errorf("BaseSha = %q, want %q", info.BaseSha, "base123")
				}
				if info.StartSha != "start123" {
					t.Errorf("StartSha = %q, want %q", info.StartSha, "start123")
				}
				if info.ProjectName != "project" {
					t.Errorf("ProjectName = %q, want %q", info.ProjectName, "project")
				}
				if info.SourceBranch != "feature" {
					t.Errorf("SourceBranch = %q, want %q", info.SourceBranch, "feature")
				}
				if info.TargetBranch != "main" {
					t.Errorf("TargetBranch = %q, want %q", info.TargetBranch, "main")
				}
				if info.ProjectId != 123 {
					t.Errorf("ProjectId = %d, want %d", info.ProjectId, 123)
				}
				if info.PullRequestId != 1 {
					t.Errorf("PullRequestId = %d, want %d", info.PullRequestId, 1)
				}
			},
		},
		{
			name: "merge request not found",
			url:  "https://gitlab.com/test/project/-/merge_requests/999",
			setupMock: func(mux *http.ServeMux) {
				mux.HandleFunc("/api/v4/projects/test%2Fproject/merge_requests/999", func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusNotFound)
					_, _ = fmt.Fprint(w, `{"message": "404 Not Found"}`)
				})
			},
			expectError: true,
		},
		{
			name:        "invalid URL",
			url:         "invalid-url",
			setupMock:   func(mux *http.ServeMux) {},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux, server, client := setupMockServer(t)
			defer server.Close()

			tt.setupMock(mux)

			svc := &GitLabService{client: client}
			info, err := svc.GetPullRequestInfo(&tt.url)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if info == nil {
				t.Fatal("expected non-nil info")
			}
			if tt.validate != nil {
				tt.validate(t, info)
			}
		})
	}
}

func TestGitLabService_SendInlineComments(t *testing.T) {
	tests := []struct {
		name        string
		comments    []*api.InlineComment
		prInfo      *api.PullRequestInfo
		setupMock   func(mux *http.ServeMux, callCount *int)
		expectError bool
		wantCalls   int
	}{
		{
			name: "send single comment successfully",
			comments: []*api.InlineComment{
				{
					Body:     util.Ptr("Test comment"),
					CommitID: util.Ptr("abc123"),
					Position: &api.InlineCommentPosition{
						BaseSha:      util.Ptr("base"),
						HeadSha:      util.Ptr("head"),
						StartSha:     util.Ptr("start"),
						NewPath:      util.Ptr("file.go"),
						PositionType: util.Ptr("text"),
						NewLine:      util.Ptr(int64(10)),
					},
				},
			},
			prInfo: &api.PullRequestInfo{
				ProjectPath:   "test/project",
				PullRequestId: 1,
			},
			setupMock: func(mux *http.ServeMux, callCount *int) {
				mux.HandleFunc("/api/v4/projects/test%2Fproject/merge_requests/1/discussions", func(w http.ResponseWriter, r *http.Request) {
					*callCount++
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusCreated)
					_, _ = fmt.Fprint(w, `{"id": "disc1", "notes": []}`)
				})
			},
			wantCalls: 1,
		},
		{
			name: "send multiple comments",
			comments: []*api.InlineComment{
				{Body: util.Ptr("Comment 1")},
				{Body: util.Ptr("Comment 2")},
				{Body: util.Ptr("Comment 3")},
			},
			prInfo: &api.PullRequestInfo{
				ProjectPath:   "test/multi",
				PullRequestId: 2,
			},
			setupMock: func(mux *http.ServeMux, callCount *int) {
				mux.HandleFunc("/api/v4/projects/test%2Fmulti/merge_requests/2/discussions", func(w http.ResponseWriter, r *http.Request) {
					*callCount++
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusCreated)
					_, _ = fmt.Fprintf(w, `{"id": "disc%d", "notes": []}`, *callCount)
				})
			},
			wantCalls: 3,
		},
		{
			name:     "empty comments list",
			comments: []*api.InlineComment{},
			prInfo: &api.PullRequestInfo{
				ProjectPath:   "test/empty",
				PullRequestId: 3,
			},
			setupMock: func(mux *http.ServeMux, callCount *int) {},
			wantCalls: 0,
		},
		{
			name: "API error returns error",
			comments: []*api.InlineComment{
				{Body: util.Ptr("Comment")},
			},
			prInfo: &api.PullRequestInfo{
				ProjectPath:   "test/error",
				PullRequestId: 4,
			},
			setupMock: func(mux *http.ServeMux, callCount *int) {
				mux.HandleFunc("/api/v4/projects/test%2Ferror/merge_requests/4/discussions", func(w http.ResponseWriter, r *http.Request) {
					*callCount++
					// Use 422 to avoid retry behavior (500 triggers retries)
					w.WriteHeader(http.StatusUnprocessableEntity)
					_, _ = fmt.Fprint(w, `{"error": "validation error"}`)
				})
			},
			expectError: true,
			wantCalls:   1,
		},
		{
			name: "skips nil comments",
			comments: []*api.InlineComment{
				nil,
				{Body: util.Ptr("Valid comment")},
			},
			prInfo: &api.PullRequestInfo{
				ProjectPath:   "test/nil",
				PullRequestId: 5,
			},
			setupMock: func(mux *http.ServeMux, callCount *int) {
				mux.HandleFunc("/api/v4/projects/test%2Fnil/merge_requests/5/discussions", func(w http.ResponseWriter, r *http.Request) {
					*callCount++
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusCreated)
					_, _ = fmt.Fprint(w, `{"id": "disc1", "notes": []}`)
				})
			},
			wantCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux, server, client := setupMockServer(t)
			defer server.Close()

			var callCount int
			tt.setupMock(mux, &callCount)

			svc := &GitLabService{client: client}
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

			if callCount != tt.wantCalls {
				t.Errorf("API calls = %d, want %d", callCount, tt.wantCalls)
			}
		})
	}
}

func setupMockServer(t *testing.T) (*http.ServeMux, *httptest.Server, *gitlab.Client) {
	t.Helper()
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)

	client, err := gitlab.NewClient("test-token", gitlab.WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	return mux, server, client
}
