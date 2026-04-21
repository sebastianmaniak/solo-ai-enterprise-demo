# AgentGateway Full Routing Upgrade Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Upgrade the demo to `management`/`kagent-enterprise` `0.3.16`, install `enterprise-agentgateway` `v2.3.2`, and route both OpenAI and bank MCP traffic through AgentGateway.

**Architecture:** Keep the existing bank wiki and MCP tool servers in `bank-wiki`, add an `agentgateway-system` namespace with an `agentgateway-proxy` `Gateway`, route OpenAI-compatible traffic to an `AgentgatewayBackend` in AgentGateway, and repoint `RemoteMCPServer` objects to AgentGateway-hosted MCP paths. Preserve the current declarative agent manifests, but update their model configs and kagent controller proxy settings so runtime traffic traverses AgentGateway.

**Tech Stack:** Bash, Helm, Kind, Kubernetes Gateway API, Solo Enterprise for agentgateway `v2.3.2`, Solo Enterprise for kagent `0.3.16`, kagent `ModelConfig` and `RemoteMCPServer` CRDs, Go-based MCP servers.

---

## File Structure

**Create:**
- `manifests/agentgateway/gateway.yaml` — `Gateway` manifest for `agentgateway-proxy`
- `manifests/agentgateway/openai-routing.yaml` — OpenAI `AgentgatewayBackend` and `HTTPRoute`
- `manifests/agentgateway/mcp-routing.yaml` — bank MCP `AgentgatewayBackend` and `HTTPRoute` resources
- `tests/setup-agentgateway-step-test.sh` — setup step guardrail for AgentGateway install and version pins
- `tests/llm-routing-manifests-test.sh` — manifest guardrail for AgentGateway LLM routing and `ModelConfig` base URL
- `tests/mcp-routing-manifests-test.sh` — manifest guardrail for MCP routes and `RemoteMCPServer` URLs
- `tests/setup-smoke-tests-step-test.sh` — setup guardrail for AgentGateway smoke tests and access info

**Modify:**
- `kind-config.yaml` — expose host port `30080`
- `manifests/namespaces.yaml` — add `agentgateway-system`
- `setup.sh` — install AgentGateway, upgrade versions, create secrets, apply manifests, run smoke tests, print access info
- `manifests/agents/model-configs.yaml` — add AgentGateway base URL for OpenAI model configs
- `manifests/mcp/remote-mcp-servers.yaml` — route MCP server URLs through AgentGateway
- `README.md` — document AgentGateway, full-routing architecture, and new versions

**Verify live cluster state with:**
- `helm list -A`
- `kubectl get pods -A`
- `kubectl get gateway,httproute -n agentgateway-system`
- `kubectl get remotemcpservers -n kagent -o yaml`
- `curl -sS http://localhost:30080/v1/chat/completions -H 'content-type: application/json' -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"Reply with exactly routed"}]}'`

### Task 1: Upgrade Scaffolding And Setup Guardrails

**Files:**
- Create: `tests/setup-agentgateway-step-test.sh`
- Modify: `kind-config.yaml`
- Modify: `manifests/namespaces.yaml`
- Modify: `setup.sh`
- Test: `tests/setup-agentgateway-step-test.sh`

- [ ] **Step 1: Write the failing setup guardrail test**

