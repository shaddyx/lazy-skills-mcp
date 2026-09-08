package server

import (
	"context"
	"errors"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shaddy/lazy-skills/internal/executor"
)

// ExecuteSkillScriptArgs are the arguments to execute_skill_script.
type ExecuteSkillScriptArgs struct {
	Path    string   `json:"path"`
	File    string   `json:"file"`
	Args    []string `json:"args,omitempty"`
	Timeout int      `json:"timeout"`
}

// ExecuteSkillScriptResult is the output of execute_skill_script.
type ExecuteSkillScriptResult struct {
	Path       string `json:"path"`
	File       string `json:"file"`
	ExitCode   int    `json:"exit_code"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	TimedOut   bool   `json:"timed_out"`
	DurationMS int64  `json:"duration_ms"`
}

func (s *Server) handleExecuteSkillScript(ctx context.Context, req *mcp.CallToolRequest, in ExecuteSkillScriptArgs) (*mcp.CallToolResult, ExecuteSkillScriptResult, error) {
	if in.Path == "" || in.File == "" {
		return nil, ExecuteSkillScriptResult{}, errCode(CodeInvalidInput, "path and file are required")
	}
	absDir, err := s.skillDir(in.Path)
	if err != nil {
		return nil, ExecuteSkillScriptResult{}, err
	}
	if !fileInside(absDir, in.File) {
		return nil, ExecuteSkillScriptResult{}, errCode(CodeNotFound, "file %q not found", in.File)
	}

	runner := executor.NewRunner(absDir, s.cfg.ScriptExts, s.cfg.ScriptTimeout)
	timeoutArg := time.Duration(in.Timeout) * time.Second
	if in.Timeout <= 0 {
		timeoutArg = 0
	}
	res, err := runner.Run(ctx, in.File, in.Args, timeoutArg)
	if err != nil {
		return nil, ExecuteSkillScriptResult{}, mapExecError(err)
	}
	out := ExecuteSkillScriptResult{
		Path:       in.Path,
		File:       in.File,
		ExitCode:   res.ExitCode,
		Stdout:     res.Stdout,
		Stderr:     res.Stderr,
		TimedOut:   res.TimedOut,
		DurationMS: res.Duration.Milliseconds(),
	}
	return nil, out, nil
}

func mapExecError(err error) error {
	if errors.Is(err, executor.ErrNotAScript) {
		return errCode(CodeNotAScript, "%v", err)
	}
	var mie *executor.MissingInterpreterError
	if errors.As(err, &mie) {
		return errCode(CodeMissingInterp, "%v", err)
	}
	return errCode(CodeExecutionFailed, "%v", err)
}

func fileInside(base, rel string) bool {
	_, ok := resolveFileInside(base, rel)
	return ok
}
