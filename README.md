<h1 align="center">gitex</h1>

<p align="center">
  <a href="https://github.com/eridan-ltu/gitex/actions/workflows/ci.yml"><img src="https://github.com/eridan-ltu/gitex/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://github.com/eridan-ltu/gitex/releases"><img src="https://img.shields.io/github/v/release/eridan-ltu/gitex" alt="Release"></a>
  <a href="https://goreportcard.com/report/github.com/eridan-ltu/gitex"><img src="https://goreportcard.com/badge/github.com/eridan-ltu/gitex" alt="Go Report Card"></a>
  <a href="https://codecov.io/gh/eridan-ltu/gitex"><img src="https://codecov.io/gh/eridan-ltu/gitex/branch/main/graph/badge.svg" alt="Coverage"></a>
  <img src="https://img.shields.io/github/go-mod/go-version/eridan-ltu/gitex" alt="Go Version">
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="License: MIT"></a>
</p>

<p align="center">AI-powered code review for your pull requests. Point it at a PR, get inline comments.</p>

## What it does

```bash
gitex https://gitlab.com/myorg/project/-/merge_requests/42
```

That's it. The tool clones the branch, analyzes the diff with Codex, and posts comments directly on the PR. It looks for real issues - null pointer risks, type mismatches, unhandled edge cases - not formatting stuff.

Works with GitLab and GitHub.

## Quick start

```bash
# Set your tokens
export VCS_API_KEY=glpat-xxxxxxxxxxxx    # GitLab/GitHub token
export AI_API_KEY=sk-xxxxxxxxxxxx         # OpenAI key

# Run it
gitex https://gitlab.com/yourorg/yourproject/-/merge_requests/123
# or
gitex https://github.com/yourorg/yourproject/pull/123
```

## Installation

**From source:**

```bash
git clone https://github.com/eridan-ltu/gitex.git
cd gitex
go build -o gitex
```

**Or with go install:**

```bash
go install github.com/eridan-ltu/gitex@latest
```

## Options

```
gitex <pr-url> [flags]

Flags:
  -vcs-api-key     VCS API key (or use VCS_API_KEY env)
  -vcs-url         VCS provider URL (for self-hosted instances)
  -ai-model        Model to use (default: gpt-5.1-codex-mini)
  -ai-api-key      OpenAI key (or use AI_API_KEY env)
  -verbose         Show what the AI is doing
```

## Requirements

- Go 1.25+
- Node.js/npm (Codex CLI is installed automatically)
- API tokens for your git host and OpenAI

## How it works

1. Parses the PR URL to figure out the project and MR
2. Clones the source branch to a temp directory
3. Installs Codex CLI if needed (`npm i @openai/codex@0.87.0` into `~/.gitex/bin`)
4. Runs `git diff` against the target branch
5. Sends the diff to Codex with review instructions
6. Parses the response and posts inline comments (with retry on failure)

Codex runs sandboxed with its own home directory (`~/.gitex/.codex`) to avoid conflicts with your local Codex config.

The AI is prompted to trace code paths and gather evidence before flagging something. It classifies issues as definite, possible, or safe - and only comments when there's a real concern.

## Roadmap

- Claude support (coming soon)

## Contributing

PRs welcome. The codebase is pretty small - `internal/` has the services, `api/` has the interfaces.

## License

MIT