```bash
#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${1:-setup.sh}"

if [[ ! -f "${SCRIPT_PATH}" ]]; then
  echo "setup script not found: ${SCRIPT_PATH}" >&2
  exit 1
fi

crds_block="$(
  awk '
    /# Step 4: Install AgentGateway Enterprise CRDs/ { in_block=1 }
    /# Step 5: Install AgentGateway Enterprise/ { print; exit }
    in_block { print }
  ' "${SCRIPT_PATH}"
)"

control_plane_block="$(
  awk '
    /# Step 5: Install AgentGateway Enterprise/ { in_block=1 }
    /# Step 6: Install Management UI/ { exit }
    in_block { print }
  ' "${SCRIPT_PATH}"
)"

if [[ -z "${crds_block}" ]]; then
  echo "could not locate AgentGateway CRD install block in ${SCRIPT_PATH}" >&2
  exit 1
fi

if [[ -z "${control_plane_block}" ]]; then
  echo "could not locate AgentGateway control plane block in ${SCRIPT_PATH}" >&2
  exit 1
fi

echo "${crds_block}" | rg -q 'enterprise-agentgateway-crds' || {
  echo "missing enterprise-agentgateway-crds helm install" >&2
  exit 1
}

echo "${crds_block}" | rg -q 'v2.3.2' || {
  echo "AgentGateway CRDs must use v2.3.2" >&2
  exit 1
}

echo "${control_plane_block}" | rg -q 'enterprise-agentgateway ' || {
  echo "missing enterprise-agentgateway helm install" >&2
  exit 1
}

echo "${control_plane_block}" | rg -q 'kubectl rollout status deployment/enterprise-agentgateway' || {
  echo "missing explicit rollout status for enterprise-agentgateway deployment" >&2
  exit 1
}

echo "${control_plane_block}" | rg -q '0.3.16' && {
  echo "AgentGateway block should not contain kagent version strings" >&2
  exit 1
}

echo "${control_plane_block}" | rg -q 'licensing\.licenseKey="\$\{AGENTGATEWAY_LICENSE_KEY\}"|licensing\.licenseKey=\$\{AGENTGATEWAY_LICENSE_KEY\}' || {
  echo "AgentGateway install must pass AGENTGATEWAY_LICENSE_KEY" >&2
  exit 1
}

echo "${control_plane_block}" | rg -q 'Step 6: Install Management UI' && {
  echo "management step marker leaked into AgentGateway block" >&2
  exit 1
}

echo "agentgateway setup steps are present and version-pinned"
```

- [ ] **Step 2: Run the test to verify it fails**

Run:
```bash
bash tests/setup-agentgateway-step-test.sh setup.sh
```

Expected: FAIL with `could not locate AgentGateway CRD install block` because the current `setup.sh` does not install AgentGateway.

- [ ] **Step 3: Update cluster/network scaffolding and setup version pins**

Update `kind-config.yaml` to expose the AgentGateway NodePort:

```yaml
# AgentGateway proxy
- containerPort: 30080
  hostPort: 30080
  protocol: TCP
```

Update `manifests/namespaces.yaml` to include the AgentGateway namespace:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: agentgateway-system
---
apiVersion: v1
kind: Namespace
metadata:
  name: bank-wiki
---
apiVersion: v1
kind: Namespace
metadata:
  name: kagent
```

Update the install section of `setup.sh` so the top-level version variables and early install flow look like this:

```bash
KAGENT_VERSION="0.3.16"
AGENTGATEWAY_VERSION="v2.3.2"
AGENTGATEWAY_NAMESPACE="agentgateway-system"

# ---------------------------------------------------------------------------
# Step 4: Install AgentGateway Enterprise CRDs
# ---------------------------------------------------------------------------
banner "Step 4: Install AgentGateway Enterprise CRDs"

helm upgrade --install enterprise-agentgateway-crds \
  oci://us-docker.pkg.dev/solo-public/enterprise-agentgateway/charts/enterprise-agentgateway-crds \
  --namespace "${AGENTGATEWAY_NAMESPACE}" \
  --create-namespace \
  --version "${AGENTGATEWAY_VERSION}" \
  --wait --timeout 120s

echo -e "${GREEN}AgentGateway Enterprise CRDs installed.${NC}"

# ---------------------------------------------------------------------------
# Step 5: Install AgentGateway Enterprise
# ---------------------------------------------------------------------------
banner "Step 5: Install AgentGateway Enterprise"

helm upgrade --install enterprise-agentgateway \
  oci://us-docker.pkg.dev/solo-public/enterprise-agentgateway/charts/enterprise-agentgateway \
  --namespace "${AGENTGATEWAY_NAMESPACE}" \
  --version "${AGENTGATEWAY_VERSION}" \
  --set-string licensing.licenseKey="${AGENTGATEWAY_LICENSE_KEY}" \
  --wait --timeout 300s

kubectl rollout status deployment/enterprise-agentgateway \
  -n "${AGENTGATEWAY_NAMESPACE}" --timeout=300s

echo -e "${GREEN}AgentGateway Enterprise installed.${NC}"

# ---------------------------------------------------------------------------
# Step 6: Install Management UI
# ---------------------------------------------------------------------------
banner "Step 6: Install Management UI"

helm upgrade --install management \
  oci://us-docker.pkg.dev/solo-public/solo-enterprise-helm/charts/management \
  --namespace kagent \
  --version "${KAGENT_VERSION}" \
  --set cluster="solo-bank-demo" \
  --set service.type=NodePort \
  --set products.kagent.enabled=true \
  --set licensing.licenseKey="${AGENTGATEWAY_LICENSE_KEY}" \
  --no-hooks

