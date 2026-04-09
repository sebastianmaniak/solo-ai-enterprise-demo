# Solo Bank Enterprise AI Demo — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a complete Kind-based demo of Solo.io's AI enterprise platform with a realistic banking scenario — wiki server, MCP tool servers, 4 agents, full docs site, and a tutorial for building custom agents.

**Architecture:** Go wiki server embeds markdown content about Solo Bank (100 customers, policies, rates). Three Go MCP tool servers expose domain-specific tools via Streamable HTTP. Four kagent agents (triage, customer service, mortgage advisor, compliance) use these tools through AgentGateway. A static HTML docs site provides usage guide and a "build your own agent" tutorial.

**Tech Stack:** Go 1.22, MCP Go SDK (`github.com/modelcontextprotocol/go-sdk`), Kind, Helm, kagent Enterprise v0.3.14, AgentGateway Enterprise v2.3.0-beta.8, AgentRegistry OSS v0.3.3

---

## File Structure

```
solo-ai-enterprise-demo/
├── setup.sh                              # Main deployment script
├── teardown.sh                           # Cleanup script
├── .env.example                          # Template for required env vars
├── kind-config.yaml                      # Kind cluster config
├── manifests/
│   ├── namespaces.yaml                   # All namespace definitions
│   ├── gateway.yaml                      # Gateway + tracing policy
│   ├── llm-backends/
│   │   ├── openai.yaml                   # Secret + Backend + HTTPRoute
│   │   └── anthropic.yaml                # Secret + Backend + HTTPRoute
│   ├── mcp/
│   │   ├── remote-mcp-servers.yaml       # RemoteMCPServer CRDs (kagent namespace)
│   │   └── mcp-routes.yaml              # MCPRoute CRDs (agentgateway namespace)
│   ├── agents/
│   │   ├── model-configs.yaml            # ModelConfig CRDs
│   │   ├── triage-agent.yaml
│   │   ├── customer-service-agent.yaml
│   │   ├── mortgage-advisor-agent.yaml
│   │   └── compliance-agent.yaml
│   └── bank-wiki/
│       ├── wiki-server.yaml              # Deployment + Service
│       ├── customer-tools.yaml           # Deployment + Service
│       ├── policy-tools.yaml             # Deployment + Service
│       └── transaction-tools.yaml        # Deployment + Service
├── wiki-server/
│   ├── Dockerfile
│   ├── go.mod
│   ├── go.sum
│   ├── main.go                           # HTTP server, routing, embed
│   ├── search.go                         # Inverted index + search
│   ├── search_test.go                    # Search unit tests
│   └── content/                          # Embedded markdown
│       ├── customers/                    # 100 customer profiles
│       ├── policies/                     # 10 policy documents
│       ├── rates/                        # 4 rate tables
│       ├── products/                     # 8 product descriptions
│       └── procedures/                   # 5 procedure documents
├── mcp-tools/
│   ├── shared/
│   │   ├── go.mod
│   │   ├── wiki_client.go               # Shared HTTP client for wiki server
│   │   └── wiki_client_test.go
│   ├── customer-tools/
│   │   ├── Dockerfile
│   │   ├── go.mod
│   │   ├── go.sum
│   │   ├── main.go                       # MCP server + tool registration
│   │   └── main_test.go
│   ├── policy-tools/
│   │   ├── Dockerfile
│   │   ├── go.mod
│   │   ├── go.sum
│   │   ├── main.go
│   │   └── main_test.go
│   └── transaction-tools/
│       ├── Dockerfile
│       ├── go.mod
│       ├── go.sum
│       ├── main.go
│       └── main_test.go
├── docs-site/
│   ├── Dockerfile
│   ├── nginx.conf                        # Lightweight nginx to serve HTML
│   ├── index.html                        # Main docs landing page
│   ├── guide.html                        # Using the demo agents
│   ├── tutorial.html                     # Build your own agent tutorial
│   ├── architecture.html                # Architecture overview
│   └── static/
│       └── style.css                     # Minimal styling
└── sample-agent/
    ├── README.md                         # Quick-start for the tutorial
    ├── mcp-server/
    │   ├── Dockerfile
    │   ├── go.mod
    │   ├── go.sum
    │   └── main.go                       # Sample "account-summary" MCP tool
    └── manifests/
        ├── remote-mcp-server.yaml        # RemoteMCPServer for the sample
        └── agent.yaml                    # Sample Agent CRD
```

---

## Phase 1: Project Scaffolding & Infrastructure Config

### Task 1: Project scaffolding and Kind config

**Files:**
- Create: `kind-config.yaml`
- Create: `.env.example`
- Create: `manifests/namespaces.yaml`

- [ ] **Step 1: Create Kind cluster config**

```yaml
# kind-config.yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  extraPortMappings:
  - containerPort: 30080
    hostPort: 30080
    protocol: TCP
  - containerPort: 30121
    hostPort: 30121
    protocol: TCP
  - containerPort: 30400
    hostPort: 30400
    protocol: TCP
  - containerPort: 30500
    hostPort: 30500
    protocol: TCP
```

Port 30500 is for the docs site.

- [ ] **Step 2: Create env template**

```bash
# .env.example
# Copy this to .env and fill in your values
# source .env before running setup.sh

export OPENAI_API_KEY="your-openai-api-key"
export ANTHROPIC_API_KEY="your-anthropic-api-key"
export AGENTGATEWAY_LICENSE_KEY="your-solo-enterprise-license-key"
```

- [ ] **Step 3: Create namespace definitions**

```yaml
# manifests/namespaces.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: bank-wiki
---
apiVersion: v1
kind: Namespace
metadata:
  name: kagent
```

Other namespaces (`agentgateway-system`, `agentregistry`) are created by Helm `--create-namespace`.

- [ ] **Step 4: Commit**

```bash
git add kind-config.yaml .env.example manifests/namespaces.yaml
git commit -m "feat: add Kind config, env template, and namespace definitions"
```

---

### Task 2: Gateway and tracing manifests

**Files:**
- Create: `manifests/gateway.yaml`

- [ ] **Step 1: Create Gateway + tracing policy**

```yaml
# manifests/gateway.yaml
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: agentgateway-proxy
  namespace: agentgateway-system
spec:
  gatewayClassName: enterprise-agentgateway
  infrastructure:
    parametersRef:
      name: tracing
      group: enterpriseagentgateway.solo.io
      kind: EnterpriseAgentgatewayParameters
  listeners:
  - protocol: HTTP
    port: 80
    name: http
    allowedRoutes:
      namespaces:
        from: All
---
apiVersion: enterpriseagentgateway.solo.io/v1alpha1
kind: EnterpriseAgentgatewayPolicy
metadata:
  name: tracing
  namespace: agentgateway-system
spec:
  targetRefs:
  - group: gateway.networking.k8s.io
    kind: Gateway
    name: agentgateway-proxy
  frontend:
    tracing:
      backendRef:
        name: solo-enterprise-telemetry-collector
        namespace: agentgateway-system
        kind: Service
        port: 4317
```

- [ ] **Step 2: Commit**

```bash
git add manifests/gateway.yaml
git commit -m "feat: add Gateway and tracing policy manifests"
```

---

### Task 3: LLM backend manifests

**Files:**
- Create: `manifests/llm-backends/openai.yaml`
- Create: `manifests/llm-backends/anthropic.yaml`

- [ ] **Step 1: Create OpenAI backend manifest**

The Secret uses `__OPENAI_API_KEY__` as a placeholder — `setup.sh` will `envsubst` or `sed` before applying.

```yaml
# manifests/llm-backends/openai.yaml
apiVersion: v1
kind: Secret
metadata:
  name: openai-secret
  namespace: agentgateway-system
type: Opaque
stringData:
  Authorization: __OPENAI_API_KEY__
---
apiVersion: agentgateway.dev/v1alpha1
kind: AgentgatewayBackend
metadata:
  name: openai
  namespace: agentgateway-system
spec:
  ai:
    provider:
      openai:
        model: gpt-4o-mini
  policies:
    auth:
      secretRef:
        name: openai-secret
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: openai
  namespace: agentgateway-system
spec:
  parentRefs:
  - name: agentgateway-proxy
    namespace: agentgateway-system
  rules:
  - matches:
    - path:
        type: PathPrefix
        value: /openai
    backendRefs:
    - name: openai
      namespace: agentgateway-system
      group: agentgateway.dev
      kind: AgentgatewayBackend
```

- [ ] **Step 2: Create Anthropic backend manifest**

```yaml
# manifests/llm-backends/anthropic.yaml
apiVersion: v1
kind: Secret
metadata:
  name: anthropic-secret
  namespace: agentgateway-system
type: Opaque
stringData:
  Authorization: __ANTHROPIC_API_KEY__
---
apiVersion: agentgateway.dev/v1alpha1
kind: AgentgatewayBackend
metadata:
  name: anthropic
  namespace: agentgateway-system
spec:
  ai:
    provider:
      anthropic:
        model: claude-sonnet-4-6
  policies:
    auth:
      secretRef:
        name: anthropic-secret
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: anthropic
  namespace: agentgateway-system
spec:
  parentRefs:
  - name: agentgateway-proxy
    namespace: agentgateway-system
  rules:
  - matches:
    - path:
        type: PathPrefix
        value: /anthropic
    backendRefs:
    - name: anthropic
      namespace: agentgateway-system
      group: agentgateway.dev
      kind: AgentgatewayBackend
```

- [ ] **Step 3: Commit**

```bash
git add manifests/llm-backends/
git commit -m "feat: add OpenAI and Anthropic LLM backend manifests"
```

---

## Phase 2: Wiki Server

### Task 4: Wiki server Go project setup

**Files:**
- Create: `wiki-server/go.mod`
- Create: `wiki-server/main.go`
- Create: `wiki-server/search.go`
- Create: `wiki-server/search_test.go`
- Create: `wiki-server/Dockerfile`

- [ ] **Step 1: Initialize Go module**

```bash
cd wiki-server
go mod init github.com/solo-io/solo-bank-demo/wiki-server
```

Create `wiki-server/go.mod`:
```
module github.com/solo-io/solo-bank-demo/wiki-server

go 1.22
```

- [ ] **Step 2: Write search tests**

```go
// wiki-server/search_test.go
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

	// "credit" appears in all 3 pages
	if postings, ok := idx.terms["credit"]; !ok || len(postings) != 3 {
		t.Errorf("expected 'credit' in 3 pages, got %v", idx.terms["credit"])
	}

	// "john" appears in 1 page
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
```

- [ ] **Step 3: Run tests to verify they fail**

```bash
cd wiki-server && go test -v ./...
```

Expected: Compilation failure — `buildIndex` and `search` not defined yet.

- [ ] **Step 4: Implement search engine**

```go
// wiki-server/search.go
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
	pages map[string]string    // path -> full content
	terms map[string][]posting // term -> list of postings
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
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd wiki-server && go test -v ./...
```

Expected: All 3 tests pass.

- [ ] **Step 6: Implement wiki server main.go**

```go
// wiki-server/main.go
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
		// Strip "content/" prefix for the page path
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
		// List categories
		categories := listCategories()
		writeJSON(w, categories)
		return
	}

	// Check if it's a page request (has a slash = category/page)
	if strings.Contains(path, "/") {
		// Try exact page
		if content, ok := idx.pages[path]; ok {
			w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
			fmt.Fprint(w, content)
			return
		}
		http.NotFound(w, r)
		return
	}

	// It's a category — list pages in it
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
```

- [ ] **Step 7: Create Dockerfile**

```dockerfile
# wiki-server/Dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download 2>/dev/null || true
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o wiki-server .

FROM alpine:3.19
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/wiki-server /wiki-server
EXPOSE 8080
ENTRYPOINT ["/wiki-server"]
```

- [ ] **Step 8: Commit**

```bash
git add wiki-server/
git commit -m "feat: add wiki server with search engine and HTTP API"
```

---

### Task 5: Bank wiki content — Policies, rates, products, procedures

**Files:**
- Create: `wiki-server/content/policies/*.md` (10 files)
- Create: `wiki-server/content/rates/*.md` (4 files)
- Create: `wiki-server/content/products/*.md` (8 files)
- Create: `wiki-server/content/procedures/*.md` (5 files)

These form the foundation that customer profiles reference. Must be created first to ensure consistency.

- [ ] **Step 1: Create credit score tiers policy**

