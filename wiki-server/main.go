package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"path/filepath"
	"strings"
)

//go:embed content/*
var contentFS embed.FS

var idx *searchIndex

type PageEntry struct {
	Name  string `json:"name"`
	Title string `json:"title"`
	Path  string `json:"path"`
}

func main() {
	pages := loadPages()
	idx = buildIndex(pages)
	log.Printf("Loaded %d pages, indexed %d terms", len(pages), len(idx.terms))

	http.HandleFunc("/wiki/", handleWiki)
	http.HandleFunc("/search", handleSearch)
	http.HandleFunc("/health", handleHealth)

	log.Println("Wiki server starting on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func loadPages() map[string]string {
	pages := make(map[string]string)
	fs.WalkDir(contentFS, "content", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if !strings.HasSuffix(path, ".md") {
			return nil
		}
		data, err := contentFS.ReadFile(path)
		if err != nil {
			return err
		}
		pagePath := strings.TrimPrefix(path, "content/")
		pagePath = strings.TrimSuffix(pagePath, ".md")
		pages[pagePath] = string(data)
		return nil
	})
	return pages
}

func handleWiki(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/wiki/")
	path = strings.TrimSuffix(path, "/")

	if path == "" {
		categories := listCategories()
		writeJSON(w, categories)
		return
	}

	if strings.Contains(path, "/") {
		if content, ok := idx.pages[path]; ok {
			w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
			fmt.Fprint(w, content)
			return
		}
		http.NotFound(w, r)
		return
	}

	entries := listPagesInCategory(path)
	if len(entries) == 0 {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, entries)
}

func handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, `{"error":"missing q parameter"}`, http.StatusBadRequest)
		return
	}
	results := idx.search(query, 10)
	writeJSON(w, results)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{"status": "ok"})
}

func listCategories() []string {
	seen := make(map[string]bool)
	for path := range idx.pages {
		parts := strings.SplitN(path, "/", 2)
		if len(parts) > 0 && !seen[parts[0]] {
			seen[parts[0]] = true
		}
	}
	var cats []string
	for cat := range seen {
		cats = append(cats, cat)
	}
	return cats
}

func listPagesInCategory(category string) []PageEntry {
	var entries []PageEntry
	prefix := category + "/"
	for path, content := range idx.pages {
		if strings.HasPrefix(path, prefix) {
			name := filepath.Base(path)
			entries = append(entries, PageEntry{
				Name:  name,
				Title: extractTitle(content),
				Path:  path,
			})
		}
	}
	return entries
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
