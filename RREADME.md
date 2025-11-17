# protoc-gen-go-axon

A powerful Protocol Buffers compiler plugin that generates gRPC-like Go code using your `axon.EventStore` interface instead of HTTP/2 transport. Built with Go templates for clean, maintainable code generation.

## 🚀 Features

- **🔄 gRPC-Compatible API**: Generated code looks and feels exactly like gRPC stubs
- **📡 All RPC Types**: Unary, server streaming, client streaming, and bidirectional streaming
- **🛡️ Proto3 Optional**: Full support with safe getter methods
- **⚡ Axon Transport**: Uses your existing `axon.EventStore` instead of HTTP/2
- **🔧 Type Safety**: Fully typed Go interfaces and implementations
- **📊 Built-in Metrics**: Optional metrics collection for observability
- **🔌 Middleware Support**: Interceptors for authentication, logging, retries, etc.
- **🧪 Testing Utilities**: Mock generation and test helpers
- **✅ Validation**: Input/output validation support
- **🎯 Clean Templates**: Maintainable Go template-based code generation

## 📦 Installation

### Via Go Install

```bash
go install github.com/your-org/protoc-gen-go-axon@latest
```

### From Source

```bash
git clone https://github.com/your-org/protoc-gen-go-axon
cd protoc-gen-go-axon
make install
```

### Docker

```bash
docker pull your-registry/protoc-gen-go-axon:latest
```

## 🛠️ Build System

### Makefile

```makefile
.PHONY: build install clean test lint fmt vet example docker

# Build the plugin
build:
	go build -o protoc-gen-go-axon .

# Install to GOPATH/bin
install: build
	cp protoc-gen-go-axon $(GOPATH)/bin/
	# Or for Go modules:
	# go install .

# Clean build artifacts
clean:
	rm -f protoc-gen-go-axon
	rm -rf generated/
	rm -rf example/generated/

# Run tests
test:
	go test -v ./...
	go test -race ./...

# Run benchmarks
bench:
	go test -bench=. -benchmem ./...

# Lint code
lint:
	golangci-lint run

# Format code
fmt:
	go fmt ./...
	gofumpt -l -w .

# Vet code
vet:
	go vet ./...

# Generate example
example: build
	mkdir -p example/generated
	protoc -I=example \
		--go_out=example/generated \
		--go-axon_out=example/generated \
		--go-axon_opt=metrics=true,debug=true \
		example/user.proto

# Build Docker image
docker:
	docker build -t protoc-gen-go-axon .

# Run all checks
check: fmt vet lint test

# Release build
release:
	goreleaser build --snapshot --rm-dist

# Development setup
dev-setup:
	go mod tidy
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install mvdan.cc/gofumpt@latest
	go install github.com/goreleaser/goreleaser@latest

# CI pipeline
ci: dev-setup check bench
```

### go.mod

```go
module github.com/your-org/protoc-gen-go-axon

go 1.21

require (
	google.golang.org/protobuf v1.31.0
)

require (
	github.com/your-org/axon v0.1.0
)

// Development dependencies
require (
	github.com/stretchr/testify v1.8.4
	github.com/golangci/golangci-lint v1.54.2
	mvdan.cc/gofumpt v0.5.0
)
```

## 🚀 Quick Start

### 1. Define Your Service

```proto
// user.proto
syntax = "proto3";
package example;
option go_package = "github.com/your-org/example;example";

message User {
  string id = 1;
  string name = 2;
  optional string email = 3;
  int32 age = 4;
}

message GetUserRequest {
  string id = 1;
}

message GetUserResponse {
  User user = 1;
}

service UserService {
  rpc GetUser(GetUserRequest) returns (GetUserResponse);
  rpc ListUsers(ListUsersRequest) returns (stream User);
}
```

### 2. Generate Code

```bash
# Basic generation
protoc --go_out=. --go-axon_out=. user.proto

# With options
protoc --go_out=. --go-axon_out=. \
  --go-axon_opt=metrics=true,debug=true,topic_prefix=myapp \
  user.proto
```

### 3. Implement Server

