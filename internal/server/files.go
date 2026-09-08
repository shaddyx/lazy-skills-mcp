package server

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ListSkillFilesArgs are the arguments to list_skill_files.
type ListSkillFilesArgs struct {
	Path string `json:"path"`
}

// SkillFile is one file entry in a list_skill_files result.
type SkillFile struct {
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	IsScript    bool   `json:"is_script"`
	IsReference bool   `json:"is_reference"`
	Executable  bool   `json:"executable"`
}

// ListSkillFilesResult is the output of list_skill_files.
type ListSkillFilesResult struct {
	Path  string      `json:"path"`
	Files []SkillFile `json:"files"`
}

// GetSkillFileArgs are the arguments to get_skill_file.
type GetSkillFileArgs struct {
	Path string `json:"path"`
	File string `json:"file"`
}

// GetSkillFileResult is the output of get_skill_file.
type GetSkillFileResult struct {
	Path    string `json:"path"`
	File    string `json:"file"`
	Size    int64  `json:"size"`
	Content string `json:"content"`
}

func (s *Server) handleListSkillFiles(_ context.Context, req *mcp.CallToolRequest, in ListSkillFilesArgs) (*mcp.CallToolResult, ListSkillFilesResult, error) {
	if in.Path == "" {
		return nil, ListSkillFilesResult{}, errCode(CodeInvalidInput, "path is required")
	}
	if _, _, err := s.resolveSkillDir(in.Path); err != nil {
		return nil, ListSkillFilesResult{}, err
	}
	absDir, err := s.index.Resolve(in.Path)
	if err != nil {
		return nil, ListSkillFilesResult{}, errCode(CodeNotFound, "skill %q not found", in.Path)
	}

	var files []SkillFile
	err = filepath.WalkDir(absDir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == absDir {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(absDir, p)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(rel)
		if name == "SKILL.md" {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		ext := strings.ToLower(filepath.Ext(name))
		files = append(files, SkillFile{
			Name:        name,
			Size:        info.Size(),
			IsScript:    scriptExtAllowed(s.exts(), ext),
			IsReference: isReference(name),
			Executable:  info.Mode()&0o111 != 0,
		})
		return nil
	})
	if err != nil {
		return nil, ListSkillFilesResult{}, errCode(CodeInternal, "walk skill files: %v", err)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	if files == nil {
		files = []SkillFile{}
	}
	return nil, ListSkillFilesResult{Path: in.Path, Files: files}, nil
}

func (s *Server) handleGetSkillFile(_ context.Context, req *mcp.CallToolRequest, in GetSkillFileArgs) (*mcp.CallToolResult, GetSkillFileResult, error) {
	if in.Path == "" || in.File == "" {
		return nil, GetSkillFileResult{}, errCode(CodeInvalidInput, "path and file are required")
	}
	absDir, err := s.skillDir(in.Path)
	if err != nil {
		return nil, GetSkillFileResult{}, err
	}
	resolved, ok := resolveFileInside(absDir, in.File)
	if !ok {
		return nil, GetSkillFileResult{}, errCode(CodeNotFound, "file %q not found", in.File)
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		return nil, GetSkillFileResult{}, errCode(CodeNotFound, "file %q not found", in.File)
	}
	if !isText(data) {
		return nil, GetSkillFileResult{}, errCode(CodeBinaryFile, "file %q is not text", in.File)
	}
	return nil, GetSkillFileResult{Path: in.Path, File: in.File, Size: int64(len(data)), Content: string(data)}, nil
}

func (s *Server) skillDir(rel string) (string, error) {
	if _, err := s.skillAt(rel); err != nil {
		return "", err
	}
	abs, err := s.index.Resolve(rel)
	if err != nil {
		return "", errCode(CodeNotFound, "skill %q not found", rel)
	}
	return abs, nil
}

func (s *Server) exts() []string { return s.cfg.ScriptExts }

func scriptExtAllowed(allowed []string, ext string) bool {
	for _, a := range allowed {
		if a == ext {
			return true
		}
	}
	return false
}

func isReference(name string) bool {
	parts := strings.Split(name, "/")
	if len(parts) < 2 || parts[0] != "references" {
		return false
	}
	return strings.EqualFold(filepath.Ext(name), ".md")
}

// resolveFileInside confines rel to base dir, returning the absolute path and
// whether it exists and stays inside base.
func resolveFileInside(base, rel string) (string, bool) {
	if rel == "" || strings.HasPrefix(rel, "/") || rel == "." || rel == ".." {
		return "", false
	}
	joined := filepath.Join(base, filepath.FromSlash(rel))
	if !isWithin(base, joined) {
		return "", false
	}
	if fi, err := os.Stat(joined); err != nil || fi.IsDir() {
		return "", false
	}
	return joined, true
}

func isText(data []byte) bool {
	if len(data) == 0 {
		return true
	}
	// A NUL byte reliably marks binary content regardless of encoding.
	return bytes.IndexByte(data, 0) < 0
}

func isWithin(base, p string) bool {
	rel, err := filepath.Rel(base, p)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
