package main

import (
	"math"
	"sort"
	"strings"
	"unicode"
)

type SearchResult struct {
	Path    string  `json:"path"`
	Title   string  `json:"title"`
	Snippet string  `json:"snippet"`
	Score   float64 `json:"score"`
}

type posting struct {
	path      string
	positions []int
}

type searchIndex struct {
	pages map[string]string
	terms map[string][]posting
}

func tokenize(text string) []string {
	lower := strings.ToLower(text)
	var tokens []string
	var current strings.Builder
	for _, r := range lower {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			current.WriteRune(r)
		} else {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens
}

func extractTitle(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimPrefix(line, "# ")
		}
	}
	return ""
}

func buildIndex(pages map[string]string) *searchIndex {
	idx := &searchIndex{
		pages: pages,
		terms: make(map[string][]posting),
	}
	for path, content := range pages {
		tokens := tokenize(content)
		termPositions := make(map[string][]int)
		for i, token := range tokens {
			termPositions[token] = append(termPositions[token], i)
		}
		for term, positions := range termPositions {
			idx.terms[term] = append(idx.terms[term], posting{
				path:      path,
				positions: positions,
			})
		}
	}
	return idx
}

func (idx *searchIndex) search(query string, limit int) []SearchResult {
	queryTerms := tokenize(query)
	if len(queryTerms) == 0 {
		return nil
	}

	scores := make(map[string]float64)
	numDocs := float64(len(idx.pages))

	for _, term := range queryTerms {
		postings, ok := idx.terms[term]
		if !ok {
			continue
		}
		idf := math.Log(1 + numDocs/float64(len(postings)))
		for _, p := range postings {
			tf := float64(len(p.positions))
			scores[p.path] += tf * idf
		}
	}

	var results []SearchResult
	for path, score := range scores {
		content := idx.pages[path]
		results = append(results, SearchResult{
			Path:    path,
			Title:   extractTitle(content),
			Snippet: makeSnippet(content, queryTerms[0], 200),
			Score:   score,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > limit {
		results = results[:limit]
	}
	return results
}

func makeSnippet(content string, term string, maxLen int) string {
	lower := strings.ToLower(content)
	termLower := strings.ToLower(term)
	pos := strings.Index(lower, termLower)
	if pos < 0 {
		if len(content) > maxLen {
			return content[:maxLen] + "..."
		}
		return content
	}

	start := pos - maxLen/2
	if start < 0 {
		start = 0
	}
	end := start + maxLen
	if end > len(content) {
		end = len(content)
	}

	snippet := content[start:end]
	snippet = strings.ReplaceAll(snippet, "\n", " ")

	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(content) {
		snippet = snippet + "..."
	}
	return snippet
}
