#!/usr/bin/env bash
# Register MCP servers, skills, and agents in AgentRegistry.
# Called by setup.sh after all services are running.
set -euo pipefail

AR_URL="${1:-http://agentregistry.agentregistry.svc.cluster.local:8080}"

echo "Registering MCP servers in AgentRegistry..."

# --- MCP Servers ---

curl -sf -X POST "${AR_URL}/v0/servers" \
  -H "Content-Type: application/json" \
  -d '{
  "$schema": "https://static.modelcontextprotocol.io/schemas/2025-10-17/server.schema.json",
  "name": "solo.io/bank-customer-tools",
  "title": "Solo Bank Customer Tools",
  "description": "Customer data tools — lookup, search, account balances",
  "version": "1.0.0",
  "remotes": [{"type": "streamable-http", "url": "http://bank-customer-tools.bank-wiki.svc.cluster.local:8081/mcp"}]
}' > /dev/null

curl -sf -X POST "${AR_URL}/v0/servers" \
  -H "Content-Type: application/json" \
  -d '{
  "$schema": "https://static.modelcontextprotocol.io/schemas/2025-10-17/server.schema.json",
  "name": "solo.io/bank-policy-tools",
  "title": "Solo Bank Policy Tools",
  "description": "Policy and rates tools — lending policies, rate tables, credit tiers",
  "version": "1.0.0",
  "remotes": [{"type": "streamable-http", "url": "http://bank-policy-tools.bank-wiki.svc.cluster.local:8082/mcp"}]
}' > /dev/null

curl -sf -X POST "${AR_URL}/v0/servers" \
  -H "Content-Type: application/json" \
  -d '{
  "$schema": "https://static.modelcontextprotocol.io/schemas/2025-10-17/server.schema.json",
  "name": "solo.io/bank-transaction-tools",
  "title": "Solo Bank Transaction Tools",
  "description": "Transaction and account tools — history, details, search",
  "version": "1.0.0",
  "remotes": [{"type": "streamable-http", "url": "http://bank-transaction-tools.bank-wiki.svc.cluster.local:8083/mcp"}]
}' > /dev/null

echo "  [OK] MCP servers registered"

# --- Skills ---

curl -sf -X POST "${AR_URL}/v0/skills" \
  -H "Content-Type: application/json" \
  -d '{
  "name": "customer-lookup",
  "title": "Customer Lookup",
  "description": "Look up customer profiles by name or ID and search across the customer database",
  "version": "1.0.0",
  "category": "banking"
}' > /dev/null

curl -sf -X POST "${AR_URL}/v0/skills" \
  -H "Content-Type: application/json" \
  -d '{
  "name": "policy-compliance-check",
  "title": "Policy Compliance Check",
  "description": "Check bank policies, lending rules, KYC/AML requirements, and rate schedules",
  "version": "1.0.0",
  "category": "compliance"
}' > /dev/null

curl -sf -X POST "${AR_URL}/v0/skills" \
  -H "Content-Type: application/json" \
  -d '{
  "name": "transaction-analysis",
  "title": "Transaction Analysis",
  "description": "Analyze transaction history, search for suspicious patterns, and review account details",
  "version": "1.0.0",
  "category": "banking"
}' > /dev/null

curl -sf -X POST "${AR_URL}/v0/skills" \
  -H "Content-Type: application/json" \
  -d '{
  "name": "mortgage-rate-quote",
  "title": "Mortgage Rate Quote",
  "description": "Generate personalized mortgage rate quotes based on credit score, salary, and existing accounts",
  "version": "1.0.0",
  "category": "lending"
}' > /dev/null

curl -sf -X POST "${AR_URL}/v0/skills" \
  -H "Content-Type: application/json" \
  -d '{
  "name": "k8s-operations",
  "title": "Kubernetes Operations",
  "description": "Cluster health monitoring, pod troubleshooting, service connectivity diagnostics",
  "version": "1.0.0",
  "category": "infrastructure"
}' > /dev/null

curl -sf -X POST "${AR_URL}/v0/skills" \
  -H "Content-Type: application/json" \
  -d '{
  "name": "helm-deployment",
  "title": "Helm Deployment Management",
  "description": "Helm release management, upgrades, rollbacks, and chart configuration",
  "version": "1.0.0",
  "category": "infrastructure"
}' > /dev/null

curl -sf -X POST "${AR_URL}/v0/skills" \
  -H "Content-Type: application/json" \
  -d '{
  "name": "it-support",
  "title": "IT Support",
  "description": "Internal IT ticket handling, system troubleshooting, and cross-team coordination",
  "version": "1.0.0",
  "category": "operations"
}' > /dev/null

echo "  [OK] Skills registered"

# --- Agents ---

curl -sf -X POST "${AR_URL}/v0/agents" \
  -H "Content-Type: application/json" \
  -d '{
  "name": "bank-triage-agent",
  "title": "Solo Bank Triage Agent",
  "description": "Front-door triage agent that routes customer inquiries to the appropriate specialist",
  "version": "1.0.0",
  "image": "kagent-dev/kagent/app:0.8.0",
  "language": "python",
  "framework": "kagent",
  "modelProvider": "Anthropic",
  "modelName": "claude-sonnet-4-6",
  "mcpServers": [],
  "skills": []
}' > /dev/null