# ---------------------------------------------------------------------------
# Step 7: Install kagent Enterprise
# ---------------------------------------------------------------------------
banner "Step 7: Install kagent Enterprise"

helm upgrade --install kagent-enterprise-crds \
  oci://us-docker.pkg.dev/solo-public/kagent-enterprise-helm/charts/kagent-enterprise-crds \
  --namespace kagent --create-namespace \
  --version "${KAGENT_VERSION}" \
  --wait --timeout 120s

helm upgrade --install kagent-enterprise \
  oci://us-docker.pkg.dev/solo-public/kagent-enterprise-helm/charts/kagent-enterprise \
  --namespace kagent \
  --version "${KAGENT_VERSION}" \
  --set controller.enabled=true \
  --set kmcp.enabled=true \
  --set kagent-tools.enabled=true \
  --set oidc.skipOBO=true \
  --set otel.tracing.enabled=true \
  --set licensing.licenseKey="${AGENTGATEWAY_LICENSE_KEY}" \
  --wait --timeout 300s
```

- [ ] **Step 4: Run syntax and guardrail verification**

Run:
```bash
bash -n setup.sh
bash tests/setup-agentgateway-step-test.sh setup.sh
```

Expected:
```text
agentgateway setup steps are present and version-pinned
```

- [ ] **Step 5: Commit the scaffolding**

```bash
git add kind-config.yaml manifests/namespaces.yaml setup.sh tests/setup-agentgateway-step-test.sh
git commit -m "feat: install agentgateway enterprise control plane"
```

### Task 2: Add LLM Routing Through AgentGateway

**Files:**
- Create: `manifests/agentgateway/gateway.yaml`
- Create: `manifests/agentgateway/openai-routing.yaml`
- Create: `tests/llm-routing-manifests-test.sh`
- Modify: `manifests/agents/model-configs.yaml`
- Modify: `setup.sh`
- Test: `tests/llm-routing-manifests-test.sh`

- [ ] **Step 1: Write the failing LLM routing guardrail test**

```bash
#!/usr/bin/env bash
set -euo pipefail

GATEWAY_FILE="manifests/agentgateway/gateway.yaml"
OPENAI_FILE="manifests/agentgateway/openai-routing.yaml"
MODEL_FILE="manifests/agents/model-configs.yaml"

[[ -f "${GATEWAY_FILE}" ]] || { echo "missing ${GATEWAY_FILE}" >&2; exit 1; }
[[ -f "${OPENAI_FILE}" ]] || { echo "missing ${OPENAI_FILE}" >&2; exit 1; }

rg -q 'kind: Gateway' "${GATEWAY_FILE}" || {
  echo "gateway manifest must contain a Gateway" >&2
  exit 1
}

rg -q 'name: agentgateway-proxy' "${GATEWAY_FILE}" || {
  echo "gateway manifest must define agentgateway-proxy" >&2
  exit 1
}

rg -q 'gatewayClassName: enterprise-agentgateway' "${GATEWAY_FILE}" || {
  echo "gateway manifest must use enterprise-agentgateway" >&2
  exit 1
}

rg -q 'kind: AgentgatewayBackend' "${OPENAI_FILE}" || {
  echo "openai routing manifest must contain an AgentgatewayBackend" >&2
  exit 1
}

rg -q 'kind: HTTPRoute' "${OPENAI_FILE}" || {
  echo "openai routing manifest must contain an HTTPRoute" >&2
  exit 1
}

rg -q 'value: /v1/chat/completions' "${OPENAI_FILE}" || {
  echo "openai route must match /v1/chat/completions" >&2
  exit 1
}

rg -q 'baseUrl: http://agentgateway-proxy.agentgateway-system.svc.cluster.local/v1' "${MODEL_FILE}" || {
  echo "model configs must point OpenAI baseUrl at AgentGateway" >&2
  exit 1
}

echo "llm routing manifests and model configs point at agentgateway"
```

- [ ] **Step 2: Run the test to verify it fails**

Run:
```bash
bash tests/llm-routing-manifests-test.sh
```

Expected: FAIL with `missing manifests/agentgateway/gateway.yaml`.

- [ ] **Step 3: Add the AgentGateway `Gateway` and OpenAI route manifests**

Create `manifests/agentgateway/gateway.yaml`:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: agentgateway-proxy
  namespace: agentgateway-system
spec:
  gatewayClassName: enterprise-agentgateway
  listeners:
  - name: http
    protocol: HTTP
    port: 80
    allowedRoutes:
      namespaces:
        from: All
```

