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

// GetPolicyArgs is the input schema for the get_policy tool.
type GetPolicyArgs struct {
	PolicyName string `json:"policy_name" jsonschema:"description=Name of the policy to retrieve (e.g. 'mortgage lending', 'credit score tiers', 'overdraft')"`
}

// SearchPoliciesArgs is the input schema for the search_policies tool.
type SearchPoliciesArgs struct {
	Query string `json:"query" jsonschema:"description=Search query to find relevant policies or procedures"`
}

// GetCurrentRatesArgs is the input schema for the get_current_rates tool.
type GetCurrentRatesArgs struct {
	RateType string `json:"rate_type" jsonschema:"description=Type of rates to retrieve: 'mortgage', 'savings', 'cd', or 'credit-card'"`
}

// GetRateForProfileArgs is the input schema for the get_rate_for_profile tool.
type GetRateForProfileArgs struct {
	CreditScore int    `json:"credit_score" jsonschema:"description=Customer credit score (e.g. 720)"`
	LoanType    string `json:"loan_type" jsonschema:"description=Type of loan (e.g. 'mortgage', 'cd', 'credit-card')"`
}

// policyNameToPath maps a normalized policy name to its wiki path.
func policyNameToPath(name string) string {
	lower := strings.ToLower(strings.TrimSpace(name))
	switch {
	case strings.Contains(lower, "mortgage lending"):
		return "policies/mortgage-lending"
	case strings.Contains(lower, "credit score tiers") || strings.Contains(lower, "credit score tier"):
		return "policies/credit-score-tiers"
	case strings.Contains(lower, "credit card products"):
		return "policies/credit-card-products"
	case strings.Contains(lower, "interest rate schedule"):
		return "policies/interest-rate-schedule"
	case strings.Contains(lower, "overdraft"):
		return "policies/overdraft-policy"
	case strings.Contains(lower, "kyc") || strings.Contains(lower, "aml"):
		return "policies/kyc-aml-compliance"
	case strings.Contains(lower, "fee schedule"):
		return "policies/fee-schedule"
	case strings.Contains(lower, "account types"):
		return "policies/account-types"
	case strings.Contains(lower, "fraud"):
		return "policies/fraud-detection"
	case strings.Contains(lower, "escalation"):
		return "policies/customer-service-escalation"
	default:
		return ""
	}
}

// rateTypeToPath maps a rate type to its wiki path.
func rateTypeToPath(rateType string) string {
	lower := strings.ToLower(strings.TrimSpace(rateType))
	switch lower {
	case "mortgage":
		return "rates/mortgage-rates"
	case "savings":
		return "rates/savings-rates"
	case "cd":
		return "rates/cd-rates"
	case "credit-card", "credit_card", "creditcard":
		return "rates/credit-card-apr"
	default:
		return ""
	}
}

func handleGetPolicy(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[GetPolicyArgs]) (*mcp.CallToolResultFor[any], error) {
	args := params.Arguments
	if args.PolicyName == "" {
		return &mcp.CallToolResultFor[any]{
			Content: []mcp.Content{&mcp.TextContent{Text: "policy_name is required"}},
			IsError: true,
		}, nil
	}

	// Try exact mapping first
	path := policyNameToPath(args.PolicyName)
	if path != "" {
		content, err := wikiClient.GetPage(path)
		if err == nil {
			return &mcp.CallToolResultFor[any]{
				Content: []mcp.Content{&mcp.TextContent{Text: content}},
			}, nil
		}
	}

	// Fall back to search
	results, err := wikiClient.Search(args.PolicyName)
	if err != nil {
		return &mcp.CallToolResultFor[any]{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("search error: %v", err)}},
			IsError: true,
		}, nil
	}

	for _, r := range results {
		if strings.HasPrefix(r.Path, "policies/") || strings.HasPrefix(r.Path, "procedures/") {
			content, err := wikiClient.GetPage(r.Path)
			if err == nil {
				return &mcp.CallToolResultFor[any]{
					Content: []mcp.Content{&mcp.TextContent{Text: content}},
				}, nil
			}
		}
	}

	return &mcp.CallToolResultFor[any]{
		Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("policy not found: %s", args.PolicyName)}},
		IsError: true,
	}, nil
}

