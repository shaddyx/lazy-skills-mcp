// Command lazy-skill-mcp exposes an agent-skill catalog over the Model Context
// Protocol over stdio, with diagnostic --list / --search modes.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shaddy/lazy-skills/internal/config"
	"github.com/shaddy/lazy-skills/internal/index"
	"github.com/shaddy/lazy-skills/internal/server"
)

const version = "1.0.0"

func main() {
	fs := flag.NewFlagSet("lazy-skill-mcp", flag.ContinueOnError)
	config.Flags(fs)
	doList := fs.Bool("list", false, "print the catalog tree and exit")
	doSearch := fs.Bool("search", false, "one-shot search and exit (use --query)")
	query := fs.String("query", "", "search query for --search mode")
	showVersion := fs.Bool("version", false, "print version and exit")

	if err := fs.Parse(os.Args[1:]); err != nil {
		os.Exit(2)
	}

	if *showVersion {
		fmt.Printf("lazy-skill-mcp %s\n", version)
		return
	}

	cfg, err := config.Parse(fs, os.Args[1:])
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	idx, err := index.Build(index.Config{
		Root:           cfg.Root,
		AllowEmptyRoot: cfg.AllowEmptyRoot,
		ScriptExts:     cfg.ScriptExts,
		MaxResults:     cfg.MaxResults,
	})
	if err != nil {
		log.Fatalf("index: %v", err)
	}

	switch {
	case *doList:
		printTree(os.Stdout, idx.Tree(), 0)
	case *doSearch:
		runOneShotSearch(os.Stdout, idx, *query)
	default:
		serve(cfg, idx)
	}
}

func printTree(w *os.File, n *index.Node, depth int) {
	if n == nil {
		return
	}
	if depth > 0 {
		indent := strings.Repeat("  ", depth-1)
		label := n.Name
		if n.IsSkill && n.Skill != nil {
			label = n.Skill.Name
		}
		fmt.Fprintf(w, "%s%s%s\n", indent, treePrefix(n), label)
	}
	for _, c := range n.Children {
		printTree(w, c, depth+1)
	}
}

func treePrefix(n *index.Node) string {
	if n.IsSkill {
		return "• "
	}
	return "▸ "
}

func runOneShotSearch(w *os.File, idx *index.Index, query string) {
	if strings.TrimSpace(query) == "" {
		fmt.Fprintln(os.Stderr, "--query is required with --search")
		os.Exit(2)
	}
	hits := idx.Search(query, "", 10, true)
	out, err := json.MarshalIndent(hits, "", "  ")
	if err != nil {
		log.Fatalf("marshaling results: %v", err)
	}
	fmt.Fprintln(w, string(out))
}

func serve(cfg config.Config, idx *index.Index) {
	s := server.New(cfg, idx)
	if err := s.MCPServer().Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
