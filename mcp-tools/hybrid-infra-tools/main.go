package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ---------------------------------------------------------------------------
// Data model structs
// ---------------------------------------------------------------------------

type Site struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Provider string `json:"provider"` // on-prem, AWS, Azure
	Region   string `json:"region"`
}

type Segment struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	SiteID string `json:"site_id"`
	CIDR   string `json:"cidr"`
}

type Server struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	SegmentID string `json:"segment_id"`
	IP        string `json:"ip"`
	OS        string `json:"os"`
	Role      string `json:"role"`
}

type Firewall struct {
	ID        string   `json:"id"`
	Model     string   `json:"model"`
	SiteID    string   `json:"site_id"`
	Zones     []string `json:"zones"`
	HAStatus  string   `json:"ha_status"`
	Placement string   `json:"placement"`
}

type AddressObject struct {
	ID          string `json:"id"`
	Type        string `json:"type"` // host, network
	Value       string `json:"value"`
	Description string `json:"description"`
}

type AddressGroup struct {
	ID      string   `json:"id"`
	Members []string `json:"members"` // AddressObject IDs
}

type SecurityRule struct {
	ID          string `json:"id"`
	Action      string `json:"action"` // allow, deny
	FirewallID  string `json:"firewall_id"`
	FromZone    string `json:"from_zone"`
	ToZone      string `json:"to_zone"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Service     string `json:"service"`
	Description string `json:"description"`
	LogEnabled  bool   `json:"log_enabled"`
}

type NATRule struct {
	ID          string `json:"id"`
	Type        string `json:"type"` // DNAT, SNAT
	FirewallID  string `json:"firewall_id"`
	FromZone    string `json:"from_zone"`
	ToZone      string `json:"to_zone"`
	Original    string `json:"original"`
	Translated  string `json:"translated"`
	Description string `json:"description"`
}

type Route struct {
	ID          string `json:"id"`
	Destination string `json:"destination"`
	NextHop     string `json:"next_hop"`
	Interface   string `json:"interface"`
	FirewallID  string `json:"firewall_id"`
	Description string `json:"description"`
}

type Link struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	FromSiteID string `json:"from_site_id"`
	ToSiteID   string `json:"to_site_id"`
	Bandwidth  string `json:"bandwidth"`
	Status     string `json:"status"`
	Type       string `json:"type"`
}

type IncidentScenario struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Symptom      string   `json:"symptom"`
	Path         string   `json:"path"`
	Affected     []string `json:"affected"`
	BrokenObject string   `json:"broken_object"`
	Explanation  string   `json:"explanation"`
}

// ---------------------------------------------------------------------------
// In-memory dataset
// ---------------------------------------------------------------------------

var sites = []Site{
	{ID: "SITE-DC1", Name: "Primary Datacenter", Provider: "on-prem", Region: "Newark, NJ"},
	{ID: "SITE-AWS1", Name: "AWS US-East", Provider: "AWS", Region: "us-east-1"},
	{ID: "SITE-AZ1", Name: "Azure East US", Provider: "Azure", Region: "eastus"},
	{ID: "SITE-EDGE1", Name: "Branch Office HQ", Provider: "on-prem", Region: "Manhattan, NY"},
}

var segments = []Segment{
	{ID: "SEG-DMZ", Name: "DMZ", SiteID: "SITE-DC1", CIDR: "10.1.0.0/24"},
	{ID: "SEG-APP", Name: "Application Tier", SiteID: "SITE-DC1", CIDR: "10.2.0.0/24"},
	{ID: "SEG-DATA", Name: "Data Tier", SiteID: "SITE-DC1", CIDR: "10.3.0.0/24"},
	{ID: "SEG-MGMT", Name: "Management", SiteID: "SITE-DC1", CIDR: "10.4.0.0/24"},
	{ID: "SEG-AWS-APP", Name: "AWS Application", SiteID: "SITE-AWS1", CIDR: "172.16.1.0/24"},
	{ID: "SEG-AWS-SHARED", Name: "AWS Shared Services", SiteID: "SITE-AWS1", CIDR: "172.16.2.0/24"},
	{ID: "SEG-AZ-MGMT", Name: "Azure Management", SiteID: "SITE-AZ1", CIDR: "192.168.1.0/24"},
	{ID: "SEG-AZ-SHARED", Name: "Azure Shared Services", SiteID: "SITE-AZ1", CIDR: "192.168.2.0/24"},
}

var servers = []Server{
	{ID: "SRV-WEB01", Name: "Customer Portal", SegmentID: "SEG-DMZ", IP: "10.1.0.10", OS: "Linux", Role: "Nginx"},
	{ID: "SRV-WEB02", Name: "API Gateway", SegmentID: "SEG-DMZ", IP: "10.1.0.11", OS: "Linux", Role: "Kong"},
	{ID: "SRV-APP01", Name: "Loan Processing Service", SegmentID: "SEG-APP", IP: "10.2.0.10", OS: "Linux", Role: "Java/Spring"},
	{ID: "SRV-APP02", Name: "Payment Engine", SegmentID: "SEG-APP", IP: "10.2.0.11", OS: "Linux", Role: "Go"},
	{ID: "SRV-DB01", Name: "Primary Database", SegmentID: "SEG-DATA", IP: "10.3.0.10", OS: "Linux", Role: "PostgreSQL"},
	{ID: "SRV-DB02", Name: "Analytics Database", SegmentID: "SEG-DATA", IP: "10.3.0.11", OS: "Linux", Role: "ClickHouse"},
	{ID: "SRV-MGMT01", Name: "Bastion Host", SegmentID: "SEG-MGMT", IP: "10.4.0.10", OS: "Linux", Role: "SSH"},
	{ID: "SRV-MGMT02", Name: "Monitoring Stack", SegmentID: "SEG-MGMT", IP: "10.4.0.11", OS: "Linux", Role: "Prometheus/Grafana"},
	{ID: "SRV-AWS01", Name: "Cloud Loan API", SegmentID: "SEG-AWS-APP", IP: "172.16.1.10", OS: "Linux", Role: "Python/FastAPI"},
	{ID: "SRV-AWS02", Name: "Cloud Document Store", SegmentID: "SEG-AWS-SHARED", IP: "172.16.2.10", OS: "Linux", Role: "MongoDB"},
	{ID: "SRV-AZ01", Name: "Azure AD Connector", SegmentID: "SEG-AZ-MGMT", IP: "192.168.1.10", OS: "Windows", Role: "AD Connect"},
	{ID: "SRV-AZ02", Name: "Central Logging", SegmentID: "SEG-AZ-SHARED", IP: "192.168.2.10", OS: "Linux", Role: "Elasticsearch"},
}

var firewalls = []Firewall{
	{ID: "FW-PA01", Model: "Palo Alto PA-5260", SiteID: "SITE-DC1", Zones: []string{"untrust", "dmz", "trust-app", "trust-data"}, HAStatus: "active", Placement: "Edge/DMZ boundary"},
	{ID: "FW-PA02", Model: "Palo Alto PA-3260", SiteID: "SITE-DC1", Zones: []string{"trust-app", "trust-data", "trust-mgmt"}, HAStatus: "standby", Placement: "Internal segmentation"},
	{ID: "FW-FG01", Model: "Fortinet FortiGate-600E", SiteID: "SITE-DC1", Zones: []string{"hybrid-ext", "hybrid-int", "mgmt-zone"}, HAStatus: "standalone", Placement: "Hybrid connectivity boundary"},
}

var addressObjects = []AddressObject{
	{ID: "ADDR-PORTAL-PUB", Type: "host", Value: "203.0.113.10", Description: "Customer Portal public VIP"},
	{ID: "ADDR-PORTAL-INT", Type: "host", Value: "10.1.0.10", Description: "Customer Portal internal"},
	{ID: "ADDR-API-PUB", Type: "host", Value: "203.0.113.11", Description: "API Gateway public VIP"},
	{ID: "ADDR-API-INT", Type: "host", Value: "10.1.0.11", Description: "API Gateway internal"},
	{ID: "ADDR-LOAN-SVC", Type: "host", Value: "10.2.0.10", Description: "Loan Processing Service"},
	{ID: "ADDR-PAY-SVC", Type: "host", Value: "10.2.0.11", Description: "Payment Engine"},
	{ID: "ADDR-DB-PRIMARY", Type: "host", Value: "10.3.0.10", Description: "Primary Database"},
	{ID: "ADDR-BASTION", Type: "host", Value: "10.4.0.10", Description: "Bastion Host"},
	{ID: "ADDR-AWS-LOAN", Type: "host", Value: "172.16.1.10", Description: "Cloud Loan API"},
	{ID: "ADDR-AZ-AD", Type: "host", Value: "192.168.1.10", Description: "Azure AD Connector"},
}

var addressGroups = []AddressGroup{
	{ID: "GRP-DMZ-SERVERS", Members: []string{"ADDR-PORTAL-INT", "ADDR-API-INT"}},
	{ID: "GRP-APP-SERVERS", Members: []string{"ADDR-LOAN-SVC", "ADDR-PAY-SVC"}},
	{ID: "GRP-DATA-SERVERS", Members: []string{"ADDR-DB-PRIMARY"}},
	{ID: "GRP-MGMT-HOSTS", Members: []string{"ADDR-BASTION"}},
	{ID: "GRP-CLOUD-SERVICES", Members: []string{"ADDR-AWS-LOAN", "ADDR-AZ-AD"}},
}

var securityRules = []SecurityRule{
	{ID: "RULE-001", Action: "allow", FirewallID: "FW-PA01", FromZone: "untrust", ToZone: "dmz", Source: "any", Destination: "GRP-DMZ-SERVERS", Service: "HTTPS/443", Description: "Inbound web traffic", LogEnabled: true},
	{ID: "RULE-002", Action: "allow", FirewallID: "FW-PA01", FromZone: "dmz", ToZone: "trust-app", Source: "GRP-DMZ-SERVERS", Destination: "GRP-APP-SERVERS", Service: "HTTPS/8443", Description: "DMZ to app tier", LogEnabled: true},
	{ID: "RULE-003", Action: "allow", FirewallID: "FW-PA01", FromZone: "trust-app", ToZone: "trust-data", Source: "GRP-APP-SERVERS", Destination: "GRP-DATA-SERVERS", Service: "PostgreSQL/5432", Description: "App to database", LogEnabled: true},
	{ID: "RULE-004", Action: "deny", FirewallID: "FW-PA01", FromZone: "dmz", ToZone: "trust-data", Source: "any", Destination: "any", Service: "any", Description: "Block DMZ direct to data", LogEnabled: true},
	{ID: "RULE-005", Action: "allow", FirewallID: "FW-PA01", FromZone: "trust-app", ToZone: "trust-mgmt", Source: "GRP-APP-SERVERS", Destination: "GRP-MGMT-HOSTS", Service: "SSH/22", Description: "App tier mgmt access", LogEnabled: false},
	{ID: "RULE-006", Action: "allow", FirewallID: "FW-PA02", FromZone: "trust-mgmt", ToZone: "trust-app", Source: "GRP-MGMT-HOSTS", Destination: "any", Service: "SSH/22+HTTPS/443", Description: "Management monitoring", LogEnabled: true},
	{ID: "RULE-007", Action: "allow", FirewallID: "FW-FG01", FromZone: "hybrid-ext", ToZone: "hybrid-int", Source: "172.16.0.0/12", Destination: "10.2.0.0/24", Service: "HTTPS/8443", Description: "AWS to on-prem app", LogEnabled: true},
	{ID: "RULE-008", Action: "allow", FirewallID: "FW-FG01", FromZone: "hybrid-ext", ToZone: "mgmt-zone", Source: "192.168.0.0/16", Destination: "GRP-MGMT-HOSTS", Service: "SSH/22", Description: "Azure to on-prem mgmt", LogEnabled: true},
	{ID: "RULE-009", Action: "allow", FirewallID: "FW-FG01", FromZone: "hybrid-int", ToZone: "hybrid-ext", Source: "10.2.0.0/24", Destination: "172.16.0.0/12", Service: "HTTPS/443", Description: "On-prem to AWS", LogEnabled: true},
	{ID: "RULE-010", Action: "deny", FirewallID: "FW-FG01", FromZone: "hybrid-ext", ToZone: "hybrid-int", Source: "any", Destination: "GRP-DATA-SERVERS", Service: "any", Description: "Block cloud direct to data", LogEnabled: true},
	{ID: "RULE-011", Action: "allow", FirewallID: "FW-PA01", FromZone: "untrust", ToZone: "dmz", Source: "any", Destination: "ADDR-API-PUB", Service: "HTTPS/443", Description: "Inbound API traffic", LogEnabled: true},
	{ID: "RULE-012", Action: "deny", FirewallID: "FW-PA01", FromZone: "any", ToZone: "any", Source: "any", Destination: "any", Service: "any", Description: "Default deny all", LogEnabled: true},
}

var natRules = []NATRule{
	{ID: "NAT-001", Type: "DNAT", FirewallID: "FW-PA01", FromZone: "untrust", ToZone: "dmz", Original: "203.0.113.10:443", Translated: "10.1.0.10:443", Description: "Portal inbound VIP"},
	{ID: "NAT-002", Type: "DNAT", FirewallID: "FW-PA01", FromZone: "untrust", ToZone: "dmz", Original: "203.0.113.11:443", Translated: "10.1.0.11:443", Description: "API inbound VIP"},
	{ID: "NAT-003", Type: "SNAT", FirewallID: "FW-FG01", FromZone: "hybrid-int", ToZone: "hybrid-ext", Original: "10.2.0.0/24", Translated: "172.16.100.1", Description: "On-prem to AWS masquerade"},
	{ID: "NAT-004", Type: "SNAT", FirewallID: "FW-PA01", FromZone: "trust-app", ToZone: "untrust", Original: "10.2.0.0/24", Translated: "203.0.113.100", Description: "App tier outbound"},
}

var routes = []Route{
	{ID: "RT-001", Destination: "172.16.0.0/12", NextHop: "10.5.0.1", Interface: "VPN tunnel", FirewallID: "FW-FG01", Description: "To AWS via Direct Connect"},
	{ID: "RT-002", Destination: "192.168.0.0/16", NextHop: "10.5.0.2", Interface: "VPN tunnel", FirewallID: "FW-FG01", Description: "To Azure via ExpressRoute"},
	{ID: "RT-003", Destination: "10.0.0.0/8", NextHop: "10.5.0.254", Interface: "internal", FirewallID: "FW-FG01", Description: "Internal routing"},
	{ID: "RT-004", Destination: "0.0.0.0/0", NextHop: "203.0.113.1", Interface: "external", FirewallID: "FW-PA01", Description: "Default internet"},
	{ID: "RT-005", Destination: "10.4.0.0/24", NextHop: "10.2.0.254", Interface: "internal", FirewallID: "FW-PA02", Description: "App to management"},
}

var links = []Link{
	{ID: "LINK-001", Name: "AWS Direct Connect", FromSiteID: "SITE-DC1", ToSiteID: "SITE-AWS1", Bandwidth: "1Gbps", Status: "active", Type: "VPN over Direct Connect"},
	{ID: "LINK-002", Name: "Azure ExpressRoute", FromSiteID: "SITE-DC1", ToSiteID: "SITE-AZ1", Bandwidth: "500Mbps", Status: "active", Type: "ExpressRoute private peering"},
	{ID: "LINK-003", Name: "Branch VPN", FromSiteID: "SITE-EDGE1", ToSiteID: "SITE-DC1", Bandwidth: "100Mbps", Status: "active", Type: "IPSec VPN"},
}

var incidentScenarios = []IncidentScenario{
	{
		ID:      "SCENARIO-001",
		Title:   "Internet to DMZ Application Failure",
		Symptom: "External customers cannot reach the Customer Portal at 203.0.113.10",
		Path:    "Internet -> FW-PA01 (untrust->dmz) -> NAT-001 -> RULE-001 -> SRV-WEB01",
		Affected: []string{
			"NAT-001", "RULE-001", "FW-PA01", "SRV-WEB01",
			"ADDR-PORTAL-PUB", "ADDR-PORTAL-INT", "GRP-DMZ-SERVERS",
		},
		BrokenObject: "GRP-DMZ-SERVERS",
		Explanation:  "ADDR-PORTAL-INT (10.1.0.10) was removed from GRP-DMZ-SERVERS during a routine address-group cleanup. RULE-001 allows traffic to GRP-DMZ-SERVERS, but since the portal IP is no longer a member, the rule no longer matches. NAT-001 still translates 203.0.113.10->10.1.0.10, but the security policy drops the post-NAT flow. Fix: re-add ADDR-PORTAL-INT to GRP-DMZ-SERVERS.",
	},
	{
		ID:      "SCENARIO-002",
		Title:   "On-Prem to AWS Application Path Failure",
		Symptom: "Loan Processing Service (10.2.0.10) cannot reach Cloud Loan API (172.16.1.10) -- connection timeouts",
		Path:    "SRV-APP01 -> FW-FG01 (hybrid-int->hybrid-ext) -> NAT-003 -> LINK-001 -> SRV-AWS01",
		Affected: []string{
			"SRV-APP01", "FW-FG01", "RULE-009", "NAT-003",
			"LINK-001", "RT-001", "SRV-AWS01",
		},
		BrokenObject: "NAT-003",
		Explanation:  "RULE-009 allows the flow from 10.2.0.0/24 to 172.16.0.0/12. NAT-003 translates the source to 172.16.100.1 before it crosses LINK-001. However, the AWS security group on SRV-AWS01 only allows 172.16.1.0/24 (the AWS application subnet), not 172.16.100.1. The SNAT address is outside the expected source range. Fix: add 172.16.100.1/32 to the AWS security group inbound rule, or change NAT-003 to use an address within 172.16.1.0/24.",
	},
	{
		ID:      "SCENARIO-003",
		Title:   "Azure to On-Prem Management Access Failure",
		Symptom: "Azure AD Connector (192.168.1.10) cannot SSH to Bastion Host (10.4.0.10) -- connection refused",
		Path:    "SRV-AZ01 -> LINK-002 -> FW-FG01 (hybrid-ext->mgmt-zone) -> RULE-008 -> SRV-MGMT01",
		Affected: []string{
			"SRV-AZ01", "LINK-002", "FW-FG01", "RULE-008",
			"GRP-MGMT-HOSTS", "ADDR-BASTION", "SRV-MGMT01",
		},
		BrokenObject: "RULE-008",
		Explanation:  "RULE-008 is configured for zone pair hybrid-ext->mgmt-zone. After a recent interface change on FW-FG01, the route to 10.4.0.0/24 now exits through the hybrid-int interface instead of the mgmt-zone interface. The traffic matches zone pair hybrid-ext->hybrid-int instead, where no allow rule exists for SSH to management hosts. Fix: either reassign the 10.4.0.0/24 route to the mgmt-zone interface, or add an allow rule for hybrid-ext->hybrid-int targeting GRP-MGMT-HOSTS on SSH/22.",
	},
}

// ---------------------------------------------------------------------------
// Lookup helpers
// ---------------------------------------------------------------------------

func findSiteByID(id string) *Site {
	for i := range sites {
		if strings.EqualFold(sites[i].ID, id) {
			return &sites[i]
		}
	}
	return nil
}

func findSegmentByID(id string) *Segment {
	for i := range segments {
		if strings.EqualFold(segments[i].ID, id) {
			return &segments[i]
		}
	}
	return nil
}

func findServerByID(id string) *Server {
	for i := range servers {
		if strings.EqualFold(servers[i].ID, id) {
			return &servers[i]
		}
	}
	return nil
}

func findFirewallByID(id string) *Firewall {
	for i := range firewalls {
		if strings.EqualFold(firewalls[i].ID, id) {
			return &firewalls[i]
		}
	}
	return nil
}

func findAddressObjectByID(id string) *AddressObject {
	for i := range addressObjects {
		if strings.EqualFold(addressObjects[i].ID, id) {
			return &addressObjects[i]
		}
	}
	return nil
}

func findAddressGroupByID(id string) *AddressGroup {
	for i := range addressGroups {
		if strings.EqualFold(addressGroups[i].ID, id) {
			return &addressGroups[i]
		}
	}
	return nil
}

// resolveAddressToIPs resolves an address reference (IP, CIDR, address object ID,
// or address group ID) to a list of IP strings for matching purposes.
func resolveAddressToIPs(ref string) []string {
	// Check if it's an address group
	grp := findAddressGroupByID(ref)
	if grp != nil {
		var ips []string
		for _, memberID := range grp.Members {
			obj := findAddressObjectByID(memberID)
			if obj != nil {
				ips = append(ips, obj.Value)
			}
		}
		return ips
	}
	// Check if it's an address object
	obj := findAddressObjectByID(ref)
	if obj != nil {
		return []string{obj.Value}
	}
	// Return as-is (raw IP or CIDR)
	return []string{ref}
}

// ipInCIDR checks if an IP address falls within a CIDR range.
func ipInCIDR(ipStr, cidrStr string) bool {
	// Strip port if present
	if h, _, err := net.SplitHostPort(ipStr); err == nil {
		ipStr = h
	}
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	_, cidrNet, err := net.ParseCIDR(cidrStr)
	if err != nil {
		return false
	}
	return cidrNet.Contains(ip)
}

// matchAddress checks if a query matches a rule's source/destination field.
// It handles "any", direct IP match, CIDR match, address object match, and
// address group membership match.
func matchAddress(query string, ruleAddr string) bool {
	if ruleAddr == "any" {
		return true
	}
	q := strings.ToLower(strings.TrimSpace(query))

	// Direct string match on rule address field
	if strings.EqualFold(ruleAddr, query) {
		return true
	}

	// Resolve rule address to IPs and check
	ruleIPs := resolveAddressToIPs(ruleAddr)
	for _, rip := range ruleIPs {
		if strings.EqualFold(rip, query) {
			return true
		}
		// Check if query IP is in a CIDR from the rule
		if strings.Contains(rip, "/") && ipInCIDR(query, rip) {
			return true
		}
	}

	// Check if the query is a CIDR and the rule address falls within it
	if strings.Contains(q, "/") {
		for _, rip := range ruleIPs {
			if !strings.Contains(rip, "/") && ipInCIDR(rip, query) {
				return true
			}
		}
	}

	// Check if query is an address object ID or group ID that matches the rule
	queryIPs := resolveAddressToIPs(query)
	for _, qip := range queryIPs {
		for _, rip := range ruleIPs {
			if strings.EqualFold(qip, rip) {
				return true
			}
			if strings.Contains(rip, "/") && ipInCIDR(qip, rip) {
				return true
			}
		}
	}

	return false
}

// findSegmentForIP returns the segment a given IP belongs to.
func findSegmentForIP(ipStr string) *Segment {
	for i := range segments {
		if ipInCIDR(ipStr, segments[i].CIDR) {
			return &segments[i]
		}
	}
	return nil
}

// findServerByIP returns the server with the given IP.
func findServerByIP(ipStr string) *Server {
	for i := range servers {
		if servers[i].IP == ipStr {
			return &servers[i]
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Tool argument structs
// ---------------------------------------------------------------------------

type FindServerArgs struct {
	Query string `json:"query" jsonschema:"Search servers by name, ID, IP address, role, or segment ID"`
}

type GetSiteSummaryArgs struct {
	SiteID string `json:"site_id" jsonschema:"Site ID (e.g. SITE-DC1, SITE-AWS1)"`
}

type GetFirewallRulesArgs struct {
	Source      string `json:"source" jsonschema:"Source IP, CIDR, address object ID, address group ID, or zone name"`
	Destination string `json:"destination" jsonschema:"Destination IP, CIDR, address object ID, address group ID, or zone name"`
}

type GetNATRulesArgs struct {
	Query string `json:"query" jsonschema:"Search NAT rules by host IP, address, service name, or description keyword"`
}

type GetAddressGroupArgs struct {
	GroupName string `json:"group_name" jsonschema:"Address group ID (e.g. GRP-DMZ-SERVERS)"`
}

type TracePathArgs struct {
	Source      string `json:"source" jsonschema:"Source IP address or server ID"`
	Destination string `json:"destination" jsonschema:"Destination IP address or server ID"`
}

type GetIncidentScenarioArgs struct {
	ScenarioID string `json:"scenario_id" jsonschema:"Scenario ID (e.g. SCENARIO-001)"`
}

// ---------------------------------------------------------------------------
// Tool handlers
// ---------------------------------------------------------------------------

func handleFindServer(_ context.Context, _ *mcp.CallToolRequest, args FindServerArgs) (*mcp.CallToolResult, any, error) {
	q := strings.ToLower(args.Query)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Server Search: \"%s\"\n\n", args.Query))

	count := 0
	for _, srv := range servers {
		blob := strings.ToLower(srv.ID + " " + srv.Name + " " + srv.IP + " " + srv.Role + " " + srv.SegmentID)
		if strings.Contains(blob, q) {
			seg := findSegmentByID(srv.SegmentID)
			segName := srv.SegmentID
			siteName := ""
			if seg != nil {
				segName = fmt.Sprintf("%s (%s)", seg.Name, seg.ID)
				site := findSiteByID(seg.SiteID)
				if site != nil {
					siteName = fmt.Sprintf("%s (%s)", site.Name, site.ID)
				}
			}
			sb.WriteString(fmt.Sprintf("## %s - %s\n", srv.ID, srv.Name))
			sb.WriteString(fmt.Sprintf("- **IP:** %s\n", srv.IP))
			sb.WriteString(fmt.Sprintf("- **OS:** %s\n", srv.OS))
			sb.WriteString(fmt.Sprintf("- **Role:** %s\n", srv.Role))
			sb.WriteString(fmt.Sprintf("- **Segment:** %s\n", segName))
			if siteName != "" {
				sb.WriteString(fmt.Sprintf("- **Site:** %s\n", siteName))
			}
			sb.WriteString("\n")
			count++
		}
	}

	if count == 0 {
		sb.WriteString("No servers matched the query.\n")
	} else {
		sb.WriteString(fmt.Sprintf("**Total: %d server(s) found**\n", count))
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: sb.String()}},
	}, nil, nil
}

func handleGetSiteSummary(_ context.Context, _ *mcp.CallToolRequest, args GetSiteSummaryArgs) (*mcp.CallToolResult, any, error) {
	site := findSiteByID(args.SiteID)
	if site == nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Site not found: %s", args.SiteID)}},
			IsError: true,
		}, nil, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Site: %s (%s)\n\n", site.Name, site.ID))
	sb.WriteString(fmt.Sprintf("- **Provider:** %s\n", site.Provider))
	sb.WriteString(fmt.Sprintf("- **Region:** %s\n\n", site.Region))

	// Segments
	sb.WriteString("## Network Segments\n\n")
	segCount := 0
	for _, seg := range segments {
		if strings.EqualFold(seg.SiteID, site.ID) {
			sb.WriteString(fmt.Sprintf("### %s (%s)\n", seg.Name, seg.ID))
			sb.WriteString(fmt.Sprintf("- **CIDR:** %s\n", seg.CIDR))

			// Servers in this segment
			sb.WriteString("- **Servers:**\n")
			srvCount := 0
			for _, srv := range servers {
				if strings.EqualFold(srv.SegmentID, seg.ID) {
					sb.WriteString(fmt.Sprintf("  - %s: %s (%s) - %s/%s\n", srv.ID, srv.Name, srv.IP, srv.OS, srv.Role))
					srvCount++
				}
			}
			if srvCount == 0 {
				sb.WriteString("  - (none)\n")
			}
			sb.WriteString("\n")
			segCount++
		}
	}
	if segCount == 0 {
		sb.WriteString("No segments at this site.\n\n")
	}

	// Firewalls at this site
	sb.WriteString("## Firewalls\n\n")
	fwCount := 0
	for _, fw := range firewalls {
		if strings.EqualFold(fw.SiteID, site.ID) {
			sb.WriteString(fmt.Sprintf("### %s - %s\n", fw.ID, fw.Model))
			sb.WriteString(fmt.Sprintf("- **Zones:** %s\n", strings.Join(fw.Zones, ", ")))
			sb.WriteString(fmt.Sprintf("- **HA:** %s\n", fw.HAStatus))
			sb.WriteString(fmt.Sprintf("- **Placement:** %s\n\n", fw.Placement))
			fwCount++
		}
	}
	if fwCount == 0 {
		sb.WriteString("No firewalls at this site.\n\n")
	}

	// Links from/to this site
	sb.WriteString("## Connectivity Links\n\n")
	linkCount := 0
	for _, lnk := range links {
		if strings.EqualFold(lnk.FromSiteID, site.ID) || strings.EqualFold(lnk.ToSiteID, site.ID) {
			sb.WriteString(fmt.Sprintf("- **%s** (%s): %s <-> %s | %s | %s | %s\n",
				lnk.Name, lnk.ID, lnk.FromSiteID, lnk.ToSiteID, lnk.Bandwidth, lnk.Status, lnk.Type))
			linkCount++
		}
	}
	if linkCount == 0 {
		sb.WriteString("No connectivity links.\n")
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: sb.String()}},
	}, nil, nil
}

func handleGetFirewallRules(_ context.Context, _ *mcp.CallToolRequest, args GetFirewallRulesArgs) (*mcp.CallToolResult, any, error) {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Firewall Rules: %s -> %s\n\n", args.Source, args.Destination))

	count := 0
	for _, rule := range securityRules {
		srcMatch := matchAddress(args.Source, rule.Source) || strings.EqualFold(args.Source, rule.FromZone)
		dstMatch := matchAddress(args.Destination, rule.Destination) || strings.EqualFold(args.Destination, rule.ToZone)

		if srcMatch && dstMatch {
			fw := findFirewallByID(rule.FirewallID)
			fwLabel := rule.FirewallID
			if fw != nil {
				fwLabel = fmt.Sprintf("%s (%s)", fw.ID, fw.Model)
			}

			actionIcon := "ALLOW"
			if rule.Action == "deny" {
				actionIcon = "DENY"
			}

			sb.WriteString(fmt.Sprintf("## %s [%s] - %s\n", rule.ID, actionIcon, rule.Description))
			sb.WriteString(fmt.Sprintf("- **Firewall:** %s\n", fwLabel))
			sb.WriteString(fmt.Sprintf("- **Zones:** %s -> %s\n", rule.FromZone, rule.ToZone))
			sb.WriteString(fmt.Sprintf("- **Source:** %s\n", rule.Source))
			sb.WriteString(fmt.Sprintf("- **Destination:** %s\n", rule.Destination))
			sb.WriteString(fmt.Sprintf("- **Service:** %s\n", rule.Service))
			sb.WriteString(fmt.Sprintf("- **Logging:** %v\n\n", rule.LogEnabled))

			// Resolve address references for context
			if rule.Source != "any" {
				srcIPs := resolveAddressToIPs(rule.Source)
				if len(srcIPs) > 0 {
					sb.WriteString(fmt.Sprintf("  *Source resolves to:* %s\n", strings.Join(srcIPs, ", ")))
				}
			}
			if rule.Destination != "any" {
				dstIPs := resolveAddressToIPs(rule.Destination)
				if len(dstIPs) > 0 {
					sb.WriteString(fmt.Sprintf("  *Destination resolves to:* %s\n", strings.Join(dstIPs, ", ")))
				}
			}
			sb.WriteString("\n")
			count++
		}
	}

	if count == 0 {
		sb.WriteString("No matching firewall rules found.\n")
	} else {
		sb.WriteString(fmt.Sprintf("**Total: %d matching rule(s)**\n", count))
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: sb.String()}},
	}, nil, nil
}

func handleGetNATRules(_ context.Context, _ *mcp.CallToolRequest, args GetNATRulesArgs) (*mcp.CallToolResult, any, error) {
	q := strings.ToLower(args.Query)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# NAT Rules: \"%s\"\n\n", args.Query))

	count := 0
	for _, nat := range natRules {
		blob := strings.ToLower(nat.ID + " " + nat.Type + " " + nat.FirewallID + " " +
			nat.Original + " " + nat.Translated + " " + nat.Description +
			" " + nat.FromZone + " " + nat.ToZone)
		if strings.Contains(blob, q) {
			fw := findFirewallByID(nat.FirewallID)
			fwLabel := nat.FirewallID
			if fw != nil {
				fwLabel = fmt.Sprintf("%s (%s)", fw.ID, fw.Model)
			}

			sb.WriteString(fmt.Sprintf("## %s [%s] - %s\n", nat.ID, nat.Type, nat.Description))
			sb.WriteString(fmt.Sprintf("- **Firewall:** %s\n", fwLabel))
			sb.WriteString(fmt.Sprintf("- **Zones:** %s -> %s\n", nat.FromZone, nat.ToZone))
			sb.WriteString(fmt.Sprintf("- **Original:** %s\n", nat.Original))
			sb.WriteString(fmt.Sprintf("- **Translated:** %s\n\n", nat.Translated))
			count++
		}
	}

	if count == 0 {
		sb.WriteString("No matching NAT rules found.\n")
	} else {
		sb.WriteString(fmt.Sprintf("**Total: %d NAT rule(s)**\n", count))
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: sb.String()}},
	}, nil, nil
}

func handleGetAddressGroup(_ context.Context, _ *mcp.CallToolRequest, args GetAddressGroupArgs) (*mcp.CallToolResult, any, error) {
	grp := findAddressGroupByID(args.GroupName)
	if grp == nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Address group not found: %s", args.GroupName)}},
			IsError: true,
		}, nil, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Address Group: %s\n\n", grp.ID))

	sb.WriteString("## Members\n\n")
	for _, memberID := range grp.Members {
		obj := findAddressObjectByID(memberID)
		if obj != nil {
			sb.WriteString(fmt.Sprintf("- **%s** (%s): %s - %s\n", obj.ID, obj.Type, obj.Value, obj.Description))
		} else {
			sb.WriteString(fmt.Sprintf("- **%s** (unresolved)\n", memberID))
		}
	}

	// Find rules that reference this group
	sb.WriteString("\n## Referenced by Security Rules\n\n")
	ruleCount := 0
	for _, rule := range securityRules {
		if strings.EqualFold(rule.Source, grp.ID) || strings.EqualFold(rule.Destination, grp.ID) {
			direction := "source"
			if strings.EqualFold(rule.Destination, grp.ID) {
				direction = "destination"
			}
			if strings.EqualFold(rule.Source, grp.ID) && strings.EqualFold(rule.Destination, grp.ID) {
				direction = "source and destination"
			}
			sb.WriteString(fmt.Sprintf("- **%s** [%s]: %s->%s, %s->%s (%s) [used as %s]\n",
				rule.ID, rule.Action, rule.FromZone, rule.ToZone,
				rule.Source, rule.Destination, rule.Description, direction))
			ruleCount++
		}
	}
	if ruleCount == 0 {
		sb.WriteString("No security rules reference this group.\n")
	}

	// Find NAT rules that reference group member IPs
	sb.WriteString("\n## Related NAT Rules\n\n")
	natCount := 0
	for _, nat := range natRules {
		for _, memberID := range grp.Members {
			obj := findAddressObjectByID(memberID)
			if obj != nil {
				if strings.Contains(nat.Original, obj.Value) || strings.Contains(nat.Translated, obj.Value) {
					sb.WriteString(fmt.Sprintf("- **%s** [%s]: %s -> %s (%s)\n",
						nat.ID, nat.Type, nat.Original, nat.Translated, nat.Description))
					natCount++
					break
				}
			}
		}
	}
	if natCount == 0 {
		sb.WriteString("No NAT rules reference members of this group.\n")
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: sb.String()}},
	}, nil, nil
}

func handleTracePath(_ context.Context, _ *mcp.CallToolRequest, args TracePathArgs) (*mcp.CallToolResult, any, error) {
	var sb strings.Builder

	// Resolve source and destination to IPs
	srcIP := args.Source
	dstIP := args.Destination
	srcLabel := args.Source
	dstLabel := args.Destination

	// Check if source/dest are server IDs
	if srv := findServerByID(args.Source); srv != nil {
		srcIP = srv.IP
		srcLabel = fmt.Sprintf("%s (%s, %s)", srv.ID, srv.Name, srv.IP)
	}
	if srv := findServerByID(args.Destination); srv != nil {
		dstIP = srv.IP
		dstLabel = fmt.Sprintf("%s (%s, %s)", srv.ID, srv.Name, srv.IP)
	}

	// Also try to find server by IP for labeling
	if srv := findServerByIP(srcIP); srv != nil && srcLabel == args.Source {
		srcLabel = fmt.Sprintf("%s (%s, %s)", srv.ID, srv.Name, srv.IP)
	}
	if srv := findServerByIP(dstIP); srv != nil && dstLabel == args.Destination {
		dstLabel = fmt.Sprintf("%s (%s, %s)", srv.ID, srv.Name, srv.IP)
	}

	sb.WriteString(fmt.Sprintf("# Network Path Trace\n\n"))
	sb.WriteString(fmt.Sprintf("**Source:** %s\n", srcLabel))
	sb.WriteString(fmt.Sprintf("**Destination:** %s\n\n", dstLabel))

	step := 1

	// Step 1: Identify source segment
	srcSeg := findSegmentForIP(srcIP)
	if srcSeg != nil {
		srcSite := findSiteByID(srcSeg.SiteID)
		siteName := srcSeg.SiteID
		if srcSite != nil {
			siteName = fmt.Sprintf("%s (%s)", srcSite.Name, srcSite.ID)
		}
		sb.WriteString(fmt.Sprintf("## Step %d: Source Segment\n", step))
		sb.WriteString(fmt.Sprintf("- **Segment:** %s (%s, CIDR: %s)\n", srcSeg.Name, srcSeg.ID, srcSeg.CIDR))
		sb.WriteString(fmt.Sprintf("- **Site:** %s\n\n", siteName))
		step++
	}

	// Step 2: Identify destination segment
	dstSeg := findSegmentForIP(dstIP)
	if dstSeg != nil {
		dstSite := findSiteByID(dstSeg.SiteID)
		siteName := dstSeg.SiteID
		if dstSite != nil {
			siteName = fmt.Sprintf("%s (%s)", dstSite.Name, dstSite.ID)
		}
		sb.WriteString(fmt.Sprintf("## Step %d: Destination Segment\n", step))
		sb.WriteString(fmt.Sprintf("- **Segment:** %s (%s, CIDR: %s)\n", dstSeg.Name, dstSeg.ID, dstSeg.CIDR))
		sb.WriteString(fmt.Sprintf("- **Site:** %s\n\n", siteName))
		step++
	}

	// Step 3: Determine if cross-site (need a link)
	crossSite := false
	if srcSeg != nil && dstSeg != nil && !strings.EqualFold(srcSeg.SiteID, dstSeg.SiteID) {
		crossSite = true
	}

	// Step 4: Find matching routes
	sb.WriteString(fmt.Sprintf("## Step %d: Routing\n", step))
	routeFound := false
	for _, rt := range routes {
		if ipInCIDR(dstIP, rt.Destination) {
			fw := findFirewallByID(rt.FirewallID)
			fwLabel := rt.FirewallID
			if fw != nil {
				fwLabel = fmt.Sprintf("%s (%s)", fw.ID, fw.Model)
			}
			sb.WriteString(fmt.Sprintf("- **%s:** %s via %s (%s) on %s - %s\n",
				rt.ID, rt.Destination, rt.NextHop, rt.Interface, fwLabel, rt.Description))
			routeFound = true
		}
	}
	if !routeFound {
		sb.WriteString("- No explicit route found (local/connected segment)\n")
	}
	sb.WriteString("\n")
	step++

	// Step 5: Find NAT rules that might apply
	sb.WriteString(fmt.Sprintf("## Step %d: NAT Translation\n", step))
	natFound := false
	for _, nat := range natRules {
		origContainsSrc := strings.Contains(nat.Original, srcIP) || (srcSeg != nil && strings.Contains(nat.Original, srcSeg.CIDR))
		origContainsDst := strings.Contains(nat.Original, dstIP)
		translatedContainsSrc := strings.Contains(nat.Translated, srcIP)
		translatedContainsDst := strings.Contains(nat.Translated, dstIP)

		if origContainsSrc || origContainsDst || translatedContainsSrc || translatedContainsDst {
			sb.WriteString(fmt.Sprintf("- **%s** [%s]: %s -> %s (%s)\n",
				nat.ID, nat.Type, nat.Original, nat.Translated, nat.Description))
			natFound = true
		}
	}
	if !natFound {
		sb.WriteString("- No NAT translation applies to this path\n")
	}
	sb.WriteString("\n")
	step++

	// Step 6: Find applicable firewall rules
	sb.WriteString(fmt.Sprintf("## Step %d: Security Policy Evaluation\n", step))
	ruleFound := false
	for _, rule := range securityRules {
		srcMatch := matchAddress(srcIP, rule.Source)
		dstMatch := matchAddress(dstIP, rule.Destination)

		if srcMatch && dstMatch {
			fw := findFirewallByID(rule.FirewallID)
			fwLabel := rule.FirewallID
			if fw != nil {
				fwLabel = fmt.Sprintf("%s (%s)", fw.ID, fw.Model)
			}

			actionIcon := "ALLOW"
			if rule.Action == "deny" {
				actionIcon = "DENY"
			}

			sb.WriteString(fmt.Sprintf("- **%s** [%s] on %s: %s->%s, %s->%s, %s - %s\n",
				rule.ID, actionIcon, fwLabel,
				rule.FromZone, rule.ToZone,
				rule.Source, rule.Destination,
				rule.Service, rule.Description))
			ruleFound = true
		}
	}
	if !ruleFound {
		sb.WriteString("- No explicit security rules match; default deny applies\n")
	}
	sb.WriteString("\n")
	step++

	// Step 7: Cross-site link
	if crossSite {
		sb.WriteString(fmt.Sprintf("## Step %d: Cross-Site Link\n", step))
		linkFound := false
		for _, lnk := range links {
			fromMatch := (srcSeg != nil && strings.EqualFold(lnk.FromSiteID, srcSeg.SiteID)) ||
				(dstSeg != nil && strings.EqualFold(lnk.FromSiteID, dstSeg.SiteID))
			toMatch := (srcSeg != nil && strings.EqualFold(lnk.ToSiteID, srcSeg.SiteID)) ||
				(dstSeg != nil && strings.EqualFold(lnk.ToSiteID, dstSeg.SiteID))
			if fromMatch && toMatch {
				sb.WriteString(fmt.Sprintf("- **%s** (%s): %s <-> %s | %s | %s | %s\n",
					lnk.Name, lnk.ID, lnk.FromSiteID, lnk.ToSiteID, lnk.Bandwidth, lnk.Status, lnk.Type))
				linkFound = true
			}
		}
		if !linkFound {
			sb.WriteString("- WARNING: No direct link found between sites\n")
		}
		sb.WriteString("\n")
		step++
	}

	// Summary
	sb.WriteString(fmt.Sprintf("## Step %d: Path Summary\n", step))
	sb.WriteString(fmt.Sprintf("```\n%s", srcLabel))
	if srcSeg != nil {
		sb.WriteString(fmt.Sprintf("\n  | [%s / %s]", srcSeg.Name, srcSeg.CIDR))
	}
	sb.WriteString("\n  v\n")
	if routeFound {
		sb.WriteString("  [Routing]\n  v\n")
	}
	if natFound {
		sb.WriteString("  [NAT Translation]\n  v\n")
	}
	if ruleFound {
		sb.WriteString("  [Security Policy]\n  v\n")
	}
	if crossSite {
		sb.WriteString("  [Cross-Site Link]\n  v\n")
	}
	sb.WriteString(fmt.Sprintf("%s", dstLabel))
	if dstSeg != nil {
		sb.WriteString(fmt.Sprintf("\n  | [%s / %s]", dstSeg.Name, dstSeg.CIDR))
	}
	sb.WriteString("\n```\n")

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: sb.String()}},
	}, nil, nil
}

