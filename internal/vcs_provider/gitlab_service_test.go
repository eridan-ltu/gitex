package vcs_provider

import (
	"fmt"
	"github.com/eridan-ltu/gitex/api"
	gitlab "gitlab.com/gitlab-org/api/client-go"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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
			expectError: false, // GitLab client may allow empty token
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

func TestParseWebUrl(t *testing.T) {
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
			expectError:     false,
		},
		{
			name:            "valid nested project path",
			url:             "https://gitlab.com/group/subgroup/project/-/merge_requests/456",
			expectedProject: "group/subgroup/project",
			expectedMRId:    456,
			expectError:     false,
		},
		{
			name:            "valid self-hosted gitlab",
			url:             "https://gitlab.example.com/team/repo/-/merge_requests/789",
			expectedProject: "team/repo",
			expectedMRId:    789,
			expectError:     false,
		},
		{
			name:            "deeply nested project",
			url:             "https://gitlab.com/org/team/subteam/project/-/merge_requests/1",
			expectedProject: "org/team/subteam/project",
			expectedMRId:    1,
			expectError:     false,
		},
		{
			name:        "invalid URL format - too short",
			url:         "https://gitlab.com/user/project",
			expectError: true,
		},
		{
			name:        "invalid URL format - missing parts",
			url:         "https://gitlab.com/-/merge_requests/123",
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
		{
			name:            "URL without merge_requests segment",
			url:             "https://gitlab.com/user/project/-/issues/123",
			expectedProject: "user/project",
			expectedMRId:    123,
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectPath, mrId, err := parseWebUrl(tt.url)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if projectPath != tt.expectedProject {
					t.Errorf("expected project path %s, got %s", tt.expectedProject, projectPath)
				}
				if mrId != tt.expectedMRId {
					t.Errorf("expected MR ID %d, got %d", tt.expectedMRId, mrId)
				}
			}
		})
	}
}

func TestConvertApiComment(t *testing.T) {
	tests := []struct {
		name     string
		input    *api.InlineComment
		expected *gitlab.CreateMergeRequestDiscussionOptions
	}{
		{
			name:     "nil comment",
			input:    nil,
			expected: nil,
		},
		{
			name: "comment with all fields",
			input: &api.InlineComment{
				Body:      gitlab.Ptr("Test comment"),
				CommitID:  gitlab.Ptr("abc123"),
				CreatedAt: &time.Time{},
				Position: &api.InlineCommentPosition{
					BaseSha:      gitlab.Ptr("base123"),
					HeadSha:      gitlab.Ptr("head123"),
					StartSha:     gitlab.Ptr("start123"),
					NewPath:      gitlab.Ptr("file.go"),
					OldPath:      gitlab.Ptr("file.go"),
					PositionType: gitlab.Ptr("text"),
					NewLine:      gitlab.Ptr(int64(10)),
					OldLine:      gitlab.Ptr(int64(5)),
				},
			},
			expected: &gitlab.CreateMergeRequestDiscussionOptions{
				Body:     gitlab.Ptr("Test comment"),
				CommitID: gitlab.Ptr("abc123"),
				Position: &gitlab.PositionOptions{
					BaseSHA:      gitlab.Ptr("base123"),
					HeadSHA:      gitlab.Ptr("head123"),
					StartSHA:     gitlab.Ptr("start123"),
					NewPath:      gitlab.Ptr("file.go"),
					OldPath:      gitlab.Ptr("file.go"),
					PositionType: gitlab.Ptr("text"),
					NewLine:      gitlab.Ptr(int64(10)),
					OldLine:      gitlab.Ptr(int64(5)),
				},
			},
		},
		{
			name: "comment with nil position",
			input: &api.InlineComment{
				Body:     gitlab.Ptr("Simple comment"),
				Position: nil,
			},
			expected: &gitlab.CreateMergeRequestDiscussionOptions{
				Body:     gitlab.Ptr("Simple comment"),
				Position: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertApiComment(tt.input)

			if tt.expected == nil && result != nil {
				t.Error("expected nil result")
			}
			if tt.expected != nil && result == nil {
				t.Error("expected non-nil result")
			}
			if tt.expected != nil && result != nil {
				if tt.expected.Body != nil && result.Body != nil {
					if *tt.expected.Body != *result.Body {
						t.Errorf("expected body %s, got %s", *tt.expected.Body, *result.Body)
					}
				}
			}
		})
	}
}

