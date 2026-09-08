package executor

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeScript writes an executable script with the given extension into a
// temp skill dir and returns the dir.
func writeScript(t *testing.T, ext, body string) string {
	t.Helper()
	dir := t.TempDir()
	name := "script" + ext
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}
	return dir
}

func TestRunSuccess(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	dir := writeScript(t, ".sh", "#!/usr/bin/env bash\necho hello-from-script\n")
	r := NewRunner(dir, []string{".sh"}, 5*time.Second)
	res, err := r.Run(context.Background(), "script.sh", nil, 0)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", res.ExitCode)
	}
	if strings.TrimSpace(res.Stdout) != "hello-from-script" {
		t.Errorf("Stdout = %q", res.Stdout)
	}
	if res.TimedOut {
		t.Error("unexpected TimedOut")
	}
}

func TestRunExitCode(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	dir := writeScript(t, ".sh", "#!/usr/bin/env bash\nexit 42\n")
	r := NewRunner(dir, []string{".sh"}, 5*time.Second)
	res, err := r.Run(context.Background(), "script.sh", nil, 0)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.ExitCode != 42 {
		t.Errorf("ExitCode = %d, want 42", res.ExitCode)
	}
}

func TestRunArgsPassed(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	dir := writeScript(t, ".sh", "#!/usr/bin/env bash\necho \"got:$1\"\n")
	r := NewRunner(dir, []string{".sh"}, 5*time.Second)
	res, err := r.Run(context.Background(), "script.sh", []string{"alpha"}, 0)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if strings.TrimSpace(res.Stdout) != "got:alpha" {
		t.Errorf("Stdout = %q, want got:alpha", res.Stdout)
	}
}

func TestRunNotAScript(t *testing.T) {
	dir := writeScript(t, ".txt", "just text")
	r := NewRunner(dir, []string{".sh"}, 5*time.Second)
	_, err := r.Run(context.Background(), "script.txt", nil, 0)
	if err != ErrNotAScript {
		t.Fatalf("err = %v, want ErrNotAScript", err)
	}
}

func TestRunMissingInterpreter(t *testing.T) {
	dir := writeScript(t, ".sh", "#!/usr/bin/env bash\necho hi\n")
	r := NewRunner(dir, []string{".sh"}, 5*time.Second)
	r.lookPath = func(name string) (string, error) {
		t.Helper()
		return "", errors.New("command not found: " + name)
	}
	_, err := r.Run(context.Background(), "script.sh", nil, 0)
	var mie *MissingInterpreterError
	if !errors.As(err, &mie) {
		t.Fatalf("err = %v, want MissingInterpreterError", err)
	}
}

func TestRunTimeout(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	dir := writeScript(t, ".sh", "#!/usr/bin/env bash\nsleep 5\necho done\n")
	r := NewRunner(dir, []string{".sh"}, 5*time.Second)
	res, err := r.Run(context.Background(), "script.sh", nil, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.TimedOut {
		t.Error("expected TimedOut")
	}
	if res.ExitCode != -1 {
		t.Errorf("ExitCode = %d, want -1 on timeout", res.ExitCode)
	}
}

func TestInterpreterDispatch(t *testing.T) {
	cases := []struct{ ext, want string }{
		{".sh", "bash"}, {".bash", "bash"}, {".py", "python3"},
		{".rb", "ruby"}, {".js", "node"}, {".ts", "tsx"}, {".pl", "perl"},
	}
	for _, tc := range cases {
		meta, ok := KnownScripts[tc.ext]
		if !ok {
			t.Errorf("KnownScripts missing %q", tc.ext)
			continue
		}
		if meta.Interpreter != tc.want {
			t.Errorf("interpreter(%s) = %q, want %q", tc.ext, meta.Interpreter, tc.want)
		}
	}
}
