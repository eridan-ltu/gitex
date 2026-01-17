package internal

import (
	"context"
	"fmt"
	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/transport/http"
)

type GitService struct {
	auth http.AuthMethod
}

func NewGitService(auth http.AuthMethod) *GitService {
	return &GitService{
		auth: auth,
	}
}

func (s *GitService) CloneRepo(path, repoUrl, ref string) error {
	return s.CloneRepoWithContext(context.Background(), path, repoUrl, ref)
}

func (s *GitService) CloneRepoWithContext(ctx context.Context, path, repoUrl, ref string) error {

	_, err := git.PlainCloneContext(ctx, path, &git.CloneOptions{
		URL:           repoUrl,
		Auth:          s.auth,
		ReferenceName: plumbing.ReferenceName(ref),
		SingleBranch:  true,
	})
	if err != nil {
		return fmt.Errorf("error clone repo: %w", err)
	}
	return nil
}
