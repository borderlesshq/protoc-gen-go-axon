package src

import "text/template"

// Server interface template
var _ = template.Must(FileTemplate.New("serverInterface").Parse(`
// {{.Name}}Server is the server API for {{.Name}} service.
type {{.Name}}Server interface {
{{range .Methods}}
	{{template "serverMethodSignature" .}}
{{end}}
}
`))

// Client interface template
var _ = template.Must(FileTemplate.New("clientInterface").Parse(`
// {{.Name}}Client is the client API for {{.Name}} service.
type {{.Name}}Client interface {
{{range .Methods}}
	{{template "clientMethodSignature" .}}
{{end}}
}
`))

// Client implementation template
var _ = template.Must(FileTemplate.New("clientImpl").Parse(`
type {{clientType .Name}} struct {
	nc   *nats.Conn
	opts clientOptions
}

// New{{.Name}}Client creates a new client for {{.Name}}
func New{{.Name}}Client(nc *nats.Conn, opts ...ClientOption) {{.Name}}Client {
	clientOpts := defaultClientOptions()
	for _, opt := range opts {
		opt(clientOpts)
	}
	return &{{clientType .Name}}{
		nc:   nc,
		opts: *clientOpts,
	}
}

{{range .Methods}}
{{template "clientMethod" .}}
{{end}}
`))

// Server registration template
var _ = template.Must(FileTemplate.New("serverRegistration").Parse(`
// Service descriptor for {{.Name}}
var {{.Name}}_ServiceDesc = ServiceDesc{
	ServiceName: "{{.Name}}",
	HandlerType: (*{{.Name}}Server)(nil),
	Methods: []MethodDesc{
{{range .Methods}}
{{if isUnary .}}
		{
			MethodName: "{{.Name}}",
			Handler:    _{{.ServiceName}}_{{.Name}}_Handler,
		},
{{end}}
{{end}}
	},
	Streams: []StreamDesc{
{{range .Methods}}
{{if not (isUnary .)}}
		{
			StreamName:    "{{.Name}}",
			Handler:       _{{.ServiceName}}_{{.Name}}_Handler,
			ServerStreams: {{.IsServerStreaming}},
			ClientStreams: {{.IsClientStreaming}},
		},
{{end}}
{{end}}
	},
}

// Register{{.Name}}Server registers the service with optional interceptors
func Register{{.Name}}Server(nc *nats.Conn, srv {{.Name}}Server, opts ...ServerOption) (*Server, error) {
	s := NewServer(nc, opts...)
	if err := s.registerService(&{{.Name}}_ServiceDesc, srv); err != nil {
		return nil, err
	}

	// Register streaming method subscriptions
	if err := _register{{.Name}}StreamHandlers(s.nc, srv); err != nil {
		return nil, err
	}

	return s, nil
}

// Register{{.Name}}ServerWithServer registers with an existing server
func Register{{.Name}}ServerWithServer(s *Server, srv {{.Name}}Server) (*Server, error) {
	if err := s.registerService(&{{.Name}}_ServiceDesc, srv); err != nil {
		return nil, err
	}

	// Register streaming method subscriptions
	if err := _register{{.Name}}StreamHandlers(s.nc, srv); err != nil {
		return nil, err
	}

	return s, nil
}

// _register{{.Name}}StreamHandlers sets up NATS subscriptions for streaming methods
func _register{{.Name}}StreamHandlers(nc *nats.Conn, srv {{.Name}}Server) error {
{{range .Methods}}
{{if not (isUnary .)}}
{{template "serverMethodRegistration" .}}
{{end}}
{{end}}
	return nil
}

{{range .Methods}}
{{if isUnary .}}
// Handler wrapper for {{.Name}}
func _{{.ServiceName}}_{{.Name}}_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor UnaryServerInterceptor) (interface{}, error) {
	in := &{{.InputTypeName}}{}
	if err := dec(in); err != nil {
		return nil, err
	}
	
	if interceptor == nil {
		return srv.({{.ServiceName}}Server).{{.Name}}(ctx, in)
	}
	
	info := &UnaryServerInfo{
		Server:     srv,
		FullMethod: "{{.ServiceName}}.{{.Name}}",
	}
	
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.({{.ServiceName}}Server).{{.Name}}(ctx, req.({{.InputType}}))
	}
	
	return interceptor(ctx, in, info, handler)
}
{{else}}
// Handler wrapper for streaming {{.Name}}
func _{{.ServiceName}}_{{.Name}}_Handler(srv interface{}, stream ServerStream, interceptor StreamServerInterceptor) error {
	if interceptor == nil {
		{{if isServerStreaming .}}
		// Server streaming: need to decode initial request from stream context
		req := stream.(*wrappedServerStream_{{.ServiceName}}_{{.Name}}).req
		return srv.({{.ServiceName}}Server).{{.Name}}(req, stream.(*wrappedServerStream_{{.ServiceName}}_{{.Name}}).{{streamType .ServiceName .Name "Server"}})
		{{else if isClientStreaming .}}
		// Client streaming: wrap stream for Recv operations
		return srv.({{.ServiceName}}Server).{{.Name}}(stream.(*wrappedServerStream_{{.ServiceName}}_{{.Name}}).{{streamType .ServiceName .Name "Server"}})
		{{else}}
		// Bidirectional streaming
		return srv.({{.ServiceName}}Server).{{.Name}}(stream.(*wrappedServerStream_{{.ServiceName}}_{{.Name}}).{{streamType .ServiceName .Name "Server"}})
		{{end}}
	}

	info := &StreamServerInfo{
		Server:         srv,
		FullMethod:     "{{.ServiceName}}.{{.Name}}",
		IsClientStream: {{.IsClientStreaming}},
		IsServerStream: {{.IsServerStreaming}},
	}

	handler := func(srv interface{}, ss ServerStream) error {
		{{if isServerStreaming .}}
		req := ss.(*wrappedServerStream_{{.ServiceName}}_{{.Name}}).req
		return srv.({{.ServiceName}}Server).{{.Name}}(req, ss.(*wrappedServerStream_{{.ServiceName}}_{{.Name}}).{{streamType .ServiceName .Name "Server"}})
		{{else if isClientStreaming .}}
		return srv.({{.ServiceName}}Server).{{.Name}}(ss.(*wrappedServerStream_{{.ServiceName}}_{{.Name}}).{{streamType .ServiceName .Name "Server"}})
		{{else}}
		return srv.({{.ServiceName}}Server).{{.Name}}(ss.(*wrappedServerStream_{{.ServiceName}}_{{.Name}}).{{streamType .ServiceName .Name "Server"}})
		{{end}}
	}

	return interceptor(srv, stream, info, handler)
}

// wrappedServerStream for {{.Name}}
type wrappedServerStream_{{.ServiceName}}_{{.Name}} struct {
	{{streamType .ServiceName .Name "Server"}}
	ctx context.Context
	{{if isServerStreaming .}}
	req {{.InputType}}
	{{end}}
}

func (w *wrappedServerStream_{{.ServiceName}}_{{.Name}}) Context() context.Context {
	return w.ctx
}

func (w *wrappedServerStream_{{.ServiceName}}_{{.Name}}) SendMsg(m interface{}) error {
	{{if or (isServerStreaming .) (isBidirectional .)}}
	return w.{{streamType .ServiceName .Name "Server"}}.Send(m.({{.OutputType}}))
	{{else}}
	return w.{{streamType .ServiceName .Name "Server"}}.SendAndClose(m.({{.OutputType}}))
	{{end}}
}

func (w *wrappedServerStream_{{.ServiceName}}_{{.Name}}) RecvMsg(m interface{}) error {
	{{if or (isClientStreaming .) (isBidirectional .)}}
	msg, err := w.{{streamType .ServiceName .Name "Server"}}.Recv()
	if err != nil {
		return err
	}
	*(m.({{.InputType}})) = *msg
	return nil
	{{else}}
	return io.EOF // Server streaming doesn't receive from client
	{{end}}
}
{{end}}
{{end}}
`))

