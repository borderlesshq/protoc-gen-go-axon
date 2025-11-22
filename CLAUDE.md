# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

`protoc-gen-go-axon` is a Protocol Buffers compiler plugin that generates gRPC-style RPC code over NATS for Go. It creates idiomatic Go code with full support for unary, server streaming, client streaming, and bidirectional streaming RPCs using NATS as the transport layer.

## Core Commands

### Building the Plugin
```bash
# Build and install the plugin to GOPATH/bin
go build -o $(go env GOPATH)/bin/protoc-gen-go-axon .

# Or use the Makefile target
make install-proto-plugins
```

### Generating Code from Protos

```bash
# Generate all protos (cleans old generated files first)
make protos

# Generate for a specific entity
make proto ENTITY=accounts
make proto-accounts  # Shortcut

# Clean generated files only
make clean-proto-generated

# Clean everything (generated files + plugins)
make clean-proto

# List available proto entities
make list-protos
```

### Development Workflow
```bash
# Format code
make fmt

# Check formatting
make fmt-check

# Run vet
make vet
```

## Architecture

### Code Generation Flow

1. **Entry Point** (`main.go`):
   - Uses `protogen` to parse `.proto` files
   - Extracts services and methods
   - Passes data to template engine
   - Formats and outputs generated code to `*_axon.pb.go` files

2. **Template System** (`src/` directory):
   - `src.go`: Main template orchestrator with helper functions
   - `interfaces.go`: Server/client interface templates and method signatures
   - `server_interceptors.go`: Interceptor infrastructure (similar to gRPC middleware)
   - `playground.go`: Web UI generator for testing services
   - `playground_html.go`: HTML/JS for the interactive playground

3. **Generated Code Structure**:
   Each service generates:
   - Server interface (methods to implement)
   - Client interface (methods to call)
   - Streaming interfaces (for each streaming method)
   - Registration functions
   - NATS message handlers
   - Tracing/telemetry hooks
   - Playground support functions

### NATS Transport Patterns

- **Unary RPC**: Uses `nc.RequestWithContext()` for request/reply
- **Server Streaming**: Uses inbox pattern with unique reply subjects; EOF signaled via `Stream-EOF` header
- **Client Streaming**: Aggregates messages by `Stream-ID`, sends final response when client closes
- **Bidirectional**: Separate `.in` and `.out` channels with unique stream IDs

### Message Framing via NATS Headers

- `Stream-ID`: Unique identifier for stream instances
- `Stream-EOF`: Signals end of stream (value: "true")
- `X-Error`: Error message if operation failed (errors NOT in payload)
- `Seq-Num`: Sequence number for message ordering
- Trace context: Injected/extracted for OpenTelemetry support

### Key Design Decisions

1. **Queue Subscriptions**: All handlers use `QueueSubscribe` with service+method as queue group for load balancing
2. **Error Propagation**: Errors sent via headers, NOT embedded in protobuf payload
3. **Context Support**: Full `context.Context` integration for timeouts and cancellation
4. **Clean EOF**: Stream termination separate from transport closure using headers
5. **Tracing**: OpenTelemetry support is opt-in via `EnableTracing()` function

## Directory Structure

```
protoc-gen-go-axon/
├── main.go              # Plugin entry point
├── src/                 # Template system
│   ├── src.go          # Main template + data structures
│   ├── interfaces.go   # Interface templates
│   ├── server_interceptors.go  # Interceptor system
│   ├── playground.go   # Playground templates
│   └── playground_html.go  # Playground UI
├── protos/             # Example proto definitions
│   ├── accounts/       # Entity-based organization
│   └── cards/
├── contracts/          # Generated code (gitignored)
│   ├── accounts/
│   │   ├── *.pb.go            # Standard protobuf
│   │   └── *_axon.pb.go      # NATS RPC code
│   └── cards/
├── _labs/              # Experimental/testing code
└── Makefile            # Build automation
```

## Template System Details

### Template Data Flow

