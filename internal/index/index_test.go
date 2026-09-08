package index

import (
	"path/filepath"
	"runtime"
	"testing"
)

// fixtureRoot returns the absolute path to testdata/skills regardless of the
// test working directory.
func fixtureRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "skills")
}

func buildFixture(t *testing.T) *Index {
	t.Helper()
	idx, err := Build(Config{Root: fixtureRoot(t), ScriptExts: []string{".sh", ".py"}})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return idx
}

func TestBuildCount(t *testing.T) {
	idx := buildFixture(t)
	if idx.Count() == 0 {
		t.Error("expected at least one skill")
	}
}

func TestBuildFindsExpectedSkills(t *testing.T) {
	idx := buildFixture(t)
	for _, p := range []string{
		"dev/golang/error-handling",
		"dev/golang/testing",
		"dev/web/responsive-design",
		"research/web-search",
	} {
		if idx.ByPath(p) == nil {
			t.Errorf("expected skill at %q", p)
		}
	}
}

func TestByPathMissing(t *testing.T) {
	idx := buildFixture(t)
	if idx.ByPath("dev/does-not-exist") != nil {
		t.Error("expected nil for missing skill")
	}
}

func TestMissingRootHardErrors(t *testing.T) {
	_, err := Build(Config{Root: "/nonexistent/definitely/missing"})
	if err == nil {
		t.Fatal("expected error for missing root")
	}
}

func TestAllowEmptyRoot(t *testing.T) {
	idx, err := Build(Config{Root: "/nonexistent/definitely/missing", AllowEmptyRoot: true})
	if err != nil {
		t.Fatalf("Build with AllowEmptyRoot: %v", err)
	}
	if idx.Count() != 0 {
		t.Errorf("Count = %d, want 0", idx.Count())
	}
}

func TestResolveConfinesPaths(t *testing.T) {
	idx := buildFixture(t)
	root := idx.Root()

	// valid path resolves inside root
	abs, err := idx.Resolve("dev/golang/testing")
	if err != nil {
		t.Fatalf("Resolve valid: %v", err)
	}
	if !isWithin(root, abs) {
		t.Errorf("Resolve returned path outside root: %q", abs)
	}

	for _, bad := range []string{"..", "../etc/passwd", "/etc/passwd", ".", "dev/../golang"} {
		if abs, err := idx.Resolve(bad); err == nil {
			t.Errorf("Resolve(%q) = %q, want error", bad, abs)
		}
	}
}

func TestResolveEmptyReturnsRoot(t *testing.T) {
	idx := buildFixture(t)
	abs, err := idx.Resolve("")
	if err != nil {
		t.Fatalf("Resolve(\"\"): %v", err)
	}
	if abs != idx.Root() {
		t.Errorf("Resolve empty = %q, want root %q", abs, idx.Root())
	}
}
