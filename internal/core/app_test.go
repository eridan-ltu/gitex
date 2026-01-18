package core

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/eridan-ltu/gitex/api"
)

type MockServiceFactory struct {
	DetectVCSProviderTypeFunc       func(url string) (api.VCSProviderType, error)
	CreateVCSProviderFunc           func(kind api.VCSProviderType) (api.RemoteGitService, error)
	CreateVersionControlServiceFunc func(kind api.VersionControlType) (api.VersionControlService, error)
	CreateAiAgentServiceFunc        func(kind api.AIAgentType) (api.AIAgentService, error)
}

func (m *MockServiceFactory) DetectVCSProviderType(url string) (api.VCSProviderType, error) {
	return m.DetectVCSProviderTypeFunc(url)
}

func (m *MockServiceFactory) CreateVCSProvider(kind api.VCSProviderType) (api.RemoteGitService, error) {
	return m.CreateVCSProviderFunc(kind)
}

func (m *MockServiceFactory) CreateVersionControlService(kind api.VersionControlType) (api.VersionControlService, error) {
	return m.CreateVersionControlServiceFunc(kind)
}

func (m *MockServiceFactory) CreateAiAgentService(kind api.AIAgentType) (api.AIAgentService, error) {
	return m.CreateAiAgentServiceFunc(kind)
}

// MockRemoteGitService implements api.RemoteGitService for testing
type MockRemoteGitService struct {
	GetPullRequestInfoFunc func(pullRequestURL *string) (*api.PullRequestInfo, error)
	SendInlineCommentsFunc func(comments []*api.InlineComment, pullRequestInfo *api.PullRequestInfo) error
}

func (m *MockRemoteGitService) GetPullRequestInfo(pullRequestURL *string) (*api.PullRequestInfo, error) {
	return m.GetPullRequestInfoFunc(pullRequestURL)
}

func (m *MockRemoteGitService) SendInlineComments(comments []*api.InlineComment, pullRequestInfo *api.PullRequestInfo) error {
	return m.SendInlineCommentsFunc(comments, pullRequestInfo)
}

// MockVersionControlService implements api.VersionControlService for testing
type MockVersionControlService struct {
	CloneRepoFunc            func(path, repoUrl, ref string) error
	CloneRepoWithContextFunc func(ctx context.Context, path, repoUrl, ref string) error
}

func (m *MockVersionControlService) CloneRepo(path, repoUrl, ref string) error {
	return m.CloneRepoFunc(path, repoUrl, ref)
}

func (m *MockVersionControlService) CloneRepoWithContext(ctx context.Context, path, repoUrl, ref string) error {
	return m.CloneRepoWithContextFunc(ctx, path, repoUrl, ref)
}

// MockAIAgentService implements api.AIAgentService for testing
type MockAIAgentService struct {
	GeneratePRInlineCommentsFunc            func(options *api.GeneratePRInlineCommentsOptions) ([]*api.InlineComment, error)
	GeneratePRInlineCommentsWithContextFunc func(ctx context.Context, options *api.GeneratePRInlineCommentsOptions) ([]*api.InlineComment, error)
}

func (m *MockAIAgentService) GeneratePRInlineComments(options *api.GeneratePRInlineCommentsOptions) ([]*api.InlineComment, error) {
	return m.GeneratePRInlineCommentsFunc(options)
}

func (m *MockAIAgentService) GeneratePRInlineCommentsWithContext(ctx context.Context, options *api.GeneratePRInlineCommentsOptions) ([]*api.InlineComment, error) {
	return m.GeneratePRInlineCommentsWithContextFunc(ctx, options)
}

func TestApp_Run_DetectVCSProviderError(t *testing.T) {
	mockFactory := &MockServiceFactory{
		DetectVCSProviderTypeFunc: func(url string) (api.VCSProviderType, error) {
			return "", io.ErrUnexpectedEOF
		},
	}

	app := NewAppWithWriters(mockFactory, io.Discard, io.Discard)
	err := app.Run("https://example.com/pr/1")
	if err == nil {
		t.Error("expected error when DetectVCSProviderType fails")
	}
}

