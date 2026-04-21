# AgentGateway Full Routing Upgrade — Design Spec

## Overview

This design upgrades the local Solo Bank demo so the deployment includes:

- Solo Enterprise for `kagent` `0.3.16`
- Solo Enterprise `management` `0.3.16`
- Solo Enterprise for `agentgateway` `v2.3.2`

The design also changes the runtime architecture so both LLM traffic and MCP traffic flow through AgentGateway instead of the current direct in-cluster and direct-to-provider paths.

**Goal:** make AgentGateway a first-class part of the demo by installing it into the cluster, exposing a local proxy endpoint, routing OpenAI requests through it, routing bank MCP server access through it, and keeping the demo deployable with a single `./setup.sh`.

## Current State

The current repo deploys:

- `management` in the `kagent` namespace
- `kagent-enterprise-crds` and `kagent-enterprise` in the `kagent` namespace
- `agentregistry` in the `agentregistry` namespace
- bank wiki services and MCP tool servers in the `bank-wiki` namespace

The current repo does **not** deploy:

- AgentGateway CRDs
- AgentGateway control plane
- an AgentGateway `Gateway`
- any `HTTPRoute`, MCP route, or AgentGateway backend resources

The current demo agents and `RemoteMCPServer` objects point directly at in-cluster service URLs. No `Gateway` or `HTTPRoute` resources are currently present in the cluster.

## Decision

Adopt the full routing path:

1. Install AgentGateway Enterprise into a dedicated `agentgateway-system` namespace.
2. Expose a shared `Gateway` named `agentgateway-proxy`.
3. Route OpenAI-compatible LLM traffic through AgentGateway.
4. Route bank MCP tool access through AgentGateway.
5. Upgrade `management`, `kagent-enterprise-crds`, and `kagent-enterprise` to `0.3.16`.
6. Keep the existing bank wiki and bank tool deployments in place as upstream services behind AgentGateway.

## Constraints

- The demo must remain runnable on a local Kind cluster with a single setup script.
- The demo should preserve the current user-facing services that already work, including the docs site, management UI, bank wiki, and AgentRegistry.
- AgentGateway must be reachable from the host through a stable local port.
- The implementation should minimize unrelated refactoring.
- The versions to use are fixed by user choice even though they are ahead of the currently documented compatibility matrix used in the present Solo Enterprise for kagent docs.

## Target Architecture

```
User / Demo Client
  -> Management UI
  -> kagent controller / agents
  -> AgentGateway proxy
     -> OpenAI backend
     -> bank MCP routes
        -> bank-customer-tools
        -> bank-policy-tools
        -> bank-transaction-tools
        -> bank-status-tools
        -> bank-incident-tools
```

### Namespaces

- `agentgateway-system`: AgentGateway CRDs-owned resources, control plane, proxy, and route objects
- `kagent`: management, kagent controller, model config, agents, and `RemoteMCPServer` objects
- `bank-wiki`: bank wiki server and MCP tool server deployments
- `agentregistry`: AgentRegistry deployment and database

### Local Ports

- `30080`: AgentGateway proxy
- `30083`: kagent controller API
- `30090`: management UI
- `30121`: AgentRegistry
- `30400`: bank wiki
- `30500`: docs site

## Resource Design

### 1. Kind

Update `kind-config.yaml` to map host port `30080` to the control-plane node so the AgentGateway proxy can be reached from the host.

### 2. Setup Script

`setup.sh` will be changed to:

1. install Kubernetes Gateway API CRDs
2. create namespaces
3. install AgentRegistry
4. install AgentGateway CRDs `v2.3.2`
5. install AgentGateway control plane `v2.3.2`
6. install management `0.3.16`
7. install kagent CRDs `0.3.16`
8. install kagent `0.3.16`
9. create required secrets
10. build and deploy bank services
11. apply AgentGateway manifests
12. apply updated model config, MCP, and agent manifests
13. run smoke tests
14. print updated access information

### 3. AgentGateway Base Manifests

Add a new `manifests/agentgateway/` directory. It should contain:

- a `Gateway` manifest for `agentgateway-proxy`
- LLM routing resources for OpenAI-compatible traffic
- MCP routing resources for each bank tool server
- supporting resources such as backend definitions and secret references required by those routes

The `Gateway` should be the minimal activation resource that causes the AgentGateway proxy deployment and service to appear.

### 4. LLM Routing

The repo currently uses direct provider configuration for model execution. This design changes the LLM path so kagent uses AgentGateway as the base URL for OpenAI-compatible traffic.

Design requirements:

- OpenAI credentials remain stored in Kubernetes secrets.
- AgentGateway owns the upstream definition for OpenAI.
- kagent model configuration points at AgentGateway rather than directly at OpenAI.
- The route must preserve compatibility with the existing OpenAI-style client behavior expected by kagent.

This means the effective call path becomes:

`kagent agent -> kagent controller runtime -> AgentGateway -> OpenAI`

### 5. MCP Routing

The current `RemoteMCPServer` objects reference:

