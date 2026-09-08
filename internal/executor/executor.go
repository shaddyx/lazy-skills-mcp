// Package executor runs skill scripts: it resolves the script path inside a
// skill directory, selects an interpreter by extension, and enforces a timeout.
package executor

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ScriptMeta describes a bundled script's extension and interpreter.
type ScriptMeta struct {
	Ext         string
	Interpreter string
}

// KnownScripts maps an allowed extension to its interpreter.
var KnownScripts = map[string]ScriptMeta{
	".sh":   {Ext: ".sh", Interpreter: "bash"},
	".bash": {Ext: ".bash", Interpreter: "bash"},
	".py":   {Ext: ".py", Interpreter: "python3"},
	".rb":   {Ext: ".rb", Interpreter: "ruby"},
	".js":   {Ext: ".js", Interpreter: "node"},
	".ts":   {Ext: ".ts", Interpreter: "tsx"},
	".pl":   {Ext: ".pl", Interpreter: "perl"},
}

// ErrNotAScript is returned when a file has a non-allowed extension.
var ErrNotAScript = errors.New("not an allowed script extension")

// Runner executes a single script and returns its result.
type Runner struct {
	exts     map[string]bool
	timeout  time.Duration
	workdir  string
	lookPath func(string) (string, error)
}

// NewRunner builds a Runner. workdir is the skill directory; allowedExts must
// be the lowercase allowed script extensions (e.g. ".sh"); timeout is the
// default per-execution timeout.
func NewRunner(workdir string, allowedExts []string, timeout time.Duration) *Runner {
	exts := make(map[string]bool, len(allowedExts))
	for _, e := range allowedExts {
		exts[e] = true
	}
	return &Runner{
		exts:     exts,
		timeout:  timeout,
		workdir:  workdir,
		lookPath: exec.LookPath,
	}
}

// Result is the outcome of running a script.
type Result struct {
	Path      string
	File      string
	ExitCode  int
	Stdout    string
	Stderr    string
	TimedOut  bool
	Duration  time.Duration
	InterpErr error
}

// Run executes the script at relPath (relative to workdir). If timeoutArg is
// > 0 it overrides the runner default. The interpreter for the extension is
// looked up on PATH; a missing interpreter returns ErrMissingInterpreter.
func (r *Runner) Run(ctx context.Context, relPath string, args []string, timeoutArg time.Duration) (*Result, error) {
	abs := filepath.Join(r.workdir, relPath)
	if err := r.checkExt(relPath); err != nil {
		return nil, err
	}

	meta, ok := KnownScripts[extOf(relPath)]
	if !ok {
		return nil, ErrNotAScript
	}

	interpPath, err := r.lookPath(meta.Interpreter)
	if err != nil {
		return nil, &MissingInterpreterError{Interpreter: meta.Interpreter}
	}

	to := r.timeout
	if timeoutArg > 0 {
		to = timeoutArg
	}
	runCtx, cancel := context.WithTimeout(ctx, to)
	defer cancel()

	cmd := exec.CommandContext(runCtx, interpPath, append([]string{abs}, args...)...)
	cmd.Dir = r.workdir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	runErr := cmd.Run()
	duration := time.Since(start)

	res := &Result{
		Path:     r.workdir,
		File:     relPath,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: duration,
	}
	if runCtx.Err() == context.DeadlineExceeded {
		res.TimedOut = true
		res.ExitCode = -1
		return res, nil
	}
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			res.ExitCode = exitErr.ExitCode()
			return res, nil
		}
		return nil, runErr
	}
	res.ExitCode = 0
	return res, nil
}

// MissingInterpreterError reports a required interpreter that is not on PATH.
type MissingInterpreterError struct {
	Interpreter string
}

func (e *MissingInterpreterError) Error() string {
	return "required interpreter " + e.Interpreter + " not found on PATH"
}

func (r *Runner) checkExt(relPath string) error {
	if !r.exts[extOf(relPath)] {
		return ErrNotAScript
	}
	return nil
}

func extOf(relPath string) string {
	return strings.ToLower(filepath.Ext(relPath))
}