```markdown
<!-- wiki-server/content/policies/credit-score-tiers.md -->
# Solo Bank — Credit Score Tier Definitions

## Overview
Solo Bank uses a five-tier credit classification system for all lending and credit decisions. These tiers determine product eligibility, interest rates, credit limits, and required documentation levels.

## Tier Definitions

### Tier 1: Excellent (800–850)
- **Risk Rating:** Very Low
- **Mortgage Eligibility:** All products including Jumbo loans
- **Credit Card Eligibility:** All cards including Platinum Rewards
- **Rate Adjustment:** Base rate + 0.000%
- **Max Credit Limit Formula:** 50% of annual salary
- **Documentation:** Standard
- **Approval Authority:** Auto-approve up to $750,000

### Tier 2: Very Good (740–799)
- **Risk Rating:** Low
- **Mortgage Eligibility:** All standard products, Jumbo with additional documentation
- **Credit Card Eligibility:** All cards
- **Rate Adjustment:** Base rate + 0.250%
- **Max Credit Limit Formula:** 40% of annual salary
- **Documentation:** Standard
- **Approval Authority:** Auto-approve up to $500,000

### Tier 3: Good (670–739)
- **Risk Rating:** Moderate
- **Mortgage Eligibility:** Conventional 30yr/15yr Fixed and 5/1 ARM only
- **Credit Card Eligibility:** Cashback Card and Secured Card
- **Rate Adjustment:** Base rate + 0.750%
- **Max Credit Limit Formula:** 30% of annual salary
- **Documentation:** Enhanced (pay stubs, 2 years tax returns)
- **Approval Authority:** Auto-approve up to $300,000, manager approval above

### Tier 4: Fair (580–669)
- **Risk Rating:** Elevated
- **Mortgage Eligibility:** Conventional 30yr Fixed only, minimum 10% down
- **Credit Card Eligibility:** Secured Card only
- **Rate Adjustment:** Base rate + 1.500%
- **Max Credit Limit Formula:** 20% of annual salary, max $10,000
- **Documentation:** Full (pay stubs, 2 years tax returns, bank statements, employment verification)
- **Approval Authority:** Manager approval required for all amounts

### Tier 5: Poor (300–579)
- **Risk Rating:** High
- **Mortgage Eligibility:** Not eligible
- **Credit Card Eligibility:** Secured Card only (with $500 minimum deposit)
- **Rate Adjustment:** Base rate + 3.000%
- **Max Credit Limit Formula:** Equal to security deposit, max $5,000
- **Documentation:** Full plus co-signer required for amounts over $2,000
- **Approval Authority:** Senior manager approval required

## Rate Adjustment Examples
For a base mortgage rate of 6.125% (30yr Fixed, Tier 1):
- Tier 1 (Excellent): 6.125%
- Tier 2 (Very Good): 6.375%
- Tier 3 (Good): 6.875%
- Tier 4 (Fair): 7.625%
- Tier 5 (Poor): Not eligible

## Relationship Discounts
Customers with 5+ years at Solo Bank receive a 0.125% rate discount. Customers with 10+ years receive 0.250%. Discounts apply after tier adjustment and cannot reduce rate below base.

## Score Update Policy
Credit scores are refreshed quarterly. Tier changes take effect on the next billing cycle. Rate adjustments on existing variable-rate products apply within 30 days of tier change.
```

- [ ] **Step 2: Create mortgage lending policy**

```markdown
<!-- wiki-server/content/policies/mortgage-lending.md -->
# Solo Bank — Mortgage Lending Policy

## Eligibility Requirements

### Minimum Requirements (All Loan Types)
- Minimum credit score: 580 (Tier 4)
- Minimum employment history: 2 years continuous
- Maximum debt-to-income ratio (DTI): 43%
- U.S. citizenship or permanent residency required
- Property must be primary residence, second home, or investment (different rules apply)

### Debt-to-Income Calculation
DTI = (Total Monthly Debt Payments + Proposed Mortgage Payment) / Gross Monthly Income
- Total Monthly Debt includes: existing mortgages, car loans, student loans, minimum credit card payments, child support/alimony
- Does NOT include: utilities, insurance premiums, groceries, discretionary spending

### Down Payment Requirements
| Credit Tier | Conventional | Jumbo |
|------------|-------------|-------|
| Tier 1 (Excellent) | 5% minimum | 10% minimum |
| Tier 2 (Very Good) | 5% minimum | 15% minimum |
| Tier 3 (Good) | 10% minimum | Not eligible |
| Tier 4 (Fair) | 10% minimum | Not eligible |
| Tier 5 (Poor) | Not eligible | Not eligible |

PMI (Private Mortgage Insurance) required for down payments below 20%.

### Loan Limits
- Conventional: Up to $766,550 (2026 conforming limit)
- Jumbo: $766,551 to $2,500,000
- Maximum loan-to-value (LTV): 95% for Conventional, 90% for Jumbo

### Property Requirements
- Independent appraisal required (Solo Bank approved appraiser list)
- Title insurance required
- Flood zone properties require flood insurance
- Condos must be on approved condo list

## Required Documentation
1. Government-issued photo ID
2. Last 2 years of W-2 forms (or 2 years tax returns for self-employed)
3. Last 30 days of pay stubs
4. Last 2 months of bank statements (all accounts)
5. Employment verification letter (for Tier 3 and below)
6. Gift letter (if using gifted funds for down payment)

## Approval Process
1. **Pre-qualification** — Soft credit pull, income estimate, basic eligibility check
2. **Application** — Full application with all documentation
3. **Processing** — Document verification, employment check, appraisal order
4. **Underwriting** — Full risk assessment against policy
5. **Conditional Approval** — May require additional documentation
6. **Clear to Close** — All conditions satisfied
7. **Closing** — Document signing, fund disbursement

Target timeline: 30-45 days from application to closing.

## Refinancing
Existing Solo Bank mortgage holders may refinance if:
- Current on all payments (no 30+ day late payments in last 12 months)
- Minimum 12 months since original closing
- Net tangible benefit demonstrated (rate reduction of 0.5%+ or term reduction)
- All standard eligibility requirements met at current credit tier

## Rate Lock
- Rate locks available for 30, 45, or 60 days
- One free float-down if rates decrease by 0.25%+ before closing
- Lock extension: $500 fee per 15-day extension
```

- [ ] **Step 3: Create remaining policy files**

Create `wiki-server/content/policies/interest-rate-schedule.md` — How rates are calculated with base rate + tier adjustment + relationship discount + product adjustment. Include full formula and worked examples.

Create `wiki-server/content/policies/credit-card-products.md` — Product matrix for Platinum Rewards Card (Tier 1-2, $195 annual fee, 3x points dining/travel, 18.99%-24.99% APR), Cashback Card (Tier 1-3, no annual fee, 2% cashback, 19.99%-26.99% APR), Secured Card (All tiers, $39 annual fee, 1% cashback, 22.99% APR fixed). Credit limit formulas per tier.

Create `wiki-server/content/policies/overdraft-policy.md` — Standard overdraft ($35 fee, max 3/day), Overdraft Protection (linked savings transfer, $12 fee), Overdraft Line of Credit (variable rate, $0 transfer fee). Opt-in required.

Create `wiki-server/content/policies/kyc-aml-compliance.md` — KYC requirements (government ID, SSN verification, address verification, beneficial ownership for business accounts). AML monitoring: flag transactions over $10,000, multiple transactions totaling $10,000+ in 24 hours, wire transfers to high-risk jurisdictions, rapid movement of funds through multiple accounts. SAR filing: within 30 days of detection. Enhanced due diligence for PEPs, customers with prior SARs, high-risk business types.

Create `wiki-server/content/policies/fee-schedule.md` — Monthly maintenance ($12 Basic Checking, $0 Premium Checking with $5,000 min balance), ATM ($0 in-network, $3 out-of-network), Wire Transfer ($25 domestic, $45 international), Stop Payment ($30), Account Closure ($0 if open 90+ days, $25 if less), Returned Check ($35), Statement Copy ($5), Cashier's Check ($10).

Create `wiki-server/content/policies/account-types.md` — Basic Checking, Premium Checking, Basic Savings (0.50% APY), High-Yield Savings (4.25% APY, $1,000 min), Money Market (4.50% APY, $10,000 min, 6 withdrawals/month), CDs (various terms).

Create `wiki-server/content/policies/fraud-detection.md` — Velocity checks (5+ transactions in 10 minutes), geographic anomaly (transactions in multiple states within 1 hour), merchant category flags (crypto exchanges, gambling for Tier 4-5), large cash withdrawals (over $5,000 require manager approval), card-not-present transactions exceeding 3x average. Auto-block triggers, manual review queue process, customer notification procedures.

Create `wiki-server/content/policies/customer-service-escalation.md` — L1 (frontline, max $100 fee waiver, max $500 dispute credit), L2 (supervisor, max $500 fee waiver, max $5,000 dispute credit, can initiate fraud investigation), L3 (manager, unlimited authority, regulatory complaint handling, legal liaison). SLAs: L1 respond within 24 hours, L2 within 4 hours, L3 within 1 hour.

Each file should be 60-150 lines of detailed, internally consistent markdown.

- [ ] **Step 4: Create rate tables**

Create `wiki-server/content/rates/mortgage-rates.md`:
```markdown
# Solo Bank — Current Mortgage Rates
*Effective: April 1, 2026*

## 30-Year Fixed Rate Mortgage
| Credit Tier | Rate | APR | Points |
|------------|------|-----|--------|
| Tier 1 (Excellent, 800+) | 6.125% | 6.234% | 0.0 |
| Tier 2 (Very Good, 740-799) | 6.375% | 6.487% | 0.0 |
| Tier 3 (Good, 670-739) | 6.875% | 6.992% | 0.5 |
| Tier 4 (Fair, 580-669) | 7.500% | 7.624% | 1.0 |

## 15-Year Fixed Rate Mortgage
| Credit Tier | Rate | APR | Points |
|------------|------|-----|--------|
| Tier 1 (Excellent, 800+) | 5.375% | 5.491% | 0.0 |
| Tier 2 (Very Good, 740-799) | 5.625% | 5.744% | 0.0 |
| Tier 3 (Good, 670-739) | 6.125% | 6.250% | 0.5 |
| Tier 4 (Fair, 580-669) | 6.750% | 6.883% | 1.0 |

## 5/1 Adjustable Rate Mortgage (ARM)
| Credit Tier | Initial Rate | APR | Caps (Initial/Annual/Lifetime) |
|------------|-------------|-----|-------------------------------|
| Tier 1 (Excellent, 800+) | 5.750% | 6.892% | 2/2/5 |
| Tier 2 (Very Good, 740-799) | 6.000% | 7.145% | 2/2/5 |
| Tier 3 (Good, 670-739) | 6.500% | 7.659% | 2/2/5 |

## Jumbo 30-Year Fixed (Loan amounts $766,551+)
| Credit Tier | Rate | APR | Points |
|------------|------|-----|--------|
| Tier 1 (Excellent, 800+) | 6.500% | 6.612% | 0.0 |
| Tier 2 (Very Good, 740-799) | 6.750% | 6.865% | 0.25 |

## Relationship Discounts
- 5-9 years as Solo Bank customer: -0.125%
- 10+ years as Solo Bank customer: -0.250%
- Auto-pay from Solo Bank checking: -0.125%
- Discounts applied after tier rate, cannot go below Tier 1 base rate.

*Rates subject to change without notice. Contact a mortgage advisor for a personalized quote.*
```

Create `wiki-server/content/rates/savings-rates.md` — APY by account type and balance tier. Basic Savings: 0.50% all balances. High-Yield Savings: 4.25% ($1K-$49K), 4.35% ($50K-$99K), 4.50% ($100K+). Money Market: 4.50% ($10K-$49K), 4.65% ($50K-$99K), 4.75% ($100K+).

Create `wiki-server/content/rates/cd-rates.md` — CD rates by term: 3-month 4.75%, 6-month 4.85%, 12-month 4.50%, 18-month 4.25%, 24-month 4.10%, 36-month 4.00%, 60-month 3.90%. Minimum $1,000. Early withdrawal penalties.

Create `wiki-server/content/rates/credit-card-apr.md` — APR by card and tier. Platinum Rewards: Tier 1 18.99%, Tier 2 20.99%. Cashback: Tier 1 19.99%, Tier 2 21.99%, Tier 3 24.99%. Secured: 22.99% fixed all tiers. Penalty APR: 29.99% (triggered after 60 days delinquent). Promotional: 0% APR for 12 months on balance transfers (Platinum only).

- [ ] **Step 5: Create product descriptions**

Create 8 files in `wiki-server/content/products/`:
- `platinum-rewards-card.md` — Premium card, $195/yr, 3x points dining/travel, 2x groceries, 1x everything else, airport lounge access, travel insurance
- `cashback-card.md` — No annual fee, 2% unlimited cashback, $200 bonus after $1,000 spend in 90 days
- `secured-card.md` — $39/yr, requires security deposit equal to credit limit, reports to all 3 bureaus, upgrade path after 12 months good standing
- `premium-checking.md` — No monthly fee with $5,000 min balance, free checks, ATM fee rebates, free cashier's checks, dedicated service line
- `basic-checking.md` — $12/month (waived with $1,500 min balance or direct deposit), free debit card, mobile banking, 1 free checkbook/year
- `high-yield-savings.md` — 4.25% APY, $1,000 minimum, 6 free withdrawals/month, no monthly fee
- `basic-savings.md` — 0.50% APY, no minimum, $5/month fee waived with $300 min balance, 6 withdrawals/month
- `home-equity-loc.md` — Variable rate (Prime + 1.0% to Prime + 3.5% by tier), 10-year draw period, 20-year repayment, 80% max CLTV, no closing costs on lines over $50,000

- [ ] **Step 6: Create procedure documents**