Create `manifests/agentgateway/openai-routing.yaml`:

```yaml
apiVersion: agentgateway.dev/v1alpha1
kind: AgentgatewayBackend
metadata:
  name: openai
  namespace: agentgateway-system
spec:
  ai:
    provider:
      openai:
        model: gpt-4o-mini
  policies:
    auth:
      secretRef:
        name: openai-secret
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: openai
  namespace: agentgateway-system
spec:
  parentRefs:
  - name: agentgateway-proxy
    namespace: agentgateway-system
  rules:
  - matches:
    - path:
        type: PathPrefix
        value: /v1/chat/completions
    backendRefs:
    - name: openai
      namespace: agentgateway-system
      group: agentgateway.dev
      kind: AgentgatewayBackend
```

Update `manifests/agents/model-configs.yaml` so both model configs include the AgentGateway base URL:

```yaml
apiVersion: kagent.dev/v1alpha2
kind: ModelConfig
metadata:
  name: openai-gpt4o-mini
  namespace: kagent
spec:
  model: gpt-4o-mini
  provider: OpenAI
  apiKeySecret: openai-secret
  apiKeySecretKey: Authorization
  openAI:
    baseUrl: http://agentgateway-proxy.agentgateway-system.svc.cluster.local/v1
---
apiVersion: kagent.dev/v1alpha2
kind: ModelConfig
metadata:
  name: openai-gpt4o
  namespace: kagent
spec:
  model: gpt-4o
  provider: OpenAI
  apiKeySecret: openai-secret
  apiKeySecretKey: Authorization
  openAI:
    baseUrl: http://agentgateway-proxy.agentgateway-system.svc.cluster.local/v1
```

Update `setup.sh` so the secret creation and AgentGateway apply flow look like this:

```bash
# ---------------------------------------------------------------------------
# Step 8: Create LLM API key secrets
# ---------------------------------------------------------------------------
banner "Step 8: Create LLM API key secrets"

kubectl create secret generic openai-secret \
  --namespace kagent \
  --from-literal=Authorization="${OPENAI_API_KEY}" \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl create secret generic openai-secret \
  --namespace "${AGENTGATEWAY_NAMESPACE}" \
  --from-literal=Authorization="Bearer ${OPENAI_API_KEY}" \
  --dry-run=client -o yaml | kubectl apply -f -

echo -e "${GREEN}LLM API key secrets created.${NC}"
```

Also add the kagent chart proxy flag:

```bash
  --set proxy.url="http://agentgateway-proxy.${AGENTGATEWAY_NAMESPACE}.svc.cluster.local" \
```

And apply/wait for AgentGateway manifests after bank services are available:

```bash
banner "Step 11: Apply AgentGateway routing resources"

kubectl apply -f "${SCRIPT_DIR}/manifests/agentgateway/"

kubectl rollout status deployment/agentgateway-proxy \
  -n "${AGENTGATEWAY_NAMESPACE}" --timeout=300s

kubectl patch svc agentgateway-proxy -n "${AGENTGATEWAY_NAMESPACE}" --type='json' -p='[
  {"op":"replace","path":"/spec/type","value":"NodePort"},
  {"op":"add","path":"/spec/ports/0/nodePort","value":30080}
]' 2>/dev/null || warn "Could not patch AgentGateway proxy NodePort (may already be set)."
```

- [ ] **Step 4: Run the manifest test**

Run:
```bash
bash tests/llm-routing-manifests-test.sh
```

Expected:
```text
llm routing manifests and model configs point at agentgateway
```

- [ ] **Step 5: Commit the LLM routing changes**

```bash
git add manifests/agentgateway/gateway.yaml manifests/agentgateway/openai-routing.yaml manifests/agents/model-configs.yaml setup.sh tests/llm-routing-manifests-test.sh
git commit -m "feat: route openai traffic through agentgateway"
```

### Task 3: Route MCP Servers Through AgentGateway

**Files:**
- Create: `manifests/agentgateway/mcp-routing.yaml`
- Create: `tests/mcp-routing-manifests-test.sh`
- Modify: `manifests/mcp/remote-mcp-servers.yaml`
- Test: `tests/mcp-routing-manifests-test.sh`

- [ ] **Step 1: Write the failing MCP routing guardrail test**

