// Package index walks the skill catalog tree at startup and builds an
// immutable, in-memory index of categories and skills. It also provides the
// path-confinement helpers used by all tools.
package index

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/shaddy/lazy-skills/internal/search"
	"github.com/shaddy/lazy-skills/internal/skill"
)

// Node is one entry in the catalog tree: a category, a skill, or both.
type Node struct {
	Name     string
	Path     string
	IsSkill  bool
	Skill    *skill.Skill
	Children []*Node
}

// Index is the immutable startup catalog index.
type Index struct {
	root     string
	tree     *Node
	byPath   map[string]*skill.Skill
	search   *search.Engine
	builtAt  time.Time
	count    int
	children map[string][]*Node
}

// Config controls how the index is built.
type Config struct {
	Root           string
	AllowEmptyRoot bool
	ScriptExts     []string
	MaxResults     int
}

// Build walks root and returns an Index. A missing or non-directory root is a
// hard error unless AllowEmptyRoot is set, in which case an empty index is
// returned.
func Build(cfg Config) (*Index, error) {
	root, err := filepath.Abs(cfg.Root)
	if err != nil {
		return nil, fmt.Errorf("resolve root %q: %w", cfg.Root, err)
	}
	fi, err := os.Stat(root)
	if err != nil {
		if cfg.AllowEmptyRoot {
			return newEmptyIndex(root)
		}
		return nil, fmt.Errorf("skill root %q: %w", cfg.Root, err)
	}
	if !fi.IsDir() {
		if cfg.AllowEmptyRoot {
			return newEmptyIndex(root)
		}
		return nil, fmt.Errorf("skill root %q is not a directory", cfg.Root)
	}

	idx := &Index{
		root:     root,
		byPath:   make(map[string]*skill.Skill),
		children: make(map[string][]*Node),
		builtAt:  time.Now(),
	}
	idx.tree = &Node{Name: root, Path: ""}

	if err := idx.walk(root, "", idx.tree); err != nil {
		return nil, err
	}

	docs := make([]*search.Doc, 0, idx.count)
	for _, s := range idx.byPath {
		docs = append(docs, &search.Doc{
			ID:   s.Path,
			Name: s.Name,
			Desc: s.Description,
			Tags: s.Tags,
			Body: s.Content,
		})
	}
	idx.search = search.Build(docs)

	return idx, nil
}

func newEmptyIndex(root string) (*Index, error) {
	return &Index{
		root:     root,
		byPath:   make(map[string]*skill.Skill),
		children: make(map[string][]*Node),
		builtAt:  time.Now(),
		tree:     &Node{Name: root, Path: ""},
		search:   search.Build(nil),
	}, nil
}

func (idx *Index) walk(absDir, rel string, parent *Node) error {
	entries, err := os.ReadDir(absDir)
	if err != nil {
		return fmt.Errorf("read dir %q: %w", rel, err)
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		childRel := joinRel(rel, name)
		childAbs := filepath.Join(absDir, name)

		child := &Node{Name: name, Path: childRel}
		parent.Children = append(parent.Children, child)

		skillPath := filepath.Join(childAbs, "SKILL.md")
		if fi, err := os.Stat(skillPath); err == nil && !fi.IsDir() {
			child.IsSkill = true
			s, perr := skill.Parse(skillPath, name, childRel)
			if s != nil {
				child.Skill = s
				idx.byPath[childRel] = s
				idx.count++
			}
			if perr != nil {
				fmt.Fprintf(os.Stderr, "warn: skill %s: %v\n", childRel, perr)
			}
		}

		if err := idx.walk(childAbs, childRel, child); err != nil {
			return err
		}
		idx.children[childRel] = nodeSlice(child.Children)
	}
	idx.children[rel] = nodeSlice(parent.Children)
	return nil
}

func nodeSlice(nodes []*Node) []*Node {
	out := make([]*Node, len(nodes))
	copy(out, nodes)
	return out
}

func joinRel(rel, name string) string {
	if rel == "" {
		return name
	}
	return rel + "/" + name
}

// Root returns the absolute skill root.
func (idx *Index) Root() string { return idx.root }

// Count returns the number of indexed skills.
func (idx *Index) Count() int { return idx.count }

// Tree returns the root node of the catalog.
func (idx *Index) Tree() *Node { return idx.tree }

// ByPath returns the skill at the given relative path, or nil.
func (idx *Index) ByPath(rel string) *skill.Skill { return idx.byPath[rel] }

// Children returns the child nodes at a given relative path.
func (idx *Index) Children(rel string) []*Node { return idx.children[rel] }

// Search runs a query against the catalog, restricted to a subtree if path is
// non-empty.
func (idx *Index) Search(query, path string, limit int, includeBody bool) []search.Hit {
	return idx.search.Search(query, path, limit, includeBody)
}

// Resolve confines a relative catalog path against the root, returning the
// absolute filesystem path if it exists inside the root.
func (idx *Index) Resolve(rel string) (string, error) {
	return resolveInside(idx.root, rel)
}

func resolveInside(root, rel string) (string, error) {
	if rel == "" {
		return root, nil
	}
	if rel == "." || rel == "/" || strings.HasPrefix(rel, "/") || rel == ".." || strings.HasPrefix(rel, "../") {
		return "", fmt.Errorf("invalid path %q", rel)
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(rel)))
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("invalid path %q", rel)
	}
	abs := filepath.Join(root, filepath.FromSlash(clean))
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	if !isWithin(root, resolved) {
		return "", fmt.Errorf("path %q escapes the skill root", rel)
	}
	return abs, nil
}

func isWithin(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
