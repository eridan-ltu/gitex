package core

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"regexp"
	"syscall"
	"time"

	"github.com/eridan-ltu/gitex/api"
)

var sanitizeRegex = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

type App struct {
	factory ServiceFactoryInterface
	stdout  io.Writer
	stderr  io.Writer
}

func NewApp(factory ServiceFactoryInterface) *App {
	return &App{
		factory: factory,
		stdout:  os.Stdout,
		stderr:  os.Stderr,
	}
}

// NewAppWithWriters for testing purposes only for now
func NewAppWithWriters(factory ServiceFactoryInterface, stdout, stderr io.Writer) *App {
	return &App{
		factory: factory,
		stdout:  stdout,
		stderr:  stderr,
	}
}

func (a *App) Run(mrUrl string) error {
	vcsProviderType, err := a.factory.DetectVCSProviderType(mrUrl)
	if err != nil {
		return fmt.Errorf("failed to detect VCS provider type: %w", err)
	}
	if vcsProviderType == VCSProviderTypeUnknown {
		return fmt.Errorf("unsupported VCS provider for URL: %s", mrUrl)
	}
	_, _ = fmt.Fprintf(a.stdout, "VCS provider type: %s\n", vcsProviderType)

	vcsProviderService, err := a.factory.CreateVCSProvider(vcsProviderType)
	if err != nil {
		return fmt.Errorf("failed to create VCS provider service: %w", err)
	}

	prInfo, err := vcsProviderService.GetPullRequestInfo(&mrUrl)
	if err != nil {
		return fmt.Errorf("failed to get PR info: %w", err)
	}

	tempDir, err := os.MkdirTemp("", sanitizeProjectName(prInfo.ProjectName)+"-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			_, _ = fmt.Fprintf(a.stderr, "Failed to cleanup directory %s: %v\n", tempDir, err)
		}
	}()

	gitService, err := a.factory.CreateVersionControlService(VCSTypeGit)
	if err != nil {
		return fmt.Errorf("failed to create version control service: %w", err)
	}

	cloneCtx, cloneCancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cloneCancel()
	if err := gitService.CloneRepoWithContext(cloneCtx, tempDir, prInfo.ProjectHttpUrl, prInfo.SourceBranch); err != nil {
		return fmt.Errorf("failed to clone repo: %w", err)
	}
	_, _ = fmt.Fprintf(a.stdout, "Successfully cloned repo: %s\n", prInfo.ProjectName)

	aiAgent, err := a.factory.CreateAiAgentService(AIAgentTypeCodex)
	if err != nil {
		return fmt.Errorf("failed to create agent service: %w", err)
	}

	ctx, cancelFunc := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancelFunc()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case <-sigChan:
			cancelFunc()
		case <-ctx.Done():
		}
	}()
	defer signal.Stop(sigChan)

	_, _ = fmt.Fprintf(a.stdout, "Starting PR analysis at %s\n", prInfo.SourceBranch)
	comments, err := aiAgent.GeneratePRInlineCommentsWithContext(ctx, &api.GeneratePRInlineCommentsOptions{
		SandBoxDir: tempDir,
		BaseSha:    prInfo.BaseSha,
		StartSha:   prInfo.StartSha,
		HeadSha:    prInfo.HeadSha,
	})
	if err != nil {
		return fmt.Errorf("failed to generate inline comments: %w", err)
	}

	_, _ = fmt.Fprintln(a.stdout, "Pushing comments to VCS provider")
	if err := vcsProviderService.SendInlineComments(comments, prInfo); err != nil {
		_, _ = fmt.Fprintf(a.stderr, "Warning: %v\n", err)
	}
	_, _ = fmt.Fprintf(a.stdout, "Finished PR analysis at %s\n", prInfo.SourceBranch)
	return nil
}

func sanitizeProjectName(name string) string {
	sanitized := sanitizeRegex.ReplaceAllString(name, "_")
	if sanitized == "" {
		return "project"
	}
	return sanitized
}