```bash
#!/usr/bin/env bash
set -euo pipefail

ROUTING_FILE="manifests/agentgateway/mcp-routing.yaml"
REMOTE_FILE="manifests/mcp/remote-mcp-servers.yaml"

[[ -f "${ROUTING_FILE}" ]] || { echo "missing ${ROUTING_FILE}" >&2; exit 1; }

for path in \
  '/mcp/customer' \
  '/mcp/policy' \
  '/mcp/transaction' \
  '/mcp/status' \
  '/mcp/incident'
do
  rg -q "value: ${path}" "${ROUTING_FILE}" || {
    echo "missing MCP route path ${path}" >&2
    exit 1
  }
done

for target in \
  'host: bank-customer-tools.bank-wiki.svc.cluster.local' \
  'host: bank-policy-tools.bank-wiki.svc.cluster.local' \
  'host: bank-transaction-tools.bank-wiki.svc.cluster.local' \
  'host: bank-status-tools.bank-wiki.svc.cluster.local' \
  'host: bank-incident-tools.bank-wiki.svc.cluster.local'
do
  rg -q "${target}" "${ROUTING_FILE}" || {
    echo "missing backend target ${target}" >&2
    exit 1
  }
done

rg -q 'protocol: StreamableHTTP' "${ROUTING_FILE}" || {
  echo "MCP backends must use StreamableHTTP" >&2
  exit 1
}

for url in \
  'http://agentgateway-proxy.agentgateway-system.svc.cluster.local/mcp/customer' \
  'http://agentgateway-proxy.agentgateway-system.svc.cluster.local/mcp/policy' \
  'http://agentgateway-proxy.agentgateway-system.svc.cluster.local/mcp/transaction' \
  'http://agentgateway-proxy.agentgateway-system.svc.cluster.local/mcp/status' \
  'http://agentgateway-proxy.agentgateway-system.svc.cluster.local/mcp/incident'
do
  rg -q "${url}" "${REMOTE_FILE}" || {
    echo "missing routed RemoteMCPServer URL ${url}" >&2
    exit 1
  }
done

echo "mcp routing manifests and remote server urls point at agentgateway"
```

- [ ] **Step 2: Run the test to verify it fails**

Run:
```bash
bash tests/mcp-routing-manifests-test.sh
```

Expected: FAIL with `missing manifests/agentgateway/mcp-routing.yaml`.

- [ ] **Step 3: Create MCP backends/routes and repoint `RemoteMCPServer` URLs**

Create `manifests/agentgateway/mcp-routing.yaml`:

```yaml
apiVersion: agentgateway.dev/v1alpha1
kind: AgentgatewayBackend
metadata:
  name: bank-customer-tools
  namespace: agentgateway-system
spec:
  mcp:
    targets:
    - name: bank-customer-tools
      static:
        host: bank-customer-tools.bank-wiki.svc.cluster.local
        port: 8081
        path: /mcp
        protocol: StreamableHTTP
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: bank-customer-tools
  namespace: agentgateway-system
spec:
  parentRefs:
  - name: agentgateway-proxy
    namespace: agentgateway-system
  rules:
  - matches:
    - path:
        type: PathPrefix
        value: /mcp/customer
    backendRefs:
    - name: bank-customer-tools
      namespace: agentgateway-system
      group: agentgateway.dev
      kind: AgentgatewayBackend
---
apiVersion: agentgateway.dev/v1alpha1
kind: AgentgatewayBackend
metadata:
  name: bank-policy-tools
  namespace: agentgateway-system
spec:
  mcp:
    targets:
    - name: bank-policy-tools
      static:
        host: bank-policy-tools.bank-wiki.svc.cluster.local
        port: 8082
        path: /mcp
        protocol: StreamableHTTP
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: bank-policy-tools
  namespace: agentgateway-system
spec:
  parentRefs:
  - name: agentgateway-proxy
    namespace: agentgateway-system
  rules:
  - matches:
    - path:
        type: PathPrefix
        value: /mcp/policy
    backendRefs:
    - name: bank-policy-tools
      namespace: agentgateway-system
      group: agentgateway.dev
      kind: AgentgatewayBackend
---
apiVersion: agentgateway.dev/v1alpha1
kind: AgentgatewayBackend
metadata:
  name: bank-transaction-tools
  namespace: agentgateway-system
spec:
  mcp:
    targets:
    - name: bank-transaction-tools
      static:
        host: bank-transaction-tools.bank-wiki.svc.cluster.local
        port: 8083
        path: /mcp
        protocol: StreamableHTTP
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: bank-transaction-tools
  namespace: agentgateway-system
spec:
  parentRefs:
  - name: agentgateway-proxy
    namespace: agentgateway-system
  rules:
  - matches:
    - path:
        type: PathPrefix
        value: /mcp/transaction
    backendRefs:
    - name: bank-transaction-tools
      namespace: agentgateway-system
      group: agentgateway.dev
      kind: AgentgatewayBackend
---
apiVersion: agentgateway.dev/v1alpha1
kind: AgentgatewayBackend
metadata:
  name: bank-status-tools
  namespace: agentgateway-system
spec:
  mcp:
    targets:
    - name: bank-status-tools
      static:
        host: bank-status-tools.bank-wiki.svc.cluster.local
        port: 8085
        path: /mcp
        protocol: StreamableHTTP
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: bank-status-tools
  namespace: agentgateway-system
spec:
  parentRefs:
  - name: agentgateway-proxy
    namespace: agentgateway-system
  rules:
  - matches:
    - path:
        type: PathPrefix
        value: /mcp/status
    backendRefs:
    - name: bank-status-tools
      namespace: agentgateway-system
      group: agentgateway.dev
      kind: AgentgatewayBackend
---
apiVersion: agentgateway.dev/v1alpha1
kind: AgentgatewayBackend
metadata:
  name: bank-incident-tools
  namespace: agentgateway-system
spec:
  mcp:
    targets:
    - name: bank-incident-tools
      static:
        host: bank-incident-tools.bank-wiki.svc.cluster.local
        port: 8086
        path: /mcp
        protocol: StreamableHTTP
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: bank-incident-tools
  namespace: agentgateway-system
spec:
  parentRefs:
  - name: agentgateway-proxy
    namespace: agentgateway-system
  rules:
  - matches:
    - path:
        type: PathPrefix
        value: /mcp/incident
    backendRefs:
    - name: bank-incident-tools
      namespace: agentgateway-system
      group: agentgateway.dev
      kind: AgentgatewayBackend
```

