package server

import (
	"context"
	"net/url"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const resourceScheme = "lazy-skill"

func registerResources(s *Server) {
	s.mcp.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: resourceScheme + "://{path}",
		Name:        "skill",
		Description: "A skill body served as a lazy-skill resource",
		MIMEType:    "text/markdown",
	}, s.handleReadResource)
}

func (s *Server) handleReadResource(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	rel := resourcePath(req.Params.URI)
	if rel == "" {
		return nil, mcp.ResourceNotFoundError(req.Params.URI)
	}
	sk, err := s.skillAt(rel)
	if err != nil {
		return nil, mcp.ResourceNotFoundError(req.Params.URI)
	}
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{{
			URI:      req.Params.URI,
			MIMEType: "text/markdown",
			Text:     sk.Content,
		}},
	}, nil
}

func resourcePath(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		return ""
	}
	if u.Scheme != resourceScheme || u.Host == "" {
		return ""
	}
	return strings.Trim(u.Host+u.Path, "/")
}
