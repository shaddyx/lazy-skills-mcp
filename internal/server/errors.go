// Package server wires the lazy-skill catalog into the MCP server: tool
// definitions, their handlers, resource handlers, and stable error codes.
package server

import "fmt"

// Stable error codes returned by tools, matching the spec (§11).
const (
	CodeInvalidInput    = "invalid_input"
	CodeNotFound        = "not_found"
	CodeNotSkill        = "not_skill"
	CodeInvalidQuery    = "invalid_query"
	CodeRootNotFound    = "root_not_found"
	CodeNotAScript      = "not_a_script"
	CodeBinaryFile      = "binary_file"
	CodeMissingInterp   = "missing_interpreter"
	CodeExecutionFailed = "execution_failed"
	CodeInternal        = "internal"
)

// errCode builds a tool error whose message carries the stable code. The SDK
// wraps a returned error into a CallToolResult with IsError set.
func errCode(code, format string, args ...any) error {
	return fmt.Errorf("%s: %s", code, fmt.Sprintf(format, args...))
}
