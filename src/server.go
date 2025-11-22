package src

import "text/template"

// Add this to the main file template after the CallOption definitions
var _ = template.Must(FileTemplate.New("serviceRegistry").Parse(`

{{template "interceptors" .}}

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
	// Streaming registration is handled by the generated service registration functions
	// which call the appropriate stream-specific setup code for each service

	return nil
}`))
