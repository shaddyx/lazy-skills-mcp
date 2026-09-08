// Package skill models a single skill: its path, frontmatter metadata, and
// SKILL.md body, plus parsing of the YAML frontmatter that precedes the body.
package skill

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ErrBadFrontmatter reports a Soft-fail: the frontmatter block could not be
// parsed, but the file is still usable with directory-name fallbacks.
var ErrBadFrontmatter = errors.New("malformed frontmatter")

// Skill is a loaded SKILL.md plus its resolved metadata.
type Skill struct {
	Path        string
	Name        string
	Description string
	Tags        []string
	Version     string
	FrontMatter map[string]any
	Content     string
	ModTime     time.Time
}

// ScriptMeta describes a declared script in frontmatter (informational only;
// not required for execution).
type ScriptMeta struct {
	Name        string
	Description string
}

// DeclaredScripts returns the scripts declared in the frontmatter.
func DeclaredScripts(fm map[string]any) []ScriptMeta {
	raw, ok := fm["scripts"]
	if !ok {
		return nil
	}
	var out []ScriptMeta
	switch t := raw.(type) {
	case []any:
		for _, e := range t {
			m, ok := e.(map[string]any)
			if !ok {
				continue
			}
			name, _ := m["name"].(string)
			if name == "" {
				continue
			}
			desc, _ := m["description"].(string)
			out = append(out, ScriptMeta{Name: name, Description: desc})
		}
	case string:
		for _, s := range strings.Split(t, ",") {
			if s = strings.TrimSpace(s); s != "" {
				out = append(out, ScriptMeta{Name: s})
			}
		}
	}
	return out
}

// Parse reads and parses the SKILL.md at path, resolving the skill name/path.
//
// A parseable file always returns a non-nil Skill. A malformed frontmatter
// soft-fails: the skill uses directory-name fallbacks and Parse returns
// ErrBadFrontmatter alongside a usable Skill. Other errors are hard failures
// (e.g. unreadable file) and the Skill is nil.
func Parse(path, skillName, relativePath string) (*Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	fm, body, softFail := splitFrontmatter(data)
	s := &Skill{
		Path:        relativePath,
		FrontMatter: fm,
		Content:     body,
	}
	if st, err := os.Stat(path); err == nil {
		s.ModTime = st.ModTime()
	}

	s.Name = skillName
	if v, ok := stringField(fm, "name"); ok && strings.TrimSpace(v) != "" {
		s.Name = v
	}

	s.Description = firstNonEmptyLine(body)
	if v, ok := stringField(fm, "description"); ok && strings.TrimSpace(v) != "" {
		s.Description = v
	}

	if v, ok := stringSliceField(fm, "tags"); ok {
		s.Tags = v
	}
	if v, ok := softStringField(fm, "version"); ok {
		s.Version = v
	}

	if softFail {
		return s, ErrBadFrontmatter
	}
	return s, nil
}

// splitFrontmatter returns the parsed frontmatter map, the markdown body, and
// whether the frontmatter was malformed (soft-fail). A file without a
// frontmatter block yields a nil map and the whole text as the body.
func splitFrontmatter(data []byte) (map[string]any, string, bool) {
	text := string(data)
	if !strings.HasPrefix(text, "---") {
		return nil, text, false
	}
	rest := strings.TrimPrefix(text, "---")
	if strings.HasPrefix(rest, "\n") {
		rest = rest[1:]
	}
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return nil, text, true
	}
	head := rest[:idx]
	body := rest[idx+len("\n---"):]
	body = strings.TrimPrefix(body, "\n")

	var fm map[string]any
	if err := yaml.Unmarshal([]byte(head), &fm); err != nil || fm == nil {
		return nil, text, true
	}
	return fm, body, false
}

func stringField(fm map[string]any, key string) (string, bool) {
	v, ok := fm[key].(string)
	return v, ok
}

// softStringField reads a field as a string, tolerating numbers and bools that
// yaml decodes into their Go scalar types (e.g. version: 2.1 -> float64).
func softStringField(fm map[string]any, key string) (string, bool) {
	v, ok := fm[key]
	if !ok || v == nil {
		return "", false
	}
	switch t := v.(type) {
	case string:
		return t, true
	case float64:
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t)), true
		}
		return fmt.Sprintf("%v", t), true
	case int:
		return fmt.Sprintf("%d", t), true
	case bool:
		return fmt.Sprintf("%v", t), true
	}
	return "", false
}

func stringSliceField(fm map[string]any, key string) ([]string, bool) {
	raw, ok := fm[key]
	if !ok {
		return nil, false
	}
	var out []string
	switch t := raw.(type) {
	case []any:
		for _, e := range t {
			if s, ok := e.(string); ok {
				out = append(out, s)
			}
		}
	case string:
		out = strings.Fields(t)
	}
	return out, true
}

func firstNonEmptyLine(body string) string {
	for _, line := range strings.Split(body, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			return t
		}
	}
	return ""
}