// Streaming interfaces template
var _ = template.Must(FileTemplate.New("streamingInterfaces").Parse(`
{{range .Methods}}
{{if isServerStreaming .}}
{{template "serverStreamingInterface" .}}
{{end}}
{{if isClientStreaming .}}
{{template "clientStreamingInterface" .}}
{{end}}
{{if isBidirectional .}}
{{template "bidirectionalStreamingInterface" .}}
{{end}}
{{end}}
`))

// Method signature templates
var _ = template.Must(FileTemplate.New("serverMethodSignature").Parse(`
{{if isUnary .}}
{{.Name}}(context.Context, {{.InputType}}) ({{.OutputType}}, error)
{{else if isServerStreaming .}}
{{.Name}}({{.InputType}}, {{streamType .ServiceName .Name "Server"}}) error
{{else if isClientStreaming .}}
{{.Name}}({{streamType .ServiceName .Name "Server"}}) error
{{else}}
{{.Name}}({{streamType .ServiceName .Name "Server"}}) error
{{end}}
`))

var _ = template.Must(FileTemplate.New("clientMethodSignature").Parse(`
{{if isUnary .}}
{{.Name}}(ctx context.Context, in {{.InputType}}, opts ...CallOption) ({{.OutputType}}, error)
{{else if isServerStreaming .}}
{{.Name}}(ctx context.Context, in {{.InputType}}, opts ...CallOption) ({{streamType .ServiceName .Name "Client"}}, error)
{{else if isClientStreaming .}}
{{.Name}}(ctx context.Context, opts ...CallOption) ({{streamType .ServiceName .Name "Client"}}, error)
{{else}}
{{.Name}}(ctx context.Context, opts ...CallOption) ({{streamType .ServiceName .Name "Client"}}, error)
{{end}}
`))

