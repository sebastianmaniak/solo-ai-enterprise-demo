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

// GetTransactionHistoryArgs is the input schema for the get_transaction_history tool.
type GetTransactionHistoryArgs struct {
	AccountID string `json:"account_id" jsonschema:"description=Account ID to look up transaction history for (required)"`
	DateFrom  string `json:"date_from,omitempty" jsonschema:"description=Start date for filtering transactions (optional, e.g. 2024-01-01)"`
	DateTo    string `json:"date_to,omitempty" jsonschema:"description=End date for filtering transactions (optional, e.g. 2024-12-31)"`
}

// SearchTransactionsArgs is the input schema for the search_transactions tool.
type SearchTransactionsArgs struct {
	CustomerID string  `json:"customer_id" jsonschema:"description=Customer ID to search transactions for (required, e.g. CUST-00001)"`
	MinAmount  float64 `json:"min_amount,omitempty" jsonschema:"description=Minimum transaction amount filter (optional)"`
	MaxAmount  float64 `json:"max_amount,omitempty" jsonschema:"description=Maximum transaction amount filter (optional)"`
	Merchant   string  `json:"merchant,omitempty" jsonschema:"description=Merchant name filter (optional)"`
}

// GetAccountDetailsArgs is the input schema for the get_account_details tool.
type GetAccountDetailsArgs struct {
	AccountID string `json:"account_id" jsonschema:"description=Account ID to retrieve details for (required)"`
}

// GetCustomerAccountsArgs is the input schema for the get_customer_accounts tool.
type GetCustomerAccountsArgs struct {
	CustomerID string `json:"customer_id" jsonschema:"description=Customer ID to retrieve accounts for (required, e.g. CUST-00001)"`
}

func handleGetTransactionHistory(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[GetTransactionHistoryArgs]) (*mcp.CallToolResultFor[any], error) {
	args := params.Arguments
	if args.AccountID == "" {
		return &mcp.CallToolResultFor[any]{
			Content: []mcp.Content{&mcp.TextContent{Text: "account_id is required"}},
			IsError: true,
		}, nil
	}

	results, err := wikiClient.Search(args.AccountID)
	if err != nil {
		return &mcp.CallToolResultFor[any]{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("search error: %v", err)}},
			IsError: true,
		}, nil
	}

	for _, r := range results {
		if strings.HasPrefix(r.Path, "customers/") {
			content, err := wikiClient.GetPage(r.Path)
			if err == nil {
				return &mcp.CallToolResultFor[any]{
					Content: []mcp.Content{&mcp.TextContent{Text: content}},
				}, nil
			}
		}
	}

	return &mcp.CallToolResultFor[any]{
		Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("no transaction history found for account ID: %s", args.AccountID)}},
		IsError: true,
	}, nil
}

func handleSearchTransactions(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[SearchTransactionsArgs]) (*mcp.CallToolResultFor[any], error) {
	args := params.Arguments
	if args.CustomerID == "" {
		return &mcp.CallToolResultFor[any]{
			Content: []mcp.Content{&mcp.TextContent{Text: "customer_id is required"}},
			IsError: true,
		}, nil
	}

	results, err := wikiClient.Search(args.CustomerID)
	if err != nil {
		return &mcp.CallToolResultFor[any]{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("search error: %v", err)}},
			IsError: true,
		}, nil
	}

	for _, r := range results {
		if strings.HasPrefix(r.Path, "customers/") {
			content, err := wikiClient.GetPage(r.Path)
			if err == nil {
				return &mcp.CallToolResultFor[any]{
					Content: []mcp.Content{&mcp.TextContent{Text: content}},
				}, nil
			}
		}
	}

	return &mcp.CallToolResultFor[any]{
		Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("no transactions found for customer ID: %s", args.CustomerID)}},
		IsError: true,
	}, nil
}

func handleGetAccountDetails(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[GetAccountDetailsArgs]) (*mcp.CallToolResultFor[any], error) {
	args := params.Arguments
	if args.AccountID == "" {
		return &mcp.CallToolResultFor[any]{
			Content: []mcp.Content{&mcp.TextContent{Text: "account_id is required"}},
			IsError: true,
		}, nil
	}

	results, err := wikiClient.Search(args.AccountID)
	if err != nil {
		return &mcp.CallToolResultFor[any]{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("search error: %v", err)}},
			IsError: true,
		}, nil
	}

	for _, r := range results {
		if strings.HasPrefix(r.Path, "customers/") {
			content, err := wikiClient.GetPage(r.Path)
			if err == nil {
				return &mcp.CallToolResultFor[any]{
					Content: []mcp.Content{&mcp.TextContent{Text: content}},
				}, nil
			}
		}
	}

	return &mcp.CallToolResultFor[any]{
		Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("no account details found for account ID: %s", args.AccountID)}},
		IsError: true,
	}, nil
}

func handleGetCustomerAccounts(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[GetCustomerAccountsArgs]) (*mcp.CallToolResultFor[any], error) {
	args := params.Arguments
	if args.CustomerID == "" {
		return &mcp.CallToolResultFor[any]{
			Content: []mcp.Content{&mcp.TextContent{Text: "customer_id is required"}},
			IsError: true,
		}, nil
	}

	results, err := wikiClient.Search(args.CustomerID)
	if err != nil {
		return &mcp.CallToolResultFor[any]{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("search error: %v", err)}},
			IsError: true,
		}, nil
	}

	for _, r := range results {
		if strings.HasPrefix(r.Path, "customers/") {
			content, err := wikiClient.GetPage(r.Path)
			if err == nil {
				return &mcp.CallToolResultFor[any]{
					Content: []mcp.Content{&mcp.TextContent{Text: content}},
				}, nil
			}
		}
	}

	return &mcp.CallToolResultFor[any]{
		Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("no accounts found for customer ID: %s", args.CustomerID)}},
		IsError: true,
	}, nil
}

func main() {
	wikiURL := os.Getenv("WIKI_SERVER_URL")
	if wikiURL == "" {
		wikiURL = "http://bank-wiki-server.bank-wiki.svc.cluster.local:8080"
	}
	wikiClient = shared.NewWikiClient(wikiURL)
	log.Printf("Using wiki server: %s", wikiURL)

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "bank-transaction-tools",
		Version: "1.0.0",
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_transaction_history",
		Description: "Get transaction history for a bank account by account ID. Optionally filter by date range. Returns the customer page containing the account and its transactions.",
	}, handleGetTransactionHistory)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_transactions",
		Description: "Search transactions for a customer by customer ID. Optionally filter by amount range or merchant name. Returns the customer profile with transaction data.",
	}, handleSearchTransactions)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_account_details",
		Description: "Get detailed account information by account ID. Returns the customer page containing the account details.",
	}, handleGetAccountDetails)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_customer_accounts",
		Description: "Get all accounts for a customer by customer ID. Returns the matching customer profile with all associated accounts.",
	}, handleGetCustomerAccounts)

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

	addr := ":8083"
	log.Printf("Starting bank-transaction-tools MCP server on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
