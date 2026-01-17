package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/eridan-ltu/gitex/api"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewCodexService(t *testing.T) {
	cfg := &api.Config{
		AiModel: "test-model",
		Verbose: true,
	}

	svc := NewCodexService(cfg)

	if svc == nil {
		t.Fatal("expected CodexService to be non-nil")
	}
	if svc.cfg != cfg {
		t.Error("expected cfg to be set correctly")
	}
	if svc.commandRunner == nil {
		t.Error("expected commandRunner to be set")
	}
	if svc.loginRunner == nil {
		t.Error("expected loginRunner to be set")
	}
	if svc.logoutRunner == nil {
		t.Error("expected logoutRunner to be set")
	}
}

func TestCodexService_GeneratePRInlineComments(t *testing.T) {
	mockLoginRunner := func(ctx context.Context, apiKey string) error {
		return nil
	}
	mockLogoutRunner := func(ctx context.Context) error {
		return nil
	}

	t.Run("successful generation with valid output", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a mock comments file
		expectedComments := []*api.InlineComment{
			{
				Body: Ptr("Test comment"),
				Position: &api.InlineCommentPosition{
					PositionType: Ptr("text"),
					BaseSha:      Ptr("base123"),
					StartSha:     Ptr("start123"),
					HeadSha:      Ptr("head123"),
					OldPath:      Ptr("old.go"),
					NewPath:      Ptr("new.go"),
					NewLine:      Ptr(int64(10)),
				},
			},
		}
		commentsData, _ := json.Marshal(expectedComments)
		commentsFilePath := filepath.Join(tmpDir, commentsFileName)

		cfg := &api.Config{
			AiModel: "test-model",
			Verbose: false,
		}

		svc := NewCodexService(cfg)
		svc.loginRunner = mockLoginRunner
		svc.logoutRunner = mockLogoutRunner
		svc.commandRunner = func(ctx context.Context, name string, args ...string) *exec.Cmd {
			return exec.Command("sh", "-c",
				"echo '"+string(commentsData)+"' > "+commentsFilePath)
		}

		options := &api.GeneratePRInlineCommentsOptions{
			BaseSha:    "base123",
			StartSha:   "start123",
			HeadSha:    "head123",
			SandBoxDir: tmpDir,
		}

		comments, err := svc.GeneratePRInlineComments(options)

		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if len(comments) != 1 {
			t.Errorf("expected 1 comment, got %d", len(comments))
		}
		if *comments[0].Body != "Test comment" {
			t.Errorf("expected body 'Test comment', got '%s'", *comments[0].Body)
		}

		// Verify cleanup - comments file should be removed
		if _, err := os.Stat(commentsFilePath); !os.IsNotExist(err) {
			t.Error("expected comments file to be cleaned up")
		}
	})

	t.Run("error when login fails", func(t *testing.T) {
		tmpDir := t.TempDir()

		cfg := &api.Config{
			AiModel:  "test-model",
			AiApiKey: "test-key",
			Verbose:  false,
			CI:       true,
		}

		svc := NewCodexService(cfg)
		svc.loginRunner = func(ctx context.Context, apiKey string) error {
			return fmt.Errorf("login failed")
		}
		svc.logoutRunner = mockLogoutRunner

		options := &api.GeneratePRInlineCommentsOptions{
			BaseSha:    "base123",
			StartSha:   "start123",
			HeadSha:    "head123",
			SandBoxDir: tmpDir,
		}

		_, err := svc.GeneratePRInlineComments(options)

		if err == nil {
			t.Error("expected error when login fails")
		}
		if !strings.Contains(err.Error(), "codex login failed") {
			t.Errorf("expected error message to contain 'codex login failed', got: %v", err)
		}
	})

	t.Run("login receives correct api key", func(t *testing.T) {
		tmpDir := t.TempDir()
		commentsFilePath := filepath.Join(tmpDir, commentsFileName)

		var capturedApiKey string

		cfg := &api.Config{
			AiModel:  "test-model",
			AiApiKey: "my-secret-key",
			Verbose:  false,
			CI:       true,
		}

		svc := NewCodexService(cfg)
		svc.loginRunner = func(ctx context.Context, apiKey string) error {
			capturedApiKey = apiKey
			return nil
		}
		svc.logoutRunner = mockLogoutRunner
		svc.commandRunner = func(ctx context.Context, name string, args ...string) *exec.Cmd {
			return exec.Command("sh", "-c", "echo '[]' > "+commentsFilePath)
		}

		options := &api.GeneratePRInlineCommentsOptions{
			BaseSha:    "base123",
			StartSha:   "start123",
			HeadSha:    "head123",
			SandBoxDir: tmpDir,
		}

		_, _ = svc.GeneratePRInlineComments(options)

		if capturedApiKey != "my-secret-key" {
			t.Errorf("expected api key 'my-secret-key', got '%s'", capturedApiKey)
		}
	})

	t.Run("login and logout not called when CI is false", func(t *testing.T) {
		tmpDir := t.TempDir()
		commentsFilePath := filepath.Join(tmpDir, commentsFileName)

		var loginCalled, logoutCalled bool

		cfg := &api.Config{
			AiModel: "test-model",
			Verbose: false,
			CI:      false,
		}

		svc := NewCodexService(cfg)
		svc.loginRunner = func(ctx context.Context, apiKey string) error {
			loginCalled = true
			return nil
		}
		svc.logoutRunner = func(ctx context.Context) error {
			logoutCalled = true
			return nil
		}
		svc.commandRunner = func(ctx context.Context, name string, args ...string) *exec.Cmd {
			return exec.Command("sh", "-c", "echo '[]' > "+commentsFilePath)
		}

		options := &api.GeneratePRInlineCommentsOptions{
			BaseSha:    "base123",
			StartSha:   "start123",
			HeadSha:    "head123",
			SandBoxDir: tmpDir,
		}

		_, _ = svc.GeneratePRInlineComments(options)

		if loginCalled {
			t.Error("expected login NOT to be called when CI is false")
		}
		if logoutCalled {
			t.Error("expected logout NOT to be called when CI is false")
		}
	})

	t.Run("error when command fails", func(t *testing.T) {
		tmpDir := t.TempDir()

		cfg := &api.Config{
			AiModel: "test-model",
			Verbose: false,
		}

		svc := NewCodexService(cfg)
		svc.loginRunner = mockLoginRunner
		svc.logoutRunner = mockLogoutRunner
		svc.commandRunner = func(ctx context.Context, name string, args ...string) *exec.Cmd {
			// Return a command that will fail
			return exec.Command("sh", "-c", "exit 1")
		}

		options := &api.GeneratePRInlineCommentsOptions{
			BaseSha:    "base123",
			StartSha:   "start123",
			HeadSha:    "head123",
			SandBoxDir: tmpDir,
		}

		_, err := svc.GeneratePRInlineComments(options)

		if err == nil {
			t.Error("expected error when command fails")
		}
		if !strings.Contains(err.Error(), "error generating PR inline-comments") {
			t.Errorf("expected error message to contain 'error generating PR inline-comments', got: %v", err)
		}
	})

	t.Run("error when comments file has invalid json", func(t *testing.T) {
		tmpDir := t.TempDir()
		commentsFilePath := filepath.Join(tmpDir, commentsFileName)

		cfg := &api.Config{
			AiModel: "test-model",
			Verbose: false,
		}

		svc := NewCodexService(cfg)
		svc.loginRunner = mockLoginRunner
		svc.logoutRunner = mockLogoutRunner
		svc.commandRunner = func(ctx context.Context, name string, args ...string) *exec.Cmd {
			// Write invalid JSON
			return exec.Command("sh", "-c",
				"echo 'invalid json' > "+commentsFilePath)
		}

		options := &api.GeneratePRInlineCommentsOptions{
			BaseSha:    "base123",
			StartSha:   "start123",
			HeadSha:    "head123",
			SandBoxDir: tmpDir,
		}

		_, err := svc.GeneratePRInlineComments(options)

		if err == nil {
			t.Error("expected error when json is invalid")
		}
		if !strings.Contains(err.Error(), "error unmarshaling comments file") {
			t.Errorf("expected error message to contain 'error unmarshaling comments file', got: %v", err)
		}
	})

	t.Run("successful when comments file does not exist", func(t *testing.T) {
		tmpDir := t.TempDir()

		cfg := &api.Config{
			AiModel: "test-model",
			Verbose: false,
		}

		svc := NewCodexService(cfg)
		svc.loginRunner = mockLoginRunner
		svc.logoutRunner = mockLogoutRunner
		svc.commandRunner = func(ctx context.Context, name string, args ...string) *exec.Cmd {
			// Command succeeds but doesn't create file
			return exec.Command("sh", "-c", "exit 0")
		}

		options := &api.GeneratePRInlineCommentsOptions{
			BaseSha:    "base123",
			StartSha:   "start123",
			HeadSha:    "head123",
			SandBoxDir: tmpDir,
		}

		_, err := svc.GeneratePRInlineComments(options)

		if err == nil {
			t.Error("expected error when comments file doesn't exist")
		}
		if !strings.Contains(err.Error(), "error unmarshaling comments file") {
			t.Errorf("expected error message to contain 'error unmarshaling comments file', got: %v", err)
		}
	})
}

