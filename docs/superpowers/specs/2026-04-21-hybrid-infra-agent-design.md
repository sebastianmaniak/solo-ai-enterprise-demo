# Hybrid Infrastructure Agent — Design Spec

## Overview

This design adds a new internal-only, read-only hybrid infrastructure scenario to the Solo Bank demo.

The scenario introduces:

- a hidden hybrid environment dataset stored in repo content
- a dedicated read-only MCP server for querying that dataset
- a new visible agent in the kagent UI for infrastructure and security questions
- a fake production-style hybrid environment spanning on-prem, AWS, and Azure
- unified security and topology data covering Palo Alto, Fortinet, routing, NAT, address groups, servers, and segments
- three fully modeled incident scenarios that the agent can reason about

The goal is to make the demo capable of answering realistic infrastructure questions such as:

- which firewall rule or NAT is involved in a flow
- how traffic traverses on-prem, AWS, and Azure
- which address group, segment, or server is part of a given path
- what likely explains an outage in a modeled incident scenario

The agent must stay strictly read-only. It should explain and correlate environment data, not propose or execute changes.

## Current State

The current repo contains:

- a bank wiki with visible categories such as customers, policies, rates, products, and procedures
- MCP servers for customer, policy, transaction, status, and incident data
- visible agents for banking, operations, incident, and infrastructure support use cases
- no dedicated hybrid infrastructure dataset
- no firewall configuration dataset
- no dedicated security or hybrid-infrastructure query MCP server
- no dedicated hybrid infrastructure agent

The current wiki and docs navigation are user-facing and do not contain hidden internal scenario content.

## Decision

Adopt a dedicated hybrid infrastructure design:

1. Create a new visible agent in the kagent UI dedicated to hybrid infrastructure questions.
2. Keep that agent internal-only and read-only.
3. Back it with a dedicated MCP server instead of extending the current bank tools.
4. Store the environment as one unified structured dataset rather than separate vendor-native datasets.
5. Keep the scenario content hidden from normal wiki and docs navigation.
6. Include enough topology and security data to explain end-to-end paths across on-prem, AWS, and Azure.
7. Include three fully modeled incident scenarios in the dataset.

## Constraints

- The agent must be read-only.
- The hybrid scenario content must be hidden from normal wiki and docs navigation.
- The new agent itself must be visible in the kagent UI.
- The dataset should feel production-like, but it is intentionally fake and self-contained.
- The design should preserve the existing demo behavior and avoid unrelated changes to current agents and tools.
- The implementation should reuse current repo patterns for wiki-backed MCP tooling where practical.

## Target Architecture

```
User
  -> kagent UI
  -> Hybrid Infrastructure Agent
  -> hybrid-infra MCP server
  -> unified hybrid environment dataset
     -> topology objects
     -> firewall/security objects
     -> incident scenario records
```

### Visibility Model

- The new hybrid infra agent is visible in the kagent UI.
- The hidden hybrid environment content is not linked from normal wiki navigation.
- The hidden scenario is not exposed in docs-site navigation as a user-facing feature.
- The dataset remains queryable by the dedicated MCP server and therefore usable by the agent.

## Dataset Design

The environment should be modeled as one unified dataset with source or vendor metadata on each object where needed.

### Dataset Principles

- Prefer one common schema over separate Palo Alto, Fortinet, AWS, and Azure schemas.
- Preserve vendor identity through fields such as `platform`, `vendor`, `device`, or `source_system`.
- Make cross-environment reasoning easy for the MCP server and agent.
- Favor structured queryability over long freeform prose.
- Add a short narrative overview only where it helps orient the user.

### Object Groups

The unified dataset should include at least these groups:

- `sites`
  - on-prem datacenters
  - branch or edge locations where useful
  - AWS region presence
  - Azure region presence

- `segments`
  - on-prem DMZ
  - internal application segments
  - data segments
  - management segments
  - cloud application and shared-services subnets

- `servers`
  - public-facing apps
  - internal apps
  - databases
  - management hosts
  - shared infrastructure services such as DNS, AD, logging, or bastions

- `applications`
  - business services mapped to concrete servers, listeners, and environments

- `firewalls`
  - Palo Alto devices
  - Fortinet devices
  - zones, interfaces, HA role, placement, and ownership

- `address_objects`
  - host objects
  - subnet objects
  - service destination objects where useful

- `address_groups`
  - reusable groups referenced by rules and NATs

- `security_rules`
  - ordered allow or deny rules
  - source and destination objects
  - source and destination zones
  - services or applications
  - logging behavior
  - platform and device association

- `nat_rules`
  - DNAT or VIP-style inbound translation
  - SNAT or egress translation
  - translated addresses and services
  - platform and device association

- `routes`
  - key inter-segment and inter-environment routing entries

- `links`
  - on-prem to AWS connectivity
  - on-prem to Azure connectivity
  - cloud-to-cloud connectivity if needed for realism
  - VPN, transit, or ExpressRoute-like constructs

- `incidents`
  - explicit scenario records with symptom, path, affected objects, expected outcome, broken object, and explanation notes

## Scenario Design

The hidden environment should represent a fake hybrid production deployment with:

- on-prem infrastructure
- a DMZ tier
- internal application and data environments
- AWS-hosted workloads
- Azure-hosted workloads
- at least ten example firewall rules, NATs, address groups, servers, and policies spread across the environment

The scenario should feel like a real enterprise environment but remain coherent and easy for the agent to explain.

### Topology Shape

Recommended baseline:

- on-prem primary datacenter
- DMZ for public applications
- internal app tier
- internal data tier
- management segment
- AWS application environment
- Azure management or shared-services environment
- hybrid links from on-prem to AWS and Azure

