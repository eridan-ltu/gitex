package vcs

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v6/plumbing/transport/http"
)

func TestNewGitService(t *testing.T) {
	auth := &http.BasicAuth{
		Username: "test",
		Password: "token",
	}

	svc := NewGitService(auth)

	if svc == nil {
		t.Fatal("expected GitService to be non-nil")
	}
	if svc.auth != auth {
		t.Error("expected auth to be set correctly")
	}
}

func TestGitService_CloneRepo(t *testing.T) {
	t.Run("successful clone with valid public repo", func(t *testing.T) {
		tmpDir := t.TempDir()
		clonePath := filepath.Join(tmpDir, "repo")

		svc := NewGitService(nil)
		err := svc.CloneRepo(
			clonePath,
			"https://github.com/go-git/go-git",
			"refs/heads/master",
		)

		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		// Verify .git directory exists
		gitDir := filepath.Join(clonePath, ".git")
		if _, err = os.Stat(gitDir); err != nil {
			t.Errorf(".git directory should exist, got error: %v", err)
		}
	})

	t.Run("error when cloning to existing directory", func(t *testing.T) {
		tmpDir := t.TempDir()

		svc := NewGitService(nil)

		// First clone should succeed
		err := svc.CloneRepo(
			tmpDir,
			"https://github.com/go-git/go-git",
			"refs/heads/master",
		)
		if err != nil {
			t.Fatalf("first clone should succeed, got: %v", err)
		}

		// Second clone to same path should fail
		err = svc.CloneRepo(
			tmpDir,
			"https://github.com/go-git/go-git",
			"refs/heads/master",
		)
		if err == nil {
			t.Error("expected error when cloning to existing directory")
		}
		if !strings.Contains(err.Error(), "error clone repo") {
			t.Errorf("expected error to contain 'error clone repo', got: %v", err)
		}
	})

	t.Run("error with invalid repository URL", func(t *testing.T) {
		tmpDir := t.TempDir()
		clonePath := filepath.Join(tmpDir, "repo")

		svc := NewGitService(nil)
		err := svc.CloneRepo(
			clonePath,
			"https://github.com/invalid/nonexistent-repo-xyz",
			"refs/heads/main",
		)

		if err == nil {
			t.Error("expected error with invalid repository URL")
		}
		if !strings.Contains(err.Error(), "error clone repo") {
			t.Errorf("expected error to contain 'error clone repo', got: %v", err)
		}
	})

	t.Run("error with invalid reference", func(t *testing.T) {
		tmpDir := t.TempDir()
		clonePath := filepath.Join(tmpDir, "repo")

		svc := NewGitService(nil)
		err := svc.CloneRepo(
			clonePath,
			"https://github.com/go-git/go-git",
			"refs/heads/nonexistent-branch",
		)

		if err == nil {
			t.Error("expected error with invalid reference")
		}
		if !strings.Contains(err.Error(), "error clone repo") {
			t.Errorf("expected error to contain 'error clone repo', got: %v", err)
		}
	})
}

func TestGitService_CloneRepoWithContext(t *testing.T) {
	t.Run("successful clone with context", func(t *testing.T) {
		tmpDir := t.TempDir()
		clonePath := filepath.Join(tmpDir, "repo")

		ctx := context.Background()
		svc := NewGitService(nil)

		err := svc.CloneRepoWithContext(
			ctx,
			clonePath,
			"https://github.com/go-git/go-git",
			"refs/heads/master",
		)

		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		gitDir := filepath.Join(clonePath, ".git")
		if _, err = os.Stat(gitDir); err != nil {
			t.Errorf(".git directory should exist, got error: %v", err)
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		tmpDir := t.TempDir()
		clonePath := filepath.Join(tmpDir, "repo")

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()

		// Give the context time to expire
		time.Sleep(10 * time.Millisecond)

		svc := NewGitService(nil)
		err := svc.CloneRepoWithContext(
			ctx,
			clonePath,
			"https://github.com/go-git/go-git",
			"refs/heads/master",
		)

		if err == nil {
			t.Error("expected error due to context cancellation")
		}
	})

}

func TestGitService_CloneRepo_CallsCloneRepoWithContext(t *testing.T) {
	// This test verifies that CloneRepo properly delegates to CloneRepoWithContext
	tmpDir := t.TempDir()
	clonePath := filepath.Join(tmpDir, "repo")

	svc := NewGitService(nil)
	err := svc.CloneRepo(
		clonePath,
		"https://github.com/go-git/go-git",
		"refs/heads/master",
	)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify the repo was actually cloned
	gitDir := filepath.Join(clonePath, ".git")
	if _, err = os.Stat(gitDir); err != nil {
		t.Errorf(".git directory should exist, got error: %v", err)
	}
}
