package skill

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSkill(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "SKILL.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

func TestParseFullFrontmatter(t *testing.T) {
	path := writeSkill(t, `---
name: my-skill
description: A skill.
tags: [foo, bar]
version: 2.1
---

# Heading
Body line.
`)
	sk, err := Parse(path, "dir-name", "dev/my-skill")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if sk.Name != "my-skill" {
		t.Errorf("Name = %q, want my-skill", sk.Name)
	}
	if sk.Description != "A skill." {
		t.Errorf("Description = %q, want A skill.", sk.Description)
	}
	if len(sk.Tags) != 2 || sk.Tags[0] != "foo" {
		t.Errorf("Tags = %v, want [foo bar]", sk.Tags)
	}
	if sk.Version != "2.1" {
		t.Errorf("Version = %q, want 2.1", sk.Version)
	}
	if sk.Path != "dev/my-skill" {
		t.Errorf("Path = %q", sk.Path)
	}
	if sk.Content != "\n# Heading\nBody line.\n" {
		t.Errorf("Content = %q", sk.Content)
	}
}

func TestParseNameFallbackToDir(t *testing.T) {
	path := writeSkill(t, `---
description: No name here.
tags: [x]
---

Body.
`)
	sk, err := Parse(path, "dir-name", "p")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if sk.Name != "dir-name" {
		t.Errorf("Name = %q, want dir-name", sk.Name)
	}
}

func TestParseDescriptionFallbackToFirstBodyLine(t *testing.T) {
	path := writeSkill(t, "---\nname: x\n---\n\nFirst meaningful body line.\nSecond.\n")
	sk, err := Parse(path, "dir", "p")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if sk.Description != "First meaningful body line." {
		t.Errorf("Description = %q, want first body line", sk.Description)
	}
}

func TestParseNoFrontmatter(t *testing.T) {
	path := writeSkill(t, "# Just Body\n\nSomething.\n")
	sk, err := Parse(path, "dir", "p")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if sk.Name != "dir" {
		t.Errorf("Name = %q, want dir", sk.Name)
	}
	if strings.TrimSpace(sk.Content) != strings.TrimSpace("# Just Body\n\nSomething.") {
		t.Errorf("Content = %q", sk.Content)
	}
}

func TestParseMalformedFrontmatterSoftFails(t *testing.T) {
	path := writeSkill(t, "---\nname: a\n  bad: indent\n---\nBody.\n")
	sk, err := Parse(path, "dir", "p")
	if err == nil || !errors.Is(err, ErrBadFrontmatter) {
		t.Fatalf("err = %v, want ErrBadFrontmatter", err)
	}
	if sk == nil {
		t.Fatal("expected a usable Skill alongside ErrBadFrontmatter")
	}
	if sk.Name != "dir" {
		t.Errorf("Name = %q, want dir (soft-fail fallback)", sk.Name)
	}
}

func TestDeclaredScripts(t *testing.T) {
	fm := map[string]any{
		"scripts": []any{
			map[string]any{"name": "a.sh", "description": "Runs A"},
			map[string]any{"name": "b.py"},
			map[string]any{"description": "no name"},
		},
	}
	got := DeclaredScripts(fm)
	if len(got) != 2 {
		t.Fatalf("DeclaredScripts len = %d, want 2", len(got))
	}
	if got[0].Name != "a.sh" || got[0].Description != "Runs A" {
		t.Errorf("got[0] = %+v", got[0])
	}
	if got[1].Name != "b.py" || got[1].Description != "" {
		t.Errorf("got[1] = %+v", got[1])
	}
}
