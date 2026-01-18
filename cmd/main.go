package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/eridan-ltu/gitex/api"
	"github.com/eridan-ltu/gitex/internal"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

func main() {

	mrUrl, cfg := parseInput()

	serviceFactory := internal.NewServiceFactory(cfg)

	vcsProviderType, err := serviceFactory.DetectVCSProviderType(mrUrl)
	if err != nil {
		log.Fatalf("Failed to detect VCS provider type: %v", err)
	}
	log.Printf("VCS provider type: %s\n", vcsProviderType)

	vcsProviderService, err := serviceFactory.CreateVCSProvider(vcsProviderType)
	if err != nil {
		log.Fatalf("Failed to create VCS provider service: %v", err)
	}

	prInfo, err := vcsProviderService.GetPullRequestInfo(&mrUrl)
	if err != nil {
		log.Fatal(err)
	}

	tempDir, err := os.MkdirTemp("", prInfo.ProjectName+"-*")
	if err != nil {
		log.Fatalf("Failed to create temp directory: %v", err)
	}

	defer func() {
		if err = os.RemoveAll(tempDir); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Failed to cleanup directory %s: %v\n", tempDir, err)
		}
	}()

	gitService, err := serviceFactory.CreateVersionControlService(internal.VCSTypeGit)
	if err != nil {
		log.Fatalf("Failed to create version control service: %v", err)
	}

	cloneCtx, cloneCancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cloneCancel()
	err = gitService.CloneRepoWithContext(cloneCtx, tempDir, prInfo.ProjectHttpUrl, prInfo.SourceBranch)
	if err != nil {
		log.Fatalf("Failed to clone repo: %v", err)
	}
	log.Printf("Successfully cloned repo: %s\n", prInfo.ProjectName)

	aiAgent, err := serviceFactory.CreateAiAgentService(internal.AIAgentTypeCodex)
	if err != nil {
		log.Fatalf("Failed to create agent service: %v", err)
	}
	ctx, cancelFunc := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancelFunc()
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		fmt.Println("Killing AiAgent command:", sig)
		cancelFunc()
	}()
	log.Printf("Starting PR analysis at %s\n", prInfo.SourceBranch)
	comments, err := aiAgent.GeneratePRInlineCommentsWithContext(ctx, &api.GeneratePRInlineCommentsOptions{
		SandBoxDir: tempDir,
		BaseSha:    prInfo.BaseSha,
		StartSha:   prInfo.StartSha,
		HeadSha:    prInfo.HeadSha,
	})
	if err != nil {
		log.Fatalf("Failed to generate inline comments: %v", err)
	}

	log.Println("Pushing comments to VCS")
	if err = vcsProviderService.SendInlineComments(comments, prInfo); err != nil {
		log.Printf("Warning: %v", err)
	}
	log.Printf("Finished PR analysis at %s\n", prInfo.SourceBranch)

}

func parseInput() (string, *api.Config) {
	if len(os.Args) < 2 {
		log.Fatalf("Usage: codex-gitlab <merge-request-url> [flags]\n")
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

	err := fs.Parse(os.Args[2:])
	if err != nil {
		log.Fatalf("Failed to parse flags: %v", err)
	}

	populateFromEnvIfNecessary(cfg)

	return mrUrl, cfg
}

func populateFromEnvIfNecessary(cfg *api.Config) {
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
