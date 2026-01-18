<h1 align="center">gitex</h1>

<p align="center">
  <a href="https://github.com/eridan-ltu/gitex/actions/workflows/ci.yml"><img src="https://github.com/eridan-ltu/gitex/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://github.com/eridan-ltu/gitex/releases"><img src="https://img.shields.io/github/v/release/eridan-ltu/gitex" alt="Release"></a>
  <a href="https://goreportcard.com/report/github.com/eridan-ltu/gitex"><img src="https://goreportcard.com/badge/github.com/eridan-ltu/gitex" alt="Go Report Card"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="License: MIT"></a>
</p>

<p align="center">AI-powered code review for your pull requests. Point it at a PR, get inline comments.</p>

## What it does

```bash
gitex https://gitlab.com/myorg/project/-/merge_requests/42
```

That's it. The tool clones the branch, analyzes the diff with Codex, and posts comments directly on the PR. It looks for real issues - null pointer risks, type mismatches, unhandled edge cases - not formatting stuff.

Works with GitLab today, GitHub support coming soon.

## Quick start

```bash
# Set your tokens
export VCS_API_KEY=glpat-xxxxxxxxxxxx    # GitLab token
export AI_API_KEY=sk-xxxxxxxxxxxx         # OpenAI key

# Run it
gitex https://gitlab.com/yourorg/yourproject/-/merge_requests/123
```

## Installation

**From source:**

```bash
git clone https://github.com/user/gitex.git
cd gitex
go build -o gitex ./cmd
```

**Or with go install:**

```bash
go install github.com/user/gitex/cmd@latest
```

## Options

```
gitex <pr-url> [flags]

Flags:
  -api-key         VCS API key (or use VCS_API_KEY env)
  -gitlab-url      GitLab URL (default: https://gitlab.com)
  -codex-model     Model to use (default: gpt-5.1-codex-mini)
  -codex-api-key   OpenAI key (or use AI_API_KEY env)
  -verbose         Show what the AI is doing
```

## Requirements

- Go 1.25+
- Node.js/npm (Codex CLI is installed automatically)
- API tokens for your git host and OpenAI

## How it works

1. Parses the PR URL to figure out the project and MR
2. Clones the source branch to a temp directory
3. Installs Codex CLI if needed (checks version, skips if already installed)
4. Runs `git diff` against the target branch
5. Sends the diff to Codex with review instructions
6. Parses the response and posts inline comments

Codex runs sandboxed with its own home directory (`~/.gitex/.codex`) to avoid conflicts with your local Codex config.

The AI is prompted to trace code paths and gather evidence before flagging something. It classifies issues as definite, possible, or safe - and only comments when there's a real concern.

## Contributing

PRs welcome. The codebase is pretty small - `internal/` has the services, `api/` has the interfaces.

## License

MIT
