package internal

import (
	"github.com/eridan-ltu/gitex/api"
	"testing"
)

func TestNewServiceFactory(t *testing.T) {
	cfg := &api.Config{
		VcsApiKey: "test-key",
	}

	factory := NewServiceFactory(cfg)

	if factory == nil {
		t.Fatal("expected non-nil factory")
	}

	if factory.cfg != cfg {
		t.Error("factory config not set correctly")
	}
}

func TestCreateAiAgentService(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &api.Config{
		BinDir:  tmpDir,
		HomeDir: tmpDir,
	}
	factory := NewServiceFactory(cfg)

	tests := []struct {
		name        string
		kind        AIAgentType
		expectError bool
	}{
		{
			name:        "valid codex type",
			kind:        AIAgentTypeCodex,
			expectError: false,
		},
		{
			name:        "invalid type",
			kind:        AIAgentType("invalid"),
			expectError: true,
		},
		{
			name:        "empty type",
			kind:        AIAgentType(""),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, err := factory.CreateAiAgentService(tt.kind)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				if svc != nil {
					t.Error("expected nil service on error")
				}
			} else {
				if err != nil {
					t.Skipf("skipping due to npm requirement: %v", err)
				}
				if svc == nil {
					t.Error("expected non-nil service")
				}
			}
		})
	}
}

func TestCreateVersionControlService(t *testing.T) {
	cfg := &api.Config{
		VcsApiKey: "test-api-key",
	}
	factory := NewServiceFactory(cfg)

	tests := []struct {
		name        string
		kind        VersionControlType
		expectError bool
	}{
		{
			name:        "valid git type",
			kind:        VersionControlTypeGit,
			expectError: false,
		},
		{
			name:        "invalid type",
			kind:        VersionControlType("svn"),
			expectError: true,
		},
		{
			name:        "empty type",
			kind:        VersionControlType(""),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, err := factory.CreateVersionControlService(tt.kind)

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
			}
		})
	}
}

func TestCreateRemoteGitService(t *testing.T) {
	cfg := &api.Config{}
	factory := NewServiceFactory(cfg)

	tests := []struct {
		name        string
		kind        RemoteGitServiceType
		expectError bool
	}{
		{
			name:        "valid gitlab type",
			kind:        RemoteGitServiceTypeGitLab,
			expectError: false,
		},
		{
			name:        "github type - not yet implemented",
			kind:        RemoteGitServiceTypeGitHub,
			expectError: true,
		},
		{
			name:        "unknown type",
			kind:        RemoteGitServiceTypeUnknown,
			expectError: true,
		},
		{
			name:        "empty type",
			kind:        RemoteGitServiceType(""),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, err := factory.CreateRemoteGitService(tt.kind)

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
			}
		})
	}
}

func TestDetectRemoteGitServiceType(t *testing.T) {
	factory := NewServiceFactory(&api.Config{})

	tests := []struct {
		name        string
		url         string
		expected    RemoteGitServiceType
		expectError bool
	}{
		{
			name:        "github.com host",
			url:         "https://github.com/user/repo",
			expected:    RemoteGitServiceTypeGitHub,
			expectError: false,
		},
		{
			name:        "github.com with pull request",
			url:         "https://github.com/user/repo/pull/123",
			expected:    RemoteGitServiceTypeGitHub,
			expectError: false,
		},
		{
			name:        "gitlab merge request",
			url:         "https://gitlab.com/user/repo/-/merge_requests/123",
			expected:    RemoteGitServiceTypeGitLab,
			expectError: false,
		},
		{
			name:        "self-hosted gitlab",
			url:         "https://gitlab.example.com/user/repo/-/merge_requests/456",
			expected:    RemoteGitServiceTypeGitLab,
			expectError: false,
		},
		{
			name:        "case insensitive github",
			url:         "https://GitHub.COM/user/repo",
			expected:    RemoteGitServiceTypeGitHub,
			expectError: false,
		},
		{
			name:        "case insensitive merge request path",
			url:         "https://example.com/user/repo/-/MERGE_REQUESTS/123",
			expected:    RemoteGitServiceTypeGitLab,
			expectError: false,
		},
		{
			name:        "non-github pull request",
			url:         "https://example.com/user/repo/pull/123",
			expected:    RemoteGitServiceTypeGitHub,
			expectError: false,
		},
		{
			name:        "invalid url format",
			url:         "://invalid-url",
			expected:    "",
			expectError: true,
		},
		{
			name:        "malformed url",
			url:         "ht!tp://bad-url",
			expected:    "",
			expectError: true,
		},
		{
			name:        "unsupported service - still has log.Fatalf",
			url:         "https://bitbucket.org/user/repo",
			expected:    RemoteGitServiceTypeUnknown,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			result, err := factory.DetectRemoteGitServiceType(tt.url)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				if result != tt.expected {
					t.Errorf("expected result %s, got %s", tt.expected, result)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("expected %s, got %s", tt.expected, result)
				}
			}
		})
	}
}
