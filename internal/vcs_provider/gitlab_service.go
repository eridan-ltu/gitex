package vcs_provider

import (
	"fmt"
	"github.com/eridan-ltu/gitex/api"
	gitlab "gitlab.com/gitlab-org/api/client-go"
	"log"
	"net/url"
	"strconv"
	"strings"
)

type GitLabService struct {
	client *gitlab.Client
}

func NewGitLabService(cfg *api.Config) (*GitLabService, error) {
	clientOptionFunc := gitlab.WithBaseURL(cfg.VcsRemoteUrl)
	client, err := gitlab.NewClient(cfg.VcsApiKey, clientOptionFunc)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitLab client: %w", err)
	}
	return &GitLabService{
		client: client,
	}, nil
}

func (g *GitLabService) GetPullRequestInfo(pullRequestURL *string) (*api.PullRequestInfo, error) {
	projectPath, mrId, err := parseWebUrl(*pullRequestURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse merge request URL: %v", err)
	}
	mr, _, err := g.client.MergeRequests.GetMergeRequest(projectPath, int64(mrId), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get merge request: %v", err)
	}
	project, _, err := g.client.Projects.GetProject(mr.ProjectID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %v", err)
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

func (g *GitLabService) SendInlineComments(comments []*api.InlineComment, pullRequestInfo *api.PullRequestInfo) {
	for i := range comments {
		gitlabComment := convertApiComment(comments[i])
		_, _, err := g.client.Discussions.CreateMergeRequestDiscussion(pullRequestInfo.ProjectPath, pullRequestInfo.PullRequestId, gitlabComment)
		if err != nil {
			log.Printf("failed to create merge request discussion: %v", err)
		}
	}
}

func parseWebUrl(webUrl string) (string, int, error) {
	parsed, err := url.Parse(webUrl)
	if err != nil {
		return "", 0, fmt.Errorf("invalid URL: %w", err)
	}

	parts := strings.Split(parsed.Path, "/")
	if len(parts) < 6 {
		return "", 0, fmt.Errorf("invalid Merge Request URL format")
	}

	projectPath := strings.Join(parts[1:len(parts)-3], "/")
	mrIdStr := parts[len(parts)-1]

	mrId, err := strconv.Atoi(mrIdStr)
	if err != nil {
		return "", 0, err
	}

	projectId := projectPath

	return projectId, mrId, nil
}

func convertApiComment(comment *api.InlineComment) *gitlab.CreateMergeRequestDiscussionOptions {
	if comment == nil {
		return nil
	}
	return &gitlab.CreateMergeRequestDiscussionOptions{
		Body:      comment.Body,
		CommitID:  comment.CommitID,
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
