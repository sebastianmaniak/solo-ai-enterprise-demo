# Solo AI Enterprise Banking Demo — Design Spec

## Overview

A local Kind-based demo environment showcasing Solo.io's AI enterprise platform (AgentGateway, AgentRegistry, kagent) with a realistic banking scenario. A custom Go-based wiki serves markdown content about "Solo Bank" — 100 customers, detailed policies, rate tables, and procedures. Three MCP tool servers provide domain-specific tools that four kagent agents use to answer banking questions through AgentGateway.

**Goal:** Demonstrate multi-agent orchestration, MCP tool routing, multi-model LLM backends, RBAC via tool access segmentation, and full observability — all in a single `kind create cluster` + `./setup.sh`.

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Kind Cluster: solo-bank-demo                │
│                                                                     │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │ agentgateway-system                                         │   │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐  │   │
│  │  │  AGW Controller│  │  AGW Proxy   │  │  Management UI   │  │   │
│  │  └──────────────┘  └──────┬───────┘  └──────────────────┘  │   │
│  │                           │                                  │   │
│  │  ┌──────────────┐  ┌─────┴────────┐  ┌──────────────────┐  │   │
│  │  │ OpenAI Backend│  │Anthropic Back│  │ OTEL Collector   │  │   │
│  │  └──────────────┘  └──────────────┘  └──────────────────┘  │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                                                     │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │ kagent                                                       │   │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐  │   │
│  │  │kagent Ctrl   │  │  kagent UI   │  │   ClickHouse     │  │   │
│  │  └──────────────┘  └──────────────┘  └──────────────────┘  │   │
│  │                                                              │   │
│  │  Agents:                                                     │   │
│  │  ┌────────────┐ ┌──────────────┐ ┌──────────┐ ┌──────────┐ │   │
│  │  │  Triage    │ │ Cust Service │ │ Mortgage │ │Compliance│ │   │
│  │  └────────────┘ └──────────────┘ └──────────┘ └──────────┘ │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                                                     │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │ agentregistry                                                │   │
│  │  ┌──────────────────┐  ┌──────────────────┐                 │   │
│  │  │ AgentRegistry OSS│  │ PostgreSQL+pgvec │                 │   │
│  │  └──────────────────┘  └──────────────────┘                 │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                                                     │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │ bank-wiki                                                    │   │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐  │   │
│  │  │  Wiki Server │  │Customer Tools│  │  Policy Tools    │  │   │
│  │  └──────────────┘  └──────────────┘  └──────────────────┘  │   │
│  │                     ┌──────────────┐                        │   │
│  │                     │Transact Tools│                        │   │
│  │                     └──────────────┘                        │   │
│  └─────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────┘
```

**Data flow:**
1. User prompt → kagent Agent → LLM (via AgentGateway HTTPRoute → OpenAI/Anthropic)
2. LLM returns tool_call → kagent → AgentGateway MCPRoute → MCP Tool Server
3. MCP Tool Server → Wiki Server REST API → markdown content → tool result
4. Tool result → LLM → final response to user
5. All traffic through AgentGateway emits OTEL traces → Collector → ClickHouse

## 1. Infrastructure

### Kind Cluster

- **Name:** `solo-bank-demo`
- **Nodes:** 1 (single control-plane node)
- **Port mappings:**
  - `30080:80` — AgentGateway proxy
  - `30121:30121` — AgentRegistry
  - `30400:30400` — Management UI

### Kind config (`kind-config.yaml`):
```yaml
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
```

### Namespaces

| Namespace | Purpose |
|-----------|---------|
| `agentgateway-system` | AgentGateway Enterprise controller, proxy, management UI, OTEL collector |
| `agentregistry` | AgentRegistry OSS + bundled PostgreSQL with pgvector |
| `kagent` | kagent Enterprise controller, UI, ClickHouse, Agent CRDs |
| `bank-wiki` | Wiki server + 3 MCP tool servers |

### Helm Releases (in deployment order)

#### 1. Gateway API CRDs
```
kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.5.0/standard-install.yaml
```

#### 2. AgentRegistry OSS
- Chart: `oci://ghcr.io/agentregistry-dev/agentregistry/charts/agentregistry`
- Version: `v0.3.3`
- Namespace: `agentregistry`
- Key values:
  - `config.jwtPrivateKey` — randomly generated 32-byte hex
  - `config.enableAnonymousAuth=true`
  - `service.type=NodePort`, `service.nodePorts.http=30121`
  - `database.postgres.vectorEnabled=true`
  - `database.postgres.bundled.image.repository=pgvector`
  - `database.postgres.bundled.image.name=pgvector`
  - `database.postgres.bundled.image.tag=pg16`

