package server

import (
	"context"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shaddy/lazy-skills/internal/index"
	"github.com/shaddy/lazy-skills/internal/search"
	"github.com/shaddy/lazy-skills/internal/skill"
)

// ListSkillsArgs are the arguments to list_skills.
type ListSkillsArgs struct {
	Path string `json:"path"`
}

// SkillEntry is one skill child in a list_skills result.
type SkillEntry struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Description string `json:"description"`
}

// CategoryEntry is one category child in a list_skills result.
type CategoryEntry struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Description string `json:"description"`
}

// Self describes the node being browsed.
type Self struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
}

// ListSkillsResult is the output of list_skills.
type ListSkillsResult struct {
	Path       string          `json:"path"`
	Self       Self            `json:"self"`
	Skills     []SkillEntry    `json:"skills"`
	Categories []CategoryEntry `json:"categories"`
}

// GetSkillArgs are the arguments to get_skill.
type GetSkillArgs struct {
	Path               string `json:"path"`
	IncludeFrontmatter bool   `json:"include_frontmatter"`
}

// GetSkillResult is the output of get_skill.
type GetSkillResult struct {
	Path            string             `json:"path"`
	Name            string             `json:"name"`
	Description     string             `json:"description"`
	Tags            []string           `json:"tags"`
	Version         string             `json:"version"`
	Content         string             `json:"content"`
	Frontmatter     map[string]any     `json:"frontmatter,omitempty"`
	DeclaredScripts []skill.ScriptMeta `json:"declared_scripts,omitempty"`
}

// SearchSkillsArgs are the arguments to search_skills.
type SearchSkillsArgs struct {
	Query       string `json:"query"`
	Limit       int    `json:"limit"`
	Path        string `json:"path"`
	IncludeBody bool   `json:"include_body"`
}

// SearchResult is one ranked hit.
type SearchResult = search.Hit

// SearchSkillsResult is the output of search_skills.
type SearchSkillsResult struct {
	Query   string         `json:"query"`
	Total   int            `json:"total"`
	Results []SearchResult `json:"results"`
}

// SkillTreeArgs are the arguments to skill_tree.
type SkillTreeArgs struct {
	MaxDepth int `json:"max_depth"`
}

func registerTools(s *Server) {
	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "list_skills",
		Description: "Browse one level of the skill catalog. Returns the immediate skill and category children of a category path; empty path = root.",
	}, s.handleListSkills)

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "get_skill",
		Description: "Fetch the full body and metadata of a skill",
	}, s.handleGetSkill)

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "search_skills",
		Description: "Lexical (BM25) search across the skill catalog",
	}, s.handleSearchSkills)

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "skill_tree",
		Description: "Return the whole catalog as a compact nested tree (names only)",
	}, s.handleSkillTree)

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "list_skill_files",
		Description: "List the non-SKILL.md files (scripts, references, auxiliary) in a skill directory",
	}, s.handleListSkillFiles)

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "get_skill_file",
		Description: "Fetch the contents of a single file from a skill directory",
	}, s.handleGetSkillFile)

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "execute_skill_script",
		Description: "Execute a script inside a skill directory and return stdout/stderr/exit code",
	}, s.handleExecuteSkillScript)
}

func (s *Server) handleListSkills(_ context.Context, req *mcp.CallToolRequest, in ListSkillsArgs) (*mcp.CallToolResult, ListSkillsResult, error) {
	rel := normalizePath(in.Path)
	node := s.findNode(rel)
	if node == nil {
		return nil, ListSkillsResult{}, errCode(CodeNotFound, "path %q not found", in.Path)
	}

	self := Self{Name: nodeName(rel, node), Type: "category"}
	if node.IsSkill && node.Skill != nil {
		self.Type = "skill"
		self.Description = node.Skill.Description
	}
	if rel == "" {
		self.Name = "root"
	}

	res := ListSkillsResult{Path: rel, Self: self}
	for _, c := range node.Children {
		if c.IsSkill && c.Skill != nil {
			res.Skills = append(res.Skills, SkillEntry{Name: c.Skill.Name, Path: c.Path, Description: c.Skill.Description})
		} else {
			res.Categories = append(res.Categories, CategoryEntry{Name: c.Name, Path: c.Path})
		}
	}
	return nil, res, nil
}