Create 5 files in `wiki-server/content/procedures/`:
- `new-account-opening.md` — Step-by-step: verify identity, check OFAC list, run ChexSystems, select product, fund account, order debit card, set up online banking
- `dispute-resolution.md` — Customer initiates, L1 logs dispute, provisional credit within 10 business days, investigation (45 days debit, 60 days credit), resolution letter
- `loan-application-process.md` — Pre-qual, full application, processing, underwriting, conditional approval, clear to close, closing. Checklist for each step
- `wire-transfer-procedures.md` — Domestic (same-day if before 4pm ET, $25 fee), International (2-3 business days, $45 fee, SWIFT required), verification call for amounts over $25,000
- `account-closure.md` — Verify identity, check outstanding holds/pending transactions, transfer remaining balance, cancel linked services, send confirmation letter, retain records 7 years

Each file should be 50-100 lines.

- [ ] **Step 7: Commit**

```bash
git add wiki-server/content/policies/ wiki-server/content/rates/ wiki-server/content/products/ wiki-server/content/procedures/
git commit -m "feat: add Solo Bank policies, rates, products, and procedures content"
```

---

### Task 6: Bank wiki content — 100 customer profiles

**Files:**
- Create: `wiki-server/content/customers/*.md` (100 files)

This is the largest content task. Each customer must be internally consistent with the policies and rate tables from Task 5.

- [ ] **Step 1: Create a Go content generator script**

Rather than hand-writing 100 files, create a one-time generator that produces consistent customer profiles.

Create `wiki-server/cmd/generate-customers/main.go`:

```go
package main

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Customer data model for generation
type Customer struct {
	ID           int
	FirstName    string
	LastName     string
	Age          int
	DOB          string
	Employer     string
	JobTitle     string
	Salary       int
	CreditScore  int
	CustomerSince string
	RiskRating   string
	Accounts     []Account
	CreditCards  []CreditCard
	Mortgage     *Mortgage
	Transactions []Transaction
	Flags        []string
	Notes        []string
}

type Account struct {
	Type    string
	ID      string
	Balance float64
	APY     float64
}

type CreditCard struct {
	Product      string
	ID           string
	Limit        int
	Balance      float64
	APR          float64
	PaymentHist  string
}

type Mortgage struct {
	Type         string
	Rate         float64
	Principal    int
	Property     string
	MonthlyPay   float64
	RemainingYrs int
}

type Transaction struct {
	Date        string
	Description string
	Amount      float64
	Account     string
}

func main() {
	outDir := "content/customers"
	os.MkdirAll(outDir, 0755)

	rng := rand.New(rand.NewSource(42)) // deterministic for reproducibility

	firstNames := []string{"James", "Maria", "Robert", "Jennifer", "Michael", "Linda", "David", "Patricia", "William", "Elizabeth", "Richard", "Barbara", "Joseph", "Susan", "Thomas", "Jessica", "Charles", "Sarah", "Christopher", "Karen", "Daniel", "Lisa", "Matthew", "Nancy", "Anthony", "Betty", "Mark", "Margaret", "Donald", "Sandra", "Steven", "Ashley", "Andrew", "Dorothy", "Paul", "Kimberly", "Joshua", "Emily", "Kenneth", "Donna", "Kevin", "Michelle", "Brian", "Carol", "George", "Amanda", "Timothy", "Melissa", "Ronald", "Deborah", "Edward", "Stephanie", "Jason", "Rebecca", "Jeffrey", "Sharon", "Ryan", "Laura", "Jacob", "Cynthia", "Gary", "Kathleen", "Nicholas", "Amy", "Eric", "Angela", "Jonathan", "Shirley", "Stephen", "Anna", "Larry", "Brenda", "Justin", "Pamela", "Scott", "Emma", "Brandon", "Nicole", "Benjamin", "Helen", "Samuel", "Samantha", "Raymond", "Katherine", "Gregory", "Christine", "Frank", "Debra", "Alexander", "Rachel", "Patrick", "Carolyn", "Jack", "Janet", "Dennis", "Catherine", "Jerry", "Maria-Luisa", "Tyler", "Heather"}
	lastNames := []string{"Smith", "Johnson", "Williams", "Brown", "Jones", "Garcia", "Miller", "Davis", "Rodriguez", "Martinez", "Hernandez", "Lopez", "Gonzalez", "Wilson", "Anderson", "Thomas", "Taylor", "Moore", "Jackson", "Martin", "Lee", "Perez", "Thompson", "White", "Harris", "Sanchez", "Clark", "Ramirez", "Lewis", "Robinson", "Walker", "Young", "Allen", "King", "Wright", "Scott", "Torres", "Nguyen", "Hill", "Flores", "Green", "Adams", "Nelson", "Baker", "Hall", "Rivera", "Campbell", "Mitchell", "Carter", "Roberts", "Gomez", "Phillips", "Evans", "Turner", "Diaz", "Parker", "Cruz", "Edwards", "Collins", "Reyes", "Stewart", "Morris", "Morales", "Murphy", "Cook", "Rogers", "Gutierrez", "Ortiz", "Morgan", "Cooper", "Peterson", "Bailey", "Reed", "Kelly", "Howard", "Ramos", "Kim", "Cox", "Ward", "Richardson", "Watson", "Brooks", "Chavez", "Wood", "James", "Bennett", "Gray", "Mendoza", "Ruiz", "Hughes", "Price", "Alvarez", "Castillo", "Sanders", "Patel", "Myers", "Long", "Ross", "Foster"}
	employers := []string{"TechCorp Inc", "Midwest Manufacturing", "Springfield Medical Center", "State University", "City of Springfield", "AutoNation Group", "Greenfield Elementary School", "Solo Financial Services", "Pacific Northwest Energy", "Continental Logistics", "Riverside Construction", "Summit Healthcare", "DataStream Analytics", "Metro Transit Authority", "Brightstar Retail", "United Agricultural Co-op", "Horizon Aerospace", "Community Legal Aid", "FreshMart Groceries", "Atlas Engineering"}
	jobTitles := []string{"Senior Software Engineer", "Registered Nurse", "High School Teacher", "Accountant", "Sales Manager", "Administrative Assistant", "Electrician", "Marketing Director", "Warehouse Supervisor", "Dental Hygienist", "Project Manager", "Restaurant Manager", "Mechanical Engineer", "Human Resources Coordinator", "Truck Driver", "Financial Analyst", "Retail Associate", "Pharmacist", "Construction Foreman", "Graphic Designer"}
	streets := []string{"Oak Lane", "Maple Drive", "Cedar Avenue", "Elm Street", "Pine Road", "Birch Court", "Walnut Way", "Cherry Blvd", "Spruce Circle", "Ash Place", "Willow Lane", "Poplar Drive", "Hickory Street", "Chestnut Avenue", "Sycamore Road"}
	cities := []string{"Springfield", "Shelbyville", "Capital City", "Ogdenville", "North Haverbrook", "Cypress Creek", "Brockway"}

	// Credit score distribution (normal-ish, centered ~700)
	// Poor (520-579): 8, Fair (580-669): 20, Good (670-739): 35, Very Good (740-799): 25, Excellent (800-830): 12
	creditScores := make([]int, 0, 100)
	for i := 0; i < 8; i++ {
		creditScores = append(creditScores, 520+rng.Intn(60))
	}
	for i := 0; i < 20; i++ {
		creditScores = append(creditScores, 580+rng.Intn(90))
	}
	for i := 0; i < 35; i++ {
		creditScores = append(creditScores, 670+rng.Intn(70))
	}
	for i := 0; i < 25; i++ {
		creditScores = append(creditScores, 740+rng.Intn(60))
	}
	for i := 0; i < 12; i++ {
		creditScores = append(creditScores, 800+rng.Intn(31))
	}
	rng.Shuffle(len(creditScores), func(i, j int) {
		creditScores[i], creditScores[j] = creditScores[j], creditScores[i]
	})

	for i := 0; i < 100; i++ {
		c := generateCustomer(rng, i+1, firstNames, lastNames, employers, jobTitles, streets, cities, creditScores[i])
		md := renderCustomerMarkdown(c)
		filename := fmt.Sprintf("%s-%s.md", strings.ToLower(c.FirstName), strings.ToLower(c.LastName))
		filename = strings.ReplaceAll(filename, " ", "-")
		path := filepath.Join(outDir, filename)
		os.WriteFile(path, []byte(md), 0644)
		fmt.Printf("Generated: %s (Credit: %d, Salary: $%d)\n", filename, c.CreditScore, c.Salary)
	}
}

func generateCustomer(rng *rand.Rand, id int, firstNames, lastNames, employers, jobTitles, streets, cities []string, creditScore int) Customer {
	firstName := firstNames[id-1]
	lastName := lastNames[id-1]
	age := 22 + rng.Intn(48) // 22-69

	// Salary correlates loosely with credit score
	baseSalary := 25000 + rng.Intn(40000)
	if creditScore >= 740 {
		baseSalary += 40000 + rng.Intn(80000)
	} else if creditScore >= 670 {
		baseSalary += 20000 + rng.Intn(50000)
	} else if creditScore >= 580 {
		baseSalary += rng.Intn(30000)
	}
	salary := (baseSalary / 1000) * 1000 // round to nearest thousand

	tier := getTier(creditScore)
	riskRating := tierToRisk(tier)
	yearsCustomer := 1 + rng.Intn(20)
	customerSince := fmt.Sprintf("%d-%02d-%02d", 2026-yearsCustomer, 1+rng.Intn(12), 1+rng.Intn(28))

	dob := time.Date(2026-age, time.Month(1+rng.Intn(12)), 1+rng.Intn(28), 0, 0, 0, 0, time.UTC)

	c := Customer{
		ID:            id,
		FirstName:     firstName,
		LastName:      lastName,
		Age:           age,
		DOB:           dob.Format("2006-01-02"),
		Employer:      employers[rng.Intn(len(employers))],
		JobTitle:      jobTitles[rng.Intn(len(jobTitles))],
		Salary:        salary,
		CreditScore:   creditScore,
		CustomerSince: customerSince,
		RiskRating:    riskRating,
	}

	// All customers get checking
	checkingBal := float64(500+rng.Intn(30000)) + float64(rng.Intn(100))/100
	if creditScore >= 740 {
		checkingBal += float64(rng.Intn(20000))
	}
	checkingType := "Basic Checking"
	if checkingBal > 5000 {
		checkingType = "Premium Checking"
	}
	c.Accounts = append(c.Accounts, Account{
		Type:    checkingType,
		ID:      fmt.Sprintf("ACC-%05d1", id),
		Balance: checkingBal,
	})

	// 90% get savings
	if rng.Float64() < 0.90 {
		savingsType := "Basic Savings"
		savingsAPY := 0.50
		savingsBal := float64(200 + rng.Intn(10000))
		if creditScore >= 670 && rng.Float64() < 0.6 {
			savingsType = "High-Yield Savings"
			savingsAPY = 4.25
			savingsBal += float64(rng.Intn(80000))
		}
		c.Accounts = append(c.Accounts, Account{
			Type:    savingsType,
			ID:      fmt.Sprintf("ACC-%05d2", id),
			Balance: savingsBal,
			APY:     savingsAPY,
		})
	}

	// 85% get credit cards (based on tier eligibility)
	if rng.Float64() < 0.85 {
		card := generateCreditCard(rng, id, creditScore, salary, tier)
		c.CreditCards = append(c.CreditCards, card)
	}

	// 5% get multiple credit cards (only Tier 1-2)
	if tier <= 2 && rng.Float64() < 0.15 {
		card2 := generateCreditCard(rng, id+100, creditScore, salary, tier)
		c.CreditCards = append(c.CreditCards, card2)
	}

	// 60% have mortgages (only Tier 1-4)
	if tier <= 4 && rng.Float64() < 0.60 {
		c.Mortgage = generateMortgage(rng, creditScore, salary, tier, streets, cities)
	}

	// Generate transactions
	c.Transactions = generateTransactions(rng, c)

	// 10% have flags
	if id <= 10 { // deterministic: first 10 IDs
		switch {
		case id <= 4:
			c.Flags = append(c.Flags, "FRAUD_ALERT")
			c.Notes = append(c.Notes, "Fraud alert placed on account after suspicious ATM withdrawals in multiple states")
		case id <= 7:
			c.Flags = append(c.Flags, "DISPUTE_IN_PROGRESS")
			c.Notes = append(c.Notes, "Active dispute on credit card transaction — provisional credit issued")
		default:
			c.Flags = append(c.Flags, "OVERDUE_PAYMENT")
			c.Notes = append(c.Notes, "Mortgage payment 45 days past due — L2 escalation initiated")
		}
	}

	// Additional notes
	if yearsCustomer >= 10 {
		c.Notes = append(c.Notes, "Eligible for loyalty rate discount (0.250%)")
	} else if yearsCustomer >= 5 {
		c.Notes = append(c.Notes, "Eligible for loyalty rate discount (0.125%)")
	}
	contacts := []string{"email", "phone", "text message"}
	c.Notes = append(c.Notes, fmt.Sprintf("Preferred contact: %s", contacts[rng.Intn(len(contacts))]))

	return c
}

func getTier(score int) int {
	switch {
	case score >= 800:
		return 1
	case score >= 740:
		return 2
	case score >= 670:
		return 3
	case score >= 580:
		return 4
	default:
		return 5
	}
}

func tierToRisk(tier int) string {
	switch tier {
	case 1:
		return "Very Low"
	case 2:
		return "Low"
	case 3:
		return "Moderate"
	case 4:
		return "Elevated"
	default:
		return "High"
	}
}

func generateCreditCard(rng *rand.Rand, id, creditScore, salary, tier int) CreditCard {
	var product string
	var apr float64
	var limit int

	switch {
	case tier <= 2 && rng.Float64() < 0.6:
		product = "Platinum Rewards Card"
		if tier == 1 {
			apr = 18.99
		} else {
			apr = 20.99
		}
		limit = int(float64(salary) * 0.4)
	case tier <= 3:
		product = "Cashback Card"
		switch tier {
		case 1:
			apr = 19.99
		case 2:
			apr = 21.99
		default:
			apr = 24.99
		}
		limit = int(float64(salary) * 0.3)
	default:
		product = "Secured Card"
		apr = 22.99
		limit = 2000 + rng.Intn(3000)
	}

	// Round limit to nearest 500
	limit = (limit / 500) * 500
	if limit < 500 {
		limit = 500
	}

	balance := float64(rng.Intn(limit))
	if rng.Float64() < 0.3 {
		balance = 0 // 30% chance of zero balance
	}

	paymentHist := "Excellent (never missed)"
	if tier >= 4 && rng.Float64() < 0.3 {
		paymentHist = "Fair (2 late payments in last 12 months)"
	}

	return CreditCard{
		Product:     product,
		ID:          fmt.Sprintf("CC-%05d", id),
		Limit:       limit,
		Balance:     balance,
		APR:         apr,
		PaymentHist: paymentHist,
	}
}

func generateMortgage(rng *rand.Rand, creditScore, salary, tier int, streets, cities []string) *Mortgage {
	mortgageTypes := []string{"30yr Fixed", "15yr Fixed", "5/1 ARM"}
	if tier >= 4 {
		mortgageTypes = []string{"30yr Fixed"} // Fair tier: 30yr only
	}
	mType := mortgageTypes[rng.Intn(len(mortgageTypes))]

	var rate float64
	switch mType {
	case "30yr Fixed":
		rate = []float64{6.125, 6.375, 6.875, 7.500}[tier-1]
	case "15yr Fixed":
		rate = []float64{5.375, 5.625, 6.125, 6.750}[tier-1]
	case "5/1 ARM":
		rate = []float64{5.750, 6.000, 6.500}[tier-1]
	}

	principal := 150000 + rng.Intn(400000)
	if tier <= 2 {
		principal += rng.Intn(300000)
	}
	principal = (principal / 5000) * 5000

	monthlyRate := rate / 100 / 12
	var numPayments float64
	if strings.HasPrefix(mType, "15") {
		numPayments = 180
	} else {
		numPayments = 360
	}
	monthlyPay := float64(principal) * (monthlyRate * pow(1+monthlyRate, numPayments)) / (pow(1+monthlyRate, numPayments) - 1)

	addr := fmt.Sprintf("%d %s, %s, IL", 100+rng.Intn(9900), streets[rng.Intn(len(streets))], cities[rng.Intn(len(cities))])

	return &Mortgage{
		Type:         mType,
		Rate:         rate,
		Principal:    principal,
		Property:     addr,
		MonthlyPay:   monthlyPay,
		RemainingYrs: 5 + rng.Intn(25),
	}
}

func pow(base, exp float64) float64 {
	result := 1.0
	for i := 0; i < int(exp); i++ {
		result *= base
	}
	return result
}

func generateTransactions(rng *rand.Rand, c Customer) []Transaction {
	var txns []Transaction
	merchants := []string{"Whole Foods Market", "Amazon.com", "Shell Gas Station", "Target", "Starbucks", "Netflix", "Spotify", "CVS Pharmacy", "Home Depot", "Costco", "AT&T Wireless", "Electric Company", "Water Utility", "Uber", "DoorDash", "Chipotle", "Best Buy", "Nordstrom", "Planet Fitness", "Walmart"}

	numTxns := 10 + rng.Intn(11) // 10-20 transactions
	for i := 0; i < numTxns; i++ {
		day := 1 + rng.Intn(30)
		date := fmt.Sprintf("2026-04-%02d", day)
		merchant := merchants[rng.Intn(len(merchants))]
		amount := -1 * (5.0 + float64(rng.Intn(200)) + float64(rng.Intn(100))/100)

		acctType := "Checking"
		if len(c.CreditCards) > 0 && rng.Float64() < 0.4 {
			acctType = "Credit Card"
		}

		txns = append(txns, Transaction{
			Date:        date,
			Description: merchant,
			Amount:      amount,
			Account:     acctType,
		})
	}

	// Add direct deposit
	payPerPeriod := float64(c.Salary) / 24 // semi-monthly
	txns = append(txns, Transaction{
		Date:        "2026-04-01",
		Description: fmt.Sprintf("Direct Deposit - %s", c.Employer),
		Amount:      payPerPeriod,
		Account:     "Checking",
	})
	txns = append(txns, Transaction{
		Date:        "2026-04-15",
		Description: fmt.Sprintf("Direct Deposit - %s", c.Employer),
		Amount:      payPerPeriod,
		Account:     "Checking",
	})

	// Sort by date desc (simple string sort works for YYYY-MM-DD)
	for i := 0; i < len(txns); i++ {
		for j := i + 1; j < len(txns); j++ {
			if txns[j].Date > txns[i].Date {
				txns[i], txns[j] = txns[j], txns[i]
			}
		}
	}

	return txns
}

func renderCustomerMarkdown(c Customer) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("# %s %s — Customer Profile\n\n", c.FirstName, c.LastName))
	b.WriteString("## Personal Information\n\n")
	b.WriteString(fmt.Sprintf("- **Customer ID:** CUST-%05d\n", c.ID))
	b.WriteString(fmt.Sprintf("- **Age:** %d | **DOB:** %s\n", c.Age, c.DOB))
	b.WriteString(fmt.Sprintf("- **Employment:** %s at %s\n", c.JobTitle, c.Employer))
	b.WriteString(fmt.Sprintf("- **Annual Salary:** $%s\n", formatMoney(float64(c.Salary))))
	b.WriteString(fmt.Sprintf("- **Credit Score:** %d (%s)\n", c.CreditScore, tierLabel(c.CreditScore)))
	b.WriteString(fmt.Sprintf("- **Customer Since:** %s\n", c.CustomerSince))
	b.WriteString(fmt.Sprintf("- **Risk Rating:** %s\n", c.RiskRating))

	if len(c.Flags) > 0 {
		b.WriteString(fmt.Sprintf("- **Active Flags:** %s\n", strings.Join(c.Flags, ", ")))
	}

	b.WriteString("\n## Accounts\n\n")
	for _, a := range c.Accounts {
		b.WriteString(fmt.Sprintf("### %s (%s)\n\n", a.Type, a.ID))
		b.WriteString(fmt.Sprintf("- **Balance:** $%s\n", formatMoney(a.Balance)))
		if a.APY > 0 {
			b.WriteString(fmt.Sprintf("- **APY:** %.2f%%\n", a.APY))
		}
		b.WriteString("\n")
	}

	if len(c.CreditCards) > 0 {
		b.WriteString("## Credit Cards\n\n")
		for _, cc := range c.CreditCards {
			b.WriteString(fmt.Sprintf("### %s (%s)\n\n", cc.Product, cc.ID))
			b.WriteString(fmt.Sprintf("- **Credit Limit:** $%s\n", formatMoney(float64(cc.Limit))))
			b.WriteString(fmt.Sprintf("- **Current Balance:** $%s\n", formatMoney(cc.Balance)))
			b.WriteString(fmt.Sprintf("- **APR:** %.2f%%\n", cc.APR))
			b.WriteString(fmt.Sprintf("- **Payment History:** %s\n", cc.PaymentHist))
			b.WriteString("\n")
		}
	}

	if c.Mortgage != nil {
		m := c.Mortgage
		b.WriteString("## Mortgage\n\n")
		b.WriteString(fmt.Sprintf("- **Type:** %s\n", m.Type))
		b.WriteString(fmt.Sprintf("- **Rate:** %.3f%%\n", m.Rate))
		b.WriteString(fmt.Sprintf("- **Original Principal:** $%s\n", formatMoney(float64(m.Principal))))
		b.WriteString(fmt.Sprintf("- **Property:** %s\n", m.Property))
		b.WriteString(fmt.Sprintf("- **Monthly Payment:** $%s\n", formatMoney(m.MonthlyPay)))
		b.WriteString(fmt.Sprintf("- **Remaining Term:** %d years\n", m.RemainingYrs))
		b.WriteString("\n")
	}

	b.WriteString("## Recent Transactions (Last 30 Days)\n\n")
	b.WriteString("| Date | Description | Amount | Account |\n")
	b.WriteString("|------|------------|--------|--------|\n")
	for _, t := range c.Transactions {
		sign := ""
		if t.Amount > 0 {
			sign = "+"
		}
		b.WriteString(fmt.Sprintf("| %s | %s | %s$%s | %s |\n", t.Date, t.Description, sign, formatMoney(abs(t.Amount)), t.Account))
	}

	if len(c.Notes) > 0 {
		b.WriteString("\n## Notes\n\n")
		for _, n := range c.Notes {
			b.WriteString(fmt.Sprintf("- %s\n", n))
		}
	}

	return b.String()
}

func tierLabel(score int) string {
	switch {
	case score >= 800:
		return "Excellent"
	case score >= 740:
		return "Very Good"
	case score >= 670:
		return "Good"
	case score >= 580:
		return "Fair"
	default:
		return "Poor"
	}
}

func formatMoney(amount float64) string {
	if amount < 0 {
		amount = -amount
	}
	whole := int(amount)
	cents := int((amount - float64(whole)) * 100)
	
	// Add commas
	s := fmt.Sprintf("%d", whole)
	if len(s) > 3 {
		var parts []string
		for len(s) > 3 {
			parts = append([]string{s[len(s)-3:]}, parts...)
			s = s[:len(s)-3]
		}
		parts = append([]string{s}, parts...)
		s = strings.Join(parts, ",")
	}
	return fmt.Sprintf("%s.%02d", s, cents)
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}
```

