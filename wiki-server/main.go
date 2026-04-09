package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"path/filepath"
	"regexp"
	"sort"
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

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/wiki/", http.StatusFound)
	})
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

// wantsJSON returns true if the client prefers JSON (API/MCP tools) over HTML (browser).
func wantsJSON(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	if accept == "" {
		return false
	}
	if strings.Contains(accept, "application/json") {
		return true
	}
	if strings.Contains(accept, "text/html") {
		return false
	}
	return false
}

func handleWiki(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/wiki/")
	path = strings.TrimSuffix(path, "/")

	if path == "" {
		categories := listCategories()
		sort.Strings(categories)
		if wantsJSON(r) {
			writeJSON(w, categories)
			return
		}
		renderHome(w, categories)
		return
	}

	if strings.Contains(path, "/") {
		if content, ok := idx.pages[path]; ok {
			if wantsJSON(r) {
				w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
				fmt.Fprint(w, content)
				return
			}
			category := strings.SplitN(path, "/", 2)[0]
			renderPage(w, path, category, content)
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
	sort.Slice(entries, func(i, j int) bool { return entries[i].Title < entries[j].Title })
	if wantsJSON(r) {
		writeJSON(w, entries)
		return
	}
	renderCategory(w, path, entries)
}

func handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		if wantsJSON(r) {
			http.Error(w, `{"error":"missing q parameter"}`, http.StatusBadRequest)
			return
		}
		renderSearch(w, "", nil)
		return
	}
	results := idx.search(query, 20)
	if wantsJSON(r) {
		writeJSON(w, results)
		return
	}
	renderSearch(w, query, results)
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

// --- HTML rendering ---

const cssStyle = `
* { margin: 0; padding: 0; box-sizing: border-box; }
body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: #f8fafc; color: #1e293b; line-height: 1.6; }
nav { background: #0f172a; color: white; padding: 1rem 2rem; display: flex; align-items: center; gap: 1.5rem; }
nav a { color: white; text-decoration: none; }
nav .logo { font-weight: 700; font-size: 1.25rem; }
nav .logo span { color: #38bdf8; }
nav .search-form { margin-left: auto; display: flex; gap: 0.5rem; }
nav input[type=text] { padding: 0.4rem 0.75rem; border-radius: 6px; border: none; font-size: 0.9rem; width: 250px; }
nav button { padding: 0.4rem 1rem; border-radius: 6px; border: none; background: #38bdf8; color: #0f172a; font-weight: 600; cursor: pointer; }
.container { max-width: 960px; margin: 0 auto; padding: 2rem; }
h1 { font-size: 1.75rem; margin-bottom: 1rem; color: #0f172a; }
h2 { font-size: 1.35rem; margin: 1.5rem 0 0.75rem; color: #0f172a; border-bottom: 2px solid #e2e8f0; padding-bottom: 0.3rem; }
h3 { font-size: 1.1rem; margin: 1.25rem 0 0.5rem; color: #334155; }
.breadcrumb { font-size: 0.9rem; color: #64748b; margin-bottom: 1.5rem; }
.breadcrumb a { color: #2563eb; text-decoration: none; }
.breadcrumb a:hover { text-decoration: underline; }
.card-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: 1rem; margin-top: 1rem; }
.card { background: white; border-radius: 10px; padding: 1.25rem; border: 1px solid #e2e8f0; transition: box-shadow 0.2s; }
.card:hover { box-shadow: 0 4px 12px rgba(0,0,0,0.08); }
.card a { text-decoration: none; color: #0f172a; }
.card h3 { margin: 0 0 0.25rem; font-size: 1rem; color: #2563eb; }
.card .count { font-size: 0.85rem; color: #64748b; }
.page-list { list-style: none; }
.page-list li { padding: 0.6rem 0; border-bottom: 1px solid #f1f5f9; }
.page-list li:last-child { border-bottom: none; }
.page-list a { color: #2563eb; text-decoration: none; font-weight: 500; }
.page-list a:hover { text-decoration: underline; }
.wiki-content { background: white; border-radius: 10px; padding: 2rem; border: 1px solid #e2e8f0; }
.wiki-content p { margin: 0.5rem 0; }
.wiki-content ul, .wiki-content ol { margin: 0.5rem 0 0.5rem 1.5rem; }
.wiki-content li { margin: 0.25rem 0; }
.wiki-content strong { color: #0f172a; }
.wiki-content table { border-collapse: collapse; width: 100%; margin: 1rem 0; }
.wiki-content th, .wiki-content td { border: 1px solid #e2e8f0; padding: 0.5rem 0.75rem; text-align: left; }
.wiki-content th { background: #f1f5f9; font-weight: 600; }
.wiki-content tr:nth-child(even) { background: #f8fafc; }
.wiki-content code { background: #f1f5f9; padding: 0.15rem 0.4rem; border-radius: 4px; font-size: 0.9em; }
.wiki-content hr { border: none; border-top: 2px solid #e2e8f0; margin: 1.5rem 0; }
.search-results .result { margin-bottom: 1.25rem; padding-bottom: 1.25rem; border-bottom: 1px solid #f1f5f9; }
.search-results .result:last-child { border-bottom: none; }
.search-results .result a { color: #2563eb; font-weight: 600; text-decoration: none; font-size: 1.05rem; }
.search-results .result a:hover { text-decoration: underline; }
.search-results .result .snippet { color: #64748b; font-size: 0.9rem; margin-top: 0.25rem; }
.search-results .result .path { color: #94a3b8; font-size: 0.8rem; }
.stats { color: #64748b; font-size: 0.9rem; margin-bottom: 1rem; }
.category-icon { font-size: 1.5rem; margin-bottom: 0.5rem; }
`

var categoryIcons = map[string]string{
	"customers":  "👥",
	"policies":   "📋",
	"rates":      "📊",
	"products":   "💳",
	"procedures": "📝",
}

func categoryCount(cat string) int {
	count := 0
	prefix := cat + "/"
	for path := range idx.pages {
		if strings.HasPrefix(path, prefix) {
			count++
		}
	}
	return count
}

func renderLayout(w http.ResponseWriter, title, body string) {
	tmpl := `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>{{.Title}} — Solo Bank Wiki</title>
<style>` + cssStyle + `</style>
</head>
<body>
<nav>
  <a href="/wiki/" class="logo"><span>Solo</span> Bank Wiki</a>
  <a href="/wiki/customers/">Customers</a>
  <a href="/wiki/policies/">Policies</a>
  <a href="/wiki/rates/">Rates</a>
  <a href="/wiki/products/">Products</a>
  <a href="/wiki/procedures/">Procedures</a>
  <form class="search-form" action="/search" method="get">
    <input type="text" name="q" placeholder="Search wiki..." value="{{.Query}}">
    <button type="submit">Search</button>
  </form>
</nav>
<div class="container">{{.Body}}</div>
</body>
</html>`

	t := template.Must(template.New("layout").Parse(tmpl))
	t.Execute(w, map[string]template.HTML{
		"Title": template.HTML(template.HTMLEscapeString(title)),
		"Body":  template.HTML(body),
		"Query": "",
	})
}

func renderHome(w http.ResponseWriter, categories []string) {
	var b strings.Builder
	b.WriteString(`<h1>Solo Bank Knowledge Base</h1>`)
	b.WriteString(fmt.Sprintf(`<p class="stats">%d total pages across %d categories</p>`, len(idx.pages), len(categories)))
	b.WriteString(`<div class="card-grid">`)
	order := []string{"customers", "policies", "rates", "products", "procedures"}
	for _, cat := range order {
		icon := categoryIcons[cat]
		count := categoryCount(cat)
		b.WriteString(fmt.Sprintf(`<div class="card"><a href="/wiki/%s/">
			<div class="category-icon">%s</div>
			<h3>%s</h3>
			<div class="count">%d pages</div>
		</a></div>`, cat, icon, strings.Title(cat), count))
	}
	b.WriteString(`</div>`)
	renderLayout(w, "Home", b.String())
}

func renderCategory(w http.ResponseWriter, category string, entries []PageEntry) {
	var b strings.Builder
	b.WriteString(fmt.Sprintf(`<div class="breadcrumb"><a href="/wiki/">Home</a> / %s</div>`, strings.Title(category)))
	icon := categoryIcons[category]
	b.WriteString(fmt.Sprintf(`<h1>%s %s</h1>`, icon, strings.Title(category)))
	b.WriteString(fmt.Sprintf(`<p class="stats">%d pages</p>`, len(entries)))
	b.WriteString(`<ul class="page-list">`)
	for _, e := range entries {
		b.WriteString(fmt.Sprintf(`<li><a href="/wiki/%s">%s</a></li>`, e.Path, e.Title))
	}
	b.WriteString(`</ul>`)
	renderLayout(w, strings.Title(category), b.String())
}

// mdToHTML does a simple markdown-to-HTML conversion for display.
var (
	reH1     = regexp.MustCompile(`(?m)^# (.+)$`)
	reH2     = regexp.MustCompile(`(?m)^## (.+)$`)
	reH3     = regexp.MustCompile(`(?m)^### (.+)$`)
	reBold   = regexp.MustCompile(`\*\*(.+?)\*\*`)
	reCode   = regexp.MustCompile("`([^`]+)`")
	reHR     = regexp.MustCompile(`(?m)^---+$`)
	reUL     = regexp.MustCompile(`(?m)^- (.+)$`)
	reTable  = regexp.MustCompile(`(?m)^\|(.+)\|$`)
	reTSep   = regexp.MustCompile(`(?m)^\|[\s\-:|]+\|$`)
)

func mdToHTML(md string) string {
	// Escape HTML first
	s := template.HTMLEscapeString(md)

	// Process tables
	s = renderTables(s)

	// Headings
	s = reH1.ReplaceAllString(s, `<h1>$1</h1>`)
	s = reH2.ReplaceAllString(s, `<h2>$1</h2>`)
	s = reH3.ReplaceAllString(s, `<h3>$1</h3>`)

	// Inline formatting
	s = reBold.ReplaceAllString(s, `<strong>$1</strong>`)
	s = reCode.ReplaceAllString(s, `<code>$1</code>`)

	// Horizontal rules
	s = reHR.ReplaceAllString(s, `<hr>`)

	// Lists — wrap consecutive list items
	lines := strings.Split(s, "\n")
	var out []string
	inList := false
	for _, line := range lines {
		if reUL.MatchString(line) {
			if !inList {
				out = append(out, "<ul>")
				inList = true
			}
			item := reUL.ReplaceAllString(line, `$1`)
			out = append(out, "<li>"+item+"</li>")
		} else {
			if inList {
				out = append(out, "</ul>")
				inList = false
			}
			// Wrap non-empty, non-tag lines in <p>
			trimmed := strings.TrimSpace(line)
			if trimmed != "" && !strings.HasPrefix(trimmed, "<") {
				out = append(out, "<p>"+line+"</p>")
			} else {
				out = append(out, line)
			}
		}
	}
	if inList {
		out = append(out, "</ul>")
	}

	return strings.Join(out, "\n")
}

func renderTables(s string) string {
	lines := strings.Split(s, "\n")
	var out []string
	inTable := false
	headerDone := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if reTable.MatchString(trimmed) {
			if reTSep.MatchString(trimmed) {
				// separator row — skip
				continue
			}
			if !inTable {
				out = append(out, "<table>")
				inTable = true
				headerDone = false
			}
			cells := strings.Split(strings.Trim(trimmed, "|"), "|")
			tag := "td"
			if !headerDone {
				out = append(out, "<thead><tr>")
				tag = "th"
			} else {
				out = append(out, "<tr>")
			}
			for _, cell := range cells {
				out = append(out, fmt.Sprintf("<%s>%s</%s>", tag, strings.TrimSpace(cell), tag))
			}
			if !headerDone {
				out = append(out, "</tr></thead><tbody>")
				headerDone = true
			} else {
				out = append(out, "</tr>")
			}
		} else {
			if inTable {
				out = append(out, "</tbody></table>")
				inTable = false
			}
			out = append(out, line)
		}
	}
	if inTable {
		out = append(out, "</tbody></table>")
	}
	return strings.Join(out, "\n")
}

