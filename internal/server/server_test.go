package server

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/shaddy/lazy-skills/internal/config"
	"github.com/shaddy/lazy-skills/internal/index"
)

func fixtureRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "skills")
}

func newTestServer(t *testing.T) *Server {
	t.Helper()
	cfg := config.Config{
		Root:          fixtureRoot(t),
		LogLevel:      "warn",
		MaxResults:    10,
		MaxResultsMax: 50,
		ScriptTimeout: 5 * time.Second,
		ScriptExts:    []string{".sh", ".py"},
	}
	idx, err := index.Build(index.Config{Root: cfg.Root, ScriptExts: cfg.ScriptExts})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return New(cfg, idx)
}

func hasCode(err error, code string) bool {
	return err != nil && strings.HasPrefix(err.Error(), code+":")
}

func TestListSkillsRoot(t *testing.T) {
	s := newTestServer(t)
	_, res, err := s.handleListSkills(context.Background(), nil, ListSkillsArgs{Path: ""})
	if err != nil {
		t.Fatalf("handleListSkills: %v", err)
	}
	if res.Self.Name != "root" {
		t.Errorf("self.Name = %q, want root", res.Self.Name)
	}
	if len(res.Categories) == 0 {
		t.Error("expected top-level categories")
	}
}

func TestListSkillsCategory(t *testing.T) {
	s := newTestServer(t)
	_, res, err := s.handleListSkills(context.Background(), nil, ListSkillsArgs{Path: "dev/golang"})
	if err != nil {
		t.Fatalf("handleListSkills: %v", err)
	}
	have := map[string]bool{}
	for _, sk := range res.Skills {
		have[sk.Path] = true
	}
	for _, c := range res.Categories {
		have[c.Path] = true
	}
	if !have["dev/golang/error-handling"] || !have["dev/golang/testing"] {
		t.Errorf("dev/golang children = %v, want error-handling and testing", have)
	}
}

func TestListSkillsNotFound(t *testing.T) {
	s := newTestServer(t)
	_, _, err := s.handleListSkills(context.Background(), nil, ListSkillsArgs{Path: "no/such"})
	if !hasCode(err, CodeNotFound) {
		t.Errorf("err = %v, want not_found", err)
	}
}

func TestGetSkill(t *testing.T) {
	s := newTestServer(t)
	_, res, err := s.handleGetSkill(context.Background(), nil, GetSkillArgs{Path: "dev/golang/error-handling"})
	if err != nil {
		t.Fatalf("handleGetSkill: %v", err)
	}
	if res.Name != "error-handling" {
		t.Errorf("Name = %q", res.Name)
	}
	if strings.TrimSpace(res.Content) != strings.TrimSpace("# Error Handling\n\nIdiomatic error handling in Go: wrap with %w, use errors.Is and errors.As,\nand define typed errors for domain rejections.\n\nUse goleak to avoid goroutine leak detection confusion.") {
		t.Errorf("Content mismatch:\n%q", res.Content)
	}
	if res.Frontmatter != nil {
		t.Error("Frontmatter should be nil by default")
	}
}

func TestGetSkillIncludeFrontmatter(t *testing.T) {
	s := newTestServer(t)
	_, res, err := s.handleGetSkill(context.Background(), nil, GetSkillArgs{Path: "dev/golang/error-handling", IncludeFrontmatter: true})
	if err != nil {
		t.Fatalf("handleGetSkill: %v", err)
	}
	if res.Frontmatter == nil {
		t.Fatal("expected Frontmatter")
	}
	if res.Frontmatter["name"] != "error-handling" {
		t.Errorf("frontmatter name = %v", res.Frontmatter["name"])
	}
	if len(res.DeclaredScripts) == 0 {
		t.Error("expected declared scripts")
	}
	if res.DeclaredScripts[0].Name != "lint.sh" {
		t.Errorf("DeclaredScripts[0] = %+v, want lint.sh", res.DeclaredScripts[0])
	}
}

func TestGetSkillNotFound(t *testing.T) {
	s := newTestServer(t)
	_, _, err := s.handleGetSkill(context.Background(), nil, GetSkillArgs{Path: "nope"})
	if !hasCode(err, CodeNotFound) {
		t.Errorf("err = %v, want not_found", err)
	}
}

func TestGetSkillNotSkill(t *testing.T) {
	s := newTestServer(t)
	_, _, err := s.handleGetSkill(context.Background(), nil, GetSkillArgs{Path: "dev/golang"})
	if !hasCode(err, CodeNotSkill) {
		t.Errorf("err = %v, want not_skill", err)
	}
}

func TestGetSkillEmptyPath(t *testing.T) {
	s := newTestServer(t)
	_, _, err := s.handleGetSkill(context.Background(), nil, GetSkillArgs{Path: ""})
	if !hasCode(err, CodeInvalidInput) {
		t.Errorf("err = %v, want invalid_input", err)
	}
}