// Client method implementations
var _ = template.Must(FileTemplate.New("clientMethod").Parse(`
{{if isUnary .}}
func (c *{{clientType .ServiceName}}) {{.Name}}(ctx context.Context, in {{.InputType}}, opts ...CallOption) ({{.OutputType}}, error) {
	// If interceptor is configured, use it
	if c.opts.unaryInt != nil {
		info := &UnaryClientInfo{
			FullMethod: "{{.ServiceName}}.{{.Name}}",
		}

		invoker := func(ctx context.Context, method string, req, reply interface{}, opts ...CallOption) error {
			out, err := c.{{.Name}}Invoke(ctx, req.({{.InputType}}), opts...)
			if err != nil {
				return err
			}
			*(reply.({{.OutputType}})) = *out
			return nil
		}

		out := &{{.OutputTypeName}}{}
		err := c.opts.unaryInt(ctx, "{{.ServiceName}}.{{.Name}}", in, out, info, invoker, opts...)
		return out, err
	}

	return c.{{.Name}}Invoke(ctx, in, opts...)
}

// {{.Name}}Invoke performs the actual unary RPC invocation
func (c *{{clientType .ServiceName}}) {{.Name}}Invoke(ctx context.Context, in {{.InputType}}, opts ...CallOption) ({{.OutputType}}, error) {
	// Start tracing span if enabled
	ctx, span := startSpan(ctx, "{{.ServiceName}}.{{.Name}}",
		trace.SpanKindClient,
		attribute.String("rpc.system", "nats"),
		attribute.String("rpc.service", "{{.ServiceName}}"),
		attribute.String("rpc.method", "{{.Name}}"),
	)
	defer span.End()

	callOpts := defaultCallOptions()
	for _, opt := range opts {
		opt(callOpts)
	}

	data, err := proto.Marshal(in)
	if err != nil {
		if tracingEnabled {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to marshal request")
		}
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create message with headers
	msg := &nats.Msg{
		Subject: "{{.Topic}}",
		Data:    data,
		Header:  make(nats.Header),
	}

	// Inject trace context into headers
	injectTraceContext(ctx, msg.Header)

	// Add custom headers
	for k, v := range callOpts.headers {
		msg.Header.Set(k, v)
	}

	resp, err := c.nc.RequestMsgWithContext(ctx, msg)
	if err != nil {
		if tracingEnabled {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		return nil, fmt.Errorf("request failed: %w", err)
	}

	// Check for error in response headers
	if errMsg := resp.Header.Get("X-Error"); errMsg != "" {
		err := fmt.Errorf("server error: %s", errMsg)
		if tracingEnabled {
			span.RecordError(err)
			span.SetStatus(codes.Error, errMsg)
		}
		return nil, err
	}

	out := &{{.OutputTypeName}}{}
	if err := proto.Unmarshal(resp.Data, out); err != nil {
		if tracingEnabled {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to unmarshal response")
		}
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if tracingEnabled {
		span.SetStatus(codes.Ok, "")
	}
	return out, nil
}
{{else if isServerStreaming .}}
func (c *{{clientType .ServiceName}}) {{.Name}}(ctx context.Context, in {{.InputType}}, opts ...CallOption) ({{streamType .ServiceName .Name "Client"}}, error) {
	// Start tracing span if enabled
	ctx, span := startSpan(ctx, "{{.ServiceName}}.{{.Name}}",
		trace.SpanKindClient,
		attribute.String("rpc.system", "nats"),
		attribute.String("rpc.service", "{{.ServiceName}}"),
		attribute.String("rpc.method", "{{.Name}}"),
		attribute.String("stream.type", "server"),
	)
	// Span is closed by stream.CloseSend()

	callOpts := defaultCallOptions()
	for _, opt := range opts {
		opt(callOpts)
	}

	data, err := proto.Marshal(in)
	if err != nil {
		if tracingEnabled {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to marshal request")
			span.End()
		}
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create unique inbox for stream responses
	inbox := nats.NewInbox()
	recvCh := make(chan {{.OutputType}}, 10)
	errCh := make(chan error, 1)

	// Create request headers
	requestHeaders := make(nats.Header)
	injectTraceContext(ctx, requestHeaders)
	for k, v := range callOpts.headers {
		requestHeaders.Set(k, v)
	}

	stream := &{{streamImplType .ServiceName .Name "Client"}}{
		nc:             c.nc,
		recvCh:         recvCh,
		errCh:          errCh,
		ctx:            ctx,
		span:           span,
		inbox:          inbox,
		requestData:    data,
		requestHeaders: requestHeaders,
		topic:          "{{.Topic}}",
		lastSeqNum:     0,
		stopMonitor:    make(chan struct{}),
	}

	// Subscribe to stream responses
	sub, err := c.nc.Subscribe(inbox, func(msg *nats.Msg) {
		stream.handleStreamMessage(msg)
	})
	if err != nil {
		if tracingEnabled {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to subscribe")
			span.End()
		}
		return nil, fmt.Errorf("failed to subscribe: %w", err)
	}
	stream.sub = sub

	// Start health monitoring for automatic reconnection
	stream.startHealthMonitor()

	// Send initial request with reply-to inbox
	if err := c.nc.PublishMsg(&nats.Msg{
		Subject: "{{.Topic}}",
		Reply:   inbox,
		Data:    data,
		Header:  requestHeaders,
	}); err != nil {
		sub.Unsubscribe()
		if tracingEnabled {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to publish request")
			span.End()
		}
		return nil, fmt.Errorf("failed to publish request: %w", err)
	}

	// Handle cleanup on context cancellation
	go func() {
		<-ctx.Done()
		sub.Unsubscribe()
		if tracingEnabled && span.IsRecording() {
			span.SetStatus(codes.Error, "context cancelled")
			span.End()
		}
		close(recvCh)
	}()

	return stream, nil
}
{{else if isClientStreaming .}}
func (c *{{clientType .ServiceName}}) {{.Name}}(ctx context.Context, opts ...CallOption) ({{streamType .ServiceName .Name "Client"}}, error) {
	// Start tracing span if enabled
	ctx, span := startSpan(ctx, "{{.ServiceName}}.{{.Name}}",
		trace.SpanKindClient,
		attribute.String("rpc.system", "nats"),
		attribute.String("rpc.service", "{{.ServiceName}}"),
		attribute.String("rpc.method", "{{.Name}}"),
		attribute.String("stream.type", "client"),
	)
	// Span is closed by stream.CloseAndRecv()

	callOpts := defaultCallOptions()
	for _, opt := range opts {
		opt(callOpts)
	}

	streamID := nats.NewInbox()
	responseInbox := streamID + ".response"
	
	// Subscribe for final response
	responseSub, err := c.nc.SubscribeSync(responseInbox)
	if err != nil {
		if tracingEnabled {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to subscribe to response")
			span.End()
		}
		return nil, fmt.Errorf("failed to subscribe to response: %w", err)
	}

	return &{{streamImplType .ServiceName .Name "Client"}}{
		nc:          c.nc,
		streamID:    streamID,
		topic:       "{{.Topic}}",
		responseSub: responseSub,
		ctx:         ctx,
		span:        span,
		seqNum:      0,
	}, nil
}
{{else}}
func (c *{{clientType .ServiceName}}) {{.Name}}(ctx context.Context, opts ...CallOption) ({{streamType .ServiceName .Name "Client"}}, error) {
	// Start tracing span if enabled
	ctx, span := startSpan(ctx, "{{.ServiceName}}.{{.Name}}",
		trace.SpanKindClient,
		attribute.String("rpc.system", "nats"),
		attribute.String("rpc.service", "{{.ServiceName}}"),
		attribute.String("rpc.method", "{{.Name}}"),
		attribute.String("stream.type", "bidirectional"),
	)
	// Span is closed by stream.CloseSend()

	callOpts := defaultCallOptions()
	for _, opt := range opts {
		opt(callOpts)
	}

	streamID := nats.NewInbox()
	recvCh := make(chan {{.OutputType}}, 10)
	errCh := make(chan error, 1)

	stream := &{{streamImplType .ServiceName .Name "Client"}}{
		nc:          c.nc,
		streamID:    streamID,
		topic:       "{{.Topic}}",
		recvCh:      recvCh,
		errCh:       errCh,
		ctx:         ctx,
		span:        span,
		seqNum:      0,
		stopMonitor: make(chan struct{}),
	}

	// Subscribe to receive stream
	sub, err := c.nc.Subscribe(streamID+".out", func(msg *nats.Msg) {
		stream.handleRecvMessage(msg)
	})
	if err != nil {
		if tracingEnabled {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to subscribe")
			span.End()
		}
		return nil, fmt.Errorf("failed to subscribe: %w", err)
	}
	stream.sub = sub

	// Start health monitoring for automatic reconnection
	stream.startHealthMonitor()

	// Notify server of stream initialization
	header := nats.Header{}
	header.Set("Stream-ID", streamID)

	// Inject trace context
	injectTraceContext(ctx, header)

	if err := c.nc.PublishMsg(&nats.Msg{
		Subject: "{{.Topic}}.init",
		Header:  header,
	}); err != nil {
		sub.Unsubscribe()
		if tracingEnabled {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to initialize stream")
			span.End()
		}
		return nil, fmt.Errorf("failed to initialize stream: %w", err)
	}

	// Handle cleanup on context cancellation
	go func() {
		<-ctx.Done()
		sub.Unsubscribe()
		if tracingEnabled && span.IsRecording() {
			span.SetStatus(codes.Error, "context cancelled")
			span.End()
		}
		close(recvCh)
	}()

	return stream, nil
}
{{end}}
`))

