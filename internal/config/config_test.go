package config

import (
	"flag"
	"path/filepath"
	"testing"
)

func TestParseDefaults(t *testing.T) {
	t.Setenv("LAZY_SKILLS_ROOT", "")
	t.Setenv("LAZY_SKILLS_LOG_LEVEL", "")
	t.Setenv("LAZY_SKILLS_MAX_RESULTS", "")
	t.Setenv("LAZY_SKILLS_MAX_RESULTS_MAX", "")
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	Flags(fs)
	cfg, err := Parse(fs, nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cfg.Root == DefaultRoot {
		// DefaultRoot contains a literal ~; Parse must expand it.
		t.Errorf("Root not expanded: %q", cfg.Root)
	}
	if cfg.MaxResults != defaultMax {
		t.Errorf("MaxResults = %d, want %d", cfg.MaxResults, defaultMax)
	}
	if cfg.MaxResultsMax != defaultMaxMX {
		t.Errorf("MaxResultsMax = %d, want %d", cfg.MaxResultsMax, defaultMaxMX)
	}
	if cfg.ScriptTimeout != 30e9 {
		t.Errorf("ScriptTimeout = %v, want 30s", cfg.ScriptTimeout)
	}
	if len(cfg.ScriptExts) != 7 {
		t.Errorf("ScriptExts len = %d, want 7", len(cfg.ScriptExts))
	}
}

func TestParseEnvOverridesDefaults(t *testing.T) {
	root := t.TempDir()
	t.Setenv("LAZY_SKILLS_ROOT", root)
	t.Setenv("LAZY_SKILLS_MAX_RESULTS", "17")
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	Flags(fs)
	cfg, err := Parse(fs, nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cfg.Root != root {
		t.Errorf("Root = %q, want %q", cfg.Root, root)
	}
	if cfg.MaxResults != 17 {
		t.Errorf("MaxResults = %d, want 17", cfg.MaxResults)
	}
}

func TestParseFlagOverridesEnv(t *testing.T) {
	envRoot := t.TempDir()
	flagRoot := t.TempDir()
	t.Setenv("LAZY_SKILLS_ROOT", envRoot)
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	Flags(fs)
	cfg, err := Parse(fs, []string{"--root", flagRoot})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cfg.Root != flagRoot {
		t.Errorf("Root = %q, want %q", cfg.Root, flagRoot)
	}
}

func TestParseClampsMaxResultsToMax(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	Flags(fs)
	cfg, err := Parse(fs, []string{"--max-results", "500"})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cfg.MaxResults != cfg.MaxResultsMax {
		t.Errorf("MaxResults = %d, want clamped to %d", cfg.MaxResults, cfg.MaxResultsMax)
	}
}

func TestExpandHome(t *testing.T) {
	got := expandHome("~/foo/bar")
	if filepath.IsAbs(got) && filepath.Base(filepath.Dir(got)) != "foo" {
		t.Errorf("expandHome(~/) = %q, want an expanded absolute path", got)
	}
	got = expandHome("/plain/path")
	if got != "/plain/path" {
		t.Errorf("expandHome(absolute) = %q, want unchanged", got)
	}
}