func TestCodexService_GeneratePRInlineCommentsWithContext(t *testing.T) {
	mockLoginRunner := func(ctx context.Context, apiKey string) error {
		return nil
	}
	mockLogoutRunner := func(ctx context.Context) error {
		return nil
	}

	t.Run("successful generation with context", func(t *testing.T) {
		tmpDir := t.TempDir()

		expectedComments := []*api.InlineComment{
			{
				Body: Ptr("Context test comment"),
				Position: &api.InlineCommentPosition{
					PositionType: Ptr("text"),
					BaseSha:      Ptr("base456"),
					HeadSha:      Ptr("head456"),
				},
			},
		}
		commentsData, _ := json.Marshal(expectedComments)
		commentsFilePath := filepath.Join(tmpDir, commentsFileName)

		cfg := &api.Config{
			AiModel: "test-model",
			Verbose: false,
		}

		svc := NewCodexService(cfg)
		svc.loginRunner = mockLoginRunner
		svc.logoutRunner = mockLogoutRunner
		svc.commandRunner = func(ctx context.Context, name string, args ...string) *exec.Cmd {
			return exec.Command("sh", "-c",
				"echo '"+string(commentsData)+"' > "+commentsFilePath)
		}

		options := &api.GeneratePRInlineCommentsOptions{
			BaseSha:    "base456",
			StartSha:   "start456",
			HeadSha:    "head456",
			SandBoxDir: tmpDir,
		}

		ctx := context.Background()
		comments, err := svc.GeneratePRInlineCommentsWithContext(ctx, options)

		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if len(comments) != 1 {
			t.Errorf("expected 1 comment, got %d", len(comments))
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		tmpDir := t.TempDir()

		cfg := &api.Config{
			AiModel: "test-model",
			Verbose: false,
		}

		svc := NewCodexService(cfg)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		svc.loginRunner = func(ctx context.Context, apiKey string) error {
			return ctx.Err()
		}
		svc.commandRunner = func(ctx context.Context, name string, args ...string) *exec.Cmd {
			// Command that would take time
			return exec.CommandContext(ctx, "sleep", "10")
		}

		options := &api.GeneratePRInlineCommentsOptions{
			BaseSha:    "base789",
			StartSha:   "start789",
			HeadSha:    "head789",
			SandBoxDir: tmpDir,
		}

		_, err := svc.GeneratePRInlineCommentsWithContext(ctx, options)

		if err == nil {
			t.Error("expected error due to context cancellation")
		}
	})

	t.Run("command receives correct arguments", func(t *testing.T) {
		tmpDir := t.TempDir()

		var capturedArgs []string

		cfg := &api.Config{
			AiModel: "gpt-4",
			Verbose: false,
		}

		svc := NewCodexService(cfg)
		svc.loginRunner = mockLoginRunner
		svc.logoutRunner = mockLogoutRunner
		svc.commandRunner = func(ctx context.Context, name string, args ...string) *exec.Cmd {
			capturedArgs = append([]string{name}, args...)

			// Create a valid response
			commentsFilePath := filepath.Join(tmpDir, commentsFileName)
			return exec.Command("sh", "-c",
				"echo '[]' > "+commentsFilePath)
		}

		options := &api.GeneratePRInlineCommentsOptions{
			BaseSha:    "abc123",
			StartSha:   "def456",
			HeadSha:    "ghi789",
			SandBoxDir: tmpDir,
		}

		ctx := context.Background()
		_, err := svc.GeneratePRInlineCommentsWithContext(ctx, options)

		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		if len(capturedArgs) == 0 {
			t.Fatal("expected arguments to be captured")
		}
		if capturedArgs[0] != "codex" {
			t.Errorf("expected command 'codex', got '%s'", capturedArgs[0])
		}
		if !contains(capturedArgs, "exec") {
			t.Error("expected 'exec' in arguments")
		}
		if !contains(capturedArgs, "--model") {
			t.Error("expected '--model' in arguments")
		}
		if !contains(capturedArgs, "gpt-4") {
			t.Error("expected 'gpt-4' in arguments")
		}
	})

	t.Run("verbose mode outputs to stdout/stderr", func(t *testing.T) {
		tmpDir := t.TempDir()
		commentsFilePath := filepath.Join(tmpDir, commentsFileName)

		cfg := &api.Config{
			AiModel: "test-model",
			Verbose: true,
		}

		svc := NewCodexService(cfg)
		svc.loginRunner = mockLoginRunner
		svc.logoutRunner = mockLogoutRunner
		svc.commandRunner = func(ctx context.Context, name string, args ...string) *exec.Cmd {
			return exec.Command("sh", "-c",
				"echo '[]' > "+commentsFilePath)
		}

		options := &api.GeneratePRInlineCommentsOptions{
			BaseSha:    "base123",
			StartSha:   "start123",
			HeadSha:    "head123",
			SandBoxDir: tmpDir,
		}

		// This test primarily ensures verbose mode doesn't crash
		_, err := svc.GeneratePRInlineComments(options)

		if err != nil {
			t.Fatalf("expected no error in verbose mode, got: %v", err)
		}
	})
}

func TestCodexService_CleanupBehavior(t *testing.T) {
	t.Run("comments file is cleaned up even on error", func(t *testing.T) {
		tmpDir := t.TempDir()
		commentsFilePath := filepath.Join(tmpDir, commentsFileName)

		// Pre-create the comments file
		if err := os.WriteFile(commentsFilePath, []byte("test"), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		cfg := &api.Config{
			AiModel: "test-model",
			Verbose: false,
		}

		svc := NewCodexService(cfg)
		svc.loginRunner = func(ctx context.Context, apiKey string) error {
			return nil
		}
		svc.logoutRunner = func(ctx context.Context) error {
			return nil
		}
		svc.commandRunner = func(ctx context.Context, name string, args ...string) *exec.Cmd {
			// Command fails but file should still be cleaned up
			return exec.Command("sh", "-c", "exit 1")
		}

		options := &api.GeneratePRInlineCommentsOptions{
			BaseSha:    "base123",
			StartSha:   "start123",
			HeadSha:    "head123",
			SandBoxDir: tmpDir,
		}

		_, _ = svc.GeneratePRInlineComments(options)

		// Verify file was cleaned up despite error
		if _, err := os.Stat(commentsFilePath); !os.IsNotExist(err) {
			t.Error("expected comments file to be cleaned up even on error")
		}
	})
}

// Helper function
func contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}
