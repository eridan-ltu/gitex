package api

import (
	"context"
	"time"
)

type RemoteGitService interface {
	GetPullRequestInfo(pullRequestURL *string) (*PullRequestInfo, error)
	SendInlineComments(comments []*InlineComment, pullRequestInfo *PullRequestInfo)
}

type Config struct {
	VcsApiKey    string
	VcsRemoteUrl string
	AiModel      string
	AiApiKey     string
	Verbose      bool
	CI           bool
}
type GeneratePRInlineCommentsOptions struct {
	SandBoxDir, BaseSha, StartSha, HeadSha string
}

type AIAgentService interface {
	GeneratePRInlineComments(options *GeneratePRInlineCommentsOptions) ([]*InlineComment, error)
	GeneratePRInlineCommentsWithContext(ctx context.Context, options *GeneratePRInlineCommentsOptions) ([]*InlineComment, error)
}

type VersionControlService interface {
	CloneRepo(path, repoUrl, ref string) error
	CloneRepoWithContext(ctx context.Context, path, repoUrl, ref string) error
}

type InlineComment struct {
	Body      *string                `url:"body,omitempty" json:"body,omitempty"`
	CommitID  *string                `url:"commit_id,omitempty" json:"commit_id,omitempty"`
	CreatedAt *time.Time             `url:"created_at,omitempty" json:"created_at,omitempty"`
	Position  *InlineCommentPosition `url:"position,omitempty" json:"position,omitempty"`
}

type InlineCommentPosition struct {
	BaseSha      *string           `url:"base_sha,omitempty" json:"base_sha,omitempty"`
	HeadSha      *string           `url:"head_sha,omitempty" json:"head_sha,omitempty"`
	StartSha     *string           `url:"start_sha,omitempty" json:"start_sha,omitempty"`
	NewPath      *string           `url:"new_path,omitempty" json:"new_path,omitempty"`
	OldPath      *string           `url:"old_path,omitempty" json:"old_path,omitempty"`
	PositionType *string           `url:"position_type,omitempty" json:"position_type"`
	NewLine      *int64            `url:"new_line,omitempty" json:"new_line,omitempty"`
	OldLine      *int64            `url:"old_line,omitempty" json:"old_line,omitempty"`
	LineRange    *LineRangeOptions `url:"line_range,omitempty" json:"line_range,omitempty"`
	Width        *int64            `url:"width,omitempty" json:"width,omitempty"`
	Height       *int64            `url:"height,omitempty" json:"height,omitempty"`
	X            *float64          `url:"x,omitempty" json:"x,omitempty"`
	Y            *float64          `url:"y,omitempty" json:"y,omitempty"`
}

type LineRangeOptions struct {
	Start *LinePositionOptions `url:"start,omitempty" json:"start,omitempty"`
	End   *LinePositionOptions `url:"end,omitempty" json:"end,omitempty"`
}

type LinePositionOptions struct {
	LineCode *string `url:"line_code,omitempty" json:"line_code,omitempty"`
	Type     *string `url:"type,omitempty" json:"type,omitempty"`
	OldLine  *int64  `url:"old_line,omitempty" json:"old_line,omitempty"`
	NewLine  *int64  `url:"new_line,omitempty" json:"new_line,omitempty"`
}
type PullRequestInfo struct {
	ProjectName    string `json:"name"`
	BaseSha        string `json:"base_sha"`
	StartSha       string `json:"start_sha"`
	HeadSha        string `json:"head_sha"`
	ProjectHttpUrl string `json:"project_http_url"`
	SourceBranch   string `json:"source_branch"`
	TargetBranch   string `json:"target_branch"`
	ProjectId      int64  `json:"project_id"`
	ProjectPath    string `json:"project_path"`
	PullRequestId  int64  `json:"pull_request_id"`
}
