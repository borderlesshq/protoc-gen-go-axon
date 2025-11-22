package src

import "text/template"

// Add this to the main file template after the CallOption definitions
var _ = template.Must(FileTemplate.New("interceptors").Parse(`
// ============================================================================
// Server Interceptors
// ============================================================================

// UnaryServerInfo contains information about a unary RPC
type UnaryServerInfo struct {
	Server      interface{}
	FullMethod  string
}

// UnaryHandler defines the handler invoked by UnaryServerInterceptor
type UnaryHandler func(ctx context.Context, req interface{}) (interface{}, error)

// UnaryServerInterceptor provides a hook to intercept unary RPC calls
type UnaryServerInterceptor func(ctx context.Context, req interface{}, info *UnaryServerInfo, handler UnaryHandler) (interface{}, error)

// StreamServerInfo contains information about a streaming RPC
type StreamServerInfo struct {
	Server      interface{}
	FullMethod  string
	IsClientStream bool
	IsServerStream bool
}

// StreamServerInterceptor provides a hook to intercept streaming RPC calls
type StreamServerInterceptor func(srv interface{}, ss ServerStream, info *StreamServerInfo, handler StreamHandler) error

// StreamHandler defines the handler invoked by StreamServerInterceptor
type StreamHandler func(srv interface{}, stream ServerStream) error

// ServerStream defines the common interface for all stream types
type ServerStream interface {
	Context() context.Context
	SendMsg(m interface{}) error
	RecvMsg(m interface{}) error
}

// ============================================================================
// Server Options
// ============================================================================

// ServerOption configures how we set up the server
type ServerOption func(*serverOptions)

type serverOptions struct {
	unaryInt       UnaryServerInterceptor
	streamInt      StreamServerInterceptor
	playground     *servicePlayground
	playgroundPort int64
}

func defaultServerOptions() *serverOptions {
	return &serverOptions{
		unaryInt:  nil,
		streamInt: nil,
		playground:     nil,
		playgroundPort: -1,
	}
}

func WithPlaygroundEnabled(port int64) ServerOption {
	return func(o *serverOptions) {
		o.playgroundPort = port
	}
}

// WithUnaryInterceptor returns a ServerOption that sets the UnaryServerInterceptor
func WithUnaryInterceptor(i UnaryServerInterceptor) ServerOption {
	return func(o *serverOptions) {
		if o.unaryInt != nil {
			o.unaryInt = chainUnaryServerInterceptors(o.unaryInt, i)
		} else {
			o.unaryInt = i
		}
	}
}

// WithStreamInterceptor returns a ServerOption that sets the StreamServerInterceptor
func WithStreamInterceptor(i StreamServerInterceptor) ServerOption {
	return func(o *serverOptions) {
		if o.streamInt != nil {
			o.streamInt = chainStreamServerInterceptors(o.streamInt, i)
		} else {
			o.streamInt = i
		}
	}
}

// chainUnaryServerInterceptors creates a single interceptor from multiple interceptors
func chainUnaryServerInterceptors(outer, inner UnaryServerInterceptor) UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *UnaryServerInfo, handler UnaryHandler) (interface{}, error) {
		return outer(ctx, req, info, func(ctx context.Context, req interface{}) (interface{}, error) {
			return inner(ctx, req, info, handler)
		})
	}
}

// chainStreamServerInterceptors creates a single interceptor from multiple interceptors
func chainStreamServerInterceptors(outer, inner StreamServerInterceptor) StreamServerInterceptor {
	return func(srv interface{}, ss ServerStream, info *StreamServerInfo, handler StreamHandler) error {
		return outer(srv, ss, info, func(srv interface{}, stream ServerStream) error {
			return inner(srv, stream, info, handler)
		})
	}
}

// ============================================================================
// Client Interceptors
// ============================================================================

// UnaryClientInfo contains information about a unary RPC call
type UnaryClientInfo struct {
	FullMethod string
}

// UnaryInvoker is called by UnaryClientInterceptor to complete the RPC
type UnaryInvoker func(ctx context.Context, method string, req, reply interface{}, opts ...CallOption) error

// UnaryClientInterceptor intercepts the execution of a unary RPC on the client
type UnaryClientInterceptor func(ctx context.Context, method string, req, reply interface{}, info *UnaryClientInfo, invoker UnaryInvoker, opts ...CallOption) error

// StreamClientInfo contains information about a streaming RPC call
type StreamClientInfo struct {
	FullMethod     string
	IsClientStream bool
	IsServerStream bool
}

// Streamer is called by StreamClientInterceptor to create a ClientStream
type Streamer func(ctx context.Context, desc *StreamDesc, method string, opts ...CallOption) (ClientStream, error)

// StreamClientInterceptor intercepts the creation of a ClientStream
type StreamClientInterceptor func(ctx context.Context, desc *StreamDesc, method string, streamer Streamer, opts ...CallOption) (ClientStream, error)

// ClientStream defines the common interface for all client-side streams
type ClientStream interface {
	Context() context.Context
	SendMsg(m interface{}) error
	RecvMsg(m interface{}) error
	CloseSend() error
}

// ============================================================================
// Client Options
// ============================================================================

// ClientOption configures the client
type ClientOption func(*clientOptions)

type clientOptions struct {
	unaryInt  UnaryClientInterceptor
	streamInt StreamClientInterceptor
}

func defaultClientOptions() *clientOptions {
	return &clientOptions{
		unaryInt:  nil,
		streamInt: nil,
	}
}

// WithUnaryClientInterceptor returns a ClientOption that sets the UnaryClientInterceptor
func WithUnaryClientInterceptor(i UnaryClientInterceptor) ClientOption {
	return func(o *clientOptions) {
		if o.unaryInt != nil {
			o.unaryInt = chainUnaryClientInterceptors(o.unaryInt, i)
		} else {
			o.unaryInt = i
		}
	}
}

// WithStreamClientInterceptor returns a ClientOption that sets the StreamClientInterceptor
func WithStreamClientInterceptor(i StreamClientInterceptor) ClientOption {
	return func(o *clientOptions) {
		if o.streamInt != nil {
			o.streamInt = chainStreamClientInterceptors(o.streamInt, i)
		} else {
			o.streamInt = i
		}
	}
}

// chainUnaryClientInterceptors creates a single interceptor from multiple interceptors
func chainUnaryClientInterceptors(outer, inner UnaryClientInterceptor) UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, info *UnaryClientInfo, invoker UnaryInvoker, opts ...CallOption) error {
		return outer(ctx, method, req, reply, info, func(ctx context.Context, method string, req, reply interface{}, opts ...CallOption) error {
			return inner(ctx, method, req, reply, info, invoker, opts...)
		}, opts...)
	}
}

// chainStreamClientInterceptors creates a single interceptor from multiple interceptors
func chainStreamClientInterceptors(outer, inner StreamClientInterceptor) StreamClientInterceptor {
	return func(ctx context.Context, desc *StreamDesc, method string, streamer Streamer, opts ...CallOption) (ClientStream, error) {
		return outer(ctx, desc, method, func(ctx context.Context, desc *StreamDesc, method string, opts ...CallOption) (ClientStream, error) {
			return inner(ctx, desc, method, streamer, opts...)
		}, opts...)
	}
}
`))