func handleGetIncidentScenario(_ context.Context, _ *mcp.CallToolRequest, args GetIncidentScenarioArgs) (*mcp.CallToolResult, any, error) {
	for _, scenario := range incidentScenarios {
		if strings.EqualFold(scenario.ID, args.ScenarioID) {
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("# Incident Scenario: %s\n\n", scenario.ID))
			sb.WriteString(fmt.Sprintf("## %s\n\n", scenario.Title))

			sb.WriteString("### Symptom\n")
			sb.WriteString(fmt.Sprintf("%s\n\n", scenario.Symptom))

			sb.WriteString("### Expected Path\n")
			sb.WriteString(fmt.Sprintf("```\n%s\n```\n\n", scenario.Path))

			sb.WriteString("### Affected Objects\n")
			for _, objID := range scenario.Affected {
				label := objID
				// Try to resolve for more context
				if srv := findServerByID(objID); srv != nil {
					label = fmt.Sprintf("%s (%s, %s)", srv.ID, srv.Name, srv.IP)
				} else if fw := findFirewallByID(objID); fw != nil {
					label = fmt.Sprintf("%s (%s)", fw.ID, fw.Model)
				} else if obj := findAddressObjectByID(objID); obj != nil {
					label = fmt.Sprintf("%s (%s: %s)", obj.ID, obj.Type, obj.Value)
				} else if grp := findAddressGroupByID(objID); grp != nil {
					members := strings.Join(grp.Members, ", ")
					label = fmt.Sprintf("%s [members: %s]", grp.ID, members)
				} else {
					// Check NAT rules
					for _, nat := range natRules {
						if strings.EqualFold(nat.ID, objID) {
							label = fmt.Sprintf("%s (%s: %s -> %s)", nat.ID, nat.Type, nat.Original, nat.Translated)
							break
						}
					}
					// Check security rules
					for _, rule := range securityRules {
						if strings.EqualFold(rule.ID, objID) {
							label = fmt.Sprintf("%s (%s %s->%s: %s)", rule.ID, rule.Action, rule.FromZone, rule.ToZone, rule.Description)
							break
						}
					}
					// Check routes
					for _, rt := range routes {
						if strings.EqualFold(rt.ID, objID) {
							label = fmt.Sprintf("%s (%s via %s)", rt.ID, rt.Destination, rt.NextHop)
							break
						}
					}
					// Check links
					for _, lnk := range links {
						if strings.EqualFold(lnk.ID, objID) {
							label = fmt.Sprintf("%s (%s: %s <-> %s)", lnk.ID, lnk.Name, lnk.FromSiteID, lnk.ToSiteID)
							break
						}
					}
				}
				sb.WriteString(fmt.Sprintf("- %s\n", label))
			}

			sb.WriteString(fmt.Sprintf("\n### Broken Object\n"))
			sb.WriteString(fmt.Sprintf("**%s**\n\n", scenario.BrokenObject))

			sb.WriteString("### Root Cause Analysis\n")
			sb.WriteString(fmt.Sprintf("%s\n", scenario.Explanation))

			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: sb.String()}},
			}, nil, nil
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Scenario not found: %s", args.ScenarioID)}},
		IsError: true,
	}, nil, nil
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "hybrid-infra-tools",
		Version: "1.0.0",
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "find_server",
		Description: "Search servers by name, ID, IP address, role, or segment. Returns matching servers with full details including site and segment information.",
	}, handleFindServer)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_site_summary",
		Description: "Get full details of a site including all network segments, servers, firewalls, and connectivity links.",
	}, handleGetSiteSummary)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_firewall_rules",
		Description: "Find security rules matching a source and destination. Accepts IPs, CIDRs, address object IDs, address group IDs, or zone names. Returns all matching rules with resolved addresses.",
	}, handleGetFirewallRules)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_nat_rules",
		Description: "Search NAT rules by host IP, address, service name, or keyword. Returns matching DNAT and SNAT rules with translation details.",
	}, handleGetNATRules)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_address_group",
		Description: "Get address group members and find all security and NAT rules that reference the group. Useful for understanding the impact of group membership changes.",
	}, handleGetAddressGroup)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "trace_path",
		Description: "Trace the expected network path between two endpoints. Identifies segments, routing, NAT translations, security policy evaluation, and cross-site links. Accepts IPs or server IDs.",
	}, handleTracePath)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_incident_scenario",
		Description: "Get a full incident scenario with symptom, expected path, affected objects, the broken object, and root cause analysis with recommended fix.",
	}, handleGetIncidentScenario)

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

	addr := ":8087"
	log.Printf("Starting hybrid-infra-tools MCP server on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
