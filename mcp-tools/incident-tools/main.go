package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Incident struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Severity    string `json:"severity"`
	Status      string `json:"status"`
	AssignedTo  string `json:"assigned_to"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
	AffectedApp string `json:"affected_app"`
	Description string `json:"description"`
	Timeline    string `json:"timeline"`
}

type Ticket struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Priority   string `json:"priority"`
	Status     string `json:"status"`
	Requester  string `json:"requester"`
	AssignedTo string `json:"assigned_to"`
	CreatedAt  string `json:"created_at"`
	Category   string `json:"category"`
	Details    string `json:"details"`
}

var incidents = []Incident{
	{
		ID: "INC-2024-001", Title: "Mortgage Processing System — Elevated Latency",
		Severity: "P2", Status: "INVESTIGATING",
		AssignedTo: "Sarah Chen (Platform Engineering)", CreatedAt: "2026-04-09T08:15:00Z", UpdatedAt: "2026-04-09T10:30:00Z",
		AffectedApp: "APP-MORTGAGE",
		Description: "Rate calculation engine experiencing 3.5x normal latency. Batch processing queue backed up by ~2,400 applications. Memory pressure detected on processing node pool.",
		Timeline:    "08:15 — Alert triggered: mortgage-processing p95 latency > 1000ms\n08:22 — On-call acknowledged (Sarah Chen)\n08:45 — Root cause identified: memory pressure on rate-calc pods after overnight batch spike\n09:30 — Horizontal pod autoscaler adjusted, new pods spinning up\n10:30 — Latency improving but still elevated. Queue draining. ETA full recovery: 2 hours.",
	},
	{
		ID: "INC-2024-002", Title: "ATM-042 Hardware Fault — Downtown Branch",
		Severity: "P3", Status: "IN PROGRESS",
		AssignedTo: "Mike Torres (Field Services)", CreatedAt: "2026-04-09T09:00:00Z", UpdatedAt: "2026-04-09T09:45:00Z",
		AffectedApp: "APP-ATM",
		Description: "ATM-042 at Downtown Branch reporting card reader malfunction. Customers unable to use card insert. Tap-to-pay still functional but intermittent.",
		Timeline:    "09:00 — Branch manager reported ATM-042 card reader not accepting cards\n09:10 — Remote diagnostic: card reader sensor fault code CR-7\n09:20 — Technician dispatched (Mike Torres, ETA 45 min)\n09:45 — Technician en route, replacement card reader module in stock.",
	},
	{
		ID: "INC-2024-003", Title: "Wire Transfer API — Intermittent Timeouts",
		Severity: "P1", Status: "MITIGATED",
		AssignedTo: "Alex Kim (Backend Engineering)", CreatedAt: "2026-04-08T22:30:00Z", UpdatedAt: "2026-04-09T06:00:00Z",
		AffectedApp: "APP-WIRE",
		Description: "SWIFT gateway experiencing intermittent 30-second timeouts on outbound transfers. Approximately 2% of transfers affected. Auto-retry handling successful for most cases.",
		Timeline:    "22:30 — PagerDuty alert: wire-transfer-gateway error rate > 1%\n22:35 — On-call acknowledged (Alex Kim)\n23:00 — Identified: upstream SWIFT network maintenance causing connection pool exhaustion\n23:30 — Mitigation: increased connection pool size and added circuit breaker\n00:15 — Error rate dropped to 0.3%\n06:00 — SWIFT maintenance completed. Error rate back to baseline 0.01%. Keeping mitigations in place.",
	},
	{
		ID: "INC-2024-004", Title: "ATM-017 Network Connectivity — Airport Terminal 2",
		Severity: "P3", Status: "WAITING ON VENDOR",
		AssignedTo: "Lisa Park (Network Operations)", CreatedAt: "2026-04-09T07:30:00Z", UpdatedAt: "2026-04-09T08:00:00Z",
		AffectedApp: "APP-ATM",
		Description: "ATM-017 at Airport Terminal 2 lost network connectivity. ISP circuit appears to be down. Backup 4G failover did not activate (SIM card expired).",
		Timeline:    "07:30 — Monitoring alert: ATM-017 heartbeat lost\n07:35 — Ping test failed; 4G failover not active\n07:45 — ISP ticket opened (Ticket #NW-88421)\n08:00 — ISP confirmed fiber cut in area, ETA 4 hours. New 4G SIM ordered for backup.",
	},
}

var tickets = []Ticket{
	{
		ID: "TKT-4501", Title: "New Employee Onboarding — Access Setup",
		Priority: "Medium", Status: "OPEN", Requester: "HR Department",
		AssignedTo: "IT Support Team", CreatedAt: "2026-04-09T08:00:00Z", Category: "Access Management",
		Details: "New hire starting Monday (James Wilson, Compliance Analyst). Needs: OIDC account, Management UI access (read-only), VPN credentials, compliance dashboard access. Manager: Patricia Gomez.",
	},
	{
		ID: "TKT-4502", Title: "Compliance Team — Password Reset (3 users)",
		Priority: "High", Status: "IN PROGRESS", Requester: "Patricia Gomez (Compliance)",
		AssignedTo: "Sarah from IT", CreatedAt: "2026-04-09T07:45:00Z", Category: "Access Management",
		Details: "Three compliance team members locked out after password policy change last night. Users: pgomez, rjohnson, mwilliams. Need emergency reset before 10:00 AM audit review.",
	},
	{
		ID: "TKT-4503", Title: "VPN Connectivity Issue — Remote Workers",
		Priority: "Medium", Status: "INVESTIGATING", Requester: "John Park (Customer Service)",
		AssignedTo: "Network Team", CreatedAt: "2026-04-08T16:30:00Z", Category: "Network",
		Details: "5 remote customer service agents reporting intermittent VPN drops since yesterday afternoon. Split-tunnel config may be conflicting with new endpoint protection update (v3.4.1 deployed yesterday).",
	},
	{
		ID: "TKT-4504", Title: "Request: New Monitoring Dashboard for Wire Transfers",
		Priority: "Low", Status: "BACKLOG", Requester: "Alex Kim (Backend Engineering)",
		AssignedTo: "Unassigned", CreatedAt: "2026-04-07T14:00:00Z", Category: "Enhancement",
		Details: "After INC-2024-003, requesting a dedicated Grafana dashboard for wire transfer gateway metrics: connection pool usage, SWIFT response times, retry rates, and circuit breaker state. Template available from the cards team dashboard.",
	},
	{
		ID: "TKT-4505", Title: "Mortgage System — Rate Table Update Deployment",
		Priority: "High", Status: "SCHEDULED", Requester: "Finance Team",
		AssignedTo: "Platform Engineering", CreatedAt: "2026-04-09T09:00:00Z", Category: "Change Request",
		Details: "Q2 rate table updates ready for deployment. New rates effective April 15. Requires: wiki update, policy-tools cache refresh, and smoke test of mortgage advisor agent. Change window: April 14, 22:00-23:00 UTC.",
	},
	{
		ID: "TKT-4506", Title: "Agent Not Responding — Compliance Agent Timeout",
		Priority: "High", Status: "OPEN", Requester: "Patricia Gomez (Compliance)",
		AssignedTo: "IT Support Team", CreatedAt: "2026-04-09T10:15:00Z", Category: "Application Issue",
		Details: "Compliance agent in Management UI returning 'request timeout' for the last 30 minutes. Other agents working fine. May be related to the mortgage system degradation — compliance agent queries customer and policy tools heavily.",
	},
}

type GetIncidentArgs struct {
	IncidentID string `json:"incident_id" jsonschema:"Incident ID (e.g. INC-2024-001)"`
}

type ListIncidentsArgs struct {
	Severity string `json:"severity,omitempty" jsonschema:"Filter by severity: P1, P2, P3, or empty for all"`
	Status   string `json:"status,omitempty" jsonschema:"Filter by status: INVESTIGATING, IN PROGRESS, MITIGATED, RESOLVED, or empty for all"`
}

type GetTicketArgs struct {
	TicketID string `json:"ticket_id" jsonschema:"Ticket ID (e.g. TKT-4501)"`
}

type ListTicketsArgs struct {
	Status   string `json:"status,omitempty" jsonschema:"Filter by status: OPEN, IN PROGRESS, SCHEDULED, BACKLOG, or empty for all"`
	Category string `json:"category,omitempty" jsonschema:"Filter by category or empty for all"`
}

type SearchIncidentsArgs struct {
	Query string `json:"query" jsonschema:"Search keyword across incidents and tickets"`
}

func handleGetIncident(_ context.Context, _ *mcp.CallToolRequest, args GetIncidentArgs) (*mcp.CallToolResult, any, error) {
	for _, inc := range incidents {
		if strings.EqualFold(inc.ID, args.IncidentID) {
			b, _ := json.MarshalIndent(inc, "", "  ")
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
			}, nil, nil
		}
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Incident not found: %s", args.IncidentID)}},
		IsError: true,
	}, nil, nil
}

func handleListIncidents(_ context.Context, _ *mcp.CallToolRequest, args ListIncidentsArgs) (*mcp.CallToolResult, any, error) {
	var sb strings.Builder
	sb.WriteString("# Open Incidents\n\n")
	count := 0
	for _, inc := range incidents {
		if args.Severity != "" && !strings.EqualFold(inc.Severity, args.Severity) {
			continue
		}
		if args.Status != "" && !strings.EqualFold(inc.Status, strings.ReplaceAll(args.Status, "_", " ")) {
			continue
		}
		icon := "🔴"
		switch inc.Severity {
		case "P2":
			icon = "🟡"
		case "P3":
			icon = "🟠"
		}
		sb.WriteString(fmt.Sprintf("%s **%s** [%s] — %s\n   Status: %s | Assigned: %s | App: %s\n   %s\n\n",
			icon, inc.ID, inc.Severity, inc.Title, inc.Status, inc.AssignedTo, inc.AffectedApp, inc.Description))
		count++
	}
	if count == 0 {
		sb.WriteString("No incidents match the filter criteria.\n")
	} else {
		sb.WriteString(fmt.Sprintf("**Total: %d incident(s)**\n", count))
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: sb.String()}},
	}, nil, nil
}

func handleGetTicket(_ context.Context, _ *mcp.CallToolRequest, args GetTicketArgs) (*mcp.CallToolResult, any, error) {
	for _, t := range tickets {
		if strings.EqualFold(t.ID, args.TicketID) {
			b, _ := json.MarshalIndent(t, "", "  ")
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
			}, nil, nil
		}
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Ticket not found: %s", args.TicketID)}},
		IsError: true,
	}, nil, nil
}

func handleListTickets(_ context.Context, _ *mcp.CallToolRequest, args ListTicketsArgs) (*mcp.CallToolResult, any, error) {
	var sb strings.Builder
	sb.WriteString("# IT Tickets\n\n")
	count := 0
	for _, t := range tickets {
		if args.Status != "" && !strings.EqualFold(t.Status, strings.ReplaceAll(args.Status, "_", " ")) {
			continue
		}
		if args.Category != "" && !strings.Contains(strings.ToLower(t.Category), strings.ToLower(args.Category)) {
			continue
		}
		icon := "📋"
		switch t.Priority {
		case "High":
			icon = "🔴"
		case "Medium":
			icon = "🟡"
		case "Low":
			icon = "🟢"
		}
		sb.WriteString(fmt.Sprintf("%s **%s** [%s] — %s\n   Status: %s | Category: %s | Requester: %s\n   %s\n\n",
			icon, t.ID, t.Priority, t.Title, t.Status, t.Category, t.Requester, t.Details))
		count++
	}
	if count == 0 {
		sb.WriteString("No tickets match the filter criteria.\n")
	} else {
		sb.WriteString(fmt.Sprintf("**Total: %d ticket(s)**\n", count))
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: sb.String()}},
	}, nil, nil
}

func handleSearchIncidents(_ context.Context, _ *mcp.CallToolRequest, args SearchIncidentsArgs) (*mcp.CallToolResult, any, error) {
	query := strings.ToLower(args.Query)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Search Results for \"%s\"\n\n", args.Query))

	sb.WriteString("## Incidents\n")
	incCount := 0
	for _, inc := range incidents {
		blob := strings.ToLower(inc.Title + " " + inc.Description + " " + inc.AffectedApp + " " + inc.Timeline)
		if strings.Contains(blob, query) {
			sb.WriteString(fmt.Sprintf("- **%s** [%s] — %s (Status: %s)\n", inc.ID, inc.Severity, inc.Title, inc.Status))
			incCount++
		}
	}
	if incCount == 0 {
		sb.WriteString("No matching incidents.\n")
	}

	sb.WriteString("\n## Tickets\n")
	tktCount := 0
	for _, t := range tickets {
		blob := strings.ToLower(t.Title + " " + t.Details + " " + t.Category + " " + t.Requester)
		if strings.Contains(blob, query) {
			sb.WriteString(fmt.Sprintf("- **%s** [%s] — %s (Status: %s)\n", t.ID, t.Priority, t.Title, t.Status))
			tktCount++
		}
	}
	if tktCount == 0 {
		sb.WriteString("No matching tickets.\n")
	}

	sb.WriteString(fmt.Sprintf("\n**Found %d incident(s) and %d ticket(s)**\n", incCount, tktCount))
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: sb.String()}},
	}, nil, nil
}

func main() {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "bank-incident-tools",
		Version: "1.0.0",
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_incident",
		Description: "Get full details of a specific incident by ID, including timeline and affected systems.",
	}, handleGetIncident)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_incidents",
		Description: "List all open incidents. Optionally filter by severity (P1/P2/P3) or status.",
	}, handleListIncidents)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_ticket",
		Description: "Get full details of a specific IT ticket by ID.",
	}, handleGetTicket)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_tickets",
		Description: "List all IT tickets. Optionally filter by status or category.",
	}, handleListTickets)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_incidents",
		Description: "Search across all incidents and tickets by keyword. Matches against titles, descriptions, and affected systems.",
	}, handleSearchIncidents)

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

	addr := ":8086"
	log.Printf("Starting bank-incident-tools MCP server on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
