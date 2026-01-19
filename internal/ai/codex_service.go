package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/eridan-ltu/gitex/api"
	"github.com/eridan-ltu/gitex/internal/util"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const (
	commentsFileName = "comments.codex"
	codexVersion     = "0.87.0"
)

func isCodexInstalled(binDir string) bool {
	packageJsonPath := path.Join(binDir, "node_modules", "@openai", "codex", "package.json")

	data, err := os.ReadFile(packageJsonPath)
	if err != nil {
		return false
	}

	var pkg struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return false
	}

	return pkg.Version == codexVersion
}

type CodexService struct {
	cfg           *api.Config
	codexBinPath  string
	env           []string
	commandRunner func(ctx context.Context, name string, args ...string) *exec.Cmd
	loginRunner   func(ctx context.Context, apiKey, codexBinPath *string, env []string) error
	logoutRunner  func(ctx context.Context, codexBinPath *string, env []string) error
}

func NewCodexService(cfg *api.Config) (*CodexService, error) {
	if err := util.EnsureDirectoryWritable(cfg.BinDir); err != nil {
		return nil, fmt.Errorf("bin directory error: %w", err)
	}

	codexHomePath := path.Join(cfg.HomeDir, ".codex")
	if err := util.EnsureDirectoryWritable(codexHomePath); err != nil {
		return nil, fmt.Errorf("codex home directory error: %w", err)
	}

	if !isCodexInstalled(cfg.BinDir) {
		ctx, cancelFunc := context.WithTimeout(context.Background(), time.Minute)
		defer cancelFunc()

		if err := defaultInstallRunner(ctx, &cfg.BinDir); err != nil {
			return nil, fmt.Errorf("codex install error: %w", err)
		}
	}

	binPath := path.Join(cfg.BinDir, "/node_modules/.bin/codex")
	environment := os.Environ()
	environment = append(environment, "CODEX_HOME="+codexHomePath)

	return &CodexService{
		cfg:           cfg,
		codexBinPath:  binPath,
		env:           environment,
		commandRunner: exec.CommandContext,
		loginRunner:   defaultLoginRunner,
		logoutRunner:  defaultLogoutRunner,
	}, nil
}

func defaultLoginRunner(ctx context.Context, apiKey, codexBinPath *string, env []string) error {
	loginCmd := exec.CommandContext(ctx, *codexBinPath, "login", "--with-api-key")
	loginCmd.Stdin = strings.NewReader(*apiKey)
	loginCmd.Env = env

	return loginCmd.Run()
}

func defaultLogoutRunner(ctx context.Context, codexBinPath *string, env []string) error {
	logoutCmd := exec.CommandContext(ctx, *codexBinPath, "logout")
	logoutCmd.Env = env
	return logoutCmd.Run()
}

func defaultInstallRunner(ctx context.Context, binDir *string) error {
	command := exec.CommandContext(ctx, "npm", "i", "@openai/codex@"+codexVersion, "--prefix", *binDir)
	return command.Run()
}

func (c *CodexService) GeneratePRInlineComments(options *api.GeneratePRInlineCommentsOptions) ([]*api.InlineComment, error) {
	return c.GeneratePRInlineCommentsWithContext(context.Background(), options)
}