// Server method registration
var _ = template.Must(FileTemplate.New("serverMethodRegistration").Parse(`
{{if isUnary .}}
	// {{.Name}} - Unary RPC
	if _, err := nc.QueueSubscribe("{{.Topic}}", "{{.ServiceName}}.{{.Name}}", func(msg *nats.Msg) {
		// Extract trace context from headers if tracing is enabled
		ctx := extractTraceContext(context.Background(), msg.Header)
		
		// Start server span if tracing is enabled
		ctx, span := startSpan(ctx, "{{.ServiceName}}.{{.Name}}",
			trace.SpanKindServer,
			attribute.String("rpc.system", "nats"),
			attribute.String("rpc.service", "{{.ServiceName}}"),
			attribute.String("rpc.method", "{{.Name}}"),
		)
		defer span.End()

		req := &{{.InputTypeName}}{}
		if err := proto.Unmarshal(msg.Data, req); err != nil {
			if tracingEnabled {
				span.RecordError(err)
				span.SetStatus(codes.Error, "failed to unmarshal request")
			}
			errHeader := nats.Header{}
			errHeader.Set("X-Error", fmt.Sprintf("failed to unmarshal request: %v", err))
			msg.RespondMsg(&nats.Msg{Header: errHeader})
			return
		}

		resp, err := srv.{{.Name}}(ctx, req)
		if err != nil {
			if tracingEnabled {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			}
			errHeader := nats.Header{}
			errHeader.Set("X-Error", err.Error())
			msg.RespondMsg(&nats.Msg{Header: errHeader})
			return
		}

		data, err := proto.Marshal(resp)
		if err != nil {
			if tracingEnabled {
				span.RecordError(err)
				span.SetStatus(codes.Error, "failed to marshal response")
			}
			errHeader := nats.Header{}
			errHeader.Set("X-Error", fmt.Sprintf("failed to marshal response: %v", err))
			msg.RespondMsg(&nats.Msg{Header: errHeader})
			return
		}

		if tracingEnabled {
			span.SetStatus(codes.Ok, "")
		}
		msg.Respond(data)
	}); err != nil {
		return err
	}
{{else if isServerStreaming .}}
	// {{.Name}} - Server streaming RPC
	if _, err := nc.QueueSubscribe("{{.Topic}}", "{{.ServiceName}}.{{.Name}}", func(msg *nats.Msg) {
		// Extract trace context
		ctx := extractTraceContext(context.Background(), msg.Header)

		// Start server span
		ctx, span := startSpan(ctx, "{{.ServiceName}}.{{.Name}}",
			trace.SpanKindServer,
			attribute.String("rpc.system", "nats"),
			attribute.String("rpc.service", "{{.ServiceName}}"),
			attribute.String("rpc.method", "{{.Name}}"),
			attribute.String("stream.type", "server"),
		)
		defer span.End()

		req := &{{.InputTypeName}}{}
		if err := proto.Unmarshal(msg.Data, req); err != nil {
			if tracingEnabled {
				span.RecordError(err)
				span.SetStatus(codes.Error, "failed to unmarshal request")
			}
			errHeader := nats.Header{}
			errHeader.Set("X-Error", fmt.Sprintf("failed to unmarshal request: %v", err))
			errHeader.Set("Stream-EOF", "true")
			nc.PublishMsg(&nats.Msg{
				Subject: msg.Reply,
				Header:  errHeader,
			})
			return
		}

		// Check if client wants to resume from a specific sequence
		var startSeq uint64 = 0
		if resumeSeq := msg.Header.Get("Resume-From-Seq"); resumeSeq != "" {
			if seq, err := strconv.ParseUint(resumeSeq, 10, 64); err == nil {
				startSeq = seq
			}
		}

		stream := &{{streamImplType .ServiceName .Name "Server"}}{
			nc:     nc,
			reply:  msg.Reply,
			seqNum: startSeq,
		}

		if err := srv.{{.Name}}(req, stream); err != nil {
			if tracingEnabled {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			}
			errHeader := nats.Header{}
			errHeader.Set("X-Error", err.Error())
			errHeader.Set("Stream-EOF", "true")
			nc.PublishMsg(&nats.Msg{
				Subject: msg.Reply,
				Header:  errHeader,
			})
			return
		}

		if tracingEnabled {
			span.SetStatus(codes.Ok, "")
		}
		stream.Close()
	}); err != nil {
		return err
	}
{{else if isClientStreaming .}}
	// {{.Name}} - Client streaming RPC
	streamAggregator := &{{.ServiceName}}_{{.Name}}_Aggregator{
		nc:      nc,
		srv:     srv,
		streams: make(map[string]*{{.ServiceName}}_{{.Name}}_StreamBuffer),
	}

	// Subscribe to data messages with queue group
	if _, err := nc.QueueSubscribe("{{.Topic}}", "{{.ServiceName}}.{{.Name}}", func(msg *nats.Msg) {
		streamID := msg.Header.Get("Stream-ID")
		seqNum := msg.Header.Get("Seq-Num")
		
		streamAggregator.mu.Lock()
		buf, exists := streamAggregator.streams[streamID]
		if !exists {
			buf = &{{.ServiceName}}_{{.Name}}_StreamBuffer{
				messages:  make([]{{.InputType}}, 0),
				responseSubject: streamID + ".response",
			}
			streamAggregator.streams[streamID] = buf
		}
		streamAggregator.mu.Unlock()

		in := &{{.InputTypeName}}{}
		if err := proto.Unmarshal(msg.Data, in); err != nil {
			return
		}
		
		buf.mu.Lock()
		buf.messages = append(buf.messages, in)
		buf.mu.Unlock()
	}); err != nil {
		return err
	}

	// Subscribe to close signal with queue group
	if _, err := nc.QueueSubscribe("{{.Topic}}.close", "{{.ServiceName}}.{{.Name}}", func(msg *nats.Msg) {
		streamID := msg.Header.Get("Stream-ID")
		
		// Extract trace context
		ctx := extractTraceContext(context.Background(), msg.Header)
		
		// Start server span
		ctx, span := startSpan(ctx, "{{.ServiceName}}.{{.Name}}",
			trace.SpanKindServer,
			attribute.String("rpc.system", "nats"),
			attribute.String("rpc.service", "{{.ServiceName}}"),
			attribute.String("rpc.method", "{{.Name}}"),
			attribute.String("stream.type", "client"),
		)
		defer span.End()

		streamAggregator.mu.Lock()
		buf := streamAggregator.streams[streamID]
		delete(streamAggregator.streams, streamID)
		streamAggregator.mu.Unlock()
		
		if buf == nil {
			return
		}

		if tracingEnabled {
			span.SetAttributes(attribute.Int("stream.messages_received", len(buf.messages)))
		}

		stream := &{{streamImplType .ServiceName .Name "Server"}}{
			messages: buf.messages,
			index:    0,
		}

		if err := srv.{{.Name}}(stream); err != nil {
			if tracingEnabled {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			}
			errHeader := nats.Header{}
			errHeader.Set("X-Error", err.Error())
			nc.PublishMsg(&nats.Msg{
				Subject: buf.responseSubject,
				Header:  errHeader,
			})
			return
		}

		if tracingEnabled {
			span.SetStatus(codes.Ok, "")
		}

		if stream.response != nil {
			data, _ := proto.Marshal(stream.response)
			nc.Publish(buf.responseSubject, data)
		}
	}); err != nil {
		return err
	}
{{else}}
	// {{.Name}} - Bidirectional streaming RPC
	streamManager := &{{.ServiceName}}_{{.Name}}_Manager{
		nc:      nc,
		srv:     srv,
		streams: make(map[string]*{{.ServiceName}}_{{.Name}}_ServerStream),
	}

	// Subscribe to init with queue group
	if _, err := nc.QueueSubscribe("{{.Topic}}.init", "{{.ServiceName}}.{{.Name}}", func(msg *nats.Msg) {
		streamID := msg.Header.Get("Stream-ID")

		// Extract trace context
		ctx := extractTraceContext(context.Background(), msg.Header)

		// Start server span
		ctx, span := startSpan(ctx, "{{.ServiceName}}.{{.Name}}",
			trace.SpanKindServer,
			attribute.String("rpc.system", "nats"),
			attribute.String("rpc.service", "{{.ServiceName}}"),
			attribute.String("rpc.method", "{{.Name}}"),
			attribute.String("stream.type", "bidirectional"),
		)

		// Check if client wants to resume from a specific sequence
		var startSeq uint64 = 0
		if resumeSeq := msg.Header.Get("Resume-From-Seq"); resumeSeq != "" {
			if seq, err := strconv.ParseUint(resumeSeq, 10, 64); err == nil {
				startSeq = seq
			}
		}

		// Check if stream already exists (reconnection scenario)
		streamManager.mu.Lock()
		existingStream := streamManager.streams[streamID]
		if existingStream != nil {
			// Update sequence number for resume
			existingStream.mu.Lock()
			existingStream.seqNum = startSeq
			existingStream.mu.Unlock()
			streamManager.mu.Unlock()
			return
		}

		stream := &{{.ServiceName}}_{{.Name}}_ServerStream{
			nc:          nc,
			streamID:    streamID,
			recvCh:      make(chan {{.InputType}}, 10),
			sendSubject: streamID + ".out",
			seqNum:      startSeq,
		}

		streamManager.streams[streamID] = stream
		streamManager.mu.Unlock()

		go func() {
			defer span.End()

			if err := srv.{{.Name}}(stream); err != nil {
				if tracingEnabled {
					span.RecordError(err)
					span.SetStatus(codes.Error, err.Error())
				}
				errHeader := nats.Header{}
				errHeader.Set("X-Error", err.Error())
				errHeader.Set("Stream-EOF", "true")
				nc.PublishMsg(&nats.Msg{
					Subject: stream.sendSubject,
					Header:  errHeader,
				})
			} else {
				if tracingEnabled {
					span.SetStatus(codes.Ok, "")
				}
			}
			stream.Close()
		}()
	}); err != nil {
		return err
	}

	// Subscribe to data with queue group
	if _, err := nc.QueueSubscribe("{{.Topic}}.in", "{{.ServiceName}}.{{.Name}}", func(msg *nats.Msg) {
		streamID := msg.Header.Get("Stream-ID")
		
		streamManager.mu.RLock()
		stream := streamManager.streams[streamID]
		streamManager.mu.RUnlock()
		
		if stream == nil {
			return
		}

		in := &{{.InputTypeName}}{}
		if err := proto.Unmarshal(msg.Data, in); err != nil {
			return
		}

		select {
		case stream.recvCh <- in:
		default:
		}
	}); err != nil {
		return err
	}

	// Subscribe to close with queue group
	if _, err := nc.QueueSubscribe("{{.Topic}}.close", "{{.ServiceName}}.{{.Name}}", func(msg *nats.Msg) {
		streamID := msg.Header.Get("Stream-ID")
		
		streamManager.mu.Lock()
		stream := streamManager.streams[streamID]
		delete(streamManager.streams, streamID)
		streamManager.mu.Unlock()
		
		if stream != nil {
			close(stream.recvCh)
		}
	}); err != nil {
		return err
	}
{{end}}
`))

