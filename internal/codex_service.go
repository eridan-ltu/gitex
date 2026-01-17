package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"gitex/api"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

const commentsFileName = "comments.codex"

type CodexService struct {
	cfg           *api.Config
	commandRunner func(ctx context.Context, name string, args ...string) *exec.Cmd
}

func NewCodexService(cfg *api.Config) *CodexService {
	return &CodexService{
		cfg:           cfg,
		commandRunner: exec.CommandContext,
	}
}

func (c *CodexService) GeneratePRInlineComments(options *api.GeneratePRInlineCommentsOptions) ([]*api.InlineComment, error) {
	return c.GeneratePRInlineCommentsWithContext(context.Background(), options)
}

func (c *CodexService) GeneratePRInlineCommentsWithContext(ctx context.Context, options *api.GeneratePRInlineCommentsOptions) ([]*api.InlineComment, error) {
	defer func() {
		commentsFilePath := filepath.Join(options.SandBoxDir, commentsFileName)
		_ = os.Remove(commentsFilePath)
	}()
	cmd := c.commandRunner(
		ctx,
		"codex",
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
			`, options.BaseSha, options.BaseSha, options.StartSha, options.HeadSha, commentsFileName),
	)
	cmd.Dir = options.SandBoxDir
	if c.cfg.Verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		devNull, err := os.OpenFile("/dev/null", os.O_WRONLY, 0)
		if err != nil {
			log.Fatalf("Failed to open /dev/null: %v", err)
		}
		defer func(devNull *os.File) {
			_ = devNull.Close()
		}(devNull)
		cmd.Stdout = devNull
		cmd.Stderr = devNull
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
