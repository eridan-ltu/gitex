# gitex

AI-powered code review for your pull requests. Point it at a PR, get inline comments.

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
- [Codex CLI](https://github.com/openai/codex) installed
- API tokens for your git host and OpenAI

## How it works

1. Parses the PR URL to figure out the project and MR
2. Clones the source branch to a temp directory
3. Runs `git diff` against the target branch
4. Sends the diff to Codex with review instructions
5. Parses the response and posts inline comments

The AI is prompted to trace code paths and gather evidence before flagging something. It classifies issues as definite, possible, or safe - and only comments when there's a real concern.

## Contributing

PRs welcome. The codebase is pretty small - `internal/` has the services, `api/` has the interfaces.

## License

MIT