func TestApp_Run_UnknownVCSProviderError(t *testing.T) {
	mockFactory := &MockServiceFactory{
		DetectVCSProviderTypeFunc: func(url string) (api.VCSProviderType, error) {
			return VCSProviderTypeUnknown, nil
		},
	}

	app := NewAppWithWriters(mockFactory, io.Discard, io.Discard)
	err := app.Run("https://bitbucket.org/org/repo/pull/1")
	if err == nil {
		t.Error("expected error for unknown VCS provider")
	}
}

func TestApp_Run_CreateVCSProviderError(t *testing.T) {
	mockFactory := &MockServiceFactory{
		DetectVCSProviderTypeFunc: func(url string) (api.VCSProviderType, error) {
			return VCSProviderTypeGithub, nil
		},
		CreateVCSProviderFunc: func(kind api.VCSProviderType) (api.RemoteGitService, error) {
			return nil, io.ErrUnexpectedEOF
		},
	}

	app := NewAppWithWriters(mockFactory, io.Discard, io.Discard)
	err := app.Run("https://github.com/org/repo/pull/1")
	if err == nil {
		t.Error("expected error when CreateVCSProvider fails")
	}
}

func TestApp_Run_GetPullRequestInfoError(t *testing.T) {
	mockVCSProvider := &MockRemoteGitService{
		GetPullRequestInfoFunc: func(pullRequestURL *string) (*api.PullRequestInfo, error) {
			return nil, io.ErrUnexpectedEOF
		},
	}

	mockFactory := &MockServiceFactory{
		DetectVCSProviderTypeFunc: func(url string) (api.VCSProviderType, error) {
			return VCSProviderTypeGithub, nil
		},
		CreateVCSProviderFunc: func(kind api.VCSProviderType) (api.RemoteGitService, error) {
			return mockVCSProvider, nil
		},
	}

	app := NewAppWithWriters(mockFactory, io.Discard, io.Discard)
	err := app.Run("https://github.com/org/repo/pull/1")
	if err == nil {
		t.Error("expected error when GetPullRequestInfo fails")
	}
}

func TestApp_Run_CreateVersionControlServiceError(t *testing.T) {
	mockVCSProvider := &MockRemoteGitService{
		GetPullRequestInfoFunc: func(pullRequestURL *string) (*api.PullRequestInfo, error) {
			return &api.PullRequestInfo{ProjectName: "test-repo"}, nil
		},
	}

	mockFactory := &MockServiceFactory{
		DetectVCSProviderTypeFunc: func(url string) (api.VCSProviderType, error) {
			return VCSProviderTypeGithub, nil
		},
		CreateVCSProviderFunc: func(kind api.VCSProviderType) (api.RemoteGitService, error) {
			return mockVCSProvider, nil
		},
		CreateVersionControlServiceFunc: func(kind api.VersionControlType) (api.VersionControlService, error) {
			return nil, io.ErrUnexpectedEOF
		},
	}

	app := NewAppWithWriters(mockFactory, io.Discard, io.Discard)
	err := app.Run("https://github.com/org/repo/pull/1")
	if err == nil {
		t.Error("expected error when CreateVersionControlService fails")
	}
}

func TestApp_Run_CloneRepoError(t *testing.T) {
	mockVCSProvider := &MockRemoteGitService{
		GetPullRequestInfoFunc: func(pullRequestURL *string) (*api.PullRequestInfo, error) {
			return &api.PullRequestInfo{ProjectName: "test-repo", ProjectHttpUrl: "https://github.com/org/repo.git", SourceBranch: "main"}, nil
		},
	}

	mockVCS := &MockVersionControlService{
		CloneRepoWithContextFunc: func(ctx context.Context, path, repoUrl, ref string) error {
			return io.ErrUnexpectedEOF
		},
	}

	mockFactory := &MockServiceFactory{
		DetectVCSProviderTypeFunc: func(url string) (api.VCSProviderType, error) {
			return VCSProviderTypeGithub, nil
		},
		CreateVCSProviderFunc: func(kind api.VCSProviderType) (api.RemoteGitService, error) {
			return mockVCSProvider, nil
		},
		CreateVersionControlServiceFunc: func(kind api.VersionControlType) (api.VersionControlService, error) {
			return mockVCS, nil
		},
	}

	app := NewAppWithWriters(mockFactory, io.Discard, io.Discard)
	err := app.Run("https://github.com/org/repo/pull/1")
	if err == nil {
		t.Error("expected error when CloneRepoWithContext fails")
	}
}

