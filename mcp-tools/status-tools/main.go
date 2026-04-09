package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Simulated banking application statuses
type AppStatus struct {
	Name        string `json:"name"`
	ID          string `json:"id"`
	Status      string `json:"status"`
	Uptime      string `json:"uptime"`
	Latency     string `json:"latency"`
	ErrorRate   string `json:"error_rate"`
	LastChecked string `json:"last_checked"`
	Details     string `json:"details"`
}

type DatacenterStatus struct {
	Name       string `json:"name"`
	ID         string `json:"id"`
	Region     string `json:"region"`
	Status     string `json:"status"`
	Load       string `json:"load"`
	Network    string `json:"network"`
	Storage    string `json:"storage"`
	LastSynced string `json:"last_synced"`
	Details    string `json:"details"`
}

var apps = []AppStatus{
	{
		Name: "Mortgage Processing System", ID: "APP-MORTGAGE",
		Status: "DEGRADED", Uptime: "99.2%", Latency: "1,240ms (avg 350ms)",
		ErrorRate: "3.7%", Details: "Elevated latency on rate calculation engine. Batch processing queue backed up by ~2,400 applications. Engineering investigating memory pressure on processing nodes. ETA to resolution: 2 hours.",
	},
	{
		Name: "Wire Transfer Gateway", ID: "APP-WIRE",
		Status: "OPERATIONAL", Uptime: "99.99%", Latency: "89ms",
		ErrorRate: "0.01%", Details: "All SWIFT and ACH channels operational. Last settlement batch completed at 14:00 UTC. Daily volume: $42.3M across 1,847 transfers. No pending alerts.",
	},
	{
		Name: "Online Banking Portal", ID: "APP-PORTAL",
		Status: "OPERATIONAL", Uptime: "99.95%", Latency: "142ms",
		ErrorRate: "0.08%", Details: "Web and mobile channels healthy. Active sessions: 12,450. Authentication service nominal. CDN cache hit ratio: 94.2%. Scheduled maintenance window: Sunday 02:00-04:00 UTC.",
	},
	{
		Name: "ATM Network Controller", ID: "APP-ATM",
		Status: "PARTIAL OUTAGE", Uptime: "97.8%", Latency: "210ms",
		ErrorRate: "2.1%", Details: "2 of 48 ATMs offline — ATM-042 (Downtown Branch: hardware fault, technician dispatched ETA 45 min) and ATM-017 (Airport Terminal 2: network connectivity, ISP notified). Remaining 46 ATMs fully operational. Cash levels nominal across fleet.",
	},
	{
		Name: "Credit Card Authorization", ID: "APP-CARDS",
		Status: "OPERATIONAL", Uptime: "99.99%", Latency: "45ms",
		ErrorRate: "0.02%", Details: "Real-time authorization pipeline healthy. Processing ~2,100 transactions/minute. Fraud detection engine running v4.2 models. No declined-transaction anomalies detected.",
	},
	{
		Name: "Core Banking Ledger", ID: "APP-LEDGER",
		Status: "OPERATIONAL", Uptime: "100%", Latency: "12ms",
		ErrorRate: "0.00%", Details: "Primary ledger database cluster healthy (3 nodes, synchronous replication). Daily reconciliation completed at 06:00 UTC — zero discrepancies. Storage utilization: 67%.",
	},
}

var datacenters = []DatacenterStatus{
	{
		Name: "US-East Primary", ID: "DC-USE1", Region: "Virginia, US",
		Status: "HEALTHY", Load: "62%", Network: "10Gbps — nominal",
		Storage: "847TB / 1.2PB (71%)", Details: "Primary production datacenter. All racks operational. Cooling nominal at 68°F. Power: dual-feed utility + diesel backup tested last week. Next maintenance: April 20.",
	},
	{
		Name: "US-West DR", ID: "DC-USW2", Region: "Oregon, US",
		Status: "HEALTHY", Load: "28%", Network: "10Gbps — nominal",
		Storage: "412TB / 800TB (52%)", Details: "Disaster recovery site. Active-passive replication lag: <500ms. Last DR failover test: March 15 (passed). All replication streams healthy.",
	},
	{
		Name: "EU-Central", ID: "DC-EUC1", Region: "Frankfurt, Germany",
		Status: "MAINTENANCE", Load: "45%", Network: "5Gbps — reduced capacity",
		Storage: "230TB / 400TB (58%)", Details: "Scheduled network upgrade in progress — migrating to 25Gbps backbone. GDPR-compliant data residency. EU customer data processing unaffected. Upgrade completion: tonight 23:00 CET.",
	},
	{
		Name: "APAC", ID: "DC-APAC1", Region: "Singapore",
		Status: "HEALTHY", Load: "38%", Network: "5Gbps — nominal",
		Storage: "180TB / 400TB (45%)", Details: "Serving APAC region customers. Cross-region replication to US-East: healthy (lag <2s). MAS compliance audit passed last month. Next capacity review: May 1.",
	},
}

func now() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05Z")
}

type GetAppStatusArgs struct {
	AppID string `json:"app_id,omitempty" jsonschema:"Application ID (e.g. APP-MORTGAGE) or name"`
}

type ListAppsArgs struct{}

type GetDatacenterStatusArgs struct {
	DcID string `json:"dc_id,omitempty" jsonschema:"Datacenter ID (e.g. DC-USE1) or name"`
}

type ListDatacentersArgs struct{}

type GetSystemOverviewArgs struct{}