Update `manifests/mcp/remote-mcp-servers.yaml`:

```yaml
apiVersion: kagent.dev/v1alpha2
kind: RemoteMCPServer
metadata:
  name: bank-customer-tools
  namespace: kagent
spec:
  url: http://agentgateway-proxy.agentgateway-system.svc.cluster.local/mcp/customer
  protocol: STREAMABLE_HTTP
  description: "Solo Bank customer data tools — lookup, search, account balances"
---
apiVersion: kagent.dev/v1alpha2
kind: RemoteMCPServer
metadata:
  name: bank-policy-tools
  namespace: kagent
spec:
  url: http://agentgateway-proxy.agentgateway-system.svc.cluster.local/mcp/policy
  protocol: STREAMABLE_HTTP
  description: "Solo Bank policy and rates tools — lending policies, rate tables, credit tier lookups"
---
apiVersion: kagent.dev/v1alpha2
kind: RemoteMCPServer
metadata:
  name: bank-transaction-tools
  namespace: kagent
spec:
  url: http://agentgateway-proxy.agentgateway-system.svc.cluster.local/mcp/transaction
  protocol: STREAMABLE_HTTP
  description: "Solo Bank transaction and account tools — transaction history, account details, search"
---
apiVersion: kagent.dev/v1alpha2
kind: RemoteMCPServer
metadata:
  name: bank-status-tools
  namespace: kagent
spec:
  url: http://agentgateway-proxy.agentgateway-system.svc.cluster.local/mcp/status
  protocol: STREAMABLE_HTTP
  description: "Solo Bank application status and datacenter monitoring tools"
---
apiVersion: kagent.dev/v1alpha2
kind: RemoteMCPServer
metadata:
  name: bank-incident-tools
  namespace: kagent
spec:
  url: http://agentgateway-proxy.agentgateway-system.svc.cluster.local/mcp/incident
  protocol: STREAMABLE_HTTP
  description: "Solo Bank incident management and IT ticketing tools"
```

- [ ] **Step 4: Run the MCP manifest test**

Run:
```bash
bash tests/mcp-routing-manifests-test.sh
```

Expected:
```text
mcp routing manifests and remote server urls point at agentgateway
```

- [ ] **Step 5: Commit the MCP routing changes**

```bash
git add manifests/agentgateway/mcp-routing.yaml manifests/mcp/remote-mcp-servers.yaml tests/mcp-routing-manifests-test.sh
git commit -m "feat: route mcp servers through agentgateway"
```

