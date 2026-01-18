package vcs_provider

import (
	"context"
	"errors"
	"fmt"
	"github.com/eridan-ltu/gitex/api"
	"github.com/eridan-ltu/gitex/internal/util"
	"github.com/google/go-github/v81/github"
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type GitHubService struct {
	client *github.Client
}

func NewGitHubService(cfg *api.Config) (*GitHubService, error) {
	client := github.NewClient(nil).WithAuthToken(cfg.VcsApiKey)
	if cfg.VcsRemoteUrl != "" {
		uploadUrl := strings.TrimRight(cfg.VcsRemoteUrl, "/") + "/uploads"
		enterpriseClient, err := client.WithEnterpriseURLs(cfg.VcsRemoteUrl, uploadUrl)
		if err != nil {
			return nil, fmt.Errorf("error initializing enterprise github client: %w", err)
		}
		client = enterpriseClient
	}
	return &GitHubService{
		client: client,
	}, nil
}

func (g *GitHubService) GetPullRequestInfo(pullRequestURL *string) (*api.PullRequestInfo, error) {
	owner, repo, number, err := g.parseWebUrl(*pullRequestURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pull request URL: %v", err)
	}
	ctx, cancelFunc := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancelFunc()

	pr, _, err := g.client.PullRequests.Get(ctx, owner, repo, number)
	if err != nil {
		return nil, fmt.Errorf("failed to get merge request: %v", err)
	}
	cloneUrl := pr.Head.Repo.GetCloneURL()
	projectName := pr.Base.Repo.GetName() // pr is created against base project

	return &api.PullRequestInfo{
		HeadSha:        pr.Head.GetSHA(),
		BaseSha:        pr.Base.GetSHA(),
		ProjectName:    projectName,
		ProjectHttpUrl: cloneUrl,
		ProjectId:      pr.Base.Repo.GetID(), //should not be used
		SourceBranch:   pr.Head.GetRef(),
		PullRequestId:  int64(pr.GetNumber()), //github accepts pr number instead of internal id
		Owner:          pr.Base.Repo.GetOwner().GetLogin(),
	}, nil
}

func (g *GitHubService) SendInlineComments(comments []*api.InlineComment, pullRequestInfo *api.PullRequestInfo) error {
	failed := util.WithRetry(comments, 3, func(comment *api.InlineComment) error {
		githubComment := g.convertApiComment(comment)
		if githubComment == nil {
			return nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, resp, err := g.client.PullRequests.CreateComment(ctx, pullRequestInfo.Owner, pullRequestInfo.ProjectName, int(pullRequestInfo.PullRequestId), githubComment)
		if resp != nil && resp.Rate.Remaining < 10 {
			sleepDuration := time.Until(resp.Rate.Reset.Time) + time.Second
			if sleepDuration > 0 {
				log.Printf("rate limit low, sleeping for %v", sleepDuration)
				time.Sleep(sleepDuration)
			}
		}
		return err
	})
	if len(failed) > 0 {
		return fmt.Errorf("failed to send %d comments after retries", len(failed))
	}
	return nil
}

func (g *GitHubService) parseWebUrl(webUrl string) (string, string, int, error) {
	u, err := url.Parse(webUrl)
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to parse URL: %v", err)
	}

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")

	if len(parts) != 4 || parts[2] != "pull" {
		return "", "", 0, errors.New("failed to parse URL")
	}

	owner := parts[0]
	repo := parts[1]
	number := parts[3]

	num, err := strconv.Atoi(number)
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to parse pull request number: %v", err)
	}

	return owner, repo, num, nil
}

func (g *GitHubService) convertApiComment(
	in *api.InlineComment,
) *github.PullRequestComment {

	if in == nil || in.Position == nil {
		return nil
	}

	out := &github.PullRequestComment{
		Body:     in.Body,
		CommitID: in.CommitID,
	}

	pos := in.Position

	if pos.NewPath != nil {
		out.Path = pos.NewPath
	} else if pos.OldPath != nil {
		out.Path = pos.OldPath
	}

	switch {
	case pos.NewLine != nil:
		out.Line = util.Ptr(int(*pos.NewLine))
		out.Side = util.Ptr("RIGHT")

	case pos.OldLine != nil:
		out.Line = util.Ptr(int(*pos.OldLine))
		out.Side = util.Ptr("LEFT")
	}

	if pos.LineRange != nil {
		if pos.LineRange.Start != nil {
			if pos.LineRange.Start.NewLine != nil {
				out.StartLine = util.Ptr(int(*pos.LineRange.Start.NewLine))
				out.StartSide = util.Ptr("RIGHT")
			} else if pos.LineRange.Start.OldLine != nil {
				out.StartLine = util.Ptr(int(*pos.LineRange.Start.OldLine))

				out.StartSide = util.Ptr("LEFT")
			}
		}
	}

	return out
}