func TestApp_Run_CreateAiAgentServiceError(t *testing.T) {
	mockVCSProvider := &MockRemoteGitService{
		GetPullRequestInfoFunc: func(pullRequestURL *string) (*api.PullRequestInfo, error) {
			return &api.PullRequestInfo{ProjectName: "test-repo", ProjectHttpUrl: "https://github.com/org/repo.git", SourceBranch: "main"}, nil
		},
	}

	mockVCS := &MockVersionControlService{
		CloneRepoWithContextFunc: func(ctx context.Context, path, repoUrl, ref string) error {
			return nil
		},
	}

	mockFactory := &MockServiceFactory{
		DetectVCSProviderTypeFunc: func(url string) (api.VCSProviderType, error) {
			return VCSProviderTypeGithub, nil
		},
		CreateVCSProviderFunc: func(kind api.VCSProviderType) (api.RemoteGitService, error) {
			return mockVCSProvider, nil
		},
		CreateVersionControlServiceFunc: func(kind api.VersionControlType) (api.VersionControlService, error) {
			return mockVCS, nil
		},
		CreateAiAgentServiceFunc: func(kind api.AIAgentType) (api.AIAgentService, error) {
			return nil, io.ErrUnexpectedEOF
		},
	}

	app := NewAppWithWriters(mockFactory, io.Discard, io.Discard)
	err := app.Run("https://github.com/org/repo/pull/1")
	if err == nil {
		t.Error("expected error when CreateAiAgentService fails")
	}
}

func TestApp_Run_GeneratePRInlineCommentsError(t *testing.T) {
	mockVCSProvider := &MockRemoteGitService{
		GetPullRequestInfoFunc: func(pullRequestURL *string) (*api.PullRequestInfo, error) {
			return &api.PullRequestInfo{ProjectName: "test-repo", ProjectHttpUrl: "https://github.com/org/repo.git", SourceBranch: "main"}, nil
		},
	}

	mockVCS := &MockVersionControlService{
		CloneRepoWithContextFunc: func(ctx context.Context, path, repoUrl, ref string) error {
			return nil
		},
	}

	mockAI := &MockAIAgentService{
		GeneratePRInlineCommentsWithContextFunc: func(ctx context.Context, options *api.GeneratePRInlineCommentsOptions) ([]*api.InlineComment, error) {
			return nil, io.ErrUnexpectedEOF
		},
	}

	mockFactory := &MockServiceFactory{
		DetectVCSProviderTypeFunc: func(url string) (api.VCSProviderType, error) {
			return VCSProviderTypeGithub, nil
		},
		CreateVCSProviderFunc: func(kind api.VCSProviderType) (api.RemoteGitService, error) {
			return mockVCSProvider, nil
		},
		CreateVersionControlServiceFunc: func(kind api.VersionControlType) (api.VersionControlService, error) {
			return mockVCS, nil
		},
		CreateAiAgentServiceFunc: func(kind api.AIAgentType) (api.AIAgentService, error) {
			return mockAI, nil
		},
	}

	app := NewAppWithWriters(mockFactory, io.Discard, io.Discard)
	err := app.Run("https://github.com/org/repo/pull/1")
	if err == nil {
		t.Error("expected error when GeneratePRInlineCommentsWithContext fails")
	}
}

