# Sample Agent — Account Summary

This is a companion to the "Build Your Own Agent" tutorial.
See the full tutorial at http://localhost:30500/tutorial.html

## Quick Start

1. Build and load the MCP server:
   ```bash
   docker build -t account-summary-tools:latest sample-agent/mcp-server/
   kind load docker-image account-summary-tools:latest --name solo-bank-demo
   ```

2. Deploy the tool server:
   ```bash
   kubectl apply -f sample-agent/manifests/deployment.yaml
   ```

3. Register the RemoteMCPServer and Agent:
   ```bash
   kubectl apply -f sample-agent/manifests/remote-mcp-server.yaml
   kubectl apply -f sample-agent/manifests/agent.yaml
   ```

4. Open the kagent UI and test:
   ```
   kubectl port-forward svc/kagent-ui -n kagent 3000:80
   ```
   Go to http://localhost:3000 and try: "Give me a summary for Maria Garcia"