```go
type userServer struct {
    users map[string]*User
}

func (s *userServer) GetUser(ctx context.Context, req *GetUserRequest) (*GetUserResponse, error) {
    user, exists := s.users[req.Id]
    if !exists {
        return nil, fmt.Errorf("user not found: %s", req.Id)
    }
    return &GetUserResponse{User: user}, nil
}

func (s *userServer) ListUsers(req *ListUsersRequest, stream UserService_ListUsersServer) error {
    for _, user := range s.users {
        if err := stream.Send(user); err != nil {
            return err
        }
    }
    return nil
}
```

### 4. Register and Use

```go
func main() {
    // Initialize EventStore
    eventStore := axon.NewEventStore(config)
    
    // Register server
    server := &userServer{users: make(map[string]*User)}
    RegisterUserServiceServer(eventStore, server)
    
    // Create client
    client := NewUserServiceClient(eventStore)
    
    // Make calls
    resp, err := client.GetUser(ctx, &GetUserRequest{Id: "123"})
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("User: %s\n", resp.User.Name)
}
```

## 🔧 Plugin Options

| Option | Description | Default | Example |
|--------|-------------|---------|---------|
| `axon_import` | Import path for axon package | `github.com/your-org/axon` | `axon_import=github.com/myorg/axon` |
| `metrics` | Generate metrics support | `false` | `metrics=true` |
| `debug` | Enable debug output | `false` | `debug=true` |
| `topic_prefix` | Prefix for all topics | _(none)_ | `topic_prefix=myapp` |
| `getters` | Generate optional field getters | `true` | `getters=false` |
| `timeout` | Add timeout support | `true` | `timeout=false` |

### Usage

```bash
protoc --go-axon_out=. \
  --go-axon_opt=metrics=true,topic_prefix=myapp,debug=true \
  user.proto
```

## 🏗️ Architecture

### RPC Type Mappings

| Proto RPC Type | Client API | Server API | Transport |
|----------------|------------|------------|-----------|
| **Unary** | `Method(ctx, req) (resp, error)` | `Method(ctx, req) (resp, error)` | `Request/Reply` |
| **Server Streaming** | `Method(ctx, req) (<-chan Resp, error)` | `Method(req, stream) error` | `Publish/Subscribe` |
| **Client Streaming** | `Method(ctx) (Stream, error)` | `Method(stream) (resp, error)` | `Streamer` |
| **Bidirectional** | `Method(ctx) (Stream, error)` | `Method(stream) error` | `Streamer` |

### Topic Naming Convention

- **Unary**: `[prefix.]ServiceName.MethodName`
- **Server Streaming**: `[prefix.]ServiceName.MethodName` + `.stream`
- **Client Streaming**: `[prefix.]ServiceName.MethodName` + `.stream` / `.close`
- **Bidirectional**: `[prefix.]ServiceName.MethodName` + `.in` / `.out` / `.connect`

## 🧪 Testing

### Unit Tests

```go
func TestUserService(t *testing.T) {
    mockServer := &MockUserServiceServer{
        GetUserFunc: func(ctx context.Context, req *GetUserRequest) (*GetUserResponse, error) {
            return &GetUserResponse{
                User: &User{Id: req.Id, Name: "Test User"},
            }, nil
        },
    }
    
    client := TestUserServiceClient(t, mockServer)
    
    resp, err := client.GetUser(context.Background(), &GetUserRequest{Id: "123"})
    assert.NoError(t, err)
    assert.Equal(t, "Test User", resp.User.Name)
}
```

### Integration Tests

```go
func TestIntegration(t *testing.T) {
    eventStore := axon.NewTestEventStore()
    server := &realUserServer{}
    
    RegisterUserServiceServer(eventStore, server)
    client := NewUserServiceClient(eventStore)
    
    // Test full workflow...
}
```

### Benchmarks

```bash
make bench
```

## 🔌 Middleware & Interceptors

### Built-in Interceptors

```go
// Logging
serverWithMiddleware.AddUnaryInterceptor(loggingInterceptor)

// Authentication
serverWithMiddleware.AddUnaryInterceptor(authInterceptor)

// Rate limiting
serverWithMiddleware.AddUnaryInterceptor(rateLimitInterceptor)

// Circuit breaker
cb := newCircuitBreaker(3, 30*time.Second)
serverWithMiddleware.AddUnaryInterceptor(cb.intercept)

// Retry logic
serverWithMiddleware.AddUnaryInterceptor(retryInterceptor(2, 100*time.Millisecond))
```