- `bank-customer-tools.bank-wiki.svc.cluster.local`
- `bank-policy-tools.bank-wiki.svc.cluster.local`
- `bank-transaction-tools.bank-wiki.svc.cluster.local`
- `bank-status-tools.bank-wiki.svc.cluster.local`
- `bank-incident-tools.bank-wiki.svc.cluster.local`

This design changes those objects so they point at AgentGateway-hosted MCP endpoints instead.

The effective call path becomes:

`kagent agent -> RemoteMCPServer URL -> AgentGateway -> bank MCP tool server`

The bank tool server deployments should keep their current Streamable HTTP behavior and existing `/mcp` backend paths. AgentGateway should adapt to those upstream services instead of forcing changes into the bank tool server implementations.

### 6. Management

Upgrade the `management` release from `0.3.14` to `0.3.16`.

The management chart should continue to provide the kagent UI integration. The implementation should preserve `products.kagent.enabled=true`. It should not add `products.agentgateway.enabled=true` unless verification against the `0.3.16` chart or the reference repo shows that the demo needs that setting for the current UI behavior.

### 7. kagent

Upgrade both:

- `kagent-enterprise-crds` -> `0.3.16`
- `kagent-enterprise` -> `0.3.16`

The implementation must preserve:

- `oidc.skipOBO=true`
- `kmcp.enabled=true`
- `kagent-tools.enabled=true`
- existing local demo assumptions that make the Kind deployment work

The implementation must also adjust model config values so the controller and agents use the AgentGateway endpoint for LLM traffic.

## File-Level Design

Expected repo areas that will change:

- `kind-config.yaml`
- `setup.sh`
- `README.md`
- `manifests/agentgateway/` (new directory)
- `manifests/mcp/remote-mcp-servers.yaml`
- `manifests/agents/model-configs.yaml`

Files in `manifests/agents/` for individual agents should stay unchanged unless a manifest needs to reference a renamed model config or adjusted tool endpoint behavior.

## Data Flow

### LLM

1. User interacts with a demo agent.
2. kagent selects the configured model.
3. The model config sends the request to AgentGateway.
4. AgentGateway forwards the request to OpenAI using the configured backend and credentials.
5. The response returns through AgentGateway back to kagent.

### MCP

1. An agent invokes an MCP tool.
2. kagent resolves the configured `RemoteMCPServer`.
3. The `RemoteMCPServer` URL points to AgentGateway.
4. AgentGateway routes the request to the matching bank tool server.
5. The tool server performs its current wiki lookup or in-memory data operation and returns the result through AgentGateway back to kagent.

## Error Handling

### Install-Time Failure

- If AgentGateway control plane install fails, stop the setup script with a clear error.
- If the `Gateway` does not reconcile into a proxy deployment or service, stop and report that the data plane was not created.
- If the upgraded kagent release fails to come up after the routing changes, stop and surface the failing workload state.

### Runtime Failure

- If the OpenAI route fails, the smoke tests must fail instead of silently falling back to direct provider access.
- If an MCP route fails, the corresponding `RemoteMCPServer` should show a failed state or the smoke tests should expose the broken path.
- The demo must not keep stale direct MCP URLs once the AgentGateway path is enabled.

## Verification Gates

The rollout is only considered successful if all of the following pass:

1. `helm list -A` shows upgraded `management`, `kagent-enterprise-crds`, `kagent-enterprise`, plus installed `enterprise-agentgateway-crds` and `enterprise-agentgateway`.
2. Pods in `agentgateway-system`, `kagent`, `bank-wiki`, and `agentregistry` are healthy.
3. The `agentgateway-proxy` `Gateway`, deployment, and service exist.
4. The AgentGateway proxy is reachable through local port `30080`.
5. A direct OpenAI-compatible request through AgentGateway succeeds.
6. MCP requests through AgentGateway succeed for the bank tool servers.
7. Updated `RemoteMCPServer` objects are accepted and discover tools successfully.
8. At least one demo agent can complete an end-to-end request that requires both LLM and tool access through the new routed architecture.

## Compatibility Position

This design intentionally follows the user-selected versions:

- `management` `0.3.16`
- `kagent-enterprise-crds` `0.3.16`
- `kagent-enterprise` `0.3.16`
- `enterprise-agentgateway-crds` `v2.3.2`
- `enterprise-agentgateway` `v2.3.2`

Implementation should use the `solo-io/agentgateway-enterprise` and `solo-io/kagent-enterprise` repositories as the primary reference points for chart values, examples, and resource semantics when local docs and the live charts diverge.

## Out Of Scope

- redesigning the demo agents themselves
- replacing the bank wiki data model
- introducing additional LLM providers beyond the OpenAI path needed for the current demo
- broader refactors unrelated to AgentGateway integration and version upgrades

## Success Criteria

The design is successful when:

- a fresh `./setup.sh` installs AgentGateway as part of the demo
- AgentGateway is reachable locally
- kagent uses AgentGateway for LLM traffic
- bank MCP traffic is routed through AgentGateway
- the existing bank demo remains usable after the version upgrades
