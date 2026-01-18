package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/eridan-ltu/gitex/api"
	"github.com/eridan-ltu/gitex/internal/core"
)

func main() {
	mrUrl, cfg, err := parseInput(os.Args[1:])
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	if err := populateFromEnv(cfg); err != nil {
		log.Fatalf("Error: %v", err)
	}

	factory := core.NewServiceFactory(cfg)
	app := core.NewApp(factory)
	if err := app.Run(mrUrl); err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func parseInput(args []string) (string, *api.Config, error) {
	if len(args) < 1 {
		return "", nil, errors.New("usage: gitex <pull-request-url> [flags]")
	}
	mrUrl := args[0]

	cfg := &api.Config{}
	fs := flag.NewFlagSet("gitex", flag.ContinueOnError)
	fs.StringVar(&cfg.VcsApiKey, "vcs-api-key", "", "VCS provider API Key")
	fs.StringVar(&cfg.VcsRemoteUrl, "vcs-url", "", "VCS provider url")
	fs.StringVar(&cfg.AiModel, "ai-model", "gpt-5.1-codex-mini", "Codex model")
	fs.StringVar(&cfg.AiApiKey, "ai-api-key", "", "AI API Key")
	fs.BoolVar(&cfg.Verbose, "verbose", false, "Verbose output")

	fs.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, "Usage: gitex <pull-request-url> [flags]\n\n")
		_, _ = fmt.Fprintf(os.Stderr, "Arguments:\n")
		_, _ = fmt.Fprintf(os.Stderr, "  pull-request-url    Pull request URL\n\n")
		_, _ = fmt.Fprintf(os.Stderr, "Flags:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args[1:]); err != nil {
		return "", nil, fmt.Errorf("failed to parse flags: %w", err)
	}

	return mrUrl, cfg, nil
}

func populateFromEnv(cfg *api.Config) error {
	if cfg.VcsApiKey == "" {
		cfg.VcsApiKey = os.Getenv("VCS_API_KEY")
		if cfg.VcsApiKey == "" {
			return errors.New("vcs-api-key is not set. Provide it as an argument or set VCS_API_KEY environment variable")
		}
	}

	if cfg.AiApiKey == "" {
		cfg.AiApiKey = os.Getenv("AI_API_KEY")
		if cfg.AiApiKey == "" {
			return errors.New("ai-api-key is not set. Provide it as an argument or set AI_API_KEY environment variable")
		}
	}

	cfg.CI = os.Getenv("CI") == "true"
	cfg.HomeDir = os.Getenv("GITEX_HOME")
	if cfg.HomeDir == "" {
		dir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home dir: %w", err)
		}
		cfg.HomeDir = filepath.Join(dir, ".gitex")
	}
	cfg.BinDir = filepath.Join(cfg.HomeDir, "bin")
	return nil
}
