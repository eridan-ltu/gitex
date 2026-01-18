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
	projectName := pr.Head.Repo.GetName()

	return &api.PullRequestInfo{
		HeadSha:        pr.Head.GetSHA(),
		ProjectName:    projectName,
		ProjectHttpUrl: cloneUrl,
		ProjectId:      pr.Head.Repo.GetID(),
		SourceBranch:   pr.Head.GetRef(),
		PullRequestId:  pr.GetID(),
		Owner:          owner,
	}, nil
}

func (g *GitHubService) SendInlineComments(comments []*api.InlineComment, pullRequestInfo *api.PullRequestInfo) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	for i := range comments {
		githubComment := g.convertApiComment(comments[i])
		_, _, err := g.client.PullRequests.CreateComment(ctx, pullRequestInfo.Owner, pullRequestInfo.ProjectName, int(pullRequestInfo.PullRequestId), githubComment)
		if err != nil {
			log.Printf("failed to create pull request comment: %v", err)
		}
	}
}

func (g *GitHubService) parseWebUrl(webUrl string) (string, string, int, error) {
	u, err := url.Parse(webUrl)
	if err != nil {
		return "", "", 0, err
	}

	if u.Host != "github.com" {
		return "", "", 0, errors.New("not a github.com URL")
	}

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")

	if len(parts) != 4 || parts[2] != "pull" {
		return "", "", 0, errors.New("invalid GitHub pull request URL format")
	}

	owner := parts[0]
	repo := parts[1]
	number := parts[3]

	num, err := strconv.Atoi(number)
	if err != nil {
		return "", "", 0, errors.New("pull request number is not numeric")
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

	if pos.PositionType != nil {
		out.SubjectType = pos.PositionType
	}

	return out
}