func (c *CodexService) GeneratePRInlineCommentsWithContext(ctx context.Context, options *api.GeneratePRInlineCommentsOptions) ([]*api.InlineComment, error) {

	defer func() {
		commentsFilePath := filepath.Join(options.SandBoxDir, commentsFileName)
		_ = os.Remove(commentsFilePath)
	}()

	if err := c.loginRunner(ctx, &c.cfg.AiApiKey, &c.codexBinPath, c.env); err != nil {
		return nil, fmt.Errorf("codex login failed: %w", err)
	}

	defer func() {
		_ = c.logoutRunner(ctx, &c.codexBinPath, c.env)
	}()

	cmd := c.commandRunner(
		ctx,
		c.codexBinPath,
		"exec",
		"-s", "workspace-write",
		"--model", c.cfg.AiModel,
		fmt.Sprintf(`
			You are an AI code reviewer. You need to git diff %s..HEAD. You need to analyze the diff and to generate inline comments strictly in the following JSON format:

				RULES:
				
				ABSOLUTE CONSTRAINTS (MUST FOLLOW)
				- You may ONLY reference line numbers that explicitly appear in the diff hunks
				- Line numbers come only from @@ -<old>,<count> +<new>,<count> @@
				- You must compute per-line numbers by incrementing from the hunk header
				- If a line number cannot be derived with certainty, DO NOT COMMENT
				- DO NOT GUESS OR INFER LINE NUMBERS
				- Do NOT assume continuity across hunks
				- Do NOT reuse numbers from examples
				- Do NOT comment on context outside the diff
				- If you cannot place a valid comment with exact line numbers, SKIP it
				
				Single-line comments:
				- Omit position[line_range].
				- Include both old_path and new_path.
				- Added line: use position[new_line] only, omit old_line.
				- Removed line: use position[old_line] only, omit new_line.
				- Unchanged line: include both old_line and new_line, using diff-provided line numbers.
                - ***Do not guess; line numbers may differ due to previous changes. ***
				- Set comment_type = SINGLE_LINE.
				- Set line_type = ADD, REMOVE, or UNCHANGED.
				
                Multi-line comments:
				- Use position[line_range] to indicate the start and end of the comment.
				- Line numbers in start and end follow the same rules as single-line comments:
				- Added lines: fill only new_line
				- Removed lines: fill only old_line
				- Unchanged lines: fill both old_line and new_line
				- line_type = ADD, REMOVE, or UNCHANGED depending on the type of lines being commented.
                - Set comment_type = MULTI_LINE.
				
				Content
				- Comment only on lines present in the diff.
				- Each comment should be a meaningful suggestion, improvement, or note.
				- Always reference the exact line numbers from the diff. Never guess the lines
				
				Output
				- JSON must follow this schema:
				
				[{
				  "body": "<YOUR_COMMENT>",
				  "commit_id": "%s",
				  "position": {
					"position_type": "text",
					"base_sha": "%s",
					"start_sha": "%s",
					"head_sha": "%s",
					"old_path": "<OLD_FILE_PATH>",
					"new_path": "<NEW_FILE_PATH>",
					"new_line": <LINE_TO_COMMENT_IF_COMMENT_BELONGS_TO_NEW_LINE>,
					"old_line": <LINE_TO_COMMENT_IF_COMMENT_BELONGS_TO_OLD_LINE>,
					"line_range": {
					  "start": {
						"new_line": <LINE_TO_COMMENT_IF_COMMENT_BELONGS_TO_NEW_LINE>,
						"old_line": <LINE_TO_COMMENT_IF_COMMENT_BELONGS_TO_OLD_LINE>
					  },
					  "end": {
						"new_line": <LINE_TO_COMMENT_IF_COMMENT_BELONGS_TO_NEW_LINE>,
						"old_line": <LINE_TO_COMMENT_IF_COMMENT_BELONGS_TO_OLD_LINE>
					  }
					},
					"comment_type": "SINGLE_LINE" | "MULTI_LINE",
					"line_type": "ADD"|"REMOVE"|"UNCHANGED"
				  }
				}]
				
				Examples
				1. Single-line added
					@@ -41,3 +41,4 @@
					 func add(a int, b int) int {
					-    return a - b
					+    return a + b
					}
				[{
				  "body": "Corrected addition here.",
				  "commit_id": "commitId",
				  "position": {
					"position_type": "text",
					"base_sha": "baseSha",
					"start_sha": "startSha",
					"head_sha": "headSha",
					"old_path": "math.go",
					"new_path": "math.go",
					"new_line": 42,
					"comment_type": "SINGLE_LINE",
					"line_type": "ADD"
				  }
				}]
				
				2. Single-line removed
					@@ -40,5 +40,4 @@
					 func add(a int, b int) int {
					-    return a - b
					+    return a + b
					}
				[{
				  "body": "This line was incorrectly subtracting.",
				  "commit_id": "commitId",
				  "position": {
					"position_type": "text",
					"base_sha": "baseSha",
					"start_sha": "startSha",
					"head_sha": "headSha",
					"old_path": "math.go",
					"new_path": "math.go",
					"old_line": 42,
					"comment_type": "SINGLE_LINE",
					"line_type": "REMOVE"
				  }
				}]
				
				3. Single-line unchanged

					@@ -30,6 +30,8 @@
					func multiply(a int, b int) int {
						 result := a * b
					+    fmt.Println("Debug start")   # line 34 in old file? added in new file
					+    log.Println("Debug info")    # line 35 in new file
						 return a / b
					}

				[{
				  "body": "This line is unchanged but review for debug code.",
				  "commit_id": "commitId",
				  "position": {
					"position_type": "text",
					"base_sha": "baseSha",
					"start_sha": "startSha",
					"head_sha": "headSha",
					"old_path": "math.go",
					"new_path": "math.go",
					"old_line": 34,
					"new_line": 36,
					"comment_type": "SINGLE_LINE",
					"line_type": "UNCHANGED"
				  }
				}]
				
				4. Multi-line added

					@@ -11,2 +11,5 @@
					-    result := a * b
					-    return result
					+    result := a * b
					+    if result < 0 {
					+        result = 0
					+    }
					+    return result

				[{
				  "body": "Adding a guard for negative results; review logic.",
				  "commit_id": "commitId",
				  "position": {
					"position_type": "text",
					"base_sha": "baseSha",
					"start_sha": "startSha",
					"head_sha": "headSha",
					"old_path": "math.go",
					"new_path": "math.go",
					"line_range": {
					  "start": { "new_line": 11 },
					  "end": { "new_line": 14 }
					},
					"comment_type": "MULTI_LINE",
					"line_type": "ADD"
				  }
				}]
			verify json validity(escape special characters).
	        store json inside %s commentsFile.
	        4. Generate summary review inside review.codex commentsFile as plain text.
	        5. In the summary do not include the findings you specified in the inline comments
			6. DO NOT PUSH ANY CHANGES.
			`, options.BaseSha, options.HeadSha, options.BaseSha, options.StartSha, options.HeadSha, commentsFileName),
	)

	cmd.Env = c.env
	cmd.Dir = options.SandBoxDir

	if c.cfg.Verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stdout = nil
		cmd.Stderr = nil
	}

	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("error generating PR inline-comments: %w", err)
	}
	commentsFile, err := os.ReadFile(filepath.Join(options.SandBoxDir, commentsFileName))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("error reading comments file: %w", err)
	}

	var comments []*api.InlineComment
	err = json.Unmarshal(commentsFile, &comments)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling comments file: %w", err)
	}
	return comments, nil
}
