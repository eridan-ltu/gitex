package vcs_provider

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/eridan-ltu/gitex/api"
	"github.com/eridan-ltu/gitex/internal/util"
	"github.com/google/go-github/v81/github"
	"github.com/hashicorp/go-retryablehttp"
)

type GitHubService struct {
	client *github.Client
}

func NewGitHubService(cfg *api.Config) (*GitHubService, error) {
	retryClient := retryablehttp.NewClient()
	retryClient.RetryMax = 3
	retryClient.Logger = nil
	retryClient.CheckRetry = RetryPolicy
	retryClient.ErrorHandler = retryablehttp.PassthroughErrorHandler

	httpClient := retryClient.StandardClient()

	client := github.NewClient(httpClient).WithAuthToken(cfg.VcsApiKey)
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
		return nil, fmt.Errorf("failed to parse pull request URL: %w", err)
	}
	ctx, cancelFunc := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancelFunc()

	pr, _, err := g.client.PullRequests.Get(ctx, owner, repo, number)
	if err != nil {
		return nil, fmt.Errorf("failed to get merge request: %w", err)
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
	var failedCount int

	for _, comment := range comments {
		githubComment := g.convertApiComment(comment)
		if githubComment == nil {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		_, _, err := g.client.PullRequests.CreateComment(ctx, pullRequestInfo.Owner, pullRequestInfo.ProjectName, int(pullRequestInfo.PullRequestId), githubComment)
		cancel()

		if err != nil {
			g.logGithubError(githubComment, err)
			failedCount++
		}
	}

	if failedCount > 0 {
		return fmt.Errorf("failed to send %d comments", failedCount)
	}
	return nil
}

func (g *GitHubService) logGithubError(githubComment *github.PullRequestComment, err error) {
	path := util.GetOrDefault(githubComment.Path, "unknown")
	line := util.GetOrDefaultInt(githubComment.Line, 0)

	var ghErr *github.ErrorResponse
	if errors.As(err, &ghErr) {
		log.Printf("failed to create comment on %s:%d: %s (status %d)",
			path, line, ghErr.Message, ghErr.Response.StatusCode)
		for _, e := range ghErr.Errors {
			log.Printf("  - %s.%s: %s (%s)", e.Resource, e.Field, e.Message, e.Code)
		}
	} else {
		log.Printf("failed to create comment on %s:%d: %v", path, line, err)
	}
}

func (g *GitHubService) parseWebUrl(webUrl string) (string, string, int, error) {
	u, err := url.Parse(webUrl)
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to parse URL: %w", err)
	}

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")

	if len(parts) < 4 || parts[2] != "pull" {
		return "", "", 0, errors.New("failed to parse URL")
	}

	owner := parts[0]
	repo := parts[1]
	number := parts[3]

	num, err := strconv.Atoi(number)
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to parse pull request number: %w", err)
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

	//multiline
	if pos.CommentType == "MULTI_LINE" && pos.LineRange != nil && pos.LineRange.Start != nil && pos.LineRange.End != nil {
		// FYI: Line = end, StartLine = start
		if pos.LineRange.End.NewLine != nil {
			out.Line = util.Ptr(int(*pos.LineRange.End.NewLine))
			out.Side = util.Ptr("RIGHT")
		} else if pos.LineRange.End.OldLine != nil {
			out.Line = util.Ptr(int(*pos.LineRange.End.OldLine))
			out.Side = util.Ptr("LEFT")
		}

		if pos.LineRange.Start.NewLine != nil {
			out.StartLine = util.Ptr(int(*pos.LineRange.Start.NewLine))
			out.StartSide = util.Ptr("RIGHT")
		} else if pos.LineRange.Start.OldLine != nil {
			out.StartLine = util.Ptr(int(*pos.LineRange.Start.OldLine))
			out.StartSide = util.Ptr("LEFT")
		}
	} else { //single line
		switch {
		case pos.NewLine != nil:
			out.Line = util.Ptr(int(*pos.NewLine))
			out.Side = util.Ptr("RIGHT")
		case pos.OldLine != nil:
			out.Line = util.Ptr(int(*pos.OldLine))
			out.Side = util.Ptr("LEFT")
		}
	}

	return out
}