func renderPage(w http.ResponseWriter, path, category, content string) {
	var b strings.Builder
	b.WriteString(fmt.Sprintf(`<div class="breadcrumb"><a href="/wiki/">Home</a> / <a href="/wiki/%s/">%s</a> / %s</div>`,
		category, strings.Title(category), extractTitle(content)))
	b.WriteString(`<div class="wiki-content">`)
	b.WriteString(mdToHTML(content))
	b.WriteString(`</div>`)
	renderLayout(w, extractTitle(content), b.String())
}

func renderSearch(w http.ResponseWriter, query string, results []SearchResult) {
	var b strings.Builder
	b.WriteString(`<h1>Search</h1>`)
	if query == "" {
		b.WriteString(`<p>Enter a search query above.</p>`)
		renderSearchLayout(w, "", b.String())
		return
	}
	b.WriteString(fmt.Sprintf(`<p class="stats">%d results for "%s"</p>`, len(results), template.HTMLEscapeString(query)))
	if len(results) == 0 {
		b.WriteString(`<p>No results found.</p>`)
	} else {
		b.WriteString(`<div class="search-results">`)
		for _, r := range results {
			b.WriteString(`<div class="result">`)
			b.WriteString(fmt.Sprintf(`<a href="/wiki/%s">%s</a>`, r.Path, template.HTMLEscapeString(r.Title)))
			b.WriteString(fmt.Sprintf(`<div class="path">%s</div>`, r.Path))
			if r.Snippet != "" {
				b.WriteString(fmt.Sprintf(`<div class="snippet">%s</div>`, template.HTMLEscapeString(r.Snippet)))
			}
			b.WriteString(`</div>`)
		}
		b.WriteString(`</div>`)
	}
	renderSearchLayout(w, query, b.String())
}

func renderSearchLayout(w http.ResponseWriter, query, body string) {
	tmpl := `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Search — Solo Bank Wiki</title>
<style>` + cssStyle + `</style>
</head>
<body>
<nav>
  <a href="/wiki/" class="logo"><span>Solo</span> Bank Wiki</a>
  <a href="/wiki/customers/">Customers</a>
  <a href="/wiki/policies/">Policies</a>
  <a href="/wiki/rates/">Rates</a>
  <a href="/wiki/products/">Products</a>
  <a href="/wiki/procedures/">Procedures</a>
  <form class="search-form" action="/search" method="get">
    <input type="text" name="q" placeholder="Search wiki..." value="` + template.HTMLEscapeString(query) + `">
    <button type="submit">Search</button>
  </form>
</nav>
<div class="container">` + body + `</div>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, tmpl)
}
