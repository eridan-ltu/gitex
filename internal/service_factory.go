package internal

import (
	"fmt"
	"github.com/eridan-ltu/gitex/api"
	"github.com/eridan-ltu/gitex/internal/ai"
	"github.com/eridan-ltu/gitex/internal/vcs"
	"github.com/eridan-ltu/gitex/internal/vcs_provider"
	"github.com/go-git/go-git/v6/plumbing/transport/http"
	"net/url"
	"strings"
)

const AIAgentTypeCodex api.AIAgentType = "codex"
const VCSTypeGit api.VersionControlType = "git"
const VCSProviderTypeGitlab api.VCSProviderType = "gitlab"
const VCSProviderTypeGithub api.VCSProviderType = "github"
const VCSProviderTypeUnknown api.VCSProviderType = "unknown"

type ServiceFactory struct {
	cfg *api.Config
}

func NewServiceFactory(cfg *api.Config) *ServiceFactory {
	return &ServiceFactory{
		cfg: cfg,
	}
}

func (a *ServiceFactory) CreateAiAgentService(kind api.AIAgentType) (api.AIAgentService, error) {
	switch kind {
	case AIAgentTypeCodex:
		codexService, err := ai.NewCodexService(a.cfg)
		if err != nil {
			return nil, fmt.Errorf("error creating CodexService: %w", err)
		}
		return codexService, nil
	default:
		return nil, fmt.Errorf("unsupported AI agent type: %s", kind)
	}
}

func (a *ServiceFactory) CreateVersionControlService(kind api.VersionControlType) (api.VersionControlService, error) {
	switch kind {
	case VCSTypeGit:
		return vcs.NewGitService(&http.BasicAuth{
			Username: "oauth",
			Password: a.cfg.VcsApiKey,
		}), nil
	default:
		return nil, fmt.Errorf("unsupported version control type: %s", kind)
	}
}

func (a *ServiceFactory) CreateVCSProvider(kind api.VCSProviderType) (api.RemoteGitService, error) {
	switch kind {
	case VCSProviderTypeGitlab:
		return vcs_provider.NewGitLabService(a.cfg)
	default:
		return nil, fmt.Errorf("unsupported remote git service: %s", kind)
	}
}

func (a *ServiceFactory) DetectVCSProviderType(rawURL string) (api.VCSProviderType, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse url %s: %v", rawURL, err)
	}

	host := strings.ToLower(u.Host)
	path := strings.ToLower(u.Path)

	if host == "github.com" {
		return VCSProviderTypeGithub, nil
	}

	if strings.Contains(path, "/-/merge_requests/") {
		return VCSProviderTypeGitlab, nil
	}

	if strings.Contains(path, "/pull/") {
		return VCSProviderTypeGithub, nil
	}
	return VCSProviderTypeUnknown, nil
}
