package main

import (
	"testing"
)

func TestBuildIndex(t *testing.T) {
	pages := map[string]string{
		"customers/john-smith": "# John Smith\nCredit Score: 782\nSalary: $145,000",
		"customers/jane-doe":   "# Jane Doe\nCredit Score: 620\nSalary: $55,000",
		"policies/mortgage":    "# Mortgage Lending Policy\nMinimum credit score: 620",
	}

	idx := buildIndex(pages)

	if len(idx.pages) != 3 {
		t.Errorf("expected 3 pages, got %d", len(idx.pages))
	}

	if postings, ok := idx.terms["credit"]; !ok || len(postings) != 3 {
		t.Errorf("expected 'credit' in 3 pages, got %v", idx.terms["credit"])
	}

	if postings, ok := idx.terms["john"]; !ok || len(postings) != 1 {
		t.Errorf("expected 'john' in 1 page, got %v", idx.terms["john"])
	}
}

func TestSearch(t *testing.T) {
	pages := map[string]string{
		"customers/john-smith": "# John Smith — Customer Profile\n\nCredit Score: 782\nSalary: $145,000\nEmployment: Senior Software Engineer at TechCorp",
		"customers/jane-doe":   "# Jane Doe — Customer Profile\n\nCredit Score: 620\nSalary: $55,000\nEmployment: Teacher",
		"policies/mortgage":    "# Mortgage Lending Policy\n\nMinimum credit score of 620 required for all mortgage products.",
	}

	idx := buildIndex(pages)

	results := idx.search("john smith", 10)
	if len(results) == 0 {
		t.Fatal("expected at least one result for 'john smith'")
	}
	if results[0].Path != "customers/john-smith" {
		t.Errorf("expected first result to be john-smith, got %s", results[0].Path)
	}

	results = idx.search("credit score 620", 10)
	if len(results) < 2 {
		t.Errorf("expected at least 2 results for 'credit score 620', got %d", len(results))
	}

	results = idx.search("nonexistent term xyz", 10)
	if len(results) != 0 {
		t.Errorf("expected 0 results for nonexistent term, got %d", len(results))
	}
}

func TestSearchSnippet(t *testing.T) {
	pages := map[string]string{
		"customers/john-smith": "# John Smith\n\nThis is a long document about John Smith who has a credit score of 782 and works at TechCorp as a senior engineer.",
	}

	idx := buildIndex(pages)
	results := idx.search("credit", 10)

	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
	if results[0].Snippet == "" {
		t.Error("expected non-empty snippet")
	}
	if len(results[0].Snippet) > 250 {
		t.Errorf("snippet too long: %d chars", len(results[0].Snippet))
	}
}