### Task 4: Add Smoke Tests, Access Info, And README Updates

**Files:**
- Create: `tests/setup-smoke-tests-step-test.sh`
- Modify: `setup.sh`
- Modify: `README.md`
- Test: `tests/setup-smoke-tests-step-test.sh`

- [ ] **Step 1: Write the failing smoke-test guardrail**

```bash
#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="${1:-setup.sh}"

if [[ ! -f "${SCRIPT_PATH}" ]]; then
  echo "setup script not found: ${SCRIPT_PATH}" >&2
  exit 1
fi

smoke_block="$(
  awk '
    /# Step 13: Smoke tests/ { in_block=1 }
    /# Step 14: Access Information/ { exit }
    in_block { print }
  ' "${SCRIPT_PATH}"
)"

access_block="$(
  awk '
    /# Step 14: Access Information/ { in_block=1 }
    in_block { print }
  ' "${SCRIPT_PATH}"
)"

[[ -n "${smoke_block}" ]] || { echo "missing smoke test block" >&2; exit 1; }
[[ -n "${access_block}" ]] || { echo "missing access info block" >&2; exit 1; }

for pattern in \
  'kubectl get gateway agentgateway-proxy -n agentgateway-system' \
  'curl -sS http://localhost:30080/v1/chat/completions' \
  'kubectl get remotemcpservers -n kagent' \
  'bank-customer-tools'
do
  echo "${smoke_block}" | rg -q "${pattern}" || {
    echo "missing smoke test pattern: ${pattern}" >&2
    exit 1
  }
done

echo "${access_block}" | rg -q 'AgentGateway Proxy: *http://localhost:30080' || {
  echo "missing AgentGateway access URL" >&2
  exit 1
}

echo "setup smoke tests and access info cover agentgateway"
```

- [ ] **Step 2: Run the test to verify it fails**

Run:
```bash
bash tests/setup-smoke-tests-step-test.sh setup.sh
```

Expected: FAIL because the current smoke-test and access-info blocks do not mention AgentGateway.

- [ ] **Step 3: Extend setup smoke tests and final access output**

Update the smoke-test section in `setup.sh` to include:

```bash
echo "Checking AgentGateway gateway resources:"
kubectl get gateway agentgateway-proxy -n "${AGENTGATEWAY_NAMESPACE}"
kubectl get httproute -n "${AGENTGATEWAY_NAMESPACE}"

echo "Checking AgentGateway LLM route:"
LLM_RESPONSE="$(curl -sS http://localhost:30080/v1/chat/completions \
  -H 'content-type: application/json' \
  -d '{
    "model":"gpt-4o-mini",
    "messages":[{"role":"user","content":"Reply with the single word routed."}]
  }' || true)"

echo "${LLM_RESPONSE}" | jq -e '.choices[0].message.content' >/dev/null || {
  fail "AgentGateway OpenAI route smoke test failed"
}

echo "Checking RemoteMCPServer acceptance:"
for name in \
  bank-customer-tools \
  bank-policy-tools \
  bank-transaction-tools \
  bank-status-tools \
  bank-incident-tools
do
  kubectl get remotemcpserver "${name}" -n kagent \
    -o jsonpath='{.status.conditions[?(@.type=="Accepted")].status}' | rg -q '^True$' || {
      fail "RemoteMCPServer ${name} is not accepted through AgentGateway"
    }
done
```

Update the access-info section in `setup.sh`:

```bash
echo "  AgentGateway Proxy: http://localhost:30080"
echo "  Management UI:      http://localhost:30090"
echo "  Bank Wiki Server:   http://localhost:30400"
echo "  AgentRegistry UI:   http://localhost:30121"
echo "  kagent API:         http://localhost:30083"
```

Update the top sections of `README.md` so the service table, platform stack, architecture, namespaces, and project structure reflect full routing. Use this content:

```markdown
| **AgentGateway Proxy** | http://localhost:30080 | OpenAI and MCP ingress point |
| **Docs Site** | http://localhost:30500 | Documentation and usage guide |
| **Management UI** | http://localhost:30090 | Chat with agents |
| **AgentRegistry** | http://localhost:30121 | Skill catalog REST API |
| **Bank Wiki** | http://localhost:30400 | Customer and policy data |
| **kagent API** | http://localhost:30083 | Agent runtime API |
```

```markdown
| Component | Version |
|-----------|---------|
| AgentGateway Enterprise | v2.3.2 |
| kagent Enterprise | v0.3.16 |
| Management UI | v0.3.16 |
| AgentRegistry OSS | v0.3.3 |
| Kind | v0.20+ |
```

