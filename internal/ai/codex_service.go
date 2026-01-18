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
			1. you are in the PR mode. consider git diff %s..HEAD.
			2. a. **Identify suspicious code**  
				   - Focus on the variable, method, or expression that might cause a problem.  
				   - Note its type and where it comes from.
				
				b. **Gather evidence**  
				   - Look at the declaration, type, annotations, or language-specific hints (e.g., @Nullable, Optional, const, final) to understand possible values.  
				   - Check documentation, comments, and tests for intended behavior
				   - Trace all assignments, return values, and constructor calls leading to this code.
				
				c. **Analyze context and code paths**  
				   - Examine control flow: consider conditions, loops, early returns, or side effects.  
				   - Look at external factors like database values, API calls, or user input that might influence the value.
				
				d. **Classify the risk**  
				   - **Definite:** evidence shows a problem can occur.  
				   - **Possible:** could occur under rare or edge cases.  
				   - **Impossible:** evidence shows the code is safe.
				
				e. **Explain reasoning step by step**  
				   - For each classification, provide the path, assignments, types, or documentation that led to the conclusion.  
				   - Avoid vague statements like “might be null” without proof.
				
				f. **Suggest evidence-based actions (optional)**  
				   - Only suggest fixes or guards if the analysis shows a real risk.  
				   - Examples: add null check, refactor initialization, add test coverage.
			3. generate inline notes for changes in this format
			[{
			"body": <YOUR_COMMENT>,
			"commit_id": "%s"
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
	          }
			}]'
	        use line_range instead of new_line and old_line if comment belongs to a range of lines.
			when there is a single line comment, use new_line or old_line (depends on where the change was), omit line_range.
	        store json inside %s commentsFile.
	        4. Generate overall review inside review.codex commentsFile as plain text.
	        5. Do not include the findings you specified in the inline comments
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
