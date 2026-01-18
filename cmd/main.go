package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/eridan-ltu/gitex/api"
	"github.com/eridan-ltu/gitex/internal/core"
)

func main() {
	mrUrl, cfg := parseInput()

	factory := core.NewServiceFactory(cfg)
	app := core.NewApp(factory)
	if err := app.Run(mrUrl); err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func parseInput() (string, *api.Config) {
	if len(os.Args) < 2 {
		log.Fatalf("Usage: gitex <pull-request-url> [flags]")
	}
	mrUrl := os.Args[1]

	cfg := &api.Config{}
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	fs.StringVar(&cfg.VcsApiKey, "vcs-api-key", "", "VCS provider API Key")
	fs.StringVar(&cfg.VcsRemoteUrl, "vcs-url", "", "VCS provider url")
	fs.StringVar(&cfg.AiModel, "ai-model", "gpt-5.1-codex-mini", "Codex model")
	fs.StringVar(&cfg.AiApiKey, "ai-api-key", "", "AI API Key")
	fs.BoolVar(&cfg.Verbose, "verbose", false, "Verbose output")

	fs.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, "Usage: %s <pull-request-url> [flags]\n\n", os.Args[0])
		_, _ = fmt.Fprintf(os.Stderr, "Arguments:\n")
		_, _ = fmt.Fprintf(os.Stderr, "  pull-request-url    Pull request URL\n\n")
		_, _ = fmt.Fprintf(os.Stderr, "Flags:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(os.Args[2:]); err != nil {
		log.Fatalf("Failed to parse flags: %v", err)
	}

	populateFromEnv(cfg)

	return mrUrl, cfg
}

func populateFromEnv(cfg *api.Config) {
	if cfg.VcsApiKey == "" {
		cfg.VcsApiKey = os.Getenv("VCS_API_KEY")
		if cfg.VcsApiKey == "" {
			log.Fatal("vcs-api-key is not set. Provide it as an argument or set VCS_API_KEY environment variable")
		}
	}

	if cfg.AiApiKey == "" {
		cfg.AiApiKey = os.Getenv("AI_API_KEY")
		if cfg.AiApiKey == "" {
			log.Fatal("ai-api-key is not set. Provide it as an argument or set AI_API_KEY environment variable")
		}
	}

	cfg.CI = os.Getenv("CI") == "true"
	cfg.HomeDir = os.Getenv("GITEX_HOME")
	if cfg.HomeDir == "" {
		dir, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("Failed to get home dir: %v", err)
		}
		cfg.HomeDir = filepath.Join(dir, ".gitex")
	}
	cfg.BinDir = filepath.Join(cfg.HomeDir, "bin")
}