// Server streaming interface
var _ = template.Must(FileTemplate.New("serverStreamingInterface").Parse(`
// {{streamType .ServiceName .Name "Server"}} is the server-side streaming interface for {{.Name}}
type {{streamType .ServiceName .Name "Server"}} interface {
	Send({{.OutputType}}) error
}

type {{streamImplType .ServiceName .Name "Server"}} struct {
	nc     *nats.Conn
	reply  string
	seqNum uint64
	mu     sync.Mutex
}

func (s *{{streamImplType .ServiceName .Name "Server"}}) Send(msg {{.OutputType}}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Add sequence number to track message order
	header := nats.Header{}
	header.Set("Seq-Num", strconv.FormatUint(s.seqNum, 10))
	s.seqNum++

	return s.nc.PublishMsg(&nats.Msg{
		Subject: s.reply,
		Data:    data,
		Header:  header,
	})
}

func (s *{{streamImplType .ServiceName .Name "Server"}}) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Send EOF signal
	header := nats.Header{}
	header.Set("Stream-EOF", "true")
	return s.nc.PublishMsg(&nats.Msg{
		Subject: s.reply,
		Header:  header,
	})
}

// {{streamType .ServiceName .Name "Client"}} is the client-side interface for server streaming
type {{streamType .ServiceName .Name "Client"}} interface {
	Recv() ({{.OutputType}}, error)
	CloseSend() error
}

type {{streamImplType .ServiceName .Name "Client"}} struct {
	nc               *nats.Conn
	sub              *nats.Subscription
	recvCh           chan {{.OutputType}}
	errCh            chan error
	ctx              context.Context
	span             trace.Span
	mu               sync.Mutex
	closed           bool
	lastSeqNum       uint64
	reconnecting     bool
	inbox            string
	requestData      []byte
	requestHeaders   nats.Header
	topic            string
	lastActivityNano int64 // atomic access only
	stopMonitor      chan struct{}
}

func (s *{{streamImplType .ServiceName .Name "Client"}}) Recv() ({{.OutputType}}, error) {
	select {
	case msg, ok := <-s.recvCh:
		if !ok {
			return nil, io.EOF
		}
		return msg, nil
	case err := <-s.errCh:
		return nil, err
	case <-s.ctx.Done():
		return nil, s.ctx.Err()
	}
}

func (s *{{streamImplType .ServiceName .Name "Client"}}) CloseSend() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true

	// Stop health monitor
	if s.stopMonitor != nil {
		close(s.stopMonitor)
	}

	if tracingEnabled && s.span.IsRecording() {
		s.span.End()
	}

	return s.sub.Unsubscribe()
}

func (s *{{streamImplType .ServiceName .Name "Client"}}) reconnect() error {
	s.mu.Lock()
	if s.closed || s.reconnecting {
		s.mu.Unlock()
		return nil
	}
	s.reconnecting = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.reconnecting = false
		s.mu.Unlock()
	}()

	// Unsubscribe old subscription
	if s.sub != nil {
		s.sub.Unsubscribe()
	}

	// Create new subscription with same inbox
	sub, err := s.nc.Subscribe(s.inbox, func(msg *nats.Msg) {
		s.handleStreamMessage(msg)
	})
	if err != nil {
		return fmt.Errorf("failed to resubscribe: %w", err)
	}
	s.sub = sub

	// Resend request with resume sequence number
	resumeHeaders := nats.Header{}
	for k, v := range s.requestHeaders {
		resumeHeaders[k] = v
	}
	resumeHeaders.Set("Resume-From-Seq", strconv.FormatUint(s.lastSeqNum, 10))

	if err := s.nc.PublishMsg(&nats.Msg{
		Subject: s.topic,
		Reply:   s.inbox,
		Data:    s.requestData,
		Header:  resumeHeaders,
	}); err != nil {
		return fmt.Errorf("failed to republish request: %w", err)
	}

	return nil
}

func (s *{{streamImplType .ServiceName .Name "Client"}}) startHealthMonitor() {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-s.stopMonitor:
				return
			case <-s.ctx.Done():
				return
			case <-ticker.C:
				// Check if we've received any activity in the last 30 seconds
				lastActivity := atomic.LoadInt64(&s.lastActivityNano)
				if lastActivity > 0 {
					elapsed := time.Since(time.Unix(0, lastActivity))
					if elapsed > 30*time.Second {
						// No activity for 30 seconds, attempt reconnection
						if err := s.reconnect(); err != nil {
							// If reconnection fails, report error
							select {
							case s.errCh <- fmt.Errorf("auto-reconnect failed: %w", err):
							default:
							}
						}
					}
				}
			}
		}
	}()
}

func (s *{{streamImplType .ServiceName .Name "Client"}}) handleStreamMessage(msg *nats.Msg) {
	// Update activity timestamp atomically (lock-free)
	atomic.StoreInt64(&s.lastActivityNano, time.Now().UnixNano())

	// Check for EOF signal
	if msg.Header.Get("Stream-EOF") == "true" {
		if tracingEnabled {
			s.span.SetAttributes(attribute.Int("stream.messages_received", int(s.lastSeqNum)))
			s.span.SetStatus(codes.Ok, "")
			s.span.End()
		}
		close(s.recvCh)
		return
	}

	// Check for error
	if errMsg := msg.Header.Get("X-Error"); errMsg != "" {
		err := fmt.Errorf("stream error: %s", errMsg)
		if tracingEnabled {
			s.span.RecordError(err)
			s.span.SetStatus(codes.Error, errMsg)
			s.span.End()
		}
		select {
		case s.errCh <- err:
		default:
		}
		close(s.recvCh)
		return
	}

	out := &{{.OutputTypeName}}{}
	if err := proto.Unmarshal(msg.Data, out); err != nil {
		if tracingEnabled {
			s.span.RecordError(err)
		}
		select {
		case s.errCh <- fmt.Errorf("unmarshal error: %w", err):
		default:
		}
		return
	}

	// Track sequence number (no lock needed for atomic read/write)
	if seqStr := msg.Header.Get("Seq-Num"); seqStr != "" {
		if seq, err := strconv.ParseUint(seqStr, 10, 64); err == nil {
			s.mu.Lock()
			s.lastSeqNum = seq + 1
			s.mu.Unlock()
		}
	}

	select {
	case s.recvCh <- out:
	case <-s.ctx.Done():
		if tracingEnabled {
			s.span.SetStatus(codes.Error, "context cancelled")
			s.span.End()
		}
		return
	}
}
`))

