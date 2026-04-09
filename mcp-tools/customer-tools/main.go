package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/solo-io/solo-bank-demo/mcp-tools/shared"
)

var wikiClient *shared.WikiClient

// LookupCustomerArgs is the input schema for the lookup_customer tool.
type LookupCustomerArgs struct {
	Name       string `json:"name,omitempty" jsonschema:"Customer name to look up"`
	CustomerID string `json:"customer_id,omitempty" jsonschema:"Customer ID (e.g. CUST-00001)"`
}

// SearchCustomersArgs is the input schema for the search_customers tool.
type SearchCustomersArgs struct {
	Query string `json:"query" jsonschema:"Search query to find customers"`
}

// GetAccountBalanceArgs is the input schema for the get_account_balance tool.
type GetAccountBalanceArgs struct {
	AccountID string `json:"account_id" jsonschema:"Account ID to look up"`
}

func nameToPath(name string) string {
	lower := strings.ToLower(name)
	hyphenated := strings.ReplaceAll(lower, " ", "-")
	return "customers/" + hyphenated
}

func handleLookupCustomer(ctx context.Context, req *mcp.CallToolRequest, args LookupCustomerArgs) (*mcp.CallToolResult, any, error) {
	// Try by name first
	if args.Name != "" {
		pagePath := nameToPath(args.Name)
		content, err := wikiClient.GetPage(pagePath)
		if err == nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: content}},
			}, nil, nil
		}
	}

	// Try by customer ID via search
	if args.CustomerID != "" {
		results, err := wikiClient.Search(args.CustomerID)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("search error: %v", err)}},
				IsError: true,
			}, nil, nil
		}
		for _, r := range results {
			if strings.HasPrefix(r.Path, "customers/") {
				content, err := wikiClient.GetPage(r.Path)
				if err == nil {
					return &mcp.CallToolResult{
						Content: []mcp.Content{&mcp.TextContent{Text: content}},
					}, nil, nil
				}
			}
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("customer not found for ID: %s", args.CustomerID)}},
			IsError: true,
		}, nil, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: "either name or customer_id is required"}},
		IsError: true,
	}, nil, nil
}

func handleSearchCustomers(ctx context.Context, req *mcp.CallToolRequest, args SearchCustomersArgs) (*mcp.CallToolResult, any, error) {
	if args.Query == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "query is required"}},
			IsError: true,
		}, nil, nil
	}

	results, err := wikiClient.Search(args.Query)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("search error: %v", err)}},
			IsError: true,
		}, nil, nil
	}

	var sb strings.Builder
	count := 0
	for _, r := range results {
		if strings.HasPrefix(r.Path, "customers/") {
			sb.WriteString(fmt.Sprintf("## %s\n", r.Title))
			sb.WriteString(fmt.Sprintf("Path: %s\n", r.Path))
			sb.WriteString(fmt.Sprintf("Score: %.2f\n", r.Score))
			if r.Snippet != "" {
				sb.WriteString(fmt.Sprintf("Snippet: %s\n", r.Snippet))
			}
			sb.WriteString("\n")
			count++
		}
	}

	if count == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("no customers found for query: %s", args.Query)}},
		}, nil, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: sb.String()}},
	}, nil, nil
}

func handleGetAccountBalance(ctx context.Context, req *mcp.CallToolRequest, args GetAccountBalanceArgs) (*mcp.CallToolResult, any, error) {
	if args.AccountID == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "account_id is required"}},
			IsError: true,
		}, nil, nil
	}

	results, err := wikiClient.Search(args.AccountID)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("search error: %v", err)}},
			IsError: true,
		}, nil, nil
	}

	for _, r := range results {
		if strings.HasPrefix(r.Path, "customers/") {
			content, err := wikiClient.GetPage(r.Path)
			if err == nil {
				return &mcp.CallToolResult{
					Content: []mcp.Content{&mcp.TextContent{Text: content}},
				}, nil, nil
			}
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("no customer found for account ID: %s", args.AccountID)}},
		IsError: true,
	}, nil, nil
}

func main() {
	wikiURL := os.Getenv("WIKI_SERVER_URL")
	if wikiURL == "" {
		wikiURL = "http://bank-wiki-server.bank-wiki.svc.cluster.local:8080"
	}
	wikiClient = shared.NewWikiClient(wikiURL)
	log.Printf("Using wiki server: %s", wikiURL)

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "bank-customer-tools",
		Version: "1.0.0",
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "lookup_customer",
		Description: "Look up a bank customer by name or customer ID. Returns the full customer profile.",
	}, handleLookupCustomer)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_customers",
		Description: "Search for customers by a query string. Returns a list of matching customers with snippets.",
	}, handleSearchCustomers)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_account_balance",
		Description: "Get account balance information by account ID. Returns the customer page containing the account.",
	}, handleGetAccountBalance)

	mcpHandler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return server
	}, nil)

	mux := http.NewServeMux()
	mux.Handle("/mcp", mcpHandler)
	mux.Handle("/mcp/", mcpHandler)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	addr := ":8081"
	log.Printf("Starting bank-customer-tools MCP server on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
