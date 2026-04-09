module github.com/solo-io/solo-bank-demo/mcp-tools/transaction-tools

go 1.23.0

require (
	github.com/modelcontextprotocol/go-sdk v0.2.0
	github.com/solo-io/solo-bank-demo/mcp-tools/shared v0.0.0
)

require github.com/yosida95/uritemplate/v3 v3.0.2 // indirect

replace github.com/solo-io/solo-bank-demo/mcp-tools/shared => ../shared