curl -sf -X POST "${AR_URL}/v0/agents" \
  -H "Content-Type: application/json" \
  -d '{
  "name": "bank-customer-service-agent",
  "title": "Solo Bank Customer Service Agent",
  "description": "Handles account inquiries, balance checks, and transaction questions with identity verification",
  "version": "1.0.0",
  "image": "kagent-dev/kagent/app:0.8.0",
  "language": "python",
  "framework": "kagent",
  "modelProvider": "OpenAI",
  "modelName": "gpt-4o-mini",
  "mcpServers": [
    {"type": "streamable-http", "name": "bank-customer-tools", "registryServerName": "solo.io/bank-customer-tools"},
    {"type": "streamable-http", "name": "bank-transaction-tools", "registryServerName": "solo.io/bank-transaction-tools"}
  ],
  "skills": [
    {"name": "customer-lookup", "registrySkillName": "customer-lookup"},
    {"name": "transaction-analysis", "registrySkillName": "transaction-analysis"}
  ]
}' > /dev/null

curl -sf -X POST "${AR_URL}/v0/agents" \
  -H "Content-Type: application/json" \
  -d '{
  "name": "bank-mortgage-advisor-agent",
  "title": "Solo Bank Mortgage Advisor Agent",
  "description": "Provides personalized mortgage rate quotes, refinancing guidance, and lending requirements",
  "version": "1.0.0",
  "image": "kagent-dev/kagent/app:0.8.0",
  "language": "python",
  "framework": "kagent",
  "modelProvider": "Anthropic",
  "modelName": "claude-sonnet-4-6",
  "mcpServers": [
    {"type": "streamable-http", "name": "bank-customer-tools", "registryServerName": "solo.io/bank-customer-tools"},
    {"type": "streamable-http", "name": "bank-policy-tools", "registryServerName": "solo.io/bank-policy-tools"}
  ],
  "skills": [
    {"name": "customer-lookup", "registrySkillName": "customer-lookup"},
    {"name": "mortgage-rate-quote", "registrySkillName": "mortgage-rate-quote"}
  ]
}' > /dev/null

curl -sf -X POST "${AR_URL}/v0/agents" \
  -H "Content-Type: application/json" \
  -d '{
  "name": "bank-compliance-agent",
  "title": "Solo Bank Compliance Agent",
  "description": "Internal compliance officer for policy audits, fraud review, and regulatory checks",
  "version": "1.0.0",
  "image": "kagent-dev/kagent/app:0.8.0",
  "language": "python",
  "framework": "kagent",
  "modelProvider": "Anthropic",
  "modelName": "claude-sonnet-4-6",
  "mcpServers": [
    {"type": "streamable-http", "name": "bank-customer-tools", "registryServerName": "solo.io/bank-customer-tools"},
    {"type": "streamable-http", "name": "bank-policy-tools", "registryServerName": "solo.io/bank-policy-tools"},
    {"type": "streamable-http", "name": "bank-transaction-tools", "registryServerName": "solo.io/bank-transaction-tools"}
  ],
  "skills": [
    {"name": "customer-lookup", "registrySkillName": "customer-lookup"},
    {"name": "policy-compliance-check", "registrySkillName": "policy-compliance-check"},
    {"name": "transaction-analysis", "registrySkillName": "transaction-analysis"}
  ]
}' > /dev/null

curl -sf -X POST "${AR_URL}/v0/agents" \
  -H "Content-Type: application/json" \
  -d '{
  "name": "bank-k8s-agent",
  "title": "Solo Bank Kubernetes Agent",
  "description": "Infrastructure operations specialist for cluster health monitoring and troubleshooting",
  "version": "1.0.0",
  "image": "kagent-dev/kagent/app:0.8.0",
  "language": "python",
  "framework": "kagent",
  "modelProvider": "Anthropic",
  "modelName": "claude-sonnet-4-6",
  "mcpServers": [],
  "skills": [
    {"name": "k8s-operations", "registrySkillName": "k8s-operations"}
  ]
}' > /dev/null

curl -sf -X POST "${AR_URL}/v0/agents" \
  -H "Content-Type: application/json" \
  -d '{
  "name": "bank-helm-agent",
  "title": "Solo Bank Helm Agent",
  "description": "Helm deployment specialist for release management, upgrades, and configuration",
  "version": "1.0.0",
  "image": "kagent-dev/kagent/app:0.8.0",
  "language": "python",
  "framework": "kagent",
  "modelProvider": "Anthropic",
  "modelName": "claude-sonnet-4-6",
  "mcpServers": [],
  "skills": [
    {"name": "helm-deployment", "registrySkillName": "helm-deployment"}
  ]
}' > /dev/null

curl -sf -X POST "${AR_URL}/v0/agents" \
  -H "Content-Type: application/json" \
  -d '{
  "name": "bank-it-agent",
  "title": "Solo Bank IT Support Agent",
  "description": "IT support lead for ticket handling, system troubleshooting, and cross-team coordination",
  "version": "1.0.0",
  "image": "kagent-dev/kagent/app:0.8.0",
  "language": "python",
  "framework": "kagent",
  "modelProvider": "Anthropic",
  "modelName": "claude-sonnet-4-6",
  "mcpServers": [
    {"type": "streamable-http", "name": "bank-customer-tools", "registryServerName": "solo.io/bank-customer-tools"},
    {"type": "streamable-http", "name": "bank-transaction-tools", "registryServerName": "solo.io/bank-transaction-tools"},
    {"type": "streamable-http", "name": "bank-policy-tools", "registryServerName": "solo.io/bank-policy-tools"}
  ],
  "skills": [
    {"name": "customer-lookup", "registrySkillName": "customer-lookup"},
    {"name": "transaction-analysis", "registrySkillName": "transaction-analysis"},
    {"name": "it-support", "registrySkillName": "it-support"}
  ]
}' > /dev/null

echo "  [OK] Agents registered"
echo "AgentRegistry catalog populated."
