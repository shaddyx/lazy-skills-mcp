package search

import (
	"strings"
	"unicode"
)

// Tokenize splits text into search tokens: split on non-alphanumeric runs and
// on camelCase / digit-letter boundaries, lowercase, keep tokens of length >=2.
func Tokenize(s string) []string {
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	var out []string
	for _, f := range fields {
		out = append(out, splitCamel(f)...)
	}
	out = lowercaseAll(out)
	return filter(out, 2)
}

// splitCamel splits camelCase tokens (goroutineLeak -> goroutine, leak) and
// digit-letter boundaries (html2 -> html, 2).
func splitCamel(s string) []string {
	if s == "" {
		return nil
	}
	if len(s) == 1 {
		return []string{s}
	}
	n := len(s)
	var words []string
	start := 0
	isBoundary := func(i int) bool {
		prev := s[i-1]
		cur := s[i]
		prevDigit := prev >= '0' && prev <= '9'
		curDigit := cur >= '0' && cur <= '9'
		if prevDigit != curDigit {
			return true
		}
		// prev lowercase (or digit), cur uppercase
		return cur >= 'A' && cur <= 'Z' && prev >= 'a' && prev <= 'z'
	}
	_ = n
	for i := 1; i < len(s); i++ {
		if isBoundary(i) {
			words = append(words, s[start:i])
			start = i
		}
	}
	words = append(words, s[start:])
	return words
}

func lowercaseAll(items []string) []string {
	for i, it := range items {
		items[i] = strings.ToLower(it)
	}
	return items
}

func filter(items []string, minLen int) []string {
	var out []string
	for _, it := range items {
		if it == "" || len(it) < minLen {
			continue
		}
		out = append(out, it)
	}
	return out
}
