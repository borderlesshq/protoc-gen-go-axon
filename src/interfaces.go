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
	nc *nats.Conn
}

// New{{.Name}}Client creates a new client for {{.Name}}
func New{{.Name}}Client(nc *nats.Conn) {{.Name}}Client {
	return &{{clientType .Name}}{nc: nc}
}

{{range .Methods}}
{{template "clientMethod" .}}
{{end}}
`))

// Server registration template
var _ = template.Must(FileTemplate.New("serverRegistration").Parse(`
// Register{{.Name}}Server registers the service implementation with NATS.
func Register{{.Name}}Server(nc *nats.Conn, srv {{.Name}}Server) error {
{{range .Methods}}
	{{template "serverMethodRegistration" .}}
{{end}}
	return nil
}
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

	messageCount := 0
	// Subscribe to stream responses
	sub, err := c.nc.Subscribe(inbox, func(msg *nats.Msg) {
		// Check for EOF signal
		if msg.Header.Get("Stream-EOF") == "true" {
			if tracingEnabled {
				span.SetAttributes(attribute.Int("stream.messages_received", messageCount))
				span.SetStatus(codes.Ok, "")
				span.End()
			}
			close(recvCh)
			return
		}

		// Check for error
		if errMsg := msg.Header.Get("X-Error"); errMsg != "" {
			err := fmt.Errorf("stream error: %s", errMsg)
			if tracingEnabled {
				span.RecordError(err)
				span.SetStatus(codes.Error, errMsg)
				span.End()
			}
			select {
			case errCh <- err:
			default:
			}
			close(recvCh)
			return
		}

		out := &{{.OutputTypeName}}{}
		if err := proto.Unmarshal(msg.Data, out); err != nil {
			if tracingEnabled {
				span.RecordError(err)
			}
			select {
			case errCh <- fmt.Errorf("unmarshal error: %w", err):
			default:
			}
			return
		}

		messageCount++
		select {
		case recvCh <- out:
		case <-ctx.Done():
			if tracingEnabled {
				span.SetStatus(codes.Error, "context cancelled")
				span.End()
			}
			return
		}
	})
	if err != nil {
		if tracingEnabled {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to subscribe")
			span.End()
		}
		return nil, fmt.Errorf("failed to subscribe: %w", err)
	}

	// Create request message
	msg := &nats.Msg{
		Subject: "{{.Topic}}",
		Reply:   inbox,
		Data:    data,
		Header:  make(nats.Header),
	}

	// Inject trace context
	injectTraceContext(ctx, msg.Header)

	// Add custom headers
	for k, v := range callOpts.headers {
		msg.Header.Set(k, v)
	}

	// Send initial request with reply-to inbox
	if err := c.nc.PublishMsg(msg); err != nil {
		sub.Unsubscribe()
		if tracingEnabled {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to publish request")
			span.End()
		}
		return nil, fmt.Errorf("failed to publish request: %w", err)
	}

	stream := &{{streamImplType .ServiceName .Name "Client"}}{
		nc:     c.nc,
		sub:    sub,
		recvCh: recvCh,
		errCh:  errCh,
		ctx:    ctx,
		span:   span,
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

	// Subscribe to receive stream
	sub, err := c.nc.Subscribe(streamID+".out", func(msg *nats.Msg) {
		// Check for EOF
		if msg.Header.Get("Stream-EOF") == "true" {
			if tracingEnabled {
				span.SetStatus(codes.Ok, "")
				span.End()
			}
			close(recvCh)
			return
		}

		// Check for error
		if errMsg := msg.Header.Get("X-Error"); errMsg != "" {
			err := fmt.Errorf("stream error: %s", errMsg)
			if tracingEnabled {
				span.RecordError(err)
				span.SetStatus(codes.Error, errMsg)
				span.End()
			}
			select {
			case errCh <- err:
			default:
			}
			close(recvCh)
			return
		}

		out := &{{.OutputTypeName}}{}
		if err := proto.Unmarshal(msg.Data, out); err != nil {
			if tracingEnabled {
				span.RecordError(err)
			}
			select {
			case errCh <- fmt.Errorf("unmarshal error: %w", err):
			default:
			}
			return
		}

		select {
		case recvCh <- out:
		case <-ctx.Done():
			if tracingEnabled {
				span.SetStatus(codes.Error, "context cancelled")
				span.End()
			}
			return
		}
	})
	if err != nil {
		if tracingEnabled {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to subscribe")
			span.End()
		}
		return nil, fmt.Errorf("failed to subscribe: %w", err)
	}

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

	return &{{streamImplType .ServiceName .Name "Client"}}{
		nc:       c.nc,
		streamID: streamID,
		topic:    "{{.Topic}}",
		recvCh:   recvCh,
		errCh:    errCh,
		sub:      sub,
		ctx:      ctx,
		span:     span,
		seqNum:   0,
	}, nil
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

		stream := &{{streamImplType .ServiceName .Name "Server"}}{
			nc:    nc,
			reply: msg.Reply,
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
		
		stream := &{{.ServiceName}}_{{.Name}}_ServerStream{
			nc:          nc,
			streamID:    streamID,
			recvCh:      make(chan {{.InputType}}, 10),
			sendSubject: streamID + ".out",
		}

		streamManager.mu.Lock()
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
	nc    *nats.Conn
	reply string
	mu    sync.Mutex
}

func (s *{{streamImplType .ServiceName .Name "Server"}}) Send(msg {{.OutputType}}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	return s.nc.Publish(s.reply, data)
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
	nc     *nats.Conn
	sub    *nats.Subscription
	recvCh chan {{.OutputType}}
	errCh  chan error
	ctx    context.Context
	span   trace.Span
	mu     sync.Mutex
	closed bool
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
	
	if tracingEnabled && s.span.IsRecording() {
		s.span.End()
	}
	
	return s.sub.Unsubscribe()
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
	nc       *nats.Conn
	streamID string
	topic    string
	recvCh   chan {{.OutputType}}
	errCh    chan error
	sub      *nats.Subscription
	ctx      context.Context
	span     trace.Span
	seqNum   uint64
	mu       sync.Mutex
	closed   bool
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

	return s.nc.Publish(s.sendSubject, data)
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