func handleSearchPolicies(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[SearchPoliciesArgs]) (*mcp.CallToolResultFor[any], error) {
	args := params.Arguments
	if args.Query == "" {
		return &mcp.CallToolResultFor[any]{
			Content: []mcp.Content{&mcp.TextContent{Text: "query is required"}},
			IsError: true,
		}, nil
	}

	results, err := wikiClient.Search(args.Query)
	if err != nil {
		return &mcp.CallToolResultFor[any]{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("search error: %v", err)}},
			IsError: true,
		}, nil
	}

	var sb strings.Builder
	count := 0
	for _, r := range results {
		if strings.HasPrefix(r.Path, "policies/") || strings.HasPrefix(r.Path, "procedures/") {
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
		return &mcp.CallToolResultFor[any]{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("no policies or procedures found for query: %s", args.Query)}},
		}, nil
	}

	return &mcp.CallToolResultFor[any]{
		Content: []mcp.Content{&mcp.TextContent{Text: sb.String()}},
	}, nil
}

func handleGetCurrentRates(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[GetCurrentRatesArgs]) (*mcp.CallToolResultFor[any], error) {
	args := params.Arguments
	if args.RateType == "" {
		return &mcp.CallToolResultFor[any]{
			Content: []mcp.Content{&mcp.TextContent{Text: "rate_type is required (mortgage, savings, cd, or credit-card)"}},
			IsError: true,
		}, nil
	}

	path := rateTypeToPath(args.RateType)
	if path == "" {
		return &mcp.CallToolResultFor[any]{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("unknown rate_type: %s. Valid values are: mortgage, savings, cd, credit-card", args.RateType)}},
			IsError: true,
		}, nil
	}

	content, err := wikiClient.GetPage(path)
	if err != nil {
		return &mcp.CallToolResultFor[any]{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("failed to retrieve rates: %v", err)}},
			IsError: true,
		}, nil
	}

	return &mcp.CallToolResultFor[any]{
		Content: []mcp.Content{&mcp.TextContent{Text: content}},
	}, nil
}

func handleGetRateForProfile(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[GetRateForProfileArgs]) (*mcp.CallToolResultFor[any], error) {
	args := params.Arguments
	if args.LoanType == "" {
		return &mcp.CallToolResultFor[any]{
			Content: []mcp.Content{&mcp.TextContent{Text: "loan_type is required"}},
			IsError: true,
		}, nil
	}
	if args.CreditScore <= 0 {
		return &mcp.CallToolResultFor[any]{
			Content: []mcp.Content{&mcp.TextContent{Text: "credit_score is required and must be a positive integer"}},
			IsError: true,
		}, nil
	}

	path := rateTypeToPath(args.LoanType)
	if path == "" {
		return &mcp.CallToolResultFor[any]{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("unknown loan_type: %s. Valid values are: mortgage, savings, cd, credit-card", args.LoanType)}},
			IsError: true,
		}, nil
	}

	// Fetch rate table for the loan type
	rateContent, err := wikiClient.GetPage(path)
	if err != nil {
		return &mcp.CallToolResultFor[any]{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("failed to retrieve rate table: %v", err)}},
			IsError: true,
		}, nil
	}

	// Fetch credit score tiers policy
	tierContent, err := wikiClient.GetPage("policies/credit-score-tiers")
	if err != nil {
		// Return rate content alone if tiers not available
		return &mcp.CallToolResultFor[any]{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Credit Score: %d\nLoan Type: %s\n\n%s\n\n(Note: could not retrieve credit score tiers: %v)", args.CreditScore, args.LoanType, rateContent, err)}},
		}, nil
	}

	combined := fmt.Sprintf("Credit Score: %d\nLoan Type: %s\n\n# Rate Table\n\n%s\n\n# Credit Score Tiers\n\n%s", args.CreditScore, args.LoanType, rateContent, tierContent)

	return &mcp.CallToolResultFor[any]{
		Content: []mcp.Content{&mcp.TextContent{Text: combined}},
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
		Name:    "bank-policy-tools",
		Version: "1.0.0",
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_policy",
		Description: "Retrieve a bank policy document by name. Supports: mortgage lending, credit score tiers, credit card products, interest rate schedule, overdraft, kyc/aml, fee schedule, account types, fraud detection, customer service escalation.",
	}, handleGetPolicy)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_policies",
		Description: "Search for bank policies and procedures by a query string. Returns matching policy and procedure documents with snippets.",
	}, handleSearchPolicies)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_current_rates",
		Description: "Get the current rate table for a given rate type. Valid types: 'mortgage', 'savings', 'cd', 'credit-card'.",
	}, handleGetCurrentRates)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_rate_for_profile",
		Description: "Get the applicable interest rate for a customer profile. Fetches the rate table for the loan type and the credit score tiers policy so the agent can determine the correct rate for the given credit score.",
	}, handleGetRateForProfile)

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

	addr := ":8082"
	log.Printf("Starting bank-policy-tools MCP server on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