func TestApp_Run_SendInlineCommentsError(t *testing.T) {
	mockVCSProvider := &MockRemoteGitService{
		GetPullRequestInfoFunc: func(pullRequestURL *string) (*api.PullRequestInfo, error) {
			return &api.PullRequestInfo{ProjectName: "test-repo", ProjectHttpUrl: "https://github.com/org/repo.git", SourceBranch: "main"}, nil
		},
		SendInlineCommentsFunc: func(comments []*api.InlineComment, pullRequestInfo *api.PullRequestInfo) error {
			return io.ErrUnexpectedEOF
		},
	}

	mockVCS := &MockVersionControlService{
		CloneRepoWithContextFunc: func(ctx context.Context, path, repoUrl, ref string) error {
			return nil
		},
	}

	mockAI := &MockAIAgentService{
		GeneratePRInlineCommentsWithContextFunc: func(ctx context.Context, options *api.GeneratePRInlineCommentsOptions) ([]*api.InlineComment, error) {
			return []*api.InlineComment{}, nil
		},
	}

	mockFactory := &MockServiceFactory{
		DetectVCSProviderTypeFunc: func(url string) (api.VCSProviderType, error) {
			return VCSProviderTypeGithub, nil
		},
		CreateVCSProviderFunc: func(kind api.VCSProviderType) (api.RemoteGitService, error) {
			return mockVCSProvider, nil
		},
		CreateVersionControlServiceFunc: func(kind api.VersionControlType) (api.VersionControlService, error) {
			return mockVCS, nil
		},
		CreateAiAgentServiceFunc: func(kind api.AIAgentType) (api.AIAgentService, error) {
			return mockAI, nil
		},
	}

	var stderr bytes.Buffer
	app := NewAppWithWriters(mockFactory, io.Discard, &stderr)
	err := app.Run("https://github.com/org/repo/pull/1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Contains(stderr.Bytes(), []byte("Warning")) {
		t.Error("expected warning in stderr for SendInlineComments error")
	}
}

func TestApp_Run_Success(t *testing.T) {
	mockVCSProvider := &MockRemoteGitService{
		GetPullRequestInfoFunc: func(pullRequestURL *string) (*api.PullRequestInfo, error) {
			return &api.PullRequestInfo{
				ProjectName:    "test-repo",
				ProjectHttpUrl: "https://github.com/org/test-repo.git",
				SourceBranch:   "feature-branch",
				BaseSha:        "abc123",
				StartSha:       "def456",
				HeadSha:        "ghi789",
			}, nil
		},
		SendInlineCommentsFunc: func(comments []*api.InlineComment, pullRequestInfo *api.PullRequestInfo) error {
			return nil
		},
	}

	mockVCS := &MockVersionControlService{
		CloneRepoWithContextFunc: func(ctx context.Context, path, repoUrl, ref string) error {
			return nil
		},
	}

	mockAI := &MockAIAgentService{
		GeneratePRInlineCommentsWithContextFunc: func(ctx context.Context, options *api.GeneratePRInlineCommentsOptions) ([]*api.InlineComment, error) {
			return []*api.InlineComment{}, nil
		},
	}

	mockFactory := &MockServiceFactory{
		DetectVCSProviderTypeFunc: func(url string) (api.VCSProviderType, error) {
			return VCSProviderTypeGithub, nil
		},
		CreateVCSProviderFunc: func(kind api.VCSProviderType) (api.RemoteGitService, error) {
			return mockVCSProvider, nil
		},
		CreateVersionControlServiceFunc: func(kind api.VersionControlType) (api.VersionControlService, error) {
			return mockVCS, nil
		},
		CreateAiAgentServiceFunc: func(kind api.AIAgentType) (api.AIAgentService, error) {
			return mockAI, nil
		},
	}

	var stdout bytes.Buffer
	app := NewAppWithWriters(mockFactory, &stdout, io.Discard)
	err := app.Run("https://github.com/org/repo/pull/1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := stdout.String()
	if !bytes.Contains([]byte(output), []byte("VCS provider type: github")) {
		t.Errorf("expected output to contain VCS provider type, got: %s", output)
	}
}

func TestSanitizeProjectName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"my-project", "my-project"},
		{"my_project", "my_project"},
		{"my project", "my_project"},
		{"my/project", "my_project"},
		{"my\\project", "my_project"},
		{"../../../etc/passwd", "_________etc_passwd"},
		{"project@name!", "project_name_"},
		{"", "project"},
		{"   ", "___"},
		{"valid123", "valid123"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeProjectName(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeProjectName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