### Security Shape

Recommended baseline:

- Palo Alto controls at least one major edge or DMZ boundary
- Fortinet controls at least one internal or hybrid boundary
- explicit zones and rule sets on both vendor types
- address objects and groups shared across realistic policy patterns
- inbound and outbound NAT examples
- enough rule order and grouping to support troubleshooting questions

## Incident Scenarios

The implementation should include three fully modeled incident scenarios.

### 1. Internet To DMZ Application Failure

Model a public customer portal or API exposed through a Palo Alto DNAT or VIP-style rule into the on-prem DMZ.

The scenario should allow the agent to answer:

- which public IP maps to which internal server
- which NAT rule is involved
- which firewall zones the flow traverses
- which security rule should allow the traffic
- whether a missing address-group membership or rule mismatch explains the failure

### 2. On-Prem To AWS Application Path Failure

Model an internal on-prem application tier calling a private AWS service over the hybrid connection.

The scenario should allow the agent to answer:

- the end-to-end traffic path
- which links and segments are involved
- whether SNAT occurs
- which firewall policy or route must allow the flow
- whether the likely issue is route, NAT expectation, or security policy

### 3. Azure To On-Prem Management Access Failure

Model administrative access from Azure into an internal on-prem management environment.

The scenario should allow the agent to answer:

- which source subnet or address group should match
- which firewall sees the flow
- which management rule applies
- which server or management zone is targeted
- whether the issue is address-group mismatch, zone mismatch, or overly narrow rule scope

## MCP Server Design

Add a new dedicated MCP server for hybrid infrastructure queries.

### Purpose

- query the hidden unified dataset
- answer inventory and path questions
- support incident-oriented reasoning
- remain strictly read-only

### Tool Surface

The MCP server should expose a small set of read-only tools. Exact names can vary, but the capability set should include:

- find application or server
- summarize site, segment, subnet, or environment
- inspect firewall rules affecting a source and destination pair
- inspect NATs for a host, service, or public IP
- inspect address groups and memberships
- trace expected path across zones, segments, and environments
- summarize a modeled incident scenario

### Non-Goals

- no write or mutation tools
- no configuration generation
- no recommendation engine that produces change commands
- no attempt to emulate real firewall CLIs

The server should answer from the structured dataset directly rather than scraping prose pages and guessing.

## Agent Design

Add a new dedicated agent, for example `bank-hybrid-infra-agent`.

### Visibility

- visible in the kagent UI
- marked internal-only
- intended for infrastructure and security demos

### Prompt Behavior

The agent should:

- stay read-only
- answer from the dedicated MCP server
- reference concrete objects such as rule IDs, NAT IDs, zones, address groups, servers, and segments
- explain path traversal across on-prem, AWS, and Azure
- summarize uncertainty when more than one object could explain a symptom
- avoid pretending to have change authority

### Example Interaction Style

The agent should be able to answer questions like:

- which firewall rule allows internet traffic to the customer portal
- what NAT is used for the public app in the DMZ
- how does traffic get from the on-prem app tier to AWS service X
- what explains the Azure-to-management failure in incident scenario three

## Hidden Content Design

The repo should contain both:

- structured hybrid infrastructure dataset files
- a short hidden narrative scenario overview for human orientation

The hidden content should not be linked from:

- wiki top navigation
- docs-site primary navigation
- user-facing guides aimed at banking workflows

The content can still exist in the wiki content tree or a parallel data path as long as it is intentionally not surfaced through normal navigation.

## File-Level Design

Expected repo areas likely to change:

- `wiki-server/content/` or a parallel hidden scenario data path for hybrid environment content
- a new MCP tool directory under `mcp-tools/`
- `manifests/bank-wiki/` for the new MCP server deployment and service
- `manifests/mcp/remote-mcp-servers.yaml` for the new remote MCP registration
- `manifests/agents/` for the new hybrid infra agent manifest
- docs-site or wiki rendering only as needed to keep the scenario hidden from normal navigation

The exact filenames can follow existing patterns, but the design expects the new feature to be isolated rather than folded into the current customer, policy, transaction, status, or incident tools.

## Error Handling

### Data Problems

- if the structured dataset is incomplete, the MCP server should return an explicit uncertainty message rather than inventing missing paths
- if an object reference is missing, the response should identify the missing dataset reference

### Query Problems

- if a user asks for changes, the agent should refuse and restate that it is read-only
- if a user asks beyond the modeled scenario, the agent should say that the environment is a simulated dataset and answer only from known objects

### Integration Problems

- if the hidden scenario content is not loaded, the MCP server should fail clearly at startup
- if the new MCP server is unavailable, the agent should not silently fall back to unrelated tools

## Verification Gates

The implementation should not be considered complete until it proves:

- the hidden dataset exists and is queryable
- the hidden dataset is not exposed in normal wiki navigation
- the new MCP server starts and exposes the expected read-only tools
- the new `RemoteMCPServer` is accepted
- the new hybrid infra agent appears in the kagent UI
- the agent can answer each of the three modeled incident scenarios with concrete environment objects
- the agent never performs or suggests direct configuration changes as if it had write authority

## Recommended Rollout

Implement in this order:

1. define the unified dataset schema and incident records
2. add the hidden content files
3. build the dedicated MCP server over that dataset
4. deploy and register the MCP server
5. add the new visible internal-only agent
6. add verification for hidden-content behavior, MCP queries, and agent responses

## Out Of Scope

- real firewall API integration
- configuration drift detection against live infrastructure
- change execution or remediation workflows
- exposing the hidden hybrid scenario as a normal customer-facing wiki section
