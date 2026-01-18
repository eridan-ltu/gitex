package vcs_provider

import (
	"github.com/eridan-ltu/gitex/api"
	"github.com/eridan-ltu/gitex/internal/util"
	"testing"
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
