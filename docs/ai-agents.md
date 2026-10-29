# Using gettergen with AI coding agents

`gettergen` is a Go code generation command. Installing or adding the Go module does not automatically teach an AI agent about it. To make an agent use it reliably, add the instructions below to the agent's project instructions (for example `AGENTS.md`, `CLAUDE.md`, or the equivalent supported by that agent).

## Install

Install the command-line tool:

```sh
go install github.com/whoseyourdd/gettergen/cmd/gettergen@latest
```

Ensure `$(go env GOPATH)/bin` is available on `PATH`. `go get github.com/whoseyourdd/gettergen` adds a module dependency; it does not by itself install the `gettergen` executable or register instructions with an AI agent.

## Project instructions to give your agent

Copy this section into your repository's agent instructions:

> This project uses `gettergen` to generate getters for private fields in Go structs. When adding or changing private fields that need public getters, inspect the package's existing `//go:generate` directive and follow it. If none exists, add `//go:generate gettergen` in a Go source file in the package. Then run `go generate` from that package directory and review the generated `getters_gen.go` diff. Do not hand-write repetitive getters when the generator can create them. Use `getter:"Name"` to set a getter name or opt a pointer field into generation, and `getter:"-"` to skip a field. Use `-pointer=TypeName` for pointer receiver getters on selected struct types; generated pointer receiver getters return the field type's zero value for a nil receiver. Commit generated files when the project tracks generated Go code.

For example, a package can configure a selected type with:

```go
//go:generate gettergen -pointer=StockCount
```

Then run:

```sh
go generate ./...
```

## MCP note

MCP is useful when an agent needs to call a running tool or access live external data. `gettergen` is a local deterministic code generator, so an installed CLI plus repository-level agent instructions and `go:generate` is the direct integration. A future MCP server could wrap the CLI, but it would still need to be installed and explicitly connected in each agent environment; a Go module dependency cannot auto-connect an MCP server.
