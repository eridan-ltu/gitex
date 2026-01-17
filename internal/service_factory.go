package internal

import (
	"fmt"
	"github.com/eridan-ltu/gitex/api"
	"github.com/go-git/go-git/v6/plumbing/transport/http"
	"net/url"
	"strings"
)

type AIAgentType string
type VersionControlType string
type RemoteGitServiceType string

const AIAgentTypeCodex AIAgentType = "codex"
const VersionControlTypeGit VersionControlType = "git"
const RemoteGitServiceTypeGitLab RemoteGitServiceType = "gitlab"
const RemoteGitServiceTypeGitHub RemoteGitServiceType = "github"
const RemoteGitServiceTypeUnknown RemoteGitServiceType = "unknown"

type ServiceFactory struct {
	cfg *api.Config
}

func NewServiceFactory(cfg *api.Config) *ServiceFactory {
	return &ServiceFactory{
		cfg: cfg,
	}
}

func (a *ServiceFactory) CreateAiAgentService(kind AIAgentType) (api.AIAgentService, error) {
	switch kind {
	case AIAgentTypeCodex:
		return NewCodexService(a.cfg), nil
	default:
		return nil, fmt.Errorf("unsupported AI agent type: %s", kind)
	}
}

func (a *ServiceFactory) CreateVersionControlService(kind VersionControlType) (api.VersionControlService, error) {
	switch kind {
	case VersionControlTypeGit:
		return NewGitService(&http.BasicAuth{
			Username: "oauth",
			Password: a.cfg.VcsApiKey,
		}), nil
	default:
		return nil, fmt.Errorf("unsupported version control type: %s", kind)
	}
}

func (a *ServiceFactory) CreateRemoteGitService(kind RemoteGitServiceType) (api.RemoteGitService, error) {
	switch kind {
	case RemoteGitServiceTypeGitLab:
		return NewGitLabService(a.cfg)
	default:
		return nil, fmt.Errorf("unsupported remote git service: %s", kind)
	}
}

func (a *ServiceFactory) DetectRemoteGitServiceType(rawURL string) (RemoteGitServiceType, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse url %s: %v", rawURL, err)
	}

	host := strings.ToLower(u.Host)
	path := strings.ToLower(u.Path)

	if host == "github.com" {
		return RemoteGitServiceTypeGitHub, nil
	}

	if strings.Contains(path, "/-/merge_requests/") {
		return RemoteGitServiceTypeGitLab, nil
	}

	if strings.Contains(path, "/pull/") {
		return RemoteGitServiceTypeGitHub, nil
	}
	return RemoteGitServiceTypeUnknown, nil
}
