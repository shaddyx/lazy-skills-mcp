package server

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shaddy/lazy-skills/internal/config"
	"github.com/shaddy/lazy-skills/internal/index"
	"github.com/shaddy/lazy-skills/internal/skill"
)

// Server wires the skill index into the MCP server.
type Server struct {
	cfg   config.Config
	index *index.Index
	mcp   *mcp.Server
}

// New builds an MCP server exposing the lazy-skill tools over the given index.
func New(cfg config.Config, idx *index.Index) *Server {
	s := &Server{
		cfg:   cfg,
		index: idx,
	}
	s.mcp = mcp.NewServer(&mcp.Implementation{Name: "lazy-skill", Version: "1.0.0"}, nil)

	registerTools(s)
	registerResources(s)
	return s
}

// MCPServer returns the underlying MCP server to run.
func (s *Server) MCPServer() *mcp.Server { return s.mcp }

// skillAt returns the skill at rel, mapping misses and category-only paths to
// the stable error codes. A directory with a SKILL.md is a skill even if it
// also has subdirectories (scripts/, references/).
func (s *Server) skillAt(rel string) (*skill.Skill, error) {
	node := s.findNode(rel)
	if node == nil {
		return nil, errCode(CodeNotFound, "skill %q not found", rel)
	}
	if !node.IsSkill || node.Skill == nil {
		return nil, errCode(CodeNotSkill, "path %q is a category, not a skill", rel)
	}
	return node.Skill, nil
}

// resolveSkillDir resolves a skill path to (skill, absolute directory).
func (s *Server) resolveSkillDir(rel string) (*skill.Skill, string, error) {
	sk, err := s.skillAt(rel)
	if err != nil {
		return nil, "", err
	}
	abs, err := s.index.Resolve(rel)
	if err != nil {
		return nil, "", errCode(CodeInternal, "resolve %q: %v", rel, err)
	}
	return sk, abs, nil
}