- [ ] **Step 2: Run the generator**

```bash
cd wiki-server && go run cmd/generate-customers/main.go
```

Expected: 100 `.md` files in `wiki-server/content/customers/`, each with complete profiles.

- [ ] **Step 3: Spot-check 3-5 generated profiles for consistency**

Verify:
- Credit card APRs match the tier in `rates/credit-card-apr.md`
- Mortgage rates match the tier in `rates/mortgage-rates.md`
- Account types match products in `products/`
- Flagged customers (IDs 1-10) have appropriate flags

- [ ] **Step 4: Commit**

```bash
git add wiki-server/content/customers/ wiki-server/cmd/
git commit -m "feat: add 100 generated customer profiles for Solo Bank"
```

---

## Phase 3: MCP Tool Servers

### Task 7: Shared wiki client library

**Files:**
- Create: `mcp-tools/shared/go.mod`
- Create: `mcp-tools/shared/wiki_client.go`
- Create: `mcp-tools/shared/wiki_client_test.go`

- [ ] **Step 1: Create Go module**

```
module github.com/solo-io/solo-bank-demo/mcp-tools/shared

go 1.22
```

- [ ] **Step 2: Write wiki client tests**

```go
// mcp-tools/shared/wiki_client_test.go
package shared

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetPage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/wiki/customers/john-smith" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/markdown")
		w.Write([]byte("# John Smith\nCredit Score: 782"))
	}))
	defer srv.Close()

	client := NewWikiClient(srv.URL)
	content, err := client.GetPage("customers/john-smith")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "# John Smith\nCredit Score: 782" {
		t.Errorf("unexpected content: %s", content)
	}
}

func TestSearch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("q") != "john" {
			t.Errorf("unexpected query: %s", r.URL.Query().Get("q"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"path":"customers/john-smith","title":"John Smith","snippet":"...","score":5.2}]`))
	}))
	defer srv.Close()

	client := NewWikiClient(srv.URL)
	results, err := client.Search("john")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 || results[0].Path != "customers/john-smith" {
		t.Errorf("unexpected results: %+v", results)
	}
}

func TestListPages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"name":"john-smith","title":"John Smith","path":"customers/john-smith"}]`))
	}))
	defer srv.Close()

	client := NewWikiClient(srv.URL)
	pages, err := client.ListPages("customers")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pages) != 1 {
		t.Errorf("expected 1 page, got %d", len(pages))
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

```bash
cd mcp-tools/shared && go test -v ./...
```

Expected: Compilation error — `NewWikiClient` not defined.

- [ ] **Step 4: Implement wiki client**

```go
// mcp-tools/shared/wiki_client.go
package shared

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type SearchResult struct {
	Path    string  `json:"path"`
	Title   string  `json:"title"`
	Snippet string  `json:"snippet"`
	Score   float64 `json:"score"`
}

type PageEntry struct {
	Name  string `json:"name"`
	Title string `json:"title"`
	Path  string `json:"path"`
}

type WikiClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewWikiClient(baseURL string) *WikiClient {
	return &WikiClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *WikiClient) GetPage(pagePath string) (string, error) {
	resp, err := c.httpClient.Get(fmt.Sprintf("%s/wiki/%s", c.baseURL, pagePath))
	if err != nil {
		return "", fmt.Errorf("wiki request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("page not found: %s", pagePath)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("wiki returned status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}
	return string(body), nil
}

func (c *WikiClient) Search(query string) ([]SearchResult, error) {
	resp, err := c.httpClient.Get(fmt.Sprintf("%s/search?q=%s", c.baseURL, url.QueryEscape(query)))
	if err != nil {
		return nil, fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()
	var results []SearchResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("failed to decode search results: %w", err)
	}
	return results, nil
}

func (c *WikiClient) ListPages(category string) ([]PageEntry, error) {
	resp, err := c.httpClient.Get(fmt.Sprintf("%s/wiki/%s", c.baseURL, category))
	if err != nil {
		return nil, fmt.Errorf("list request failed: %w", err)
	}
	defer resp.Body.Close()
	var entries []PageEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, fmt.Errorf("failed to decode page list: %w", err)
	}
	return entries, nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd mcp-tools/shared && go test -v ./...
```

Expected: All 3 tests pass.

- [ ] **Step 6: Commit**

```bash
git add mcp-tools/shared/
git commit -m "feat: add shared wiki client library for MCP tool servers"
```

---

### Task 8: Customer Data MCP tool server

**Files:**
- Create: `mcp-tools/customer-tools/go.mod`
- Create: `mcp-tools/customer-tools/main.go`
- Create: `mcp-tools/customer-tools/Dockerfile`

- [ ] **Step 1: Create Go module**

```
module github.com/solo-io/solo-bank-demo/mcp-tools/customer-tools

go 1.22

require (
    github.com/modelcontextprotocol/go-sdk v0.2.0
    github.com/solo-io/solo-bank-demo/mcp-tools/shared v0.0.0
)

replace github.com/solo-io/solo-bank-demo/mcp-tools/shared => ../shared
```

Run `go mod tidy` to resolve exact versions.

- [ ] **Step 2: Implement customer tools MCP server**

```go
// mcp-tools/customer-tools/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/solo-io/solo-bank-demo/mcp-tools/shared"
)

var wikiClient *shared.WikiClient

type LookupCustomerArgs struct {
	Name       string `json:"name,omitempty" jsonschema:"description=Customer name to look up (e.g. 'john smith')"`
	CustomerID string `json:"customer_id,omitempty" jsonschema:"description=Customer ID (e.g. 'CUST-00001')"`
}

type SearchCustomersArgs struct {
	Query string `json:"query" jsonschema:"description=Search query to find customers (e.g. 'credit score above 750' or 'software engineer'),required"`
}

type GetAccountBalanceArgs struct {
	AccountID string `json:"account_id" jsonschema:"description=Account ID (e.g. 'ACC-000011' or 'CC-00001'),required"`
}

type GetTransactionHistoryArgs struct {
	CustomerID string `json:"customer_id" jsonschema:"description=Customer ID (e.g. 'CUST-00001'),required"`
}

func main() {
	wikiURL := os.Getenv("WIKI_SERVER_URL")
	if wikiURL == "" {
		wikiURL = "http://bank-wiki-server.bank-wiki.svc.cluster.local:8080"
	}
	wikiClient = shared.NewWikiClient(wikiURL)

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "bank-customer-tools",
		Version: "1.0.0",
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "lookup_customer",
		Description: "Look up a Solo Bank customer profile by name or customer ID. Returns the complete customer profile including accounts, credit cards, mortgage details, and recent transactions.",
	}, handleLookupCustomer)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_customers",
		Description: "Search across all Solo Bank customer profiles. Returns matching customer summaries with relevance scores. Useful for finding customers by criteria like job title, employer, credit score range, or account flags.",
	}, handleSearchCustomers)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_account_balance",
		Description: "Get the current balance for a specific bank account or credit card by account ID.",
	}, handleGetAccountBalance)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_transaction_history",
		Description: "Get recent transaction history for a customer. Returns the last 30 days of transactions across all accounts.",
	}, handleGetTransactionHistory)

	handler := mcp.NewStreamableHTTPHandler(
		func(r *http.Request) *mcp.Server { return server },
		nil,
	)

	http.Handle("/mcp", handler)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})

	log.Println("Customer tools MCP server starting on :8081")
	if err := http.ListenAndServe(":8081", nil); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func handleLookupCustomer(ctx context.Context, req *mcp.CallToolRequest, args LookupCustomerArgs) (*mcp.CallToolResult, any, error) {
	if args.Name == "" && args.CustomerID == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent("Error: provide either 'name' or 'customer_id'")},
			IsError: true,
		}, nil, nil
	}

	if args.CustomerID != "" {
		// Search by customer ID
		results, err := wikiClient.Search(args.CustomerID)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error searching: %v", err))},
				IsError: true,
			}, nil, nil
		}
		if len(results) == 0 {
			return &mcp.CallToolResult{
				Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("No customer found with ID: %s", args.CustomerID))},
			}, nil, nil
		}
		content, err := wikiClient.GetPage(results[0].Path)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error fetching profile: %v", err))},
				IsError: true,
			}, nil, nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(content)},
		}, nil, nil
	}

	// Search by name — convert to filename format
	namePath := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(args.Name), " ", "-"))
	content, err := wikiClient.GetPage("customers/" + namePath)
	if err != nil {
		// Try search as fallback
		results, searchErr := wikiClient.Search(args.Name)
		if searchErr != nil || len(results) == 0 {
			return &mcp.CallToolResult{
				Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("No customer found with name: %s", args.Name))},
			}, nil, nil
		}
		content, err = wikiClient.GetPage(results[0].Path)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error fetching profile: %v", err))},
				IsError: true,
			}, nil, nil
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(content)},
	}, nil, nil
}

func handleSearchCustomers(ctx context.Context, req *mcp.CallToolRequest, args SearchCustomersArgs) (*mcp.CallToolResult, any, error) {
	results, err := wikiClient.Search(args.Query)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error searching: %v", err))},
			IsError: true,
		}, nil, nil
	}

	// Filter to only customer results
	var customerResults []shared.SearchResult
	for _, r := range results {
		if strings.HasPrefix(r.Path, "customers/") {
			customerResults = append(customerResults, r)
		}
	}

	if len(customerResults) == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent("No matching customers found.")},
		}, nil, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d matching customers:\n\n", len(customerResults)))
	for _, r := range customerResults {
		sb.WriteString(fmt.Sprintf("- **%s** (%s)\n  %s\n\n", r.Title, r.Path, r.Snippet))
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(sb.String())},
	}, nil, nil
}

func handleGetAccountBalance(ctx context.Context, req *mcp.CallToolRequest, args GetAccountBalanceArgs) (*mcp.CallToolResult, any, error) {
	// Search for the account ID across all customers
	results, err := wikiClient.Search(args.AccountID)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error searching: %v", err))},
			IsError: true,
		}, nil, nil
	}

	if len(results) == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("No account found with ID: %s", args.AccountID))},
		}, nil, nil
	}

	content, err := wikiClient.GetPage(results[0].Path)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error fetching page: %v", err))},
			IsError: true,
		}, nil, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(content)},
	}, nil, nil
}

func handleGetTransactionHistory(ctx context.Context, req *mcp.CallToolRequest, args GetTransactionHistoryArgs) (*mcp.CallToolResult, any, error) {
	results, err := wikiClient.Search(args.CustomerID)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error searching: %v", err))},
			IsError: true,
		}, nil, nil
	}

	if len(results) == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("No customer found with ID: %s", args.CustomerID))},
		}, nil, nil
	}

	content, err := wikiClient.GetPage(results[0].Path)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error fetching profile: %v", err))},
			IsError: true,
		}, nil, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(content)},
	}, nil, nil
}
```

- [ ] **Step 3: Create Dockerfile**

```dockerfile
# mcp-tools/customer-tools/Dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY shared/ /app/shared/
COPY customer-tools/ /app/customer-tools/
WORKDIR /app/customer-tools
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o customer-tools .

FROM alpine:3.19
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/customer-tools/customer-tools /customer-tools
EXPOSE 8081
ENTRYPOINT ["/customer-tools"]
```

- [ ] **Step 4: Commit**

```bash
git add mcp-tools/customer-tools/
git commit -m "feat: add customer data MCP tool server"
```

---

### Task 9: Policy & Rates MCP tool server

**Files:**
- Create: `mcp-tools/policy-tools/go.mod`
- Create: `mcp-tools/policy-tools/main.go`
- Create: `mcp-tools/policy-tools/Dockerfile`

- [ ] **Step 1: Create Go module**

Same pattern as customer-tools, with replace directive for shared module.

- [ ] **Step 2: Implement policy tools MCP server**

Same MCP server pattern. Four tools:
- `get_policy(policy_name string)` — Maps policy name to file path (e.g., "mortgage lending" → "policies/mortgage-lending"), fetches via `wikiClient.GetPage()`
- `search_policies(query string)` — Calls `wikiClient.Search()`, filters to `policies/` and `procedures/` prefixed results
- `get_current_rates(rate_type string)` — Maps rate type to file path (e.g., "mortgage" → "rates/mortgage-rates"), fetches content
- `get_rate_for_profile(credit_score int, loan_type string)` — Fetches the rate table for the loan type, then fetches the credit score tiers policy, returns both so the LLM can determine the applicable rate

Port: 8082. `WIKI_SERVER_URL` env var, same default.

- [ ] **Step 3: Create Dockerfile**

Same pattern as customer-tools Dockerfile, adjusted for policy-tools directory and port 8082.

- [ ] **Step 4: Commit**

```bash
git add mcp-tools/policy-tools/
git commit -m "feat: add policy and rates MCP tool server"
```

---

### Task 10: Transaction & Account MCP tool server

**Files:**
- Create: `mcp-tools/transaction-tools/go.mod`
- Create: `mcp-tools/transaction-tools/main.go`
- Create: `mcp-tools/transaction-tools/Dockerfile`

- [ ] **Step 1: Create Go module**

Same pattern.

- [ ] **Step 2: Implement transaction tools MCP server**

Four tools:
- `get_transaction_history(account_id string, date_from string, date_to string)` — Searches for account ID, returns the customer page (which contains transactions)
- `search_transactions(customer_id string, min_amount float64, max_amount float64, merchant string)` — Searches for customer by ID, returns their profile for the LLM to parse
- `get_account_details(account_id string)` — Fetches the customer page containing the account
- `get_customer_accounts(customer_id string)` — Fetches the customer page

Port: 8083.

- [ ] **Step 3: Create Dockerfile**

Same pattern, port 8083.

- [ ] **Step 4: Commit**

```bash
git add mcp-tools/transaction-tools/
git commit -m "feat: add transaction and account MCP tool server"
```

---

## Phase 4: Kubernetes Manifests

### Task 11: Bank wiki Kubernetes manifests

**Files:**
- Create: `manifests/bank-wiki/wiki-server.yaml`
- Create: `manifests/bank-wiki/customer-tools.yaml`
- Create: `manifests/bank-wiki/policy-tools.yaml`
- Create: `manifests/bank-wiki/transaction-tools.yaml`

- [ ] **Step 1: Create wiki server Deployment + Service**

```yaml
# manifests/bank-wiki/wiki-server.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: bank-wiki-server
  namespace: bank-wiki
  labels:
    app: bank-wiki-server
spec:
  replicas: 1
  selector:
    matchLabels:
      app: bank-wiki-server
  template:
    metadata:
      labels:
        app: bank-wiki-server
    spec:
      containers:
      - name: wiki-server
        image: bank-wiki-server:latest
        imagePullPolicy: Never
        ports:
        - containerPort: 8080
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 2
          periodSeconds: 5
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
---
apiVersion: v1
kind: Service
metadata:
  name: bank-wiki-server
  namespace: bank-wiki
spec:
  selector:
    app: bank-wiki-server
  ports:
  - port: 8080
    targetPort: 8080
```

- [ ] **Step 2: Create customer-tools Deployment + Service**

```yaml
# manifests/bank-wiki/customer-tools.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: bank-customer-tools
  namespace: bank-wiki
  labels:
    app: bank-customer-tools
spec:
  replicas: 1
  selector:
    matchLabels:
      app: bank-customer-tools
  template:
    metadata:
      labels:
        app: bank-customer-tools
    spec:
      containers:
      - name: customer-tools
        image: bank-customer-tools:latest
        imagePullPolicy: Never
        ports:
        - containerPort: 8081
        env:
        - name: WIKI_SERVER_URL
          value: "http://bank-wiki-server.bank-wiki.svc.cluster.local:8080"
        readinessProbe:
          httpGet:
            path: /health
            port: 8081
          initialDelaySeconds: 2
          periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: bank-customer-tools
  namespace: bank-wiki
spec:
  selector:
    app: bank-customer-tools
  ports:
  - port: 8081
    targetPort: 8081