#### 3. AgentGateway Enterprise CRDs
- Chart: `oci://us-docker.pkg.dev/solo-public/enterprise-agentgateway/charts/enterprise-agentgateway-crds`
- Version: `v2.3.0-beta.8`
- Namespace: `agentgateway-system`

#### 4. AgentGateway Enterprise
- Chart: `oci://us-docker.pkg.dev/solo-public/enterprise-agentgateway/charts/enterprise-agentgateway`
- Version: `v2.3.0-beta.8`
- Namespace: `agentgateway-system`
- Key values:
  - `licensing.licenseKey` — from `$AGENTGATEWAY_LICENSE_KEY` env var

#### 5. Gateway + Tracing Policy
- `Gateway` resource: `agentgateway-proxy` in `agentgateway-system`, class `enterprise-agentgateway`, HTTP port 80
- `EnterpriseAgentgatewayPolicy` resource: `tracing`, targets the Gateway, sends traces to `solo-enterprise-telemetry-collector:4317`

#### 6. Management UI
- Chart: `oci://us-docker.pkg.dev/solo-public/solo-enterprise-helm/charts/management`
- Version: `v0.3.14`
- Namespace: `agentgateway-system`
- Key values:
  - `cluster=solo-bank-demo`
  - `products.agentgateway.enabled=true`
  - `licensing.licenseKey` — from `$AGENTGATEWAY_LICENSE_KEY` env var

#### 7. LLM Backends
**OpenAI:**
- Secret `openai-secret` with `Authorization: $OPENAI_API_KEY`
- `AgentgatewayBackend` named `openai` — provider `openai`, model `gpt-4o-mini`
- `HTTPRoute` named `openai` — path prefix `/openai` → backend

**Anthropic:**
- Secret `anthropic-secret` with `Authorization: $ANTHROPIC_API_KEY`
- `AgentgatewayBackend` named `anthropic` — provider `anthropic`, model `claude-sonnet-4-6`
- `HTTPRoute` named `anthropic` — path prefix `/anthropic` → backend

#### 8. kagent Enterprise CRDs
- Chart: `us-docker.pkg.dev/solo-public/kagent-enterprise-helm/charts/kagent-enterprise-crds`
- Version: `v0.3.14`
- Namespace: `kagent`

#### 9. kagent Enterprise
- Chart: `us-docker.pkg.dev/solo-public/kagent-enterprise-helm/charts/kagent-enterprise`
- Version: `v0.3.14`
- Namespace: `kagent`
- Key values:
  - Default LLM provider: OpenAI (gpt-4o-mini)
  - OTEL tracing to `solo-enterprise-telemetry-collector.kagent.svc.cluster.local:4317`
  - Controller + KMCP enabled

### Required Environment Variables
- `OPENAI_API_KEY` — OpenAI API key
- `ANTHROPIC_API_KEY` — Anthropic API key  
- `AGENTGATEWAY_LICENSE_KEY` — Solo.io enterprise license

### Prerequisites
- `docker`
- `kind`
- `kubectl`
- `helm`
- `curl`
- `jq`
- `openssl`

## 2. Bank Wiki Server