func TestSearchSkills(t *testing.T) {
	s := newTestServer(t)
	_, res, err := s.handleSearchSkills(context.Background(), nil, SearchSkillsArgs{Query: "testing", IncludeBody: false})
	if err != nil {
		t.Fatalf("handleSearchSkills: %v", err)
	}
	if res.Total == 0 {
		t.Error("expected results for 'testing'")
	}
}

func TestSearchSkillsInvalidQuery(t *testing.T) {
	s := newTestServer(t)
	_, _, err := s.handleSearchSkills(context.Background(), nil, SearchSkillsArgs{Query: "   "})
	if !hasCode(err, CodeInvalidQuery) {
		t.Errorf("err = %v, want invalid_query", err)
	}
}

func TestSkillTree(t *testing.T) {
	s := newTestServer(t)
	_, res, err := s.handleSkillTree(context.Background(), nil, SkillTreeArgs{MaxDepth: 0})
	if err != nil {
		t.Fatalf("handleSkillTree: %v", err)
	}
	if res["type"] != "root" {
		t.Errorf("root type = %v, want root", res["type"])
	}
	if _, ok := res["children"].([]map[string]any); !ok {
		t.Errorf("expected tree children in %v", res)
	}
}

func TestListSkillFiles(t *testing.T) {
	s := newTestServer(t)
	_, res, err := s.handleListSkillFiles(context.Background(), nil, ListSkillFilesArgs{Path: "dev/golang/testing"})
	if err != nil {
		t.Fatalf("handleListSkillFiles: %v", err)
	}
	seen := map[string]bool{}
	for _, f := range res.Files {
		seen[f.Name] = true
	}
	if !seen["scripts/setup.sh"] {
		t.Error("expected scripts/setup.sh in files")
	}
	if !seen["references/setup-guide.md"] {
		t.Error("expected references/setup-guide.md in files")
	}
	if seen["SKILL.md"] {
		t.Error("SKILL.md should be excluded")
	}
	// verify is_reference flag on the reference file
	for _, f := range res.Files {
		if f.Name == "references/setup-guide.md" && !f.IsReference {
			t.Error("reference file should have IsReference true")
		}
	}
}

func TestGetSkillFile(t *testing.T) {
	s := newTestServer(t)
	_, res, err := s.handleGetSkillFile(context.Background(), nil, GetSkillFileArgs{Path: "dev/golang/testing", File: "references/setup-guide.md"})
	if err != nil {
		t.Fatalf("handleGetSkillFile: %v", err)
	}
	if strings.TrimSpace(res.Content) == "" {
		t.Error("expected file content")
	}
}

func TestGetSkillFileBinary(t *testing.T) {
	s := newTestServer(t)
	_, _, err := s.handleGetSkillFile(context.Background(), nil, GetSkillFileArgs{Path: "dev/golang/testing", File: "scripts/blob.bin"})
	if !hasCode(err, CodeBinaryFile) {
		t.Errorf("err = %v, want binary_file", err)
	}
}

func TestGetSkillFileTraversalRejected(t *testing.T) {
	s := newTestServer(t)
	for _, f := range []string{"../../outside.md", "/etc/passwd", "..", "a/../../b"} {
		_, _, err := s.handleGetSkillFile(context.Background(), nil, GetSkillFileArgs{Path: "dev/golang/testing", File: f})
		if !hasCode(err, CodeNotFound) {
			t.Errorf("file %q: err = %v, want not_found", f, err)
		}
	}
}

func TestExecuteSkillScript(t *testing.T) {
	s := newTestServer(t)
	_, res, err := s.handleExecuteSkillScript(context.Background(), nil, ExecuteSkillScriptArgs{Path: "dev/golang/testing", File: "scripts/setup.sh"})
	if err != nil {
		t.Fatalf("handleExecuteSkillScript: %v", err)
	}
	if res.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0 (stderr=%q)", res.ExitCode, res.Stderr)
	}
	if strings.TrimSpace(res.Stdout) != "Setting up dev environment...\nDone." {
		t.Errorf("Stdout = %q", res.Stdout)
	}
}

func TestExecuteSkillScriptNotAScript(t *testing.T) {
	s := newTestServer(t)
	_, _, err := s.handleExecuteSkillScript(context.Background(), nil, ExecuteSkillScriptArgs{Path: "dev/golang/testing", File: "references/setup-guide.md"})
	if !hasCode(err, CodeNotAScript) {
		t.Errorf("err = %v, want not_a_script", err)
	}
}

func TestExecuteSkillScriptTraversal(t *testing.T) {
	s := newTestServer(t)
	_, _, err := s.handleExecuteSkillScript(context.Background(), nil, ExecuteSkillScriptArgs{Path: "dev/golang/testing", File: "../../etc/passwd"})
	if !hasCode(err, CodeNotFound) {
		t.Errorf("err = %v, want not_found", err)
	}
}