```text
User -> Management UI -> kagent Controller -> Agent
                                           -> AgentGateway Proxy
                                              -> OpenAI
                                              -> Remote MCP Routes
                                                 -> Bank MCP Tool Servers
                                                    -> Simulated Data
```

- [ ] **Step 4: Run all repo-local tests**

Run:
```bash
bash -n setup.sh
bash tests/setup-management-step-test.sh setup.sh
bash tests/setup-bank-wiki-step-test.sh setup.sh
bash tests/setup-agentgateway-step-test.sh setup.sh
bash tests/llm-routing-manifests-test.sh
bash tests/mcp-routing-manifests-test.sh
bash tests/setup-smoke-tests-step-test.sh setup.sh
```

Expected:
```text
management step waits on concrete workloads instead of helm release wait
bank wiki step waits on concrete deployments instead of racing pod creation
agentgateway setup steps are present and version-pinned
llm routing manifests and model configs point at agentgateway
mcp routing manifests and remote server urls point at agentgateway
setup smoke tests and access info cover agentgateway
```

- [ ] **Step 5: Commit the docs and smoke-test changes**

```bash
git add README.md setup.sh tests/setup-smoke-tests-step-test.sh
git commit -m "docs: describe full agentgateway routing"
```

### Task 5: Reconcile The Live Cluster And Verify End-To-End Routing

**Files:**
- Test: live cluster state only

- [ ] **Step 1: Re-run the installer against the current Kind cluster**

Run:
```bash
./setup.sh
```

Expected: the script completes successfully and prints the AgentGateway proxy URL at `http://localhost:30080`.

- [ ] **Step 2: Verify Helm releases and workloads**

Run:
```bash
helm list -A
kubectl get pods -A
kubectl get gateway,httproute -n agentgateway-system
kubectl get svc -n agentgateway-system agentgateway-proxy
```

Expected:
```text
enterprise-agentgateway-crds   agentgateway-system   deployed
enterprise-agentgateway        agentgateway-system   deployed
management                     kagent               deployed
kagent-enterprise-crds         kagent               deployed
kagent-enterprise              kagent               deployed
```

And:
```text
Gateway/agentgateway-proxy exists
Service/agentgateway-proxy is NodePort and exposes 30080
```

- [ ] **Step 3: Verify the OpenAI route through AgentGateway**

Run:
```bash
curl -sS http://localhost:30080/v1/chat/completions \
  -H 'content-type: application/json' \
  -d '{
    "model":"gpt-4o-mini",
    "messages":[{"role":"user","content":"Reply with exactly routed"}]
  }' | jq -er '.choices[0].message.content | ascii_downcase | gsub("[^a-z]"; "")'
```

Expected:
```text
routed
```

- [ ] **Step 4: Verify MCP discovery through AgentGateway**

Run:
```bash
kubectl get remotemcpservers -n kagent -o custom-columns=NAME:.metadata.name,ACCEPTED:.status.conditions[0].status
kubectl get remotemcpserver bank-customer-tools -n kagent -o jsonpath='{.status.discoveredTools[*].name}'; echo
kubectl get remotemcpserver bank-policy-tools -n kagent -o jsonpath='{.status.discoveredTools[*].name}'; echo
kubectl get remotemcpserver bank-transaction-tools -n kagent -o jsonpath='{.status.discoveredTools[*].name}'; echo
kubectl get remotemcpserver bank-status-tools -n kagent -o jsonpath='{.status.discoveredTools[*].name}'; echo
kubectl get remotemcpserver bank-incident-tools -n kagent -o jsonpath='{.status.discoveredTools[*].name}'; echo
```

Expected:
```text
bank-customer-tools     True
bank-policy-tools       True
bank-transaction-tools  True
bank-status-tools       True
bank-incident-tools     True
```

And each `jsonpath` command prints at least one tool name.

- [ ] **Step 5: Commit the final verified implementation**

```bash
git add kind-config.yaml manifests/namespaces.yaml setup.sh README.md manifests/agentgateway manifests/agents/model-configs.yaml manifests/mcp/remote-mcp-servers.yaml tests/setup-agentgateway-step-test.sh tests/llm-routing-manifests-test.sh tests/mcp-routing-manifests-test.sh tests/setup-smoke-tests-step-test.sh
git commit -m "feat: route kagent llm and mcp traffic through agentgateway"
```