### Overview
A Go HTTP server that embeds and serves a library of markdown files about Solo Bank. Provides page retrieval and full-text keyword search.

### API

| Method | Endpoint | Description | Response |
|--------|----------|-------------|----------|
| `GET` | `/wiki/{category}/{page}` | Get a single page | Raw markdown (`text/markdown`) |
| `GET` | `/wiki/{category}/` | List pages in a category | JSON array of `{name, title, path}` |
| `GET` | `/wiki/` | List all categories | JSON array of category names |
| `GET` | `/search?q={query}` | Full-text keyword search | JSON array of `{path, title, snippet, score}` |
| `GET` | `/health` | Health check | `{"status": "ok"}` |

### Implementation
- Uses Go `embed` to bake all markdown content into the binary at compile time
- On startup, builds an in-memory inverted index (word → list of `{page, positions}`)
- Search tokenizes the query, looks up each term, scores by term frequency across the page, returns top 10 results with a ~200 character context snippet around the first match
- Single binary, no external dependencies, no database

### Docker Image
- Multi-stage build: `golang:1.22-alpine` → `scratch` (or `alpine` for debugging)
- Final image size: ~15-20MB
- Exposes port 8080

### Kubernetes Resources
- `Deployment` (1 replica) + `Service` (ClusterIP, port 8080) in `bank-wiki` namespace

## 3. Bank Wiki Content

### Bank Identity
- **Name:** Solo Bank
- **Founded:** 1952
- **Headquarters:** Springfield, IL
- **Tagline:** "Building Futures, One Account at a Time"
- **FDIC Member, Equal Housing Lender**

### Content Categories

#### `customers/` — 100 Customer Profiles
Each customer markdown file contains:
- **Personal info:** Name, customer ID (CUST-00001 through CUST-00100), age, DOB, employment, salary, credit score, customer-since date, risk rating
- **Accounts:** Checking, savings, credit cards with balances, limits, APRs, account numbers
- **Recent transactions:** Last 30 days, 10-20 transactions per customer
- **Mortgage details** (where applicable): Type, rate, principal, property address, monthly payment
- **Notes:** Contact preferences, flags (fraud alert, dispute in progress), eligibility notes

**Distribution across 100 customers:**
- Credit scores: 520-830 (normal distribution centered ~700)
  - Poor (520-579): ~8 customers
  - Fair (580-669): ~20 customers
  - Good (670-739): ~35 customers
  - Very Good (740-799): ~25 customers
  - Excellent (800-830): ~12 customers
- Salaries: $28,000-$320,000
- ~60 have mortgages
- ~85 have credit cards
- ~10 have active flags (fraud alerts, disputes, overdue payments)
- ~5 have multiple credit cards
- All 100 have checking accounts, ~90 have savings accounts

#### `policies/` — Banking Policies (~10 documents)

1. **mortgage-lending.md** — Full lending criteria: minimum credit scores by loan type, DTI ratio requirements, down payment minimums, documentation requirements, approval workflow
2. **credit-card-products.md** — Product matrix: card names, annual fees, rewards structures, APR ranges by credit tier, credit limit formulas
3. **credit-score-tiers.md** — The bank's internal tier definitions and what each tier qualifies for
4. **interest-rate-schedule.md** — How rates are calculated: base rate + credit adjustment + relationship discount + term adjustment. Full formula with examples.
5. **overdraft-policy.md** — Overdraft protection options, fees, opt-in requirements, daily limits
6. **kyc-aml-compliance.md** — KYC verification requirements, AML transaction monitoring thresholds ($10K+), SAR filing criteria, enhanced due diligence triggers
7. **fee-schedule.md** — All bank fees: monthly maintenance, ATM, wire transfer, stop payment, account closure, etc.
8. **account-types.md** — Detailed descriptions of each account product (Basic Checking, Premium Checking, Basic Savings, High-Yield Savings, Money Market, CDs)
9. **fraud-detection.md** — Internal fraud detection rules, velocity checks, geographic anomaly detection, merchant category flags
10. **customer-service-escalation.md** — Escalation tiers (L1/L2/L3), response time SLAs, authority levels (what each tier can approve), complaint resolution procedures