```

- [ ] **Step 3: Create policy-tools and transaction-tools manifests**

Same pattern, adjusted for names, ports (8082, 8083), and image names.

- [ ] **Step 4: Commit**

```bash
git add manifests/bank-wiki/
git commit -m "feat: add bank wiki Kubernetes manifests"
```

---

### Task 12: MCP routing and agent manifests

**Files:**
- Create: `manifests/mcp/remote-mcp-servers.yaml`
- Create: `manifests/mcp/mcp-routes.yaml`
- Create: `manifests/agents/model-configs.yaml`
- Create: `manifests/agents/triage-agent.yaml`
- Create: `manifests/agents/customer-service-agent.yaml`
- Create: `manifests/agents/mortgage-advisor-agent.yaml`
- Create: `manifests/agents/compliance-agent.yaml`

- [ ] **Step 1: Create RemoteMCPServer resources**

```yaml
# manifests/mcp/remote-mcp-servers.yaml
apiVersion: kagent.dev/v1alpha2
kind: RemoteMCPServer
metadata:
  name: bank-customer-tools
  namespace: kagent
spec:
  url: http://bank-customer-tools.bank-wiki.svc.cluster.local:8081/mcp
  protocol: STREAMABLE_HTTP
  description: "Solo Bank customer data tools — lookup, search, account balances, transaction history"
---
apiVersion: kagent.dev/v1alpha2
kind: RemoteMCPServer
metadata:
  name: bank-policy-tools
  namespace: kagent
spec:
  url: http://bank-policy-tools.bank-wiki.svc.cluster.local:8082/mcp
  protocol: STREAMABLE_HTTP
  description: "Solo Bank policy and rates tools — lending policies, rate tables, credit tier lookups"
---
apiVersion: kagent.dev/v1alpha2
kind: RemoteMCPServer
metadata:
  name: bank-transaction-tools
  namespace: kagent
spec:
  url: http://bank-transaction-tools.bank-wiki.svc.cluster.local:8083/mcp
  protocol: STREAMABLE_HTTP
  description: "Solo Bank transaction and account tools — transaction history, account details, search"
```

- [ ] **Step 2: Create ModelConfig resources**

```yaml
# manifests/agents/model-configs.yaml
apiVersion: kagent.dev/v1alpha2
kind: ModelConfig
metadata:
  name: openai-gpt4o-mini
  namespace: kagent
spec:
  model: gpt-4o-mini
  provider: OpenAI
  apiKeySecret: openai-secret
  apiKeySecretKey: Authorization
---
apiVersion: kagent.dev/v1alpha2
kind: ModelConfig
metadata:
  name: anthropic-claude-sonnet
  namespace: kagent
spec:
  model: claude-sonnet-4-6
  provider: Anthropic
  apiKeySecret: anthropic-secret
  apiKeySecretKey: Authorization
```

Note: The API key secrets need to exist in the `kagent` namespace. The setup script will create copies.

- [ ] **Step 3: Create Triage Agent**

```yaml
# manifests/agents/triage-agent.yaml
apiVersion: kagent.dev/v1alpha2
kind: Agent
metadata:
  name: bank-triage-agent
  namespace: kagent
spec:
  description: "Solo Bank — Front Door Triage Agent. Routes customer inquiries to the appropriate specialist."
  type: Declarative
  declarative:
    modelConfig: anthropic-claude-sonnet
    systemMessage: |
      You are the front-door triage agent for Solo Bank. Your role is to greet customers, understand their needs, and route them to the appropriate specialist.

      Available specialists:
      - **Customer Service Agent**: Account inquiries, balance checks, transaction questions, general banking needs, dispute filing
      - **Mortgage Advisor Agent**: Mortgage rates, refinancing opportunities, home equity lines, lending qualification questions
      - **Compliance Agent**: Internal use only — policy audits, fraud investigations, regulatory reviews (never route customers here)

      Instructions:
      1. Greet the customer warmly and professionally
      2. Listen to their request carefully
      3. If the request clearly maps to one specialist, route immediately with a brief explanation of who will help them
      4. If ambiguous, ask ONE clarifying question before routing
      5. Never attempt to answer banking questions yourself — always route to a specialist
      6. If a customer raises fraud or compliance concerns, route to Customer Service (Compliance is internal only)
      7. Keep responses concise — you are a router, not an advisor
    tools: []
```

- [ ] **Step 4: Create Customer Service Agent**

```yaml
# manifests/agents/customer-service-agent.yaml
apiVersion: kagent.dev/v1alpha2
kind: Agent
metadata:
  name: bank-customer-service-agent
  namespace: kagent
spec:
  description: "Solo Bank — Customer Service Agent. Handles account inquiries, balance checks, and transaction questions."
  type: Declarative
  declarative:
    modelConfig: openai-gpt4o-mini
    systemMessage: |
      You are a customer service representative at Solo Bank. You help customers with account inquiries, balance checks, transaction questions, and general banking needs.

      IMPORTANT — Identity Verification:
      Before sharing ANY account details, you MUST verify the customer's identity by asking for their full name or Customer ID. Do not skip this step.

      Your capabilities:
      - Look up customer profiles by name or Customer ID
      - Check account balances
      - Review recent transactions
      - Search for customers by various criteria
      - Answer general questions about accounts

      Guidelines:
      - Be professional, helpful, and concise
      - Always verify identity before sharing account information
      - If a customer asks about mortgage rates, refinancing, or lending, let them know you'll connect them with a Mortgage Advisor
      - If a customer reports suspicious activity, document it and escalate to your supervisor
      - Reference specific account numbers and transaction details when discussing accounts
      - Never share information about other customers
    tools:
    - type: McpServer
      mcpServer:
        name: bank-customer-tools
        kind: RemoteMCPServer
    - type: McpServer
      mcpServer:
        name: bank-transaction-tools
        kind: RemoteMCPServer
```

- [ ] **Step 5: Create Mortgage Advisor Agent**

```yaml
# manifests/agents/mortgage-advisor-agent.yaml
apiVersion: kagent.dev/v1alpha2
kind: Agent
metadata:
  name: bank-mortgage-advisor-agent
  namespace: kagent
spec:
  description: "Solo Bank — Mortgage Advisor Agent. Provides personalized mortgage rate quotes and lending guidance."
  type: Declarative
  declarative:
    modelConfig: anthropic-claude-sonnet
    systemMessage: |
      You are a senior mortgage advisor at Solo Bank. You help customers understand mortgage options, rate eligibility, refinancing opportunities, and lending requirements.

      Your capabilities:
      - Look up customer profiles to see their credit score, salary, and existing accounts
      - Access current mortgage rate tables by credit tier
      - Reference lending policies for eligibility requirements
      - Access all bank policies and rate schedules

      How to provide personalized advice:
      1. Look up the customer's profile to get their credit score and salary
      2. Determine their credit tier using the credit-score-tiers policy
      3. Look up current rates for the relevant mortgage type
      4. Calculate DTI ratio using their salary and existing debts
      5. Check down payment requirements for their tier
      6. Present a clear, personalized recommendation

      Guidelines:
      - Always cite the specific policy or rate table you're referencing
      - Mention relationship discounts if the customer qualifies (5+ or 10+ years)
      - Be thorough but avoid jargon — explain terms when first used
      - If the customer doesn't qualify, explain exactly why and what they'd need to improve
      - For refinancing, compare their current rate to what they'd qualify for now
      - Never promise approval — always say "based on current policies, you would likely qualify for..."
    tools:
    - type: McpServer
      mcpServer:
        name: bank-customer-tools
        kind: RemoteMCPServer
    - type: McpServer
      mcpServer:
        name: bank-policy-tools
        kind: RemoteMCPServer
```

- [ ] **Step 6: Create Compliance Agent**

```yaml
# manifests/agents/compliance-agent.yaml
apiVersion: kagent.dev/v1alpha2
kind: Agent
metadata:
  name: bank-compliance-agent
  namespace: kagent
spec:
  description: "Solo Bank — Compliance Agent. Internal-only agent for policy audits, fraud review, and regulatory compliance checks."
  type: Declarative
  declarative:
    modelConfig: anthropic-claude-sonnet
    systemMessage: |
      You are an internal compliance officer at Solo Bank. You review customer accounts for policy violations, suspicious transactions, and regulatory concerns. You are NOT customer-facing — you serve internal bank staff.

      Your capabilities:
      - Full access to all customer data and transaction histories
      - Access to all bank policies including KYC/AML, fraud detection, and lending policies
      - Access to all rate tables and product details

      Standard review process:
      1. When asked to review a customer, look up their complete profile
      2. Check credit card APRs against their credit tier — flag any mismatches
      3. Review transactions for AML red flags: amounts over $10,000, rapid movements, unusual patterns
      4. Check if customers with active flags have been properly escalated
      5. Verify mortgage rates match their credit tier per the rate schedule
      6. Check DTI ratios against lending policy limits

      Output format for reviews:
      Use this structure:
      ## Compliance Review: [Customer Name] (CUST-XXXXX)
      ### Risk Rating: [Low/Medium/High/Critical]
      ### Findings:
      - [Finding 1 with specific policy reference]
      - [Finding 2 with specific policy reference]
      ### Recommendations:
      - [Action item 1]
      - [Action item 2]

      Guidelines:
      - Always reference specific policy documents and sections
      - Flag ALL violations, even minor ones
      - Prioritize findings by risk level
      - For AML flags, reference specific transaction amounts and dates
      - If no issues found, explicitly state the account is compliant
    tools:
    - type: McpServer
      mcpServer:
        name: bank-customer-tools
        kind: RemoteMCPServer
    - type: McpServer
      mcpServer:
        name: bank-policy-tools
        kind: RemoteMCPServer
    - type: McpServer
      mcpServer:
        name: bank-transaction-tools
        kind: RemoteMCPServer
```

- [ ] **Step 7: Commit**

```bash
git add manifests/mcp/ manifests/agents/
git commit -m "feat: add MCP routing, model configs, and agent manifests"
```

---

## Phase 5: Setup Script

### Task 13: Main setup script

**Files:**
- Create: `setup.sh`
- Create: `teardown.sh`

- [ ] **Step 1: Create setup.sh**

```bash
#!/usr/bin/env bash
set -euo pipefail

CLUSTER_NAME="solo-bank-demo"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

banner() {
  echo ""
  echo -e "${GREEN}==========================================${NC}"
  echo -e "${GREEN} $1${NC}"
  echo -e "${GREEN}==========================================${NC}"
}

warn() { echo -e "${YELLOW}[WARN] $1${NC}"; }
fail() { echo -e "${RED}[FAIL] $1${NC}"; exit 1; }

###########################################################
# Step 0: Prerequisites
###########################################################
banner "Step 0: Checking prerequisites"

for cmd in docker kind kubectl helm curl jq openssl; do
  command -v "$cmd" >/dev/null 2>&1 || fail "$cmd is required but not installed"
  echo "  ✓ $cmd"
done

# Check env vars
[ -n "${OPENAI_API_KEY:-}" ]            || fail "OPENAI_API_KEY is not set. Copy .env.example to .env, fill in values, and run: source .env"
[ -n "${ANTHROPIC_API_KEY:-}" ]         || fail "ANTHROPIC_API_KEY is not set"
[ -n "${AGENTGATEWAY_LICENSE_KEY:-}" ]  || fail "AGENTGATEWAY_LICENSE_KEY is not set"

echo -e "${GREEN}All prerequisites met.${NC}"

###########################################################
# Step 1: Create Kind cluster
###########################################################
banner "Step 1: Creating Kind cluster"

if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
  warn "Cluster ${CLUSTER_NAME} already exists. Skipping creation."
else
  kind create cluster --name "${CLUSTER_NAME}" --config "${SCRIPT_DIR}/kind-config.yaml" --wait 60s
fi

kubectl cluster-info --context "kind-${CLUSTER_NAME}"

###########################################################
# Step 2: Install Gateway API CRDs
###########################################################
banner "Step 2: Installing Gateway API CRDs"

kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.5.0/standard-install.yaml
echo "  ✓ Gateway API CRDs installed"

###########################################################
# Step 3: Create namespaces
###########################################################
banner "Step 3: Creating namespaces"

kubectl apply -f "${SCRIPT_DIR}/manifests/namespaces.yaml"
echo "  ✓ Namespaces created"

###########################################################
# Step 4: Install AgentRegistry OSS
###########################################################
banner "Step 4: Installing AgentRegistry OSS"

JWT_KEY=$(openssl rand -hex 32)

helm upgrade --install agentregistry \
  oci://ghcr.io/agentregistry-dev/agentregistry/charts/agentregistry \
  --namespace agentregistry \
  --create-namespace \
  --set config.jwtPrivateKey="${JWT_KEY}" \
  --set config.enableAnonymousAuth="true" \
  --set service.type=NodePort \
  --set service.nodePorts.http=30121 \
  --set database.postgres.vectorEnabled=true \
  --set database.postgres.bundled.image.repository=pgvector \
  --set database.postgres.bundled.image.name=pgvector \
  --set database.postgres.bundled.image.tag=pg16 \
  --set image.tag=v0.3.3 \
  --wait --timeout 300s

echo "  ✓ AgentRegistry installed"

###########################################################
# Step 5: Install AgentGateway Enterprise
###########################################################
banner "Step 5: Installing AgentGateway Enterprise"

