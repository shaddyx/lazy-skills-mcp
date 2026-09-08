// Package search provides lexical (BM25-lite) search over the skill catalog:
// tokenization, an inverted index built at startup, field-weighted scoring,
// and snippet extraction.
package search

import (
	"sort"
	"strings"
)

// Doc is one searchable skill.
type Doc struct {
	ID   string
	Name string
	Desc string
	Tags []string
	Body string
}

// Hit is a ranked match.
type Hit struct {
	Path          string
	Name          string
	Description   string
	Score         float64
	MatchedFields []string
	Snippet       string
}

// fieldSpec binds a document field to its BM25 weight.
type fieldSpec struct {
	name   string
	weight float64
	get    func(*Doc) string
}

type fieldIndex struct {
	spec fieldSpec
	dict map[string][]posting // token -> postings (docID, freq)
}

type posting struct {
	docID int
	freq  int
}

// Engine holds the inverted index and ranking parameters. It is immutable
// after Build.
type Engine struct {
	docs    []*Doc
	byID    map[string]int
	avgDL   map[string]float64 // field name -> average token length
	len     [][]int            // [fieldIdx][docID] -> token length
	indexes []fieldIndex
}

const (
	k1 = 1.5
	b  = 0.75

	weightName = 3.0
	weightDesc = 2.0
	weightTags = 2.0
	weightBody = 1.0
)

var fields = []fieldSpec{
	{"name", weightName, func(d *Doc) string { return d.Name }},
	{"description", weightDesc, func(d *Doc) string { return d.Desc }},
	{"tags", weightTags, func(d *Doc) string { return strings.Join(d.Tags, " ") }},
	{"body", weightBody, func(d *Doc) string { return d.Body }},
}

// Build tokenizes the docs and constructs the inverted index.
func Build(docs []*Doc) *Engine {
	e := &Engine{
		docs:    docs,
		byID:    make(map[string]int, len(docs)),
		avgDL:   make(map[string]float64, len(fields)),
		indexes: make([]fieldIndex, len(fields)),
	}
	if len(docs) == 0 {
		return e
	}
	for i, d := range docs {
		e.byID[d.ID] = i
	}
	e.len = make([][]int, len(fields))

	for fi, fs := range fields {
		dict := make(map[string][]posting)
		lens := make([]int, len(docs))
		sum := 0
		for docID, d := range docs {
			toks := Tokenize(fs.get(d))
			lens[docID] = len(toks)
			sum += len(toks)
			freq := make(map[string]int)
			for _, t := range toks {
				freq[t]++
			}
			for t, c := range freq {
				dict[t] = append(dict[t], posting{docID: docID, freq: c})
			}
		}
		e.len[fi] = lens
		e.avgDL[fs.name] = float64(sum) / float64(len(docs))
		e.indexes[fi] = fieldIndex{spec: fs, dict: dict}
	}
	return e
}

// Search ranks docs matching query, restricted to a subtree when pathPrefix is
// non-empty, returning up to limit hits.
func (e *Engine) Search(query, pathPrefix string, limit int, includeBody bool) []Hit {
	toks := Tokenize(query)
	if len(toks) == 0 || e.total() == 0 {
		return nil
	}

	scores := make(map[int]float64)
	matched := make(map[int][]string)

	for fi, fix := range e.indexes {
		if !includeBody && fix.spec.name == "body" {
			continue
		}
		avg := e.avgDL[fix.spec.name]
		for _, tok := range toks {
			for _, p := range fix.dict[tok] {
				id := p.docID
				dl := e.len[fi][id]
				denom := float64(p.freq) + k1*(1-b+b*(float64(dl)/avg))
				scores[id] += float64(p.freq) * (1 + k1) * fix.spec.weight / denom
				addField(matched, id, fix.spec.name)
			}
		}
	}

	if len(scores) == 0 {
		return nil
	}

	ids := make([]int, 0, len(scores))
	for id := range scores {
		if pathPrefix != "" && !within(e.docs[id].ID, pathPrefix) {
			continue
		}
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		a, b := ids[i], ids[j]
		if scores[a] != scores[b] {
			return scores[a] > scores[b]
		}
		return e.docs[a].ID < e.docs[b].ID
	})

	if limit < 0 || limit > len(ids) {
		limit = len(ids)
	}
	ids = ids[:limit]

	hits := make([]Hit, 0, len(ids))
	for _, id := range ids {
		d := e.docs[id]
		hits = append(hits, Hit{
			Path:          d.ID,
			Name:          d.Name,
			Description:   d.Desc,
			Score:         scores[id],
			MatchedFields: matched[id],
			Snippet:       snippet(d, toks),
		})
	}
	return hits
}

func (e *Engine) total() int { return len(e.docs) }

func addField(m map[int][]string, id int, field string) {
	for _, f := range m[id] {
		if f == field {
			return
		}
	}
	m[id] = append(m[id], field)
}

func within(docID, prefix string) bool {
	return docID == prefix || strings.HasPrefix(docID, prefix+"/")
}