#### `rates/` — Current Rate Tables (~4 documents)

1. **mortgage-rates.md** — Current rates by type and term:
   - 30yr Fixed: 6.125% - 7.500% (by credit tier)
   - 15yr Fixed: 5.375% - 6.750%
   - 5/1 ARM: 5.750% - 7.125%
   - Jumbo 30yr: 6.500% - 7.875%
2. **savings-rates.md** — APY by account type and balance tier
3. **cd-rates.md** — CD rates by term (3mo through 5yr)
4. **credit-card-apr.md** — APR by card product and credit tier, promotional rates, penalty APR

#### `products/` — Product Descriptions (~8 documents)
- Platinum Rewards Card, Cashback Card, Secured Credit Card
- Premium Checking, Basic Checking
- High-Yield Savings, Basic Savings
- Home Equity Line of Credit

#### `procedures/` — Internal Procedures (~5 documents)
- New Account Opening
- Dispute Resolution
- Loan Application Process
- Wire Transfer Procedures
- Account Closure

### Internal Consistency Rules
All content must be internally consistent:
- A customer's mortgage rate must match their credit score tier in the rate table
- A customer's credit card APR must match their card product and credit tier
- Account types referenced in customer profiles must exist in `products/`
- Any policy referenced in procedures must exist in `policies/`
- Flagged customers (fraud, disputes) must have corresponding transaction patterns

## 4. MCP Tool Servers

### Overview
Three Go HTTP containers implementing MCP Streamable HTTP transport. Each provides domain-specific tools that read from the wiki server's REST API.

### Common Implementation
- Each tool server is a Go binary with an MCP Streamable HTTP endpoint at `/mcp`
- Tools call the wiki server at `http://bank-wiki-server.bank-wiki.svc.cluster.local:8080`
- Each tool server has its own Dockerfile (multi-stage, same pattern as wiki server)
- Each gets a `Deployment` (1 replica) + `Service` (ClusterIP) in `bank-wiki` namespace

### Tool Server 1: Customer Data Tools (`bank-customer-tools`)

**Port:** 8081

| Tool | Input | Output | Description |
|------|-------|--------|-------------|
| `lookup_customer` | `name` or `customer_id` | Full customer markdown | Fetch a customer's complete profile |
| `search_customers` | `query` string | List of matching customer summaries | Search across customer profiles |
| `get_account_balance` | `account_id` | Balance + account type | Get current balance for a specific account |
| `get_transaction_history` | `customer_id` or `account_id` | Transaction table as markdown | Get recent transactions |

### Tool Server 2: Policy & Rates Tools (`bank-policy-tools`)

**Port:** 8082

| Tool | Input | Output | Description |
|------|-------|--------|-------------|
| `get_policy` | `policy_name` | Full policy markdown | Fetch a specific policy document |
| `search_policies` | `query` string | Matching policy snippets | Search across policies and procedures |
| `get_current_rates` | `rate_type` (mortgage/savings/cd/credit-card) | Rate table markdown | Get current rate tables |
| `get_rate_for_profile` | `credit_score`, `loan_type` | Applicable rate + tier info | Calculate rate for a specific profile |

### Tool Server 3: Transaction & Account Tools (`bank-transaction-tools`)

**Port:** 8083

| Tool | Input | Output | Description |
|------|-------|--------|-------------|
| `get_transaction_history` | `account_id`, optional `date_from`/`date_to` | Transaction list | Detailed transaction history |
| `search_transactions` | `customer_id`, optional `min_amount`/`max_amount`/`merchant` | Matching transactions | Search transactions by criteria |
| `get_account_details` | `account_id` | Full account info | Account type, terms, status, balances |
| `get_customer_accounts` | `customer_id` | List of all accounts | Summary of all accounts for a customer |