helm upgrade --install enterprise-agentgateway-crds \
  oci://us-docker.pkg.dev/solo-public/enterprise-agentgateway/charts/enterprise-agentgateway-crds \
  --create-namespace \
  --namespace agentgateway-system \
  --version v2.3.0-beta.8 \
  --wait --timeout 120s

helm upgrade --install enterprise-agentgateway \
  oci://us-docker.pkg.dev/solo-public/enterprise-agentgateway/charts/enterprise-agentgateway \
  --namespace agentgateway-system \
  --version v2.3.0-beta.8 \
  --set-string licensing.licenseKey="${AGENTGATEWAY_LICENSE_KEY}" \
  --wait --timeout 300s

echo "  ✓ AgentGateway Enterprise installed"

###########################################################
# Step 6: Apply Gateway and tracing
###########################################################
banner "Step 6: Applying Gateway and tracing policy"

kubectl apply -f "${SCRIPT_DIR}/manifests/gateway.yaml"
echo "  ✓ Gateway and tracing applied"

# Wait for proxy to be ready
echo "  Waiting for agentgateway-proxy..."
kubectl wait --for=condition=available deployment/agentgateway-proxy \
  -n agentgateway-system --timeout=120s 2>/dev/null || true

###########################################################
# Step 7: Install Management UI
###########################################################
banner "Step 7: Installing Management UI"

helm upgrade --install management \
  oci://us-docker.pkg.dev/solo-public/solo-enterprise-helm/charts/management \
  --namespace agentgateway-system \
  --create-namespace \
  --version 0.3.14 \
  --set cluster="solo-bank-demo" \
  --set products.agentgateway.enabled=true \
  --set licensing.licenseKey="${AGENTGATEWAY_LICENSE_KEY}" \
  --wait --timeout 300s

echo "  ✓ Management UI installed"

###########################################################
# Step 8: Configure LLM backends
###########################################################
banner "Step 8: Configuring LLM backends"

# Substitute env vars and apply
sed "s|__OPENAI_API_KEY__|${OPENAI_API_KEY}|g" \
  "${SCRIPT_DIR}/manifests/llm-backends/openai.yaml" | kubectl apply -f -

sed "s|__ANTHROPIC_API_KEY__|${ANTHROPIC_API_KEY}|g" \
  "${SCRIPT_DIR}/manifests/llm-backends/anthropic.yaml" | kubectl apply -f -

# Also create copies of the secrets in kagent namespace for ModelConfig
kubectl create secret generic openai-secret \
  --namespace kagent \
  --from-literal=Authorization="${OPENAI_API_KEY}" \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl create secret generic anthropic-secret \
  --namespace kagent \
  --from-literal=Authorization="${ANTHROPIC_API_KEY}" \
  --dry-run=client -o yaml | kubectl apply -f -

echo "  ✓ LLM backends configured"

###########################################################
# Step 9: Install kagent Enterprise
###########################################################
banner "Step 9: Installing kagent Enterprise"

helm upgrade --install kagent-enterprise-crds \
  oci://us-docker.pkg.dev/solo-public/kagent-enterprise-helm/charts/kagent-enterprise-crds \
  --namespace kagent \
  --version 0.3.14 \
  --wait --timeout 120s

helm upgrade --install kagent-enterprise \
  oci://us-docker.pkg.dev/solo-public/kagent-enterprise-helm/charts/kagent-enterprise \
  --namespace kagent \
  --version 0.3.14 \
  --set defaultModelConfig.provider=OpenAI \
  --set defaultModelConfig.model=gpt-4o-mini \
  --set controller.enabled=true \
  --set kmcp.enabled=true \
  --wait --timeout 300s

echo "  ✓ kagent Enterprise installed"

###########################################################
# Step 10: Build and load Docker images
###########################################################
banner "Step 10: Building Docker images"

echo "  Building wiki-server..."
docker build -t bank-wiki-server:latest "${SCRIPT_DIR}/wiki-server/"

echo "  Building customer-tools..."
docker build -t bank-customer-tools:latest -f "${SCRIPT_DIR}/mcp-tools/customer-tools/Dockerfile" "${SCRIPT_DIR}/mcp-tools/"

echo "  Building policy-tools..."
docker build -t bank-policy-tools:latest -f "${SCRIPT_DIR}/mcp-tools/policy-tools/Dockerfile" "${SCRIPT_DIR}/mcp-tools/"

echo "  Building transaction-tools..."
docker build -t bank-transaction-tools:latest -f "${SCRIPT_DIR}/mcp-tools/transaction-tools/Dockerfile" "${SCRIPT_DIR}/mcp-tools/"

echo "  Building docs-site..."
docker build -t bank-docs-site:latest "${SCRIPT_DIR}/docs-site/"

echo "  Loading images into Kind..."
kind load docker-image bank-wiki-server:latest --name "${CLUSTER_NAME}"
kind load docker-image bank-customer-tools:latest --name "${CLUSTER_NAME}"
kind load docker-image bank-policy-tools:latest --name "${CLUSTER_NAME}"
kind load docker-image bank-transaction-tools:latest --name "${CLUSTER_NAME}"
kind load docker-image bank-docs-site:latest --name "${CLUSTER_NAME}"

echo "  ✓ All images built and loaded"

###########################################################
# Step 11: Deploy bank wiki and tool servers
###########################################################
banner "Step 11: Deploying bank wiki and tool servers"

kubectl apply -f "${SCRIPT_DIR}/manifests/bank-wiki/"
echo "  Waiting for wiki pods..."
kubectl wait --for=condition=ready pod -l app=bank-wiki-server -n bank-wiki --timeout=120s
kubectl wait --for=condition=ready pod -l app=bank-customer-tools -n bank-wiki --timeout=120s
kubectl wait --for=condition=ready pod -l app=bank-policy-tools -n bank-wiki --timeout=120s
kubectl wait --for=condition=ready pod -l app=bank-transaction-tools -n bank-wiki --timeout=120s

echo "  ✓ Bank wiki and tool servers deployed"

###########################################################
# Step 12: Apply MCP routing and agents
###########################################################
banner "Step 12: Applying MCP routing and agent configurations"

kubectl apply -f "${SCRIPT_DIR}/manifests/mcp/"
kubectl apply -f "${SCRIPT_DIR}/manifests/agents/"

echo "  ✓ MCP routing and agents configured"

###########################################################
# Step 13: Deploy docs site
###########################################################
banner "Step 13: Deploying documentation site"

kubectl apply -f "${SCRIPT_DIR}/manifests/bank-wiki/docs-site.yaml"
kubectl wait --for=condition=ready pod -l app=bank-docs-site -n bank-wiki --timeout=60s

echo "  ✓ Documentation site deployed"

###########################################################
# Step 14: Smoke tests
###########################################################
banner "Step 14: Running smoke tests"

echo "  Checking wiki server..."
WIKI_POD=$(kubectl get pod -l app=bank-wiki-server -n bank-wiki -o jsonpath='{.items[0].metadata.name}')
WIKI_STATUS=$(kubectl exec -n bank-wiki "${WIKI_POD}" -- wget -qO- http://localhost:8080/health 2>/dev/null || echo '{"status":"fail"}')
echo "  Wiki health: ${WIKI_STATUS}"

echo "  Checking agents..."
kubectl get agents -n kagent 2>/dev/null || echo "  (agents CRD not yet available)"

echo "  Checking MCP servers..."
kubectl get remotemcpservers -n kagent 2>/dev/null || echo "  (RemoteMCPServer CRD not yet available)"

###########################################################
# Done!
###########################################################
banner "Setup complete!"

echo ""
echo "Access points:"
echo "  AgentGateway Proxy:  http://localhost:30080"
echo "  AgentRegistry:       http://localhost:30121"
echo "  Documentation:       http://localhost:30500"
echo ""
echo "Port-forward commands for additional services:"
echo "  Management UI:  kubectl port-forward svc/solo-enterprise-ui -n agentgateway-system 4000:80"
echo "  kagent UI:      kubectl port-forward svc/kagent-ui -n kagent 3000:80"
echo ""
echo "Test LLM backends:"
echo '  curl localhost:30080/openai/v1/chat/completions -H "Content-Type: application/json" -d '"'"'{"model":"","messages":[{"role":"user","content":"Hello!"}]}'"'"' | jq'
echo ""
echo "Happy demoing! 🏦"
```

- [ ] **Step 2: Create teardown.sh**

```bash
#!/usr/bin/env bash
set -euo pipefail

CLUSTER_NAME="solo-bank-demo"

echo "Deleting Kind cluster: ${CLUSTER_NAME}"
kind delete cluster --name "${CLUSTER_NAME}"

echo "Removing Docker images..."
docker rmi bank-wiki-server:latest bank-customer-tools:latest bank-policy-tools:latest bank-transaction-tools:latest bank-docs-site:latest 2>/dev/null || true

echo "Done. Cluster and images removed."
```

- [ ] **Step 3: Make scripts executable and commit**

```bash
chmod +x setup.sh teardown.sh
git add setup.sh teardown.sh
git commit -m "feat: add setup and teardown scripts"
```

---

## Phase 6: Documentation Site

### Task 14: Documentation HTML site

**Files:**
- Create: `docs-site/Dockerfile`
- Create: `docs-site/nginx.conf`
- Create: `docs-site/index.html`
- Create: `docs-site/guide.html`
- Create: `docs-site/tutorial.html`
- Create: `docs-site/architecture.html`
- Create: `docs-site/static/style.css`
- Create: `manifests/bank-wiki/docs-site.yaml`

- [ ] **Step 1: Create nginx config**

```nginx
# docs-site/nginx.conf
server {
    listen 8080;
    server_name _;
    root /usr/share/nginx/html;
    index index.html;

    location / {
        try_files $uri $uri/ =404;
    }
}
```

- [ ] **Step 2: Create CSS**

```css
/* docs-site/static/style.css */
:root {
  --primary: #1a56db;
  --primary-dark: #1e40af;
  --bg: #f8fafc;
  --surface: #ffffff;
  --text: #1e293b;
  --text-muted: #64748b;
  --border: #e2e8f0;
  --code-bg: #f1f5f9;
  --accent: #059669;
}

* { margin: 0; padding: 0; box-sizing: border-box; }

body {
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
  color: var(--text);
  background: var(--bg);
  line-height: 1.6;
}

.container { max-width: 960px; margin: 0 auto; padding: 0 24px; }

nav {
  background: var(--surface);
  border-bottom: 1px solid var(--border);
  padding: 16px 0;
  position: sticky;
  top: 0;
  z-index: 10;
}

nav .container {
  display: flex;
  align-items: center;
  gap: 32px;
}

nav .logo {
  font-size: 20px;
  font-weight: 700;
  color: var(--primary);
  text-decoration: none;
}

nav a {
  color: var(--text-muted);
  text-decoration: none;
  font-size: 14px;
  font-weight: 500;
}

nav a:hover, nav a.active { color: var(--primary); }

.hero {
  padding: 64px 0;
  text-align: center;
}

.hero h1 { font-size: 36px; margin-bottom: 16px; }
.hero p { font-size: 18px; color: var(--text-muted); max-width: 640px; margin: 0 auto; }

.cards {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 24px;
  padding: 32px 0;
}

.card {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 24px;
  text-decoration: none;
  color: inherit;
  transition: box-shadow 0.2s;
}

.card:hover { box-shadow: 0 4px 12px rgba(0,0,0,0.08); }
.card h3 { margin-bottom: 8px; }
.card p { font-size: 14px; color: var(--text-muted); }

main { padding: 32px 0 64px; }
main h1 { font-size: 28px; margin-bottom: 24px; }
main h2 { font-size: 22px; margin: 32px 0 16px; border-bottom: 1px solid var(--border); padding-bottom: 8px; }
main h3 { font-size: 18px; margin: 24px 0 12px; }
main p { margin-bottom: 16px; }
main ul, main ol { margin: 0 0 16px 24px; }
main li { margin-bottom: 8px; }

pre {
  background: var(--code-bg);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 16px;
  overflow-x: auto;
  margin-bottom: 16px;
  font-size: 13px;
  line-height: 1.5;
}

code {
  background: var(--code-bg);
  padding: 2px 6px;
  border-radius: 3px;
  font-size: 13px;
}

pre code { background: none; padding: 0; }

.step {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 24px;
  margin-bottom: 24px;
}

.step-number {
  display: inline-block;
  background: var(--primary);
  color: white;
  width: 28px;
  height: 28px;
  border-radius: 50%;
  text-align: center;
  line-height: 28px;
  font-size: 14px;
  font-weight: 600;
  margin-right: 8px;
}

.badge {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 600;
}

.badge-agent { background: #dbeafe; color: #1e40af; }
.badge-tool { background: #d1fae5; color: #065f46; }
.badge-model { background: #fef3c7; color: #92400e; }

table {
  width: 100%;
  border-collapse: collapse;
  margin-bottom: 16px;
}

th, td {
  border: 1px solid var(--border);
  padding: 10px 14px;
  text-align: left;
  font-size: 14px;
}

th { background: var(--code-bg); font-weight: 600; }
```

- [ ] **Step 3: Create index.html**

Landing page with hero section, 3 cards linking to Guide, Tutorial, Architecture. Include nav bar with Solo Bank logo and links.

- [ ] **Step 4: Create guide.html — Using the Demo**

Detailed guide covering:
- **Accessing the UI** — port-forward commands, URLs
- **Talking to the Triage Agent** — example prompts ("I want to check my account balance", "What mortgage rate can I get?")
- **Talking to Customer Service** — example: look up John Smith, check balance, review transactions
- **Talking to Mortgage Advisor** — example: "What rate would customer CUST-00042 qualify for on a 30-year fixed?"
- **Using the Compliance Agent** — example: "Review customer CUST-00003 for compliance issues"
- **Viewing traces in the Management UI** — how to see OTEL traces flowing through AgentGateway
- **Exploring the wiki** — how to curl the wiki endpoints directly

- [ ] **Step 5: Create tutorial.html — Build Your Own Agent**

Step-by-step tutorial covering:

1. **Create an MCP Tool Server** — Walk through building a simple "account-summary" tool that aggregates a customer's data into a brief summary. Show the Go code, Dockerfile, build commands.

2. **Build and deploy the tool server** — `docker build`, `kind load`, `kubectl apply` with the Deployment + Service YAML

3. **Register the skill in AgentRegistry** — Show how to use the AgentRegistry API to publish the skill:
   ```bash
   curl -X POST http://localhost:30121/api/v1/skills \
     -H "Content-Type: application/json" \
     -d '{
       "name": "account-summary",
       "description": "Generate a brief account summary for a customer",
       "version": "1.0.0",
       "tools": [{"name": "get_account_summary", "description": "..."}]
     }'
   ```

4. **Create a RemoteMCPServer resource** — YAML for the new tool server

5. **Create a new Agent** — Full Agent CRD YAML with the new tool, custom system prompt: "You are an account summary specialist..."

6. **Deploy to kagent** — `kubectl apply` the manifests

7. **Test the agent** — Example prompts in the kagent UI

- [ ] **Step 6: Create architecture.html**

Architecture overview page with:
- ASCII art diagram (same as spec but HTML formatted)
- Namespace layout table
- Data flow explanation
- Component version table
- Network/port map

- [ ] **Step 7: Create Dockerfile**

```dockerfile
# docs-site/Dockerfile
FROM nginx:alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY index.html guide.html tutorial.html architecture.html /usr/share/nginx/html/
COPY static/ /usr/share/nginx/html/static/
EXPOSE 8080
```

- [ ] **Step 8: Create docs-site Kubernetes manifest**

```yaml
# manifests/bank-wiki/docs-site.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: bank-docs-site
  namespace: bank-wiki
  labels:
    app: bank-docs-site
spec:
  replicas: 1
  selector:
    matchLabels:
      app: bank-docs-site
  template:
    metadata:
      labels:
        app: bank-docs-site
    spec:
      containers:
      - name: docs
        image: bank-docs-site:latest
        imagePullPolicy: Never
        ports:
        - containerPort: 8080
---
apiVersion: v1
kind: Service
metadata:
  name: bank-docs-site
  namespace: bank-wiki
spec:
  type: NodePort
  selector:
    app: bank-docs-site
  ports:
  - port: 8080
    targetPort: 8080
    nodePort: 30500
```

- [ ] **Step 9: Commit**

```bash
git add docs-site/ manifests/bank-wiki/docs-site.yaml
git commit -m "feat: add documentation site with usage guide and agent tutorial"
```

---

## Phase 7: Sample Agent (Tutorial Companion)

### Task 15: Sample agent for the tutorial

**Files:**
- Create: `sample-agent/README.md`
- Create: `sample-agent/mcp-server/go.mod`
- Create: `sample-agent/mcp-server/main.go`
- Create: `sample-agent/mcp-server/Dockerfile`
- Create: `sample-agent/manifests/remote-mcp-server.yaml`
- Create: `sample-agent/manifests/agent.yaml`

- [ ] **Step 1: Create sample MCP server**

A simple MCP server with one tool: `get_account_summary` — takes a customer name, fetches their wiki page, and returns a condensed summary (just the key facts).

```go
// sample-agent/mcp-server/main.go
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var wikiBaseURL string
var httpClient = &http.Client{Timeout: 10 * time.Second}

type GetAccountSummaryArgs struct {
	CustomerName string `json:"customer_name" jsonschema:"description=Full name of the customer (e.g. 'John Smith'),required"`
}

func main() {
	wikiBaseURL = os.Getenv("WIKI_SERVER_URL")
	if wikiBaseURL == "" {
		wikiBaseURL = "http://bank-wiki-server.bank-wiki.svc.cluster.local:8080"
	}

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "account-summary-tools",
		Version: "1.0.0",
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_account_summary",
		Description: "Get a brief account summary for a Solo Bank customer. Returns key facts: name, credit score, total balances, and account status.",
	}, handleGetAccountSummary)

	handler := mcp.NewStreamableHTTPHandler(
		func(r *http.Request) *mcp.Server { return server },
		nil,
	)

	http.Handle("/mcp", handler)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})

	log.Println("Account Summary MCP server starting on :8084")
	if err := http.ListenAndServe(":8084", nil); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func handleGetAccountSummary(ctx context.Context, req *mcp.CallToolRequest, args GetAccountSummaryArgs) (*mcp.CallToolResult, any, error) {
	// Convert name to wiki path
	namePath := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(args.CustomerName), " ", "-"))
	pageURL := fmt.Sprintf("%s/wiki/customers/%s", wikiBaseURL, url.PathEscape(namePath))

	resp, err := httpClient.Get(pageURL)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error fetching customer: %v", err))},
			IsError: true,
		}, nil, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Customer '%s' not found.", args.CustomerName))},
		}, nil, nil
	}

	body, _ := io.ReadAll(resp.Body)
	content := string(body)

	// Extract key facts from the markdown
	summary := extractSummary(content, args.CustomerName)

	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(summary)},
	}, nil, nil
}