func TestConvertInlineCommentPosition(t *testing.T) {
	tests := []struct {
		name     string
		input    *api.InlineCommentPosition
		expected *gitlab.PositionOptions
	}{
		{
			name:     "nil position",
			input:    nil,
			expected: nil,
		},
		{
			name: "complete position",
			input: &api.InlineCommentPosition{
				BaseSha:      gitlab.Ptr("base"),
				HeadSha:      gitlab.Ptr("head"),
				StartSha:     gitlab.Ptr("start"),
				NewPath:      gitlab.Ptr("new.go"),
				OldPath:      gitlab.Ptr("old.go"),
				PositionType: gitlab.Ptr("text"),
				NewLine:      gitlab.Ptr(int64(20)),
				OldLine:      gitlab.Ptr(int64(15)),
			},
			expected: &gitlab.PositionOptions{
				BaseSHA:      gitlab.Ptr("base"),
				HeadSHA:      gitlab.Ptr("head"),
				StartSHA:     gitlab.Ptr("start"),
				NewPath:      gitlab.Ptr("new.go"),
				OldPath:      gitlab.Ptr("old.go"),
				PositionType: gitlab.Ptr("text"),
				NewLine:      gitlab.Ptr(int64(20)),
				OldLine:      gitlab.Ptr(int64(15)),
			},
		},
		{
			name: "position with line range",
			input: &api.InlineCommentPosition{
				BaseSha:      gitlab.Ptr("base"),
				HeadSha:      gitlab.Ptr("head"),
				StartSha:     gitlab.Ptr("start"),
				NewPath:      gitlab.Ptr("file.go"),
				PositionType: gitlab.Ptr("text"),
				LineRange: &api.LineRangeOptions{
					Start: &api.LinePositionOptions{
						LineCode: gitlab.Ptr("code1"),
						Type:     gitlab.Ptr("new"),
						NewLine:  gitlab.Ptr(int64(10)),
					},
					End: &api.LinePositionOptions{
						LineCode: gitlab.Ptr("code2"),
						Type:     gitlab.Ptr("new"),
						NewLine:  gitlab.Ptr(int64(15)),
					},
				},
			},
			expected: &gitlab.PositionOptions{
				BaseSHA:      gitlab.Ptr("base"),
				HeadSHA:      gitlab.Ptr("head"),
				StartSHA:     gitlab.Ptr("start"),
				NewPath:      gitlab.Ptr("file.go"),
				PositionType: gitlab.Ptr("text"),
				LineRange: &gitlab.LineRangeOptions{
					Start: &gitlab.LinePositionOptions{
						LineCode: gitlab.Ptr("code1"),
						Type:     gitlab.Ptr("new"),
						NewLine:  gitlab.Ptr(int64(10)),
					},
					End: &gitlab.LinePositionOptions{
						LineCode: gitlab.Ptr("code2"),
						Type:     gitlab.Ptr("new"),
						NewLine:  gitlab.Ptr(int64(15)),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertInlineCommentPosition(tt.input)

			if tt.expected == nil && result != nil {
				t.Error("expected nil result")
			}
			if tt.expected != nil && result == nil {
				t.Error("expected non-nil result")
			}
		})
	}
}

func TestConvertInlineLineRange(t *testing.T) {
	tests := []struct {
		name     string
		input    *api.LineRangeOptions
		expected *gitlab.LineRangeOptions
	}{
		{
			name:     "nil line range",
			input:    nil,
			expected: nil,
		},
		{
			name: "complete line range",
			input: &api.LineRangeOptions{
				Start: &api.LinePositionOptions{
					LineCode: gitlab.Ptr("start_code"),
					Type:     gitlab.Ptr("new"),
					NewLine:  gitlab.Ptr(int64(5)),
				},
				End: &api.LinePositionOptions{
					LineCode: gitlab.Ptr("end_code"),
					Type:     gitlab.Ptr("new"),
					NewLine:  gitlab.Ptr(int64(10)),
				},
			},
			expected: &gitlab.LineRangeOptions{
				Start: &gitlab.LinePositionOptions{
					LineCode: gitlab.Ptr("start_code"),
					Type:     gitlab.Ptr("new"),
					NewLine:  gitlab.Ptr(int64(5)),
				},
				End: &gitlab.LinePositionOptions{
					LineCode: gitlab.Ptr("end_code"),
					Type:     gitlab.Ptr("new"),
					NewLine:  gitlab.Ptr(int64(10)),
				},
			},
		},
		{
			name: "line range with nil start",
			input: &api.LineRangeOptions{
				Start: nil,
				End: &api.LinePositionOptions{
					LineCode: gitlab.Ptr("end_code"),
					Type:     gitlab.Ptr("new"),
					NewLine:  gitlab.Ptr(int64(10)),
				},
			},
			expected: &gitlab.LineRangeOptions{
				Start: nil,
				End: &gitlab.LinePositionOptions{
					LineCode: gitlab.Ptr("end_code"),
					Type:     gitlab.Ptr("new"),
					NewLine:  gitlab.Ptr(int64(10)),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertInlineLineRange(tt.input)

			if tt.expected == nil && result != nil {
				t.Error("expected nil result")
			}
			if tt.expected != nil && result == nil {
				t.Error("expected non-nil result")
			}
		})
	}
}

func TestConvertInlineLinePosition(t *testing.T) {
	tests := []struct {
		name     string
		input    *api.LinePositionOptions
		expected *gitlab.LinePositionOptions
	}{
		{
			name:     "nil position",
			input:    nil,
			expected: nil,
		},
		{
			name: "complete position",
			input: &api.LinePositionOptions{
				LineCode: gitlab.Ptr("code123"),
				Type:     gitlab.Ptr("new"),
				OldLine:  gitlab.Ptr(int64(5)),
				NewLine:  gitlab.Ptr(int64(10)),
			},
			expected: &gitlab.LinePositionOptions{
				LineCode: gitlab.Ptr("code123"),
				Type:     gitlab.Ptr("new"),
				OldLine:  gitlab.Ptr(int64(5)),
				NewLine:  gitlab.Ptr(int64(10)),
			},
		},
		{
			name: "position with only new line",
			input: &api.LinePositionOptions{
				LineCode: gitlab.Ptr("code456"),
				Type:     gitlab.Ptr("new"),
				NewLine:  gitlab.Ptr(int64(15)),
			},
			expected: &gitlab.LinePositionOptions{
				LineCode: gitlab.Ptr("code456"),
				Type:     gitlab.Ptr("new"),
				NewLine:  gitlab.Ptr(int64(15)),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertInlineLinePosition(tt.input)

			if tt.expected == nil && result != nil {
				t.Error("expected nil result")
			}
			if tt.expected != nil && result == nil {
				t.Error("expected non-nil result")
			}
		})
	}
}

func TestGetPullRequestInfo(t *testing.T) {
	// Create a mock server
	mux, server, client := setupMockServer(t)
	defer server.Close()

	tests := []struct {
		name        string
		url         string
		setupMock   func()
		expectError bool
		validate    func(*testing.T, *api.PullRequestInfo)
	}{
		{
			name: "successful fetch",
			url:  "https://gitlab.com/test/project/-/merge_requests/1",
			setupMock: func() {
				// Mock GetMergeRequest
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
				// Mock GetProject
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
			expectError: false,
			validate: func(t *testing.T, info *api.PullRequestInfo) {
				if info.HeadSha != "head123" {
					t.Errorf("expected HeadSha 'head123', got %s", info.HeadSha)
				}
				if info.BaseSha != "base123" {
					t.Errorf("expected BaseSha 'base123', got %s", info.BaseSha)
				}
				if info.StartSha != "start123" {
					t.Errorf("expected StartSha 'start123', got %s", info.StartSha)
				}
				if info.ProjectName != "project" {
					t.Errorf("expected ProjectName 'project', got %s", info.ProjectName)
				}
				if info.SourceBranch != "feature" {
					t.Errorf("expected SourceBranch 'feature', got %s", info.SourceBranch)
				}
				if info.TargetBranch != "main" {
					t.Errorf("expected TargetBranch 'main', got %s", info.TargetBranch)
				}
				if info.ProjectId != 123 {
					t.Errorf("expected ProjectId 123, got %d", info.ProjectId)
				}
				if info.PullRequestId != 1 {
					t.Errorf("expected PullRequestId 1, got %d", info.PullRequestId)
				}
			},
		},
		{
			name: "merge request not found",
			url:  "https://gitlab.com/test/project/-/merge_requests/999",
			setupMock: func() {
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
			setupMock:   func() {},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()

			svc := &GitLabService{client: client}
			info, err := svc.GetPullRequestInfo(&tt.url)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if info == nil {
					t.Fatal("expected non-nil info")
				}
				if tt.validate != nil {
					tt.validate(t, info)
				}
			}
		})
	}
}

func TestSendInlineComments(t *testing.T) {
	mux, server, client := setupMockServer(t)
	defer server.Close()

	tests := []struct {
		name      string
		comments  []*api.InlineComment
		prInfo    *api.PullRequestInfo
		setupMock func()
		validate  func(*testing.T, int)
	}{
		{
			name: "send single comment successfully",
			comments: []*api.InlineComment{
				{
					Body:     gitlab.Ptr("Test comment"),
					CommitID: gitlab.Ptr("abc123"),
					Position: &api.InlineCommentPosition{
						BaseSha:      gitlab.Ptr("base"),
						HeadSha:      gitlab.Ptr("head"),
						StartSha:     gitlab.Ptr("start"),
						NewPath:      gitlab.Ptr("file.go"),
						PositionType: gitlab.Ptr("text"),
						NewLine:      gitlab.Ptr(int64(10)),
					},
				},
			},
			prInfo: &api.PullRequestInfo{
				ProjectPath:   "test/project",
				PullRequestId: 1,
			},
			setupMock: func() {
				callCount := 0
				mux.HandleFunc("/api/v4/projects/test%2Fproject/merge_requests/1/discussions", func(w http.ResponseWriter, r *http.Request) {
					callCount++
					if r.Method != http.MethodPost {
						t.Errorf("expected POST request, got %s", r.Method)
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusCreated)
					_, _ = fmt.Fprint(w, `{"id": "disc1", "notes": []}`)

				})
			},
		},
		{
			name: "send multiple comments",
			comments: []*api.InlineComment{
				{
					Body: gitlab.Ptr("Comment 1"),
				},
				{
					Body: gitlab.Ptr("Comment 2"),
				},
				{
					Body: gitlab.Ptr("Comment 3"),
				},
			},
			prInfo: &api.PullRequestInfo{
				ProjectPath:   "test/project",
				PullRequestId: 2,
			},
			setupMock: func() {
				callCount := 0
				mux.HandleFunc("/api/v4/projects/test%2Fproject/merge_requests/2/discussions", func(w http.ResponseWriter, r *http.Request) {
					callCount++
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusCreated)
					_, _ = fmt.Fprintf(w, `{"id": "disc%d", "notes": []}`, callCount)
				})
			},
		},
		{
			name:     "empty comments list",
			comments: []*api.InlineComment{},
			prInfo: &api.PullRequestInfo{
				ProjectPath:   "test/project",
				PullRequestId: 3,
			},
			setupMock: func() {

			},
		},
		{
			name: "API error - should log but not fail",
			comments: []*api.InlineComment{
				{
					Body: gitlab.Ptr("Comment"),
				},
			},
			prInfo: &api.PullRequestInfo{
				ProjectPath:   "test/project",
				PullRequestId: 4,
			},
			setupMock: func() {
				mux.HandleFunc("/api/v4/projects/test%2Fproject/merge_requests/4/discussions", func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = fmt.Fprint(w, `{"error": "internal error"}`)
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux = http.NewServeMux()
			tt.setupMock()

			svc := &GitLabService{client: client}

			svc.SendInlineComments(tt.comments, tt.prInfo)

		})
	}
}

func setupMockServer(t *testing.T) (*http.ServeMux, *httptest.Server, *gitlab.Client) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)

	client, err := gitlab.NewClient("test-token", gitlab.WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	return mux, server, client
}
