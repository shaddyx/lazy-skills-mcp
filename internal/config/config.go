// Package config resolves the lazy-skill MCP server configuration from CLI
// flags and environment variables (flag > env > default).
package config

import (
	"flag"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config holds the fully-resolved server configuration.
type Config struct {
	Root           string
	LogLevel       string
	MaxResults     int
	MaxResultsMax  int
	ScriptTimeout  time.Duration
	ScriptExts     []string
	AllowEmptyRoot bool
}

const (
	// DefaultRoot is the skill catalog root when none is configured.
	DefaultRoot = "~/.agents/lazy-skills"

	defaultLog   = "info"
	defaultMax   = 10
	defaultMaxMX = 50
)

var defaultScriptExts = []string{".sh", ".bash", ".py", ".rb", ".js", ".ts", ".pl"}

// Parse resolves a Config from the given args. The flags must already be
// registered on flagSet (see Flags). Priority: flag > env > default.
func Parse(flagSet *flag.FlagSet, args []string) (Config, error) {
	if !flagSet.Parsed() {
		if err := flagSet.Parse(args); err != nil {
			return Config{}, err
		}
	}

	set := make(map[string]bool)
	flagSet.Visit(func(f *flag.Flag) { set[f.Name] = true })

	cfg := Config{
		Root:           value(flagSet, set, "root", envStr("LAZY_SKILLS_ROOT", DefaultRoot)),
		LogLevel:       value(flagSet, set, "loglevel", envStr("LAZY_SKILLS_LOG_LEVEL", defaultLog)),
		MaxResults:     valueInt(flagSet, set, "max-results", envInt("LAZY_SKILLS_MAX_RESULTS", defaultMax)),
		MaxResultsMax:  valueInt(flagSet, set, "max-results-max", envInt("LAZY_SKILLS_MAX_RESULTS_MAX", defaultMaxMX)),
		ScriptTimeout:  envDuration("LAZY_SKILLS_SCRIPT_TIMEOUT", 30*time.Second),
		ScriptExts:     envExts("LAZY_SKILLS_SCRIPT_EXTS", defaultScriptExts),
		AllowEmptyRoot: valueBool(flagSet, set, "allow-empty-root", false),
	}
	cfg.Root = expandHome(cfg.Root)

	if cfg.MaxResultsMax < 1 {
		cfg.MaxResultsMax = defaultMaxMX
	}
	if cfg.MaxResults < 1 {
		cfg.MaxResults = defaultMax
	}
	if cfg.MaxResults > cfg.MaxResultsMax {
		cfg.MaxResults = cfg.MaxResultsMax
	}

	return cfg, nil
}

// Flags registers the lazy-skill command line flags on flagSet.
func Flags(flagSet *flag.FlagSet) {
	flagSet.String("root", "", "skill catalog root (default ~/.agents/lazy-skills)")
	flagSet.String("loglevel", "", "log level: debug, info, warn, error")
	flagSet.Int("max-results", 0, "default search limit")
	flagSet.Int("max-results-max", 0, "hard cap on search limit")
	flagSet.Bool("allow-empty-root", false, "start even if the root is empty/missing")
}

func value(flagSet *flag.FlagSet, set map[string]bool, name, envVal string) string {
	if set[name] {
		return flagSet.Lookup(name).Value.String()
	}
	return envVal
}

func valueInt(flagSet *flag.FlagSet, set map[string]bool, name string, envVal int) int {
	if set[name] {
		if n, err := strconv.Atoi(flagSet.Lookup(name).Value.String()); err == nil {
			return n
		}
	}
	return envVal
}

func valueBool(flagSet *flag.FlagSet, set map[string]bool, name string, envVal bool) bool {
	if set[name] {
		if v, err := strconv.ParseBool(flagSet.Lookup(name).Value.String()); err == nil {
			return v
		}
	}
	return envVal
}

func envStr(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
	}
	return def
}

func envExts(key string, def []string) []string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		parts := strings.Split(v, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if t := strings.ToLower(strings.TrimSpace(p)); t != "" {
				out = append(out, t)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return def
}

func expandHome(p string) string {
	if p == "~" {
		h, _ := os.UserHomeDir()
		return h
	}
	if strings.HasPrefix(p, "~/") {
		h, _ := os.UserHomeDir()
		return filepath.Join(h, p[2:])
	}
	return p
}