func handleGetAppStatus(_ context.Context, _ *mcp.CallToolRequest, args GetAppStatusArgs) (*mcp.CallToolResult, any, error) {
	query := strings.ToUpper(args.AppID)
	for _, app := range apps {
		if strings.EqualFold(app.ID, query) || strings.Contains(strings.ToUpper(app.Name), query) {
			app.LastChecked = now()
			// Add slight random variation to latency display
			b, _ := json.MarshalIndent(app, "", "  ")
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
			}, nil, nil
		}
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Application not found: %s. Use list_apps to see all applications.", args.AppID)}},
		IsError: true,
	}, nil, nil
}

func handleListApps(_ context.Context, _ *mcp.CallToolRequest, _ ListAppsArgs) (*mcp.CallToolResult, any, error) {
	var sb strings.Builder
	sb.WriteString("# Solo Bank Application Status Dashboard\n\n")
	sb.WriteString(fmt.Sprintf("Report generated: %s\n\n", now()))

	healthy := 0
	degraded := 0
	outage := 0
	for _, app := range apps {
		icon := "🟢"
		switch app.Status {
		case "DEGRADED":
			icon = "🟡"
			degraded++
		case "PARTIAL OUTAGE":
			icon = "🔴"
			outage++
		default:
			healthy++
		}
		sb.WriteString(fmt.Sprintf("%s **%s** (%s) — %s | Latency: %s | Errors: %s\n", icon, app.Name, app.ID, app.Status, app.Latency, app.ErrorRate))
	}
	sb.WriteString(fmt.Sprintf("\n**Summary:** %d Healthy, %d Degraded, %d Outage — %d total applications\n", healthy, degraded, outage, len(apps)))
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: sb.String()}},
	}, nil, nil
}

func handleGetDatacenterStatus(_ context.Context, _ *mcp.CallToolRequest, args GetDatacenterStatusArgs) (*mcp.CallToolResult, any, error) {
	query := strings.ToUpper(args.DcID)
	for _, dc := range datacenters {
		if strings.EqualFold(dc.ID, query) || strings.Contains(strings.ToUpper(dc.Name), query) {
			dc.LastSynced = now()
			b, _ := json.MarshalIndent(dc, "", "  ")
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
			}, nil, nil
		}
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Datacenter not found: %s. Use list_datacenters to see all datacenters.", args.DcID)}},
		IsError: true,
	}, nil, nil
}

func handleListDatacenters(_ context.Context, _ *mcp.CallToolRequest, _ ListDatacentersArgs) (*mcp.CallToolResult, any, error) {
	var sb strings.Builder
	sb.WriteString("# Solo Bank Datacenter Status\n\n")
	sb.WriteString(fmt.Sprintf("Report generated: %s\n\n", now()))
	for _, dc := range datacenters {
		icon := "🟢"
		if dc.Status == "MAINTENANCE" {
			icon = "🟡"
		}
		sb.WriteString(fmt.Sprintf("%s **%s** (%s) — %s | Region: %s | Load: %s | Network: %s\n", icon, dc.Name, dc.ID, dc.Status, dc.Region, dc.Load, dc.Network))
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: sb.String()}},
	}, nil, nil
}

func handleGetSystemOverview(_ context.Context, _ *mcp.CallToolRequest, _ GetSystemOverviewArgs) (*mcp.CallToolResult, any, error) {
	var sb strings.Builder
	sb.WriteString("# Solo Bank — System Overview\n\n")
	sb.WriteString(fmt.Sprintf("Generated: %s\n\n", now()))

	sb.WriteString("## Applications\n")
	for _, app := range apps {
		icon := "🟢"
		switch app.Status {
		case "DEGRADED":
			icon = "🟡"
		case "PARTIAL OUTAGE":
			icon = "🔴"
		}
		sb.WriteString(fmt.Sprintf("%s %s — %s\n", icon, app.Name, app.Status))
	}

	sb.WriteString("\n## Datacenters\n")
	for _, dc := range datacenters {
		icon := "🟢"
		if dc.Status == "MAINTENANCE" {
			icon = "🟡"
		}
		sb.WriteString(fmt.Sprintf("%s %s (%s) — %s | Load: %s\n", icon, dc.Name, dc.Region, dc.Status, dc.Load))
	}

	// Simulated summary metrics
	sb.WriteString(fmt.Sprintf("\n## Key Metrics\n"))
	sb.WriteString(fmt.Sprintf("- Active customer sessions: %d\n", 12000+rand.Intn(1000)))
	sb.WriteString(fmt.Sprintf("- Transactions/min: %d\n", 2000+rand.Intn(300)))
	sb.WriteString(fmt.Sprintf("- Open incidents: 3 (1 P1, 1 P2, 1 P3)\n"))
	sb.WriteString(fmt.Sprintf("- Scheduled maintenances: 1 (EU-Central network upgrade)\n"))

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: sb.String()}},
	}, nil, nil
}

func main() {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "bank-status-tools",
		Version: "1.0.0",
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_app_status",
		Description: "Get detailed status of a specific banking application by ID or name. Returns health, latency, error rate, and current issues.",
	}, handleGetAppStatus)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_apps",
		Description: "List all monitored banking applications with their current status, latency, and error rates.",
	}, handleListApps)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_datacenter_status",
		Description: "Get detailed status of a specific datacenter by ID or name. Returns load, network, storage, and maintenance info.",
	}, handleGetDatacenterStatus)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_datacenters",
		Description: "List all Solo Bank datacenters with their current status, load, and network health.",
	}, handleListDatacenters)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_system_overview",
		Description: "Get a high-level overview of all applications, datacenters, and key metrics in a single report.",
	}, handleGetSystemOverview)

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

	addr := ":8085"
	log.Printf("Starting bank-status-tools MCP server on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