// Client streaming interface
var _ = template.Must(FileTemplate.New("clientStreamingInterface").Parse(`
// {{streamType .ServiceName .Name "Client"}} is the client-side streaming interface for {{.Name}}
type {{streamType .ServiceName .Name "Client"}} interface {
	Send({{.InputType}}) error
	CloseAndRecv() ({{.OutputType}}, error)
}

type {{streamImplType .ServiceName .Name "Client"}} struct {
	nc          *nats.Conn
	streamID    string
	topic       string
	responseSub *nats.Subscription
	ctx         context.Context
	span        trace.Span
	seqNum      uint64
	sentCount   int
	mu          sync.Mutex
	closed      bool
}

func (s *{{streamImplType .ServiceName .Name "Client"}}) Send(msg {{.InputType}}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return fmt.Errorf("stream is closed")
	}

	data, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	header := nats.Header{}
	header.Set("Stream-ID", s.streamID)
	header.Set("Seq-Num", strconv.FormatUint(s.seqNum, 10))
	s.seqNum++
	s.sentCount++

	return s.nc.PublishMsg(&nats.Msg{
		Subject: s.topic,
		Data:    data,
		Header:  header,
	})
}

func (s *{{streamImplType .ServiceName .Name "Client"}}) CloseAndRecv() ({{.OutputType}}, error) {
	s.mu.Lock()
	s.closed = true
	sentCount := s.sentCount
	s.mu.Unlock()

	// Send close signal
	header := nats.Header{}
	header.Set("Stream-ID", s.streamID)

	if err := s.nc.PublishMsg(&nats.Msg{
		Subject: s.topic + ".close",
		Header:  header,
	}); err != nil {
		if tracingEnabled && s.span.IsRecording() {
			s.span.RecordError(err)
			s.span.SetStatus(codes.Error, "failed to close stream")
			s.span.End()
		}
		return nil, fmt.Errorf("failed to close stream: %w", err)
	}

	// Wait for final response
	msg, err := s.responseSub.NextMsgWithContext(s.ctx)
	if err != nil {
		if tracingEnabled && s.span.IsRecording() {
			s.span.RecordError(err)
			s.span.SetStatus(codes.Error, "failed to receive response")
			s.span.End()
		}
		return nil, fmt.Errorf("failed to receive response: %w", err)
	}

	// Check for error
	if errMsg := msg.Header.Get("X-Error"); errMsg != "" {
		err := fmt.Errorf("server error: %s", errMsg)
		if tracingEnabled && s.span.IsRecording() {
			s.span.RecordError(err)
			s.span.SetStatus(codes.Error, errMsg)
			s.span.End()
		}
		return nil, err
	}

	out := &{{.OutputTypeName}}{}
	if err := proto.Unmarshal(msg.Data, out); err != nil {
		if tracingEnabled && s.span.IsRecording() {
			s.span.RecordError(err)
			s.span.SetStatus(codes.Error, "failed to unmarshal response")
			s.span.End()
		}
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if tracingEnabled && s.span.IsRecording() {
		s.span.SetAttributes(attribute.Int("stream.messages_sent", sentCount))
		s.span.SetStatus(codes.Ok, "")
		s.span.End()
	}

	s.responseSub.Unsubscribe()
	return out, nil
}

// Server-side interface for client streaming
type {{streamType .ServiceName .Name "Server"}} interface {
	Recv() ({{.InputType}}, error)
	SendAndClose({{.OutputType}}) error
}

type {{streamImplType .ServiceName .Name "Server"}} struct {
	messages []{{.InputType}}
	index    int
	response {{.OutputType}}
	mu       sync.Mutex
}

func (s *{{streamImplType .ServiceName .Name "Server"}}) Recv() ({{.InputType}}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.index >= len(s.messages) {
		return nil, io.EOF
	}

	msg := s.messages[s.index]
	s.index++
	return msg, nil
}

func (s *{{streamImplType .ServiceName .Name "Server"}}) SendAndClose(msg {{.OutputType}}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.response = msg
	return nil
}

// Aggregator for client streaming
type {{.ServiceName}}_{{.Name}}_Aggregator struct {
	nc      *nats.Conn
	srv     {{.ServiceName}}Server
	streams map[string]*{{.ServiceName}}_{{.Name}}_StreamBuffer
	mu      sync.RWMutex
}

type {{.ServiceName}}_{{.Name}}_StreamBuffer struct {
	messages        []{{.InputType}}
	responseSubject string
	mu              sync.Mutex
}
`))

