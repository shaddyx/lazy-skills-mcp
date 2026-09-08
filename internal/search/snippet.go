package search

import "strings"

// snippet returns up to ~240 chars of the body centered on the first query
// token match, falling back to the description then the first body line.
func snippet(d *Doc, queryToks []string) string {
	if i := firstBodyMatch(d.Body, queryToks); i >= 0 {
		return around(d.Body, i, 240)
	}
	if d.Desc != "" {
		return d.Desc
	}
	return strings.TrimSpace(d.Body)
}

// firstBodyMatch returns the byte offset of the first case-insensitive match
// of any query token in body, or -1 if none match.
func firstBodyMatch(body string, toks []string) int {
	lower := strings.ToLower(body)
	for _, t := range toks {
		if t == "" {
			continue
		}
		if i := strings.Index(lower, t); i >= 0 {
			return i
		}
	}
	return -1
}

// around returns a window of up to max bytes centered on offset, ellipsized at
// word boundaries when truncated.
func around(s string, center, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	start := center - max/2
	if start < 0 {
		start = 0
	}
	end := start + max
	if end > len(s) {
		end = len(s)
		start = end - max
		if start < 0 {
			start = 0
		}
	}
	out := s[start:end]
	if start > 0 {
		if i := strings.IndexByte(out[1:], ' '); i >= 0 {
			out = out[i+1:]
		}
		out = "…" + out
	}
	if end < len(s) {
		out += "…"
	}
	return out
}