func extractSummary(markdown, name string) string {
	lines := strings.Split(markdown, "\n")
	var customerID, creditScore, riskRating, employer string
	var accounts []string
	var flags []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		switch {
		case strings.Contains(line, "Customer ID:"):
			customerID = extractValue(line)
		case strings.Contains(line, "Credit Score:"):
			creditScore = extractValue(line)
		case strings.Contains(line, "Risk Rating:"):
			riskRating = extractValue(line)
		case strings.Contains(line, "Employment:"):
			employer = extractValue(line)
		case strings.Contains(line, "Balance:"):
			accounts = append(accounts, line)
		case strings.Contains(line, "Active Flags:"):
			flags = append(flags, extractValue(line))
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Account Summary: %s\n\n", name))
	sb.WriteString(fmt.Sprintf("- **ID:** %s\n", customerID))
	sb.WriteString(fmt.Sprintf("- **Credit Score:** %s\n", creditScore))
	sb.WriteString(fmt.Sprintf("- **Risk:** %s\n", riskRating))
	sb.WriteString(fmt.Sprintf("- **Employment:** %s\n", employer))
	if len(flags) > 0 {
		sb.WriteString(fmt.Sprintf("- **Flags:** %s\n", strings.Join(flags, ", ")))
	}
	sb.WriteString(fmt.Sprintf("\n### Balances\n"))
	for _, a := range accounts {
		sb.WriteString(fmt.Sprintf("%s\n", a))
	}

	return sb.String()
}

func extractValue(line string) string {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) == 2 {
		return strings.TrimSpace(strings.Trim(parts[1], "* "))
	}
	return ""
}
```

- [ ] **Step 2: Create Dockerfile and go.mod**

```dockerfile
# sample-agent/mcp-server/Dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download 2>/dev/null || true
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o account-summary-server .

FROM alpine:3.19
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/account-summary-server /account-summary-server
EXPOSE 8084
ENTRYPOINT ["/account-summary-server"]
```

- [ ] **Step 3: Create Kubernetes manifests for the sample**

```yaml
# sample-agent/manifests/remote-mcp-server.yaml
apiVersion: kagent.dev/v1alpha2
kind: RemoteMCPServer
metadata:
  name: account-summary-tools
  namespace: kagent
spec:
  url: http://account-summary-tools.bank-wiki.svc.cluster.local:8084/mcp
  protocol: STREAMABLE_HTTP
  description: "Account summary tool — generates brief customer summaries"
```

```yaml
# sample-agent/manifests/agent.yaml
apiVersion: kagent.dev/v1alpha2
kind: Agent
metadata:
  name: account-summary-agent
  namespace: kagent
spec:
  description: "Solo Bank — Account Summary Agent. Quickly generates brief summaries of customer accounts."
  type: Declarative
  declarative:
    modelConfig: openai-gpt4o-mini
    systemMessage: |
      You are an account summary specialist at Solo Bank. When asked about a customer, use your tools to fetch their account summary and present it clearly. Keep responses brief and factual.
    tools:
    - type: McpServer
      mcpServer:
        name: account-summary-tools
        kind: RemoteMCPServer
```

- [ ] **Step 4: Create README with tutorial quick-start**

```markdown
# Sample Agent — Account Summary

This is a companion to the "Build Your Own Agent" tutorial.
See the full tutorial at http://localhost:30500/tutorial.html

## Quick Start

1. Build and load the MCP server:
   ```bash
   docker build -t account-summary-tools:latest sample-agent/mcp-server/
   kind load docker-image account-summary-tools:latest --name solo-bank-demo
   ```

2. Deploy the tool server:
   ```bash
   kubectl apply -f sample-agent/manifests/deployment.yaml
   ```

3. Register the RemoteMCPServer and Agent:
   ```bash
   kubectl apply -f sample-agent/manifests/remote-mcp-server.yaml
   kubectl apply -f sample-agent/manifests/agent.yaml
   ```

4. Open the kagent UI and test:
   ```
   kubectl port-forward svc/kagent-ui -n kagent 3000:80
   ```
   Go to http://localhost:3000 and try: "Give me a summary for John Smith"
```

- [ ] **Step 5: Commit**

```bash
git add sample-agent/
git commit -m "feat: add sample agent for build-your-own-agent tutorial"
```

---

## Phase 8: Final Integration

### Task 16: Integration test and final polish

- [ ] **Step 1: Add sample-agent deployment manifest**

Create `sample-agent/manifests/deployment.yaml`:
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: account-summary-tools
  namespace: bank-wiki
  labels:
    app: account-summary-tools
spec:
  replicas: 1
  selector:
    matchLabels:
      app: account-summary-tools
  template:
    metadata:
      labels:
        app: account-summary-tools
    spec:
      containers:
      - name: summary-tools
        image: account-summary-tools:latest
        imagePullPolicy: Never
        ports:
        - containerPort: 8084
        env:
        - name: WIKI_SERVER_URL
          value: "http://bank-wiki-server.bank-wiki.svc.cluster.local:8080"
        readinessProbe:
          httpGet:
            path: /health
            port: 8084
          initialDelaySeconds: 2
          periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: account-summary-tools
  namespace: bank-wiki
spec:
  selector:
    app: account-summary-tools
  ports:
  - port: 8084
    targetPort: 8084
```

- [ ] **Step 2: Run full setup end-to-end**

```bash
source .env
./setup.sh
```

Expected: All 14 steps complete without errors. All pods running.

- [ ] **Step 3: Verify wiki server**

```bash
kubectl exec -n bank-wiki deploy/bank-wiki-server -- wget -qO- http://localhost:8080/wiki/customers | jq length
# Expected: 100

kubectl exec -n bank-wiki deploy/bank-wiki-server -- wget -qO- http://localhost:8080/search?q=credit+score+782
# Expected: JSON results including John Smith
```

- [ ] **Step 4: Verify MCP tools respond**

```bash
kubectl exec -n bank-wiki deploy/bank-customer-tools -- wget -qO- http://localhost:8081/health
# Expected: {"status":"ok"}
```

- [ ] **Step 5: Verify agents exist**

```bash
kubectl get agents -n kagent
# Expected: 4 agents listed
kubectl get remotemcpservers -n kagent
# Expected: 3 RemoteMCPServer resources
```

- [ ] **Step 6: Test LLM backend through AgentGateway**

```bash
curl localhost:30080/openai/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"","messages":[{"role":"user","content":"Say hello"}]}' | jq .choices[0].message.content
```

- [ ] **Step 7: Verify docs site**

```bash
curl -s localhost:30500 | grep -o '<title>.*</title>'
# Expected: <title>Solo Bank Demo</title>
```

- [ ] **Step 8: Commit any fixes**

```bash
git add -A
git commit -m "fix: integration test fixes and polish"
```

---

## Task Summary

| Phase | Tasks | Description |
|-------|-------|-------------|
| 1 | 1-3 | Project scaffolding, Kind config, K8s manifests for gateway/backends |
| 2 | 4-6 | Wiki server (Go), bank content (policies/rates/products), 100 customer profiles |
| 3 | 7-10 | Shared wiki client, 3 MCP tool servers |
| 4 | 11-12 | Bank wiki K8s manifests, MCP routing + agent configs |
| 5 | 13 | Setup + teardown scripts |
| 6 | 14 | Documentation HTML site |
| 7 | 15 | Sample agent for tutorial |
| 8 | 16 | End-to-end integration test |
