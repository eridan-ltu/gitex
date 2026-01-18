package vcs_provider

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"

	"github.com/eridan-ltu/gitex/api"
	"github.com/eridan-ltu/gitex/internal/util"
	"github.com/hashicorp/go-retryablehttp"
	gitlab "gitlab.com/gitlab-org/api/client-go"
)

type GitLabService struct {
	client *gitlab.Client
}

func NewGitLabService(cfg *api.Config) (*GitLabService, error) {
	baseUrl := util.GetOrDefault(&cfg.VcsRemoteUrl, "https://gitlab.com/")

	client, err := gitlab.NewClient(
		cfg.VcsApiKey,
		gitlab.WithBaseURL(baseUrl),
		gitlab.WithCustomRetry(RetryPolicy),
		gitlab.WithCustomRetryMax(3),
		gitlab.WithErrorHandler(retryablehttp.PassthroughErrorHandler),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitLab client: %w", err)
	}
	return &GitLabService{
		client: client,
	}, nil
}

func (g *GitLabService) GetPullRequestInfo(pullRequestURL *string) (*api.PullRequestInfo, error) {
	projectPath, mrId, err := g.parseWebUrl(*pullRequestURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse merge request URL: %w", err)
	}
	mr, _, err := g.client.MergeRequests.GetMergeRequest(projectPath, int64(mrId), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get merge request: %w", err)
	}
	project, _, err := g.client.Projects.GetProject(mr.ProjectID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	return &api.PullRequestInfo{
		HeadSha:        mr.DiffRefs.HeadSha,
		BaseSha:        mr.DiffRefs.BaseSha,
		StartSha:       mr.DiffRefs.StartSha,
		ProjectName:    project.Name,
		ProjectHttpUrl: project.HTTPURLToRepo,
		ProjectId:      project.ID,
		SourceBranch:   mr.SourceBranch,
		TargetBranch:   mr.TargetBranch,
		ProjectPath:    project.PathWithNamespace,
		PullRequestId:  mr.IID,
	}, nil
}

func (g *GitLabService) SendInlineComments(comments []*api.InlineComment, pullRequestInfo *api.PullRequestInfo) error {
	var failedCount int

	for _, comment := range comments {
		gitlabComment := convertApiComment(comment)
		if gitlabComment == nil {
			continue
		}

		_, _, err := g.client.Discussions.CreateMergeRequestDiscussion(pullRequestInfo.ProjectPath, pullRequestInfo.PullRequestId, gitlabComment)
		if err != nil {
			path := "unknown"
			var line int64
			if gitlabComment.Position != nil {
				path = util.GetOrDefault(gitlabComment.Position.NewPath, util.GetOrDefault(gitlabComment.Position.OldPath, "unknown"))
				if gitlabComment.Position.NewLine != nil {
					line = *gitlabComment.Position.NewLine
				} else if gitlabComment.Position.OldLine != nil {
					line = *gitlabComment.Position.OldLine
				}
			}

			g.logGitlabError(err, path, line)
			failedCount++
		}
	}

	if failedCount > 0 {
		return fmt.Errorf("failed to send %d comments", failedCount)
	}
	return nil
}

func (g *GitLabService) logGitlabError(err error, path string, line int64) {
	var glErr *gitlab.ErrorResponse
	if errors.As(err, &glErr) {
		log.Printf("failed to create comment on %s:%d: %s (status %d)",
			path, line, glErr.Message, glErr.Response.StatusCode)
	} else {
		log.Printf("failed to create comment on %s:%d: %v", path, line, err)
	}
}

func (g *GitLabService) parseWebUrl(webUrl string) (string, int, error) {
	parsed, err := url.Parse(webUrl)
	if err != nil {
		return "", 0, fmt.Errorf("invalid URL: %w", err)
	}

	parts := strings.Split(parsed.Path, "/")

	mrIndex := -1
	for i, p := range parts {
		if p == "merge_requests" {
			mrIndex = i
			break
		}
	}

	if mrIndex == -1 || mrIndex+1 >= len(parts) {
		return "", 0, errors.New("invalid Merge Request URL format")
	}

	projectPath := strings.Join(parts[1:mrIndex-1], "/")
	mrIdStr := parts[mrIndex+1]

	mrId, err := strconv.Atoi(mrIdStr)
	if err != nil {
		return "", 0, err
	}

	return projectPath, mrId, nil
}

func convertApiComment(comment *api.InlineComment) *gitlab.CreateMergeRequestDiscussionOptions {
	if comment == nil {
		return nil
	}
	return &gitlab.CreateMergeRequestDiscussionOptions{
		Body:      comment.Body,
		CreatedAt: comment.CreatedAt,
		Position:  convertInlineCommentPosition(comment.Position),
	}
}

func convertInlineCommentPosition(p *api.InlineCommentPosition) *gitlab.PositionOptions {
	if p == nil {
		return nil
	}

	return &gitlab.PositionOptions{
		BaseSHA:      p.BaseSha,
		HeadSHA:      p.HeadSha,
		StartSHA:     p.StartSha,
		NewPath:      p.NewPath,
		OldPath:      p.OldPath,
		PositionType: p.PositionType,
		NewLine:      p.NewLine,
		OldLine:      p.OldLine,
		LineRange:    convertInlineLineRange(p.LineRange),
		Width:        p.Width,
		Height:       p.Height,
		X:            p.X,
		Y:            p.Y,
	}
}

func convertInlineLineRange(r *api.LineRangeOptions) *gitlab.LineRangeOptions {
	if r == nil {
		return nil
	}

	return &gitlab.LineRangeOptions{
		Start: convertInlineLinePosition(r.Start),
		End:   convertInlineLinePosition(r.End),
	}
}

func convertInlineLinePosition(p *api.LinePositionOptions) *gitlab.LinePositionOptions {
	if p == nil {
		return nil
	}

	return &gitlab.LinePositionOptions{
		LineCode: p.LineCode,
		Type:     p.Type,
		OldLine:  p.OldLine,
		NewLine:  p.NewLine,
	}
}
