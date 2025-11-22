package src

import "text/template"

// Add this to the main file template after the CallOption definitions
var _ = template.Must(FileTemplate.New("serverInterceptors").Parse(`
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
// Service Registry
// ============================================================================

// ServiceDesc represents a service descriptor (similar to gRPC)
type ServiceDesc struct {
	ServiceName string
	HandlerType interface{}
	Methods     []MethodDesc
	Streams     []StreamDesc
}

// MethodDesc represents a unary method descriptor
type MethodDesc struct {
	MethodName string
	Handler    methodHandler
}

type methodHandler func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor UnaryServerInterceptor) (interface{}, error)

// StreamDesc represents a streaming method descriptor
type StreamDesc struct {
	StreamName    string
	Handler       streamHandler
	ServerStreams bool
	ClientStreams bool
}

type streamHandler func(srv interface{}, stream ServerStream, interceptor StreamServerInterceptor) error

// Server holds the registered services
type Server struct {
	nc          *nats.Conn
	shutdownSig chan struct{}
	opts        serverOptions
	mu          sync.RWMutex
	services    map[string]*ServiceDesc
}

func (s *Server) Serve() error {
	if s.nc == nil {
		panic("not initialized, ensure to call: accounts.")
	}

	if s.opts.playground != nil {
		return s.opts.playground.server.ListenAndServe()
	}

	<-s.shutdownSig
	fmt.Println("shutting down...")
	return nil
}

func (s *Server) Shutdown() error {
	close(s.shutdownSig)
	if err := s.nc.Drain(); err != nil {
	}
	defer s.nc.Close()

	if s.opts.playground != nil {
		return s.opts.playground.Shutdown(context.Background())
	}

	return nil
}


// NewServer creates a new server with options
func NewServer(nc *nats.Conn, opts ...ServerOption) *Server {
	s := &Server{
		nc:       nc,
		opts:     *defaultServerOptions(),
		services: make(map[string]*ServiceDesc),
		shutdownSig: make(chan struct{}, 1),
	}
	
	for _, opt := range opts {
		opt(&s.opts)
	}

	if s.opts.playgroundPort > -1 {
		s.opts.playground = enablePlayground(s.nc, fmt.Sprintf(":%d", s.opts.playgroundPort))
	}
	
	return s
}

// registerService registers a service with validation (like gRPC)
func (s *Server) registerService(sd *ServiceDesc, ss interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Validate that ss implements the service interface
	if ss != nil {
		ht := reflect.TypeOf(sd.HandlerType).Elem()
		st := reflect.TypeOf(ss)
		if !st.Implements(ht) {
			return fmt.Errorf("handler of type %v does not satisfy %v", st, ht)
		}
	}
	
	// Check for duplicate registration
	if _, exists := s.services[sd.ServiceName]; exists {
		return fmt.Errorf("service %s already registered", sd.ServiceName)
	}
	
	s.services[sd.ServiceName] = sd
	
	// Register handlers with NATS
	return s.registerHandlers(sd, ss)
}

// registerHandlers registers all method handlers with NATS
func (s *Server) registerHandlers(sd *ServiceDesc, ss interface{}) error {
	// Register unary methods
	for _, method := range sd.Methods {
		fullMethod := sd.ServiceName + "." + method.MethodName

		if _, err := s.nc.QueueSubscribe(fullMethod, sd.ServiceName+"."+method.MethodName, func(msg *nats.Msg) {
			// Extract context with tracing
			ctx := extractTraceContext(context.Background(), msg.Header)

			// Create decode function
			dec := func(v interface{}) error {
				return proto.Unmarshal(msg.Data, v.(proto.Message))
			}

			// Call handler with interceptor
			resp, err := method.Handler(ss, ctx, dec, s.opts.unaryInt)

			if err != nil {
				errHeader := nats.Header{}
				errHeader.Set("X-Error", err.Error())
				msg.RespondMsg(&nats.Msg{Header: errHeader})
				return
			}

			// Marshal response
			data, err := proto.Marshal(resp.(proto.Message))
			if err != nil {
				errHeader := nats.Header{}
				errHeader.Set("X-Error", fmt.Sprintf("failed to marshal response: %v", err))
				msg.RespondMsg(&nats.Msg{Header: errHeader})
				return
			}

			msg.Respond(data)
		}); err != nil {
			return fmt.Errorf("failed to subscribe to %s: %w", fullMethod, err)
		}
	}

	// Register streaming methods
	// Note: Streaming RPC registration requires the generated code to set up
	// the NATS subscriptions because each streaming pattern (server, client,
	// bidirectional) has unique message handling requirements.
	//
	// The ServiceDesc.Streams entries provide the handler wrappers that integrate
	// interceptors. These handlers are invoked by the generated subscription code.
	//
	// When interceptors are configured, the generated code should call the handler
	// from ServiceDesc which will invoke the interceptor chain.
	//
	// The actual subscription setup is delegated to the generated code per service
	// because it requires:
	// - Stream aggregators for client streaming
	// - Stream managers for bidirectional streaming
	// - Proper subject patterns (.init, .in, .out, .close)
	// - Context extraction and tracing setup
	//
	// This design maintains backward compatibility: services can still register
	// directly (without Server) or through Server (with interceptors).

	return nil
}
`))