// Bidirectional streaming interface
var _ = template.Must(FileTemplate.New("bidirectionalStreamingInterface").Parse(`
// {{streamType .ServiceName .Name "Client"}} is the bidirectional streaming interface for {{.Name}}
type {{streamType .ServiceName .Name "Client"}} interface {
	Send({{.InputType}}) error
	Recv() ({{.OutputType}}, error)
	CloseSend() error
}

type {{streamImplType .ServiceName .Name "Client"}} struct {
	nc               *nats.Conn
	streamID         string
	topic            string
	recvCh           chan {{.OutputType}}
	errCh            chan error
	sub              *nats.Subscription
	ctx              context.Context
	span             trace.Span
	seqNum           uint64
	lastRecvSeq      uint64
	mu               sync.Mutex
	closed           bool
	reconnecting     bool
	lastActivityNano int64 // atomic access only
	stopMonitor      chan struct{}
}

func (s *{{streamImplType .ServiceName .Name "Client"}}) Send(msg {{.InputType}}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return fmt.Errorf("stream is closed")
	}

	data, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	header := nats.Header{}
	header.Set("Stream-ID", s.streamID)
	header.Set("Seq-Num", strconv.FormatUint(s.seqNum, 10))
	s.seqNum++

	return s.nc.PublishMsg(&nats.Msg{
		Subject: s.topic + ".in",
		Data:    data,
		Header:  header,
	})
}

func (s *{{streamImplType .ServiceName .Name "Client"}}) Recv() ({{.OutputType}}, error) {
	select {
	case msg, ok := <-s.recvCh:
		if !ok {
			return nil, io.EOF
		}
		return msg, nil
	case err := <-s.errCh:
		return nil, err
	case <-s.ctx.Done():
		return nil, s.ctx.Err()
	}
}

func (s *{{streamImplType .ServiceName .Name "Client"}}) CloseSend() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true

	// Stop health monitor
	if s.stopMonitor != nil {
		close(s.stopMonitor)
	}

	header := nats.Header{}
	header.Set("Stream-ID", s.streamID)

	s.nc.PublishMsg(&nats.Msg{
		Subject: s.topic + ".close",
		Header:  header,
	})

	if tracingEnabled && s.span.IsRecording() {
		s.span.SetStatus(codes.Ok, "")
		s.span.End()
	}

	s.sub.Unsubscribe()
	close(s.recvCh)
	return nil
}

func (s *{{streamImplType .ServiceName .Name "Client"}}) reconnect() error {
	s.mu.Lock()
	if s.closed || s.reconnecting {
		s.mu.Unlock()
		return nil
	}
	s.reconnecting = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.reconnecting = false
		s.mu.Unlock()
	}()

	// Resubscribe to output channel
	if s.sub != nil {
		s.sub.Unsubscribe()
	}

	sub, err := s.nc.Subscribe(s.streamID+".out", func(msg *nats.Msg) {
		s.handleRecvMessage(msg)
	})
	if err != nil {
		return fmt.Errorf("failed to resubscribe: %w", err)
	}
	s.sub = sub

	// Reinitialize stream with resume sequence
	header := nats.Header{}
	header.Set("Stream-ID", s.streamID)
	header.Set("Resume-From-Seq", strconv.FormatUint(s.lastRecvSeq, 10))

	if err := s.nc.PublishMsg(&nats.Msg{
		Subject: s.topic + ".init",
		Header:  header,
	}); err != nil {
		return fmt.Errorf("failed to reinitialize stream: %w", err)
	}

	return nil
}

func (s *{{streamImplType .ServiceName .Name "Client"}}) startHealthMonitor() {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-s.stopMonitor:
				return
			case <-s.ctx.Done():
				return
			case <-ticker.C:
				// Check if we've received any activity in the last 30 seconds
				lastActivity := atomic.LoadInt64(&s.lastActivityNano)
				if lastActivity > 0 {
					elapsed := time.Since(time.Unix(0, lastActivity))
					if elapsed > 30*time.Second {
						// No activity for 30 seconds, attempt reconnection
						if err := s.reconnect(); err != nil {
							// If reconnection fails, report error
							select {
							case s.errCh <- fmt.Errorf("auto-reconnect failed: %w", err):
							default:
							}
						}
					}
				}
			}
		}
	}()
}

func (s *{{streamImplType .ServiceName .Name "Client"}}) handleRecvMessage(msg *nats.Msg) {
	// Update activity timestamp atomically (lock-free)
	atomic.StoreInt64(&s.lastActivityNano, time.Now().UnixNano())

	// Check for EOF
	if msg.Header.Get("Stream-EOF") == "true" {
		if tracingEnabled {
			s.span.SetStatus(codes.Ok, "")
			s.span.End()
		}
		close(s.recvCh)
		return
	}

	// Check for error
	if errMsg := msg.Header.Get("X-Error"); errMsg != "" {
		err := fmt.Errorf("stream error: %s", errMsg)
		if tracingEnabled {
			s.span.RecordError(err)
			s.span.SetStatus(codes.Error, errMsg)
			s.span.End()
		}
		select {
		case s.errCh <- err:
		default:
		}
		close(s.recvCh)
		return
	}

	out := &{{.OutputTypeName}}{}
	if err := proto.Unmarshal(msg.Data, out); err != nil {
		if tracingEnabled {
			s.span.RecordError(err)
		}
		select {
		case s.errCh <- fmt.Errorf("unmarshal error: %w", err):
		default:
		}
		return
	}

	// Track sequence number
	if seqStr := msg.Header.Get("Seq-Num"); seqStr != "" {
		if seq, err := strconv.ParseUint(seqStr, 10, 64); err == nil {
			s.mu.Lock()
			s.lastRecvSeq = seq + 1
			s.mu.Unlock()
		}
	}

	select {
	case s.recvCh <- out:
	case <-s.ctx.Done():
		if tracingEnabled {
			s.span.SetStatus(codes.Error, "context cancelled")
			s.span.End()
		}
		return
	}
}

// Server-side interface for bidirectional streaming
type {{streamType .ServiceName .Name "Server"}} interface {
	Send({{.OutputType}}) error
	Recv() ({{.InputType}}, error)
}

type {{streamImplType .ServiceName .Name "Server"}} struct {
	nc          *nats.Conn
	streamID    string
	recvCh      chan {{.InputType}}
	sendSubject string
	seqNum      uint64
	mu          sync.Mutex
	closed      bool
}

func (s *{{streamImplType .ServiceName .Name "Server"}}) Send(msg {{.OutputType}}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return fmt.Errorf("stream is closed")
	}

	data, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Add sequence number for tracking
	header := nats.Header{}
	header.Set("Seq-Num", strconv.FormatUint(s.seqNum, 10))
	s.seqNum++

	return s.nc.PublishMsg(&nats.Msg{
		Subject: s.sendSubject,
		Data:    data,
		Header:  header,
	})
}

func (s *{{streamImplType .ServiceName .Name "Server"}}) Recv() ({{.InputType}}, error) {
	msg, ok := <-s.recvCh
	if !ok {
		return nil, io.EOF
	}
	return msg, nil
}

func (s *{{streamImplType .ServiceName .Name "Server"}}) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true

	// Send EOF
	header := nats.Header{}
	header.Set("Stream-EOF", "true")
	return s.nc.PublishMsg(&nats.Msg{
		Subject: s.sendSubject,
		Header:  header,
	})
}

// Manager for bidirectional streaming
type {{.ServiceName}}_{{.Name}}_Manager struct {
	nc      *nats.Conn
	srv     {{.ServiceName}}Server
	streams map[string]*{{.ServiceName}}_{{.Name}}_ServerStream
	mu      sync.RWMutex
}

type {{.ServiceName}}_{{.Name}}_ServerStream = {{streamImplType .ServiceName .Name "Server"}}
`))