### AgentGateway MCP Routing

Each tool server is registered as a `RemoteMCPServer` CRD in `agentgateway-system`:

```yaml
apiVersion: agentgateway.dev/v1alpha1
kind: RemoteMCPServer
metadata:
  name: bank-customer-tools
  namespace: agentgateway-system
spec:
  remote:
    url: http://bank-customer-tools.bank-wiki.svc.cluster.local:8081/mcp
    transport: streamablehttp
```

MCPRoutes connect agents to their authorized tool servers:
```yaml
apiVersion: agentgateway.dev/v1alpha1
kind: MCPRoute
metadata:
  name: customer-service-tools
  namespace: agentgateway-system
spec:
  parentRefs:
  - name: agentgateway-proxy
    namespace: agentgateway-system
  rules:
  - backendRefs:
    - name: bank-customer-tools
      namespace: agentgateway-system
      group: agentgateway.dev
      kind: RemoteMCPServer
    - name: bank-transaction-tools
      namespace: agentgateway-system
      group: agentgateway.dev
      kind: RemoteMCPServer
```

## 5. Agent Configurations

### Agent 1: Triage Agent (`bank-triage-agent`)

```yaml
apiVersion: kagent.dev/v1alpha1
kind: Agent
metadata:
  name: bank-triage-agent
  namespace: kagent
spec:
  description: "Solo Bank — Front Door Triage Agent"
  modelConfig:
    provider: anthropic
    model: claude-sonnet-4-6
    apiKeySecretRef:
      name: anthropic-secret
      namespace: agentgateway-system
  systemPrompt: |
    You are the front-door triage agent for Solo Bank. Your role is to greet customers, understand their needs, and route them to the appropriate specialist.

    Available specialists:
    - Customer Service Agent: Account inquiries, balance checks, transaction questions, general banking
    - Mortgage Advisor Agent: Mortgage rates, refinancing, home equity, lending questions
    - Compliance Agent: Internal use only — policy reviews, fraud investigations, regulatory concerns

    Instructions:
    1. Greet the customer professionally
    2. Listen to their request
    3. If the request clearly maps to one specialist, route immediately with a brief explanation
    4. If ambiguous, ask ONE clarifying question before routing
    5. Never attempt to answer banking questions yourself — always route to a specialist
    6. For compliance/fraud concerns raised by customers, route to Customer Service (not Compliance — that's internal only)
  tools: []
```

### Agent 2: Customer Service Agent (`bank-customer-service-agent`)

- **Model:** OpenAI gpt-4o-mini via AgentGateway
- **Tools:** `bank-customer-tools`, `bank-transaction-tools`
- **System prompt focus:** Identity verification (ask for name/customer ID), account inquiries, transaction questions, professional tone, escalation to specialists for mortgage/compliance topics

### Agent 3: Mortgage Advisor Agent (`bank-mortgage-advisor-agent`)

- **Model:** Anthropic claude-sonnet-4-6 via AgentGateway
- **Tools:** `bank-customer-tools`, `bank-policy-tools`
- **System prompt focus:** Personalized rate quotes using customer's actual credit score and salary, cite specific policies and rate tables, explain qualification criteria, suggest refinancing when beneficial

### Agent 4: Compliance Agent (`bank-compliance-agent`)

- **Model:** Anthropic claude-sonnet-4-6 via AgentGateway
- **Tools:** `bank-customer-tools`, `bank-policy-tools`, `bank-transaction-tools`
- **System prompt focus:** Internal-only agent, review accounts for policy violations, flag suspicious transactions against KYC/AML thresholds, provide specific policy references, structured risk assessments

## 6. Project File Structure

