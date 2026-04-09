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
	CustomerName string `json:"customer_name" jsonschema:"Full name of the customer (e.g. John Smith),required"`
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
	}, func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[GetAccountSummaryArgs]) (*mcp.CallToolResultFor[any], error) {
		args := params.Arguments
		namePath := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(args.CustomerName), " ", "-"))
		pageURL := fmt.Sprintf("%s/wiki/customers/%s", wikiBaseURL, url.PathEscape(namePath))

		resp, err := httpClient.Get(pageURL)
		if err != nil {
			return &mcp.CallToolResultFor[any]{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error fetching customer: %v", err)}},
				IsError: true,
			}, nil
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			return &mcp.CallToolResultFor[any]{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Customer '%s' not found.", args.CustomerName)}},
			}, nil
		}

		body, _ := io.ReadAll(resp.Body)
		summary := extractSummary(string(body), args.CustomerName)

		return &mcp.CallToolResultFor[any]{
			Content: []mcp.Content{&mcp.TextContent{Text: summary}},
		}, nil
	})

	handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server { return server }, nil)

	http.Handle("/mcp", handler)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})

	log.Println("Account Summary MCP server starting on :8084")
	if err := http.ListenAndServe(":8084", nil); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func extractSummary(markdown, name string) string {
	lines := strings.Split(markdown, "\n")
	var customerID, creditScore, riskRating, employer string
	var balanceLines []string
	var flags []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.Contains(trimmed, "Customer ID:"):
			customerID = extractValue(trimmed)
		case strings.Contains(trimmed, "Credit Score:"):
			creditScore = extractValue(trimmed)
		case strings.Contains(trimmed, "Risk Rating:"):
			riskRating = extractValue(trimmed)
		case strings.Contains(trimmed, "Employment:"):
			employer = extractValue(trimmed)
		case strings.Contains(trimmed, "Balance:"):
			balanceLines = append(balanceLines, trimmed)
		case strings.Contains(trimmed, "Active Flags:"):
			flags = append(flags, extractValue(trimmed))
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
	sb.WriteString("\n### Balances\n")
	for _, bl := range balanceLines {
		sb.WriteString(fmt.Sprintf("%s\n", bl))
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
