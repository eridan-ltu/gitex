package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eridan-ltu/gitex/api"
)

func TestParseInput(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantUrl     string
		wantCfg     *api.Config
		expectError bool
	}{
		{
			name:    "minimal args - just URL",
			args:    []string{"https://github.com/owner/repo/pull/123"},
			wantUrl: "https://github.com/owner/repo/pull/123",
			wantCfg: &api.Config{
				AiModel: "gpt-5.1-codex-mini",
			},
		},
		{
			name:    "with vcs-api-key flag",
			args:    []string{"https://github.com/owner/repo/pull/1", "-vcs-api-key", "token123"},
			wantUrl: "https://github.com/owner/repo/pull/1",
			wantCfg: &api.Config{
				VcsApiKey: "token123",
				AiModel:   "gpt-5.1-codex-mini",
			},
		},
		{
			name:    "with all flags",
			args:    []string{"https://gitlab.com/owner/repo/-/merge_requests/42", "-vcs-api-key", "vcs-token", "-vcs-url", "https://gitlab.example.com", "-ai-api-key", "ai-token", "-ai-model", "gpt-4", "-verbose"},
			wantUrl: "https://gitlab.com/owner/repo/-/merge_requests/42",
			wantCfg: &api.Config{
				VcsApiKey:    "vcs-token",
				VcsRemoteUrl: "https://gitlab.example.com",
				AiApiKey:     "ai-token",
				AiModel:      "gpt-4",
				Verbose:      true,
			},
		},
		{
			name:        "no args - error",
			args:        []string{},
			expectError: true,
		},
		{
			name:        "invalid flag - error",
			args:        []string{"https://github.com/owner/repo/pull/1", "-invalid-flag"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, cfg, err := parseInput(tt.args)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if url != tt.wantUrl {
				t.Errorf("url = %q, want %q", url, tt.wantUrl)
			}

			if cfg.VcsApiKey != tt.wantCfg.VcsApiKey {
				t.Errorf("VcsApiKey = %q, want %q", cfg.VcsApiKey, tt.wantCfg.VcsApiKey)
			}
			if cfg.VcsRemoteUrl != tt.wantCfg.VcsRemoteUrl {
				t.Errorf("VcsRemoteUrl = %q, want %q", cfg.VcsRemoteUrl, tt.wantCfg.VcsRemoteUrl)
			}
			if cfg.AiApiKey != tt.wantCfg.AiApiKey {
				t.Errorf("AiApiKey = %q, want %q", cfg.AiApiKey, tt.wantCfg.AiApiKey)
			}
			if cfg.AiModel != tt.wantCfg.AiModel {
				t.Errorf("AiModel = %q, want %q", cfg.AiModel, tt.wantCfg.AiModel)
			}
			if cfg.Verbose != tt.wantCfg.Verbose {
				t.Errorf("Verbose = %v, want %v", cfg.Verbose, tt.wantCfg.Verbose)
			}
		})
	}
}

func TestPopulateFromEnv(t *testing.T) {
	// Save original env and restore after test
	origVcsKey := os.Getenv("VCS_API_KEY")
	origAiKey := os.Getenv("AI_API_KEY")
	origCI := os.Getenv("CI")
	origHome := os.Getenv("GITEX_HOME")
	defer func() {
		_ = os.Setenv("VCS_API_KEY", origVcsKey)
		_ = os.Setenv("AI_API_KEY", origAiKey)
		_ = os.Setenv("CI", origCI)
		_ = os.Setenv("GITEX_HOME", origHome)
	}()

	t.Run("uses env vars when flags not set", func(t *testing.T) {
		_ = os.Setenv("VCS_API_KEY", "env-vcs-key")
		_ = os.Setenv("AI_API_KEY", "env-ai-key")
		_ = os.Setenv("CI", "true")
		_ = os.Setenv("GITEX_HOME", "/custom/home")

		cfg := &api.Config{}
		err := populateFromEnv(cfg)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.VcsApiKey != "env-vcs-key" {
			t.Errorf("VcsApiKey = %q, want %q", cfg.VcsApiKey, "env-vcs-key")
		}
		if cfg.AiApiKey != "env-ai-key" {
			t.Errorf("AiApiKey = %q, want %q", cfg.AiApiKey, "env-ai-key")
		}
		if !cfg.CI {
			t.Error("CI should be true")
		}
		if cfg.HomeDir != "/custom/home" {
			t.Errorf("HomeDir = %q, want %q", cfg.HomeDir, "/custom/home")
		}
		if cfg.BinDir != "/custom/home/bin" {
			t.Errorf("BinDir = %q, want %q", cfg.BinDir, "/custom/home/bin")
		}
	})

	t.Run("flags take precedence over env", func(t *testing.T) {
		_ = os.Setenv("VCS_API_KEY", "env-vcs-key")
		_ = os.Setenv("AI_API_KEY", "env-ai-key")

		cfg := &api.Config{
			VcsApiKey: "flag-vcs-key",
			AiApiKey:  "flag-ai-key",
		}
		err := populateFromEnv(cfg)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.VcsApiKey != "flag-vcs-key" {
			t.Errorf("VcsApiKey = %q, want %q", cfg.VcsApiKey, "flag-vcs-key")
		}
		if cfg.AiApiKey != "flag-ai-key" {
			t.Errorf("AiApiKey = %q, want %q", cfg.AiApiKey, "flag-ai-key")
		}
	})

	t.Run("error when VCS_API_KEY missing", func(t *testing.T) {
		_ = os.Unsetenv("VCS_API_KEY")
		_ = os.Setenv("AI_API_KEY", "ai-key")

		cfg := &api.Config{}
		err := populateFromEnv(cfg)

		if err == nil {
			t.Error("expected error for missing VCS_API_KEY")
		}
	})

	t.Run("error when AI_API_KEY missing", func(t *testing.T) {
		_ = os.Setenv("VCS_API_KEY", "vcs-key")
		_ = os.Unsetenv("AI_API_KEY")

		cfg := &api.Config{}
		err := populateFromEnv(cfg)

		if err == nil {
			t.Error("expected error for missing AI_API_KEY")
		}
	})

	t.Run("default home dir when GITEX_HOME not set", func(t *testing.T) {
		_ = os.Setenv("VCS_API_KEY", "vcs-key")
		_ = os.Setenv("AI_API_KEY", "ai-key")
		_ = os.Unsetenv("GITEX_HOME")

		cfg := &api.Config{}
		err := populateFromEnv(cfg)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		homeDir, _ := os.UserHomeDir()
		expectedHome := filepath.Join(homeDir, ".gitex")
		if cfg.HomeDir != expectedHome {
			t.Errorf("HomeDir = %q, want %q", cfg.HomeDir, expectedHome)
		}
	})

	t.Run("CI false when not set to true", func(t *testing.T) {
		_ = os.Setenv("VCS_API_KEY", "vcs-key")
		_ = os.Setenv("AI_API_KEY", "ai-key")
		_ = os.Setenv("CI", "false")

		cfg := &api.Config{}
		err := populateFromEnv(cfg)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.CI {
			t.Error("CI should be false")
		}
	})
}
