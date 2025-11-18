# protoc-gen-go-axon

A Protocol Buffers compiler plugin that generates gRPC-style RPC code over NATS for Go. This plugin creates idiomatic Go code with full support for unary, server streaming, client streaming, and bidirectional streaming RPCs using NATS as the transport layer.

## Features

- ✅ **Direct NATS Client** - Uses `github.com/nats-io/nats.go` directly, no heavy abstractions
- ✅ **All gRPC Patterns** - Unary, server streaming, client streaming, and bidirectional streaming
- ✅ **Header Support** - Built-in support for metadata via NATS headers
- ✅ **Context Propagation** - Full `context.Context` support for timeouts and cancellation
- ✅ **Clean EOF Semantics** - Proper stream termination separate from transport closure
- ✅ **Error Handling** - Errors propagated via headers, not embedded in payload
- ✅ **Type Safe** - Generated code is fully type-safe from your `.proto` definitions
- ✅ **Production Ready** - Battle-tested patterns with proper cleanup and resource management

## Table of Contents

- [Installation](#installation)
- [Quick Start](#quick-start)
- [Project Structure](#project-structure)
- [Makefile Usage](#makefile-usage)
- [Generated Code](#generated-code)
- [Usage Examples](#usage-examples)
- [Architecture](#architecture)
- [Advanced Topics](#advanced-topics)
- [Contributing](#contributing)

## Installation

### Prerequisites

- Go 1.19 or later
- Protocol Buffers compiler (`protoc`)
- NATS Server (for running examples)

### Install protoc

**macOS:**
```bash
brew install protobuf
```

**Linux:**
```bash
# Ubuntu/Debian
apt-get install -y protobuf-compiler

# Or download from GitHub releases
# https://github.com/protocolbuffers/protobuf/releases
```

### Install the Plugin

```bash
go install github.com/borderlesshq/protoc-gen-go-axon@latest
```

This installs the plugin to `$GOPATH/bin/protoc-gen-go-axon`, which `protoc` will automatically find.

## Quick Start

### 1. Define Your Service

Create `protos/accounts/accounts.proto`:

```protobuf
syntax = "proto3";

package accounts;
option go_package = "yourapp/contracts/accounts";

service AccountService {
  // Unary RPC
  rpc CreateAccount(CreateAccountRequest) returns (CreateAccountResponse);
  
  // Server streaming
  rpc ListAccounts(ListAccountsRequest) returns (stream Account);
  
  // Client streaming
  rpc UploadBatch(stream Account) returns (UploadResponse);
  
  // Bidirectional streaming
  rpc Chat(stream Message) returns (stream Message);
}

message CreateAccountRequest {
  string email = 1;
  string name = 2;
}

message CreateAccountResponse {
  Account account = 1;
}

message Account {
  string id = 1;
  string email = 2;
  string name = 3;
}

message ListAccountsRequest {
  int32 limit = 1;
}

message UploadResponse {
  int32 count = 1;
}

message Message {
  string text = 1;
}
```

### 2. Generate Code

```bash
# Generate all protos
make protos

# Or generate just accounts
make proto-accounts
```

### 3. Implement Your Service

```go
package main

import (
    "context"
    "log"
    
    "github.com/nats-io/nats.go"
    "yourapp/contracts/accounts"
)

type accountService struct{}

func (s *accountService) CreateAccount(
    ctx context.Context,
    req *accounts.CreateAccountRequest,
) (*accounts.CreateAccountResponse, error) {
    // Your business logic here
    return &accounts.CreateAccountResponse{
        Account: &accounts.Account{
            Id:    "acc_123",
            Email: req.Email,
            Name:  req.Name,
        },
    }, nil
}

func (s *accountService) ListAccounts(
    req *accounts.ListAccountsRequest,
    stream accounts.AccountService_ListAccountsServer,
) error {
    // Stream multiple accounts
    for i := 0; i < int(req.Limit); i++ {
        if err := stream.Send(&accounts.Account{
            Id:    fmt.Sprintf("acc_%d", i),
            Email: fmt.Sprintf("user%d@example.com", i),
            Name:  fmt.Sprintf("User %d", i),
        }); err != nil {
            return err
        }
    }
    return nil
}

func main() {
    // Connect to NATS
    nc, err := nats.Connect("nats://localhost:4222")
    if err != nil {
        log.Fatal(err)
    }
    defer nc.Close()

    // Register service
    if err := accounts.RegisterAccountServiceServer(nc, &accountService{}); err != nil {
        log.Fatal(err)
    }

    log.Println("Server running on NATS...")
    select {} // Block forever
}
```

### 4. Use the Client

```go
package main

import (
    "context"
    "io"
    "log"
    "time"
    
    "github.com/nats-io/nats.go"
    "yourapp/contracts/accounts"
)

func main() {
    // Connect to NATS
    nc, err := nats.Connect("nats://localhost:4222")
    if err != nil {
        log.Fatal(err)
    }
    defer nc.Close()

    // Create client
    client := accounts.NewAccountServiceClient(nc)
    ctx := context.Background()

    // Unary call
    resp, err := client.CreateAccount(ctx, &accounts.CreateAccountRequest{
        Email: "user@example.com",
        Name:  "Justice",
    })
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("Created: %s", resp.Account.Id)

    // Server streaming
    stream, err := client.ListAccounts(ctx, &accounts.ListAccountsRequest{
        Limit: 10,
    })
    if err != nil {
        log.Fatal(err)
    }

    for {
        account, err := stream.Recv()
        if err == io.EOF {
            break
        }
        if err != nil {
            log.Fatal(err)
        }
        log.Printf("Received: %s - %s", account.Id, account.Email)
    }
}
```

## Project Structure

```
your-project/
├── protos/                      # Proto definitions organized by entity
│   ├── accounts/
│   │   └── accounts.proto
│   ├── cards/
│   │   └── cards.proto
│   └── payments/
│       └── payments.proto
│
├── contracts/                   # Generated code (gitignore this)
│   ├── accounts/
│   │   ├── accounts.pb.go       # Standard protobuf
│   │   └── accounts_axon.pb.go  # NATS RPC code
│   ├── cards/
│   │   ├── cards.pb.go
│   │   └── cards_axon.pb.go
│   └── payments/
│       ├── payments.pb.go
│       └── payments_axon.pb.go
│
├── services/                    # Your service implementations
│   ├── account_service.go
│   ├── card_service.go
│   └── payment_service.go
│
├── cmd/
│   ├── server/
│   │   └── main.go
│   └── client/
│       └── main.go
│
├── Makefile
├── go.mod
└── README.md
```

## Makefile Usage

### Core Commands

#### Generate All Protos
```bash
make protos
```
Generates code for all entities in `protos/` directory.

Output:
```
🔄 Generating protos for all entities...

📝 Processing: accounts
   Source: /path/to/protos/accounts
   Output: /path/to/contracts/accounts
   ✓ Generated accounts successfully

📝 Processing: cards
   Source: /path/to/protos/cards
   Output: /path/to/contracts/cards
   ✓ Generated cards successfully

✅ All protos generated
```

#### Generate Specific Entity
```bash
# Method 1: Using ENTITY variable
make proto ENTITY=accounts

# Method 2: Using convenience target (recommended)
make proto-accounts
make proto-cards
make proto-payments
```

#### List Available Entities
```bash
make list-protos
```

Output:
```
Available proto entities:
  - accounts
  - cards
  - payments
```

### Cleanup Commands

```bash
# Clean everything (generated files + plugins)
make clean-proto

# Clean only generated .pb.go files
make clean-proto-generated

# Clean only installed plugins
make clean-proto-plugins
```

### Utility Commands

#### Watch Mode (Auto-regenerate)
```bash
make watch-protos
```
Watches for changes in `protos/` and automatically regenerates code.

**Requirements:**
- macOS: `brew install fswatch`
- Linux: `apt-get install inotify-tools`

#### Help
```bash
make help-proto
```
Shows all available targets and examples.

### Complete Workflow Examples

#### Fresh Start
```bash
# Clean everything and regenerate
make clean-proto
make protos
```

#### Development Workflow
```bash
# 1. Modify protos/accounts/accounts.proto
# 2. Regenerate just accounts
make proto-accounts

# 3. Test your changes
go test ./services/...
```

#### CI/CD Pipeline
```bash
# In your CI script
make clean-proto-generated  # Clean old generated files
make protos                  # Generate fresh code
go test ./...                # Run tests
```

## Generated Code

### Server Interface

For each service, a server interface is generated:

```go
type AccountServiceServer interface {
    CreateAccount(context.Context, *CreateAccountRequest) (*CreateAccountResponse, error)
    ListAccounts(*ListAccountsRequest, AccountService_ListAccountsServer) error
    UploadBatch(AccountService_UploadBatchServer) error
    Chat(AccountService_ChatServer) error
}
```

### Client Interface

And a corresponding client interface:

```go
type AccountServiceClient interface {
    CreateAccount(ctx context.Context, in *CreateAccountRequest, opts ...CallOption) (*CreateAccountResponse, error)
    ListAccounts(ctx context.Context, in *ListAccountsRequest, opts ...CallOption) (AccountService_ListAccountsClient, error)
    UploadBatch(ctx context.Context, opts ...CallOption) (AccountService_UploadBatchClient, error)
    Chat(ctx context.Context, opts ...CallOption) (AccountService_ChatClient, error)
}
```

### CallOptions

Generated code includes support for call options:

```go
// Set timeout
resp, err := client.CreateAccount(ctx, req, 
    WithTimeout(5*time.Second))

// Add custom headers
resp, err := client.CreateAccount(ctx, req,
    WithHeader("Authorization", "Bearer token"),
    WithHeader("X-Request-ID", "12345"))
```

## Usage Examples

### Unary RPC

**Server:**
```go
func (s *myService) CreateAccount(
    ctx context.Context,
    req *CreateAccountRequest,
) (*CreateAccountResponse, error) {
    // Validate input
    if req.Email == "" {
        return nil, fmt.Errorf("email is required")
    }
    
    // Business logic
    account := s.db.Create(req.Email, req.Name)
    
    return &CreateAccountResponse{
        Account: account,
    }, nil
}
```

**Client:**
```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

resp, err := client.CreateAccount(ctx, &CreateAccountRequest{
    Email: "user@example.com",
    Name:  "Justice",
})
if err != nil {
    log.Fatal(err)
}
log.Printf("Created: %s", resp.Account.Id)
```

### Server Streaming

**Server:**
```go
func (s *myService) ListAccounts(
    req *ListAccountsRequest,
    stream AccountService_ListAccountsServer,
) error {
    accounts, err := s.db.List(req.Limit)
    if err != nil {
        return err
    }
    
    for _, account := range accounts {
        if err := stream.Send(account); err != nil {
            return err
        }
    }
    
    return nil
}
```

**Client:**
```go
stream, err := client.ListAccounts(ctx, &ListAccountsRequest{Limit: 100})
if err != nil {
    log.Fatal(err)
}

for {
    account, err := stream.Recv()
    if err == io.EOF {
        break // Stream complete
    }
    if err != nil {
        log.Fatal(err)
    }
    
    log.Printf("Account: %s - %s", account.Id, account.Email)
}
```

### Client Streaming

**Server:**
```go
func (s *myService) UploadBatch(
    stream AccountService_UploadBatchServer,
) error {
    count := 0
    
    for {
        account, err := stream.Recv()
        if err == io.EOF {
            // Client finished sending
            return stream.SendAndClose(&UploadResponse{
                Count: int32(count),
            })
        }
        if err != nil {
            return err
        }
        
        // Process account
        s.db.Create(account)
        count++
    }
}
```

**Client:**
```go
stream, err := client.UploadBatch(ctx)
if err != nil {
    log.Fatal(err)
}

accounts := []*Account{
    {Email: "user1@example.com", Name: "User 1"},
    {Email: "user2@example.com", Name: "User 2"},
    {Email: "user3@example.com", Name: "User 3"},
}

for _, account := range accounts {
    if err := stream.Send(account); err != nil {
        log.Fatal(err)
    }
}

resp, err := stream.CloseAndRecv()
if err != nil {
    log.Fatal(err)
}

log.Printf("Uploaded %d accounts", resp.Count)
```

### Bidirectional Streaming

**Server:**
```go
func (s *myService) Chat(
    stream AccountService_ChatServer,
) error {
    for {
        msg, err := stream.Recv()
        if err == io.EOF {
            return nil
        }
        if err != nil {
            return err
        }
        
        // Echo back with prefix
        response := &Message{
            Text: "Echo: " + msg.Text,
        }
        
        if err := stream.Send(response); err != nil {
            return err
        }
    }
}
```

**Client:**
```go
stream, err := client.Chat(ctx)
if err != nil {
    log.Fatal(err)
}

// Send messages in a goroutine
go func() {
    messages := []string{"Hello", "How are you?", "Goodbye"}
    for _, msg := range messages {
        if err := stream.Send(&Message{Text: msg}); err != nil {
            log.Printf("Send error: %v", err)
            return
        }
        time.Sleep(time.Second)
    }
    stream.CloseSend()
}()

// Receive messages
for {
    msg, err := stream.Recv()
    if err == io.EOF {
        break
    }
    if err != nil {
        log.Fatal(err)
    }
    
    log.Printf("Received: %s", msg.Text)
}
```

## Architecture

### Transport Layer

The plugin generates code that uses NATS as the transport:

- **Unary RPC**: Uses `nc.RequestWithContext()` for request/reply pattern
- **Server Streaming**: Uses inbox pattern with unique reply subjects
- **Client Streaming**: Aggregates messages by stream ID, sends final response
- **Bidirectional**: Separate `.in` and `.out` channels for full duplex

### Message Framing

Messages are framed using NATS headers:

```go
// Control headers
"Stream-ID"     // Unique identifier for stream instances
"Stream-EOF"    // Signals end of stream
"X-Error"       // Error message if operation failed
"Seq-Num"       // Sequence number for ordering
```

### Error Handling

Errors are propagated via headers, not payload:

```go
// Server sends error
header := nats.Header{}
header.Set("X-Error", "validation failed: email required")
msg.RespondMsg(&nats.Msg{Header: header})

// Client receives error
if errMsg := msg.Header.Get("X-Error"); errMsg != "" {
    return fmt.Errorf("server error: %s", errMsg)
}
```

### Stream Lifecycle

1. **Initialization**: Client creates unique stream ID
2. **Communication**: Messages tagged with stream ID
3. **Termination**: EOF signal sent via header
4. **Cleanup**: Subscriptions unsubscribed, channels closed

## Advanced Topics

### Context Propagation

All RPCs support context for timeout and cancellation:

```go
// Timeout after 5 seconds
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

resp, err := client.CreateAccount(ctx, req)
```

### Custom Headers

Add metadata to requests:

```go
resp, err := client.CreateAccount(ctx, req,
    WithHeader("X-Request-ID", requestID),
    WithHeader("X-User-ID", userID),
)
```

### Error Handling Patterns

```go
resp, err := client.CreateAccount(ctx, req)
if err != nil {
    // Check for context errors
    if ctx.Err() == context.DeadlineExceeded {
        log.Println("Request timed out")
        return
    }
    
    // Check for NATS errors
    if err == nats.ErrTimeout {
        log.Println("NATS timeout")
        return
    }
    
    // Application errors
    log.Printf("Application error: %v", err)
    return
}
```

### Integration with Axon

For event-driven workflows, combine NATS RPC with Axon:

```go
func (s *AccountService) CreateAccount(
    ctx context.Context,
    req *CreateAccountRequest,
) (*CreateAccountResponse, error) {
    // 1. Handle RPC (synchronous)
    account := s.createAccountLogic(req)
    
    // 2. Publish event via Axon (asynchronous)
    event := &AccountCreatedEvent{
        AccountId: account.ID,
        Email:     account.Email,
    }
    eventData, _ := proto.Marshal(event)
    s.axon.Publish("account.created", eventData)
    
    // 3. Return RPC response immediately
    return &CreateAccountResponse{Account: account}, nil
}
```

### Performance Tuning

**Buffer Sizes:**
```go
// In generated code, default buffer is 10
recvCh := make(chan *Account, 10)

// For high-throughput streams, regenerate with larger buffers
// (modify template in plugin source)
```

**Timeouts:**
```go
// Set default timeout
client := NewAccountServiceClient(nc)
resp, err := client.CreateAccount(ctx, req,
    WithTimeout(30*time.Second))

// Or use context
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
```

## Troubleshooting

### Plugin Not Found

```bash
Error: protoc-gen-go-axon: program not found or is not executable
```

**Solution:**
```bash
# Ensure plugin is installed
go install github.com/borderlesshq/protoc-gen-go-axon@latest

# Verify it's in PATH
which protoc-gen-go-axon

# Add GOPATH/bin to PATH if needed
export PATH=$PATH:$(go env GOPATH)/bin
```

### Proto3 Optional Not Supported

**Solution:** Update dependencies:
```bash
go get -u google.golang.org/protobuf@latest
go mod tidy
```

### NATS Connection Issues

```go
Error: nats: no servers available for connection
```

**Solution:**
```bash
# Start NATS server
docker run -p 4222:4222 nats:latest

# Or install locally
brew install nats-server  # macOS
nats-server
```

### Stream Not Receiving Messages

**Check:**
1. Server registered? `RegisterAccountServiceServer(nc, srv)`
2. Topics match? Check generated subject names
3. NATS connected? Verify `nc.Status()`
4. Context cancelled? Check `ctx.Done()`

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

### Development Setup

```bash
# Clone repo
git clone https://github.com/borderlesshq/protoc-gen-go-axon
cd protoc-gen-go-axon

# Install dependencies
go mod download

# Build plugin
go build -o protoc-gen-go-axon main.go

# Test with example protos
make protos
```

## License

MIT License - see LICENSE file for details

## Credits

Created by [Justice Nefe](https://github.com/borderlesshq) at BorderlessHQ

Inspired by gRPC and the NATS ecosystem.

## Support

- 📧 Email: support@borderlesshq.com
- 🐛 Issues: [GitHub Issues](https://github.com/borderlesshq/protoc-gen-go-axon/issues)
- 💬 Discord: [Join our community](https://discord.gg/borderlesshq)

---

**Happy coding with NATS RPC! 🚀**