```
solo-ai-enterprise-demo/
├── setup.sh                          # Main deployment script
├── kind-config.yaml                  # Kind cluster configuration
├── manifests/
│   ├── gateway.yaml                  # Gateway + tracing policy
│   ├── llm-backends/
│   │   ├── openai.yaml              # Secret + Backend + HTTPRoute
│   │   └── anthropic.yaml           # Secret + Backend + HTTPRoute
│   ├── mcp/
│   │   ├── remote-mcp-servers.yaml  # RemoteMCPServer CRDs
│   │   └── mcp-routes.yaml          # MCPRoute CRDs
│   ├── agents/
│   │   ├── triage-agent.yaml
│   │   ├── customer-service-agent.yaml
│   │   ├── mortgage-advisor-agent.yaml
│   │   └── compliance-agent.yaml
│   └── bank-wiki/
│       ├── namespace.yaml
│       ├── wiki-server.yaml          # Deployment + Service
│       ├── customer-tools.yaml       # Deployment + Service
│       ├── policy-tools.yaml         # Deployment + Service
│       └── transaction-tools.yaml    # Deployment + Service
├── wiki-server/
│   ├── Dockerfile
│   ├── main.go
│   ├── search.go                     # Inverted index + search
│   └── content/                      # All markdown files (embedded)
│       ├── customers/
│       ├── policies/
│       ├── rates/
│       ├── products/
│       └── procedures/
├── mcp-tools/
│   ├── customer-tools/
│   │   ├── Dockerfile
│   │   └── main.go
│   ├── policy-tools/
│   │   ├── Dockerfile
│   │   └── main.go
│   └── transaction-tools/
│       ├── Dockerfile
│       └── main.go
└── docs/
    └── superpowers/
        └── specs/
            └── 2026-04-08-bank-demo-design.md
```

## 7. Setup Script Flow

`setup.sh` runs everything end-to-end:

1. **Prerequisite check** — Verify docker, kind, kubectl, helm, curl, jq, openssl are installed
2. **Environment check** — Verify OPENAI_API_KEY, ANTHROPIC_API_KEY, AGENTGATEWAY_LICENSE_KEY are set
3. **Create Kind cluster** — `kind create cluster --config kind-config.yaml --name solo-bank-demo`
4. **Install Gateway API CRDs**
5. **Install AgentRegistry OSS** — Helm with pgvector
6. **Install AgentGateway Enterprise** — CRDs + controller
7. **Apply Gateway + tracing policy**
8. **Install Management UI**
9. **Configure LLM backends** — OpenAI + Anthropic (secrets, backends, HTTPRoutes)
10. **Install kagent Enterprise** — CRDs + workload
11. **Build wiki server image** — `docker build` + `kind load docker-image`
12. **Build MCP tool server images** — 3x `docker build` + `kind load docker-image`
13. **Deploy bank-wiki namespace** — Wiki server + 3 tool servers
14. **Wait for wiki pods ready**
15. **Apply MCP routing** — RemoteMCPServer + MCPRoute resources
16. **Apply agent configurations** — 4 Agent CRDs
17. **Print access info** — URLs for Management UI, AgentGateway, AgentRegistry, and port-forward commands

Each step prints clear status with `echo` banners. Steps that can fail use `--wait --timeout 300s` on Helm and `kubectl wait` for pods.

## 8. Port-Forward Commands (printed at end)

```bash
# AgentGateway proxy (already exposed via NodePort on 30080)
# Management UI
kubectl port-forward service/solo-enterprise-ui -n agentgateway-system 4000:80
# AgentRegistry
# Already exposed via NodePort on 30121
# kagent UI
kubectl port-forward service/kagent-ui -n kagent 3000:80
```

## 9. Verification Tests

The setup script ends with basic smoke tests:
1. `curl localhost:30080/openai/v1/chat/completions` — Verify OpenAI backend
2. `curl localhost:30080/anthropic` — Verify Anthropic backend
3. `curl` wiki server (via port-forward or exec) — Verify wiki content
4. `kubectl get agents -n kagent` — Verify all 4 agents are deployed