### Custom Interceptors

```go
func customInterceptor(ctx context.Context, req interface{}, info *UnaryServerInfo, handler UnaryHandler) (interface{}, error) {
    // Pre-processing
    start := time.Now()
    
    // Call handler
    resp, err := handler(ctx, req)
    
    // Post-processing
    duration := time.Since(start)
    log.Printf("Method %s took %v", info.FullMethod, duration)
    
    return resp, err
}
```

## 📊 Metrics & Observability

### Built-in Metrics

```go
type MetricsCollector interface {
    RecordRPCDuration(service, method string, duration time.Duration)
    RecordRPCCount(service, method string, success bool)
    RecordStreamMessage(service, method string, direction string)
}

// Use with client
client := NewUserServiceClientWithMetrics(baseClient, metricsCollector)
```

### Custom Metrics

```go
type prometheusMetrics struct {
    requestDuration prometheus.HistogramVec
    requestCount    prometheus.CounterVec
}

func (p *prometheusMetrics) RecordRPCDuration(service, method string, duration time.Duration) {
    p.requestDuration.WithLabelValues(service, method).Observe(duration.Seconds())
}

func (p *prometheusMetrics) RecordRPCCount(service, method string, success bool) {
    status := "success"
    if !success {
        status = "error"
    }
    p.requestCount.WithLabelValues(service, method, status).Inc()
}
```

## 🐳 Deployment

### Docker

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN make build

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/protoc-gen-go-axon .
CMD ["./protoc-gen-go-axon"]
```

### Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: user-service
spec:
  replicas: 3
  selector:
    matchLabels:
      app: user-service
  template:
    metadata:
      labels:
        app: user-service
    spec:
      containers:
      - name: user-service
        image: your-registry/user-service:latest
        ports:
        - containerPort: 8080
        env:
        - name: AXON_NATS_URL
          value: "nats://nats:4222"
```

## 🔍 Troubleshooting

### Common Issues

**Import path errors**
```bash
# Fix: Update axon import path
protoc --go-axon_opt=axon_import=github.com/myorg/axon ...
```

**Missing methods**
```go
// Ensure axon.EventStore implements all required methods:
type EventStore interface {
    Publish(topic string, data []byte, opts ...options.PublisherOption) error
    Subscribe(topic string, handler SubscriptionHandler, opts ...options.SubscriptionOption) error
    Request(topic string, params []byte, opts ...options.PublisherOption) (*messages.Message, error)
    Reply(topic string, handler ReplyHandler, opts ...options.SubscriptionOption) error
    NewStreamer(opts ...options.StreamerOption) (Streamer, error)
}
```

**Context timeouts**
```go
// Set appropriate timeouts for streaming operations
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
```

**Debug Mode**
```bash
# Enable verbose output
export PROTOC_GEN_GO_AXON_DEBUG=1
protoc --go-axon_out=. --go-axon_opt=debug=true user.proto
```

### Performance Tips

1. **Use buffered channels** for streaming operations
2. **Set appropriate timeouts** for all contexts
3. **Implement connection pooling** in your EventStore
4. **Use metrics** to monitor performance bottlenecks
5. **Enable compression** in your axon transport layer

## 🧩 Advanced Features

### Template Customization

The plugin uses Go templates for code generation. You can extend or modify templates:

```go
// Custom template functions
var customFuncs = template.FuncMap{
    "myCustomHelper": func(s string) string {
        return strings.ToUpper(s)
    },
}

// Register custom templates
template.Must(fileTemplate.Funcs(customFuncs).Parse(myCustomTemplate))
```

### Plugin Extensions

Extend the plugin with custom generators:

```go
// Add custom generator
func generateCustomCode(g *protogen.GeneratedFile, service *protogen.Service) {
    data := &CustomTemplateData{
        ServiceName: service.GoName,
        // ... other data
    }
    
    var buf bytes.Buffer
    if err := customTemplate.Execute(&buf, data); err != nil {
        log.Fatalf("Template execution failed: %v", err)
    }
    
    g.P(buf.String())
}
```

### Multi-Transport Support

Generate code for multiple transports:

```bash
# Generate for both gRPC and Axon
protoc --go_out=. --go-grpc_out=. --go-axon_out=. user.proto
```

## 📈 Roadmap

- [ ] **HTTP/JSON Gateway** - REST API generation
- [ ] **OpenAPI/Swagger** - API documentation generation
- [ ] **GraphQL** - GraphQL schema generation
- [ ] **WebSocket** - WebSocket transport support
- [ ] **CLI Tools** - Command-line client generation
- [ ] **Proto Validation** - Built-in validation rules
- [ ] **Tracing Support** - OpenTelemetry integration
- [ ] **Schema Evolution** - Backward compatibility tools

## 🤝 Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

### Development Setup

```bash
# Clone the repository
git clone https://github.com/your-org/protoc-gen-go-axon
cd protoc-gen-go-axon

# Install dependencies
make dev-setup

# Run tests
make test

# Format and lint
make check

# Generate example
make example
```

### Submitting Changes

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/amazing-feature`
3. Make your changes and add tests
4. Run the full test suite: `make ci`
5. Commit your changes: `git commit -m 'Add amazing feature'`
6. Push to the branch: `git push origin feature/amazing-feature`
7. Open a Pull Request

### Code Style

- Follow standard Go conventions
- Use `go fmt` and `gofumpt` for formatting
- Write comprehensive tests
- Document public APIs
- Use meaningful commit messages

## 📜 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- **Protocol Buffers** team for the excellent protoc plugin system
- **gRPC** project for API design inspiration
- **Go Templates** for flexible code generation
- **NATS** and **axon** projects for messaging patterns

## 📞 Support

- 📧 Email: support@your-org.com
- 💬 Slack: [#protoc-gen-go-axon](https://your-org.slack.com/channels/protoc-gen-go-axon)
- 🐛 Issues: [GitHub Issues](https://github.com/your-org/protoc-gen-go-axon/issues)
- 📖 Docs: [Documentation Site](https://your-org.github.io/protoc-gen-go-axon)

## 📚 Examples

### Complete Working Example

See the [examples/](examples/) directory for:
- **Basic Usage** - Simple unary and streaming RPCs
- **Advanced Features** - Middleware, metrics, validation
- **Testing** - Unit tests, integration tests, benchmarks
- **Deployment** - Docker, Kubernetes, monitoring
- **Migration** - Moving from gRPC to Axon

### Real-World Usage

```go
// Production server setup
func setupProductionServer() {
    // Initialize EventStore with production config
    eventStore := axon.NewEventStore(axon.Config{
        NATSURL:     os.Getenv("NATS_URL"),
        ClusterID:   os.Getenv("CLUSTER_ID"),
        Compression: true,
        TLS:         &axon.TLSConfig{/* ... */},
    })
    
    // Create service with business logic
    userService := &productionUserService{
        db:    database.Connect(),
        cache: redis.Connect(),
        auth:  auth.NewService(),
    }
    
    // Set up comprehensive middleware
    server := NewUserServiceServerWithMiddleware(userService)
    
    // Authentication & authorization
    server.AddUnaryInterceptor(authInterceptor)
    server.AddUnaryInterceptor(rbacInterceptor)
    
    // Observability
    server.AddUnaryInterceptor(tracingInterceptor)
    server.AddUnaryInterceptor(metricsInterceptor)
    server.AddUnaryInterceptor(loggingInterceptor)
    
    // Resilience
    server.AddUnaryInterceptor(rateLimitInterceptor(1000, time.Minute))
    server.AddUnaryInterceptor(circuitBreakerInterceptor(5, 30*time.Second))
    server.AddUnaryInterceptor(retryInterceptor(3, time.Second))
    
    // Validation
    server.AddUnaryInterceptor(validationInterceptor)
    
    // Register with EventStore
    RegisterUserServiceServer(eventStore, server)
    
    // Graceful shutdown
    c := make(chan os.Signal, 1)
    signal.Notify(c, os.Interrupt, syscall.SIGTERM)
    <-c
    
    log.Println("Shutting down gracefully...")
    eventStore.Close()
}
```

---

**Happy coding! 🚀**

---

## Development Files

### .golangci.yml

```yaml
# .golangci.yml
linters-settings:
  govet:
    check-shadowing: true
  golint:
    min-confidence: 0
  gocyclo:
    min-complexity: 15
  maligned:
    suggest-new: true
  dupl:
    threshold: 100
  goconst:
    min-len: 2
    min-occurrences: 2
  depguard:
    list-type: blacklist
    packages:
      - github.com/sirupsen/logrus
    packages-with-error-message:
      - github.com/sirupsen/logrus: "logging is allowed only by our logger"
  misspell:
    locale: US
  lll:
    line-length: 140
  goimports:
    local-prefixes: github.com/your-org/protoc-gen-go-axon
  gocritic:
    enabled-tags:
      - diagnostic
      - experimental
      - opinionated
      - performance
      - style
    disabled-checks:
      - dupImport
      - ifElseChain
      - octalLiteral
      - whyNoLint
      - wrapperFunc