`TemplateData` (src.go) contains:
- `PackageName`: Go package name
- `SourceFile`: Original .proto file path
- `Services`: Array of `ServiceData`

Each `ServiceData` contains:
- `Name`: Service name
- `Methods`: Array of `MethodData`

Each `MethodData` contains:
- Streaming flags (`IsClientStreaming`, `IsServerStreaming`)
- Type information (`InputType`, `OutputType`)
- NATS topic (format: `ServiceName.MethodName`)

### Template Helper Functions

Defined in `templateFuncs` (src/src.go):
- `isUnary`: Returns true if method is unary (no streaming)
- `isServerStreaming`: Server sends multiple messages
- `isClientStreaming`: Client sends multiple messages
- `isBidirectional`: Both directions stream
- `clientType`: Generates client struct name
- `streamType`: Generates stream interface name
- `streamImplType`: Generates stream implementation name

## Interceptor System

The generated code includes a full interceptor system similar to gRPC:

- **UnaryServerInterceptor**: Intercepts unary RPC calls
- **StreamServerInterceptor**: Intercepts streaming calls
- **Server Options**: `WithUnaryInterceptor()`, `WithStreamInterceptor()`
- **Chaining**: Multiple interceptors can be chained

Use `Register[Service]Server(nc, srv, opts...)` to register with interceptors.

## Playground Feature

Each generated service includes an `EnablePlayground()` function that launches a web UI for testing:

```go
playground := accounts.EnablePlayground(nc, ":8080")
defer playground.Shutdown(context.Background())
```

The playground provides:
- Method listing with stream types
- Interactive request/response testing
- Support for all streaming patterns
- Real-time message streaming

## Proto File Organization

Protos are organized by entity under `protos/`:
- Each entity is a directory (e.g., `protos/accounts/`)
- Multiple `.proto` files per entity are supported
- Import paths are relative to `protos/` base
- Generated code goes to `contracts/[entity]/`

The Makefile auto-discovers entities via `find $(PROTO_SRC_BASE) -mindepth 1 -maxdepth 1 -type d`.

## Tracing Support

Generated code includes OpenTelemetry tracing:
- Opt-in via `EnableTracing()` function in generated code
- Traces both client and server sides
- Context propagation via NATS headers
- Spans include RPC method, service, stream type
- Records errors and message counts

## Important Implementation Notes

1. **Modifying Templates**: Templates use Go's `text/template` with custom functions. Be careful with escaping (backticks, quotes) when generating Go code.

2. **Stream Lifecycle**:
   - Client streams: Messages buffered by stream ID until `.close` signal
   - Server streams: Each message sent immediately, EOF sent on completion
   - Bidirectional: Init message establishes stream, then `.in`/`.out` channels

3. **Type Handling**:
   - Input/output types use `QualifiedGoIdent` for cross-package imports
   - Pointer types are added in template (`*` prefix)

4. **Error Handling**: Always set `X-Error` header, never embed errors in protobuf payload

5. **Makefile Mappings**: The proto generation builds import mappings dynamically to ensure cross-proto imports resolve correctly

## Testing Approach

When testing generated code:
1. Start a NATS server (e.g., `docker run -p 4222:4222 nats:latest`)
2. Generate contracts: `make protos`
3. Implement server interface
4. Register with `Register[Service]Server(nc, srv)`
5. Create client with `New[Service]Client(nc)`
6. Optionally enable playground for manual testing

## Common Gotchas

1. **Queue Groups**: All subscriptions use queue groups for load balancing. Don't manually subscribe to RPC topics.
2. **Stream Closure**: Always call `CloseSend()` or `CloseAndRecv()` to properly terminate streams.
3. **Context Cancellation**: Streams monitor context; cancelled contexts immediately terminate the stream.
4. **Generated Code Location**: Contracts are generated to `contracts/[entity]/` based on proto organization.
5. **Import Paths**: The `go_package` option in proto files determines the Go import path, not the directory structure.
