# Solo Bank Enterprise AI Demo

A fully functional multi-agent banking demo built on the Solo.io platform — featuring kagent Enterprise, AgentRegistry, and MCP tool servers running end-to-end inside a local Kubernetes cluster.

## Quick Start

```bash
# Set your OpenAI API key
export OPENAI_API_KEY="sk-..."

# Optional: Enterprise license (works in trial mode without it)
# export AGENTGATEWAY_LICENSE_KEY="..."

# Run setup — creates Kind cluster, installs everything, deploys all agents
./setup.sh
```

Once complete:

| Service | URL | Description |
|---------|-----|-------------|
| **Docs Site** | http://localhost:30500 | Documentation and usage guide |
| **Management UI** | http://localhost:30090 | Chat with agents |
| **AgentRegistry** | http://localhost:30121 | Skill catalog REST API |
| **Bank Wiki** | http://localhost:30400 | Customer and policy data |
| **kagent API** | http://localhost:30083 | Agent runtime API |

## What's Included

### 11 AI Agents

| Agent | Model | Tools | Description |
|-------|-------|-------|-------------|
| **Triage** | GPT-4o | None (router) | Front-door agent that routes to the right specialist |
| **Customer Service** | GPT-4o Mini | customer-tools, transaction-tools | Account inquiries, balance checks, transaction reviews |
| **Mortgage Advisor** | GPT-4o | customer-tools, policy-tools | Personalized rate quotes, lending guidance |
| **Compliance** | GPT-4o | All 3 bank tools | Internal AML/KYC audits, policy compliance (internal only) |
| **IT Support** | GPT-4o | All 3 bank tools | IT tickets, system troubleshooting, cross-team coordination |
| **Kubernetes** | GPT-4o | 18 K8s tools | Live cluster access — pods, logs, manifests, diagnostics |
| **Helm** | GPT-4o Mini | None (knowledge-based) | Helm release management, upgrades, rollbacks |
| **Operations Monitor** | GPT-4o | status-tools | Real-time app health and datacenter monitoring |
| **Incident Manager** | GPT-4o | incident-tools | Incident tracking (P1/P2/P3), IT ticket management |
| **Infrastructure Support** | GPT-4o | All 3 bank tools | Multi-domain coordinator (K8s + Helm + IT) |
| **Operations Center** | GPT-4o | 3 sub-agents | Multi-agent coordinator (Ops + Incidents + IT) |

### 6 MCP Tool Servers

| Server | Port | Tools | Description |
|--------|------|-------|-------------|
| **bank-customer-tools** | 8081 | 5 | Customer profiles, account balances, search |
| **bank-policy-tools** | 8082 | 5 | Lending policies, rate tables, credit tiers |
| **bank-transaction-tools** | 8083 | 5 | Transaction history, search, account analytics |
| **bank-status-tools** | 8085 | 5 | App health monitoring, datacenter status |
| **bank-incident-tools** | 8086 | 5 | Incident management, IT ticketing |
| **kagent-tool-server** | 8084 | 18 | Built-in Kubernetes operations tools |

### Platform Stack

| Component | Version |
|-----------|---------|
| kagent Enterprise | v0.3.14 |
| Management UI | v0.3.14 |
| AgentRegistry OSS | v0.3.3 |
| Kind | v0.20+ |

## Architecture

```
User → Management UI → kagent Controller → Agent (GPT-4o)
                                              ↓
                                         KMCP routing
                                              ↓
                                     RemoteMCPServer CRD
                                              ↓
                                    MCP Tool Server (Go)
                                              ↓
                                      Simulated Data
```

**Namespaces:**
- `kagent` — Agent runtime, controller, KMCP, Management UI, all Agent/ModelConfig/RemoteMCPServer CRDs
- `bank-wiki` — All MCP tool servers, wiki server, docs site
- `agentregistry` — Skill catalog REST API

**Multi-Agent Patterns:**
- **Infrastructure Support**: Uses MCP tools across all domains for cross-layer diagnosis
- **Operations Center**: Uses `type: Agent` tools to delegate to 3 sub-agents for unified operations view

## Prerequisites

- Docker
- Kind
- kubectl
- Helm
- curl, jq, openssl
- `OPENAI_API_KEY` environment variable

## Project Structure

```
.
├── setup.sh                    # One-command setup script
├── kind-config.yaml            # Kind cluster with NodePort mappings
├── .env.example                # Environment variable template
├── manifests/
│   ├── namespaces.yaml
│   ├── agents/                 # Agent CRDs + ModelConfig CRDs
│   ├── mcp/                    # RemoteMCPServer CRDs
│   └── bank-wiki/              # Deployment + Service for each tool server
├── mcp-tools/
│   ├── customer-tools/         # Go MCP server (port 8081)
│   ├── policy-tools/           # Go MCP server (port 8082)
│   ├── transaction-tools/      # Go MCP server (port 8083)
│   ├── status-tools/           # Go MCP server (port 8085)
│   └── incident-tools/         # Go MCP server (port 8086)
├── wiki-server/                # Bank wiki content server
├── docs-site/                  # Static documentation site (nginx)
└── sample-agent/               # Tutorial: build your own agent
```

## Teardown

```bash
kind delete cluster --name solo-bank-demo
```