linters:
  disable-all: true
  enable:
    - bodyclose
    - deadcode
    - depguard
    - dogsled
    - dupl
    - errcheck
    - exportloopref
    - exhaustive
    - funlen
    - gochecknoinits
    - goconst
    - gocritic
    - gocyclo
    - gofmt
    - goimports
    - golint
    - gomnd
    - goprintffuncname
    - gosec
    - gosimple
    - govet
    - ineffassign
    - interfacer
    - lll
    - misspell
    - nakedret
    - noctx
    - nolintlint
    - rowserrcheck
    - scopelint
    - staticcheck
    - structcheck
    - stylecheck
    - typecheck
    - unconvert
    - unparam
    - unused
    - varcheck
    - whitespace

issues:
  exclude-rules:
    - path: _test\.go
      linters:
        - gomnd
        - funlen
        - gocyclo

run:
  timeout: 5m
  issues-exit-code: 1
  tests: true
  skip-dirs:
    - bin
    - vendor
    - var
    - tmp
    - generated
  skip-files:
    - ".*\\.pb\\.go$"
    - ".*_axon\\.pb\\.go$"
```

### .goreleaser.yml

```yaml
# .goreleaser.yml
before:
  hooks:
    - go mod tidy
    - go generate ./...

builds:
  - env:
      - CGO_ENABLED=0
    goos:
      - linux
      - windows
      - darwin
    goarch:
      - amd64
      - arm64
    main: ./
    binary: protoc-gen-go-axon

archives:
  - replacements:
      darwin: Darwin
      linux: Linux
      windows: Windows
      386: i386
      amd64: x86_64

checksum:
  name_template: 'checksums.txt'

snapshot:
  name_template: "{{ incpatch .Version }}-next"

changelog:
  sort: asc
  filters:
    exclude:
      - '^docs:'
      - '^test:'

release:
  github:
    owner: your-org
    name: protoc-gen-go-axon
  draft: false
  prerelease: auto
```

### GitHub Actions CI

```yaml
# .github/workflows/ci.yml
name: CI

on:
  push:
    branches: [ main, develop ]
  pull_request:
    branches: [ main ]

jobs:
  test:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        go-version: [1.20, 1.21]
    
    steps:
    - uses: actions/checkout@v3
    
    - name: Set up Go
      uses: actions/setup-go@v3
      with:
        go-version: ${{ matrix.go-version }}
    
    - name: Cache Go modules
      uses: actions/cache@v3
      with:
        path: ~/go/pkg/mod
        key: ${{ runner.os }}-go-${{ hashFiles('**/go.sum') }}
        restore-keys: |
          ${{ runner.os }}-go-
    
    - name: Install dependencies
      run: make dev-setup
    
    - name: Run tests
      run: make test
    
    - name: Run benchmarks
      run: make bench
    
    - name: Lint
      run: make lint
    
    - name: Build
      run: make build
    
    - name: Generate example
      run: make example

  release:
    needs: test
    runs-on: ubuntu-latest
    if: github.ref == 'refs/heads/main'
    
    steps:
    - uses: actions/checkout@v3
      with:
        fetch-depth: 0
    
    - name: Set up Go
      uses: actions/setup-go@v3
      with:
        go-version: 1.21
    
    - name: Run GoReleaser
      uses: goreleaser/goreleaser-action@v4
      with:
        distribution: goreleaser
        version: latest
        args: release --rm-dist
      env:
        GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```