func (s *Server) handleGetSkill(_ context.Context, req *mcp.CallToolRequest, in GetSkillArgs) (*mcp.CallToolResult, GetSkillResult, error) {
	rel := in.Path
	if rel == "" {
		return nil, GetSkillResult{}, errCode(CodeInvalidInput, "path is required")
	}
	sk, err := s.skillAt(rel)
	if err != nil {
		return nil, GetSkillResult{}, err
	}
	out := GetSkillResult{
		Path:            sk.Path,
		Name:            sk.Name,
		Description:     sk.Description,
		Tags:            sk.Tags,
		Version:         sk.Version,
		Content:         sk.Content,
		DeclaredScripts: skill.DeclaredScripts(sk.FrontMatter),
	}
	if in.IncludeFrontmatter {
		out.Frontmatter = sk.FrontMatter
	}
	return nil, out, nil
}

func (s *Server) handleSearchSkills(_ context.Context, req *mcp.CallToolRequest, in SearchSkillsArgs) (*mcp.CallToolResult, SearchSkillsResult, error) {
	if strings.TrimSpace(in.Query) == "" {
		return nil, SearchSkillsResult{}, errCode(CodeInvalidQuery, "query is required")
	}
	limit := defaultLimit(s, in.Limit)
	hits := s.index.Search(in.Query, in.Path, limit, in.IncludeBody)
	if hits == nil {
		hits = []search.Hit{}
	}
	return nil, SearchSkillsResult{Query: in.Query, Total: len(hits), Results: hits}, nil
}

func (s *Server) handleSkillTree(_ context.Context, req *mcp.CallToolRequest, in SkillTreeArgs) (*mcp.CallToolResult, map[string]any, error) {
	return nil, buildTreeMap(s.index.Tree(), in.MaxDepth), nil
}

func (s *Server) findNode(rel string) *index.Node {
	if rel == "" {
		return s.index.Tree()
	}
	for _, c := range s.index.Children(parentOf(rel)) {
		if c.Path == rel {
			return c
		}
	}
	return nil
}

func parentOf(rel string) string {
	i := strings.LastIndex(rel, "/")
	if i < 0 {
		return ""
	}
	return rel[:i]
}

func nodeName(rel string, n *index.Node) string {
	if rel == "" {
		return "root"
	}
	if n.Skill != nil {
		return n.Skill.Name
	}
	return n.Name
}

func normalizePath(p string) string {
	return strings.Trim(p, "/")
}

func defaultLimit(s *Server, given int) int {
	if given <= 0 {
		return s.cfg.MaxResults
	}
	if given > s.cfg.MaxResultsMax {
		return s.cfg.MaxResultsMax
	}
	return given
}

func buildTreeMap(n *index.Node, maxDepth int) map[string]any {
	if n == nil {
		return nil
	}
	tn := map[string]any{"name": n.Name, "path": n.Path}
	if n.IsSkill {
		tn["type"] = "skill"
		if n.Skill != nil {
			tn["name"] = n.Skill.Name
		}
	} else {
		tn["type"] = "category"
	}
	if n.Path == "" {
		tn["type"] = "root"
		tn["name"] = "root"
	}
	// maxDepth == 0 means unlimited; otherwise stop once depth is exhausted.
	if maxDepth < 0 || maxDepth == 0 || maxDepth > 1 {
		var children []map[string]any
		for _, c := range n.Children {
			if child := buildTreeMap(c, maxDepthChildren(maxDepth)); child != nil {
				children = append(children, child)
			}
		}
		if children != nil {
			tn["children"] = children
		}
	}
	return tn
}

func maxDepthChildren(d int) int {
	if d > 0 {
		return d - 1
	}
	return d
}
