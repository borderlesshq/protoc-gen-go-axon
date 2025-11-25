// templates.go - Code generation templates including playground
package src

import (
	"fmt"
	"text/template"
)

// Main file template - this is executed first and includes all others

// Playground support template - generates EnablePlayground() method for each service
var _ = template.Must(FileTemplate.New("playgroundSupport").Parse(`
// ===============================================
// Playground Support for {{.Name}}
// ===============================================

// streamSession represents an active streaming session
type streamSession struct {
	streamID    string
	method      string
	clientStream interface{}  // Stores the generated client stream object
	bidiStream  interface{}   // Stores bidirectional stream object
	closeCh     chan struct{}
	sub         *nats.Subscription
	createdAt   time.Time
}

// servicePlayground provides a web interface for testing {{.Name}} methods
type servicePlayground struct {
	nc            *nats.Conn
	client        {{.Name}}Client
	server        *http.Server
	mu            sync.Mutex
	streamSessions map[string]*streamSession
}

// EnablePlayground starts an HTTP server with a web UI for testing this service
//
// Example:
//   playground := cards.EnablePlayground(nc, ":8080")
//   defer playground.Shutdown(context.Background())
//
// Then open http://localhost:8080 in your browser
func enablePlayground(nc *nats.Conn, addr string) *servicePlayground {
	pg := &servicePlayground{
		nc:            nc,
		client:        New{{.Name}}Client(nc),
		streamSessions: make(map[string]*streamSession),
	}
	
	mux := http.NewServeMux()
	mux.HandleFunc("/", pg.handleUI)
	mux.HandleFunc("/api/methods", pg.handleListMethods)
	mux.HandleFunc("/api/invoke", pg.handleInvoke)
{{if hasServerStreaming .Methods}}
	mux.HandleFunc("/api/stream", pg.handleStream)
{{end}}
{{if hasClientStreaming .Methods}}
	mux.HandleFunc("/api/client-stream/init", pg.handleClientStreamInit)
	mux.HandleFunc("/api/client-stream/send", pg.handleClientStreamSend)
	mux.HandleFunc("/api/client-stream/close", pg.handleClientStreamClose)
{{end}}
{{if hasBidirectional .Methods}}
	mux.HandleFunc("/api/bidi-stream", pg.handleBidiStream)
	mux.HandleFunc("/api/bidi-stream/send", pg.handleBidiStreamSend)
	mux.HandleFunc("/api/bidi-stream/close", pg.handleBidiStreamClose)
{{end}}
	
	pg.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	
	return pg
}

// Shutdown gracefully shuts down the playground server
func (pg *servicePlayground) Shutdown(ctx context.Context) error {
	pg.mu.Lock()
	defer pg.mu.Unlock()
	
	if pg.server != nil {
		return pg.server.Shutdown(ctx)
	}
	return nil
}

// handleUI serves the playground HTML interface
func (pg *servicePlayground) handleUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(playgroundHTML))
}

// MethodInfo describes a method for the playground
type {{.Name}}MethodInfo struct {
	Name       string ` + "`" + `json:"name"` + "`" + `
	StreamType string ` + "`" + `json:"streamType"` + "`" + `
	InputType  string ` + "`" + `json:"inputType"` + "`" + `
	OutputType string ` + "`" + `json:"outputType"` + "`" + `
	Topic      string ` + "`" + `json:"topic"` + "`" + `
}

// handleListMethods returns all methods for this service
func (pg *servicePlayground) handleListMethods(w http.ResponseWriter, r *http.Request) {
	methods := []{{.Name}}MethodInfo{
{{range .Methods}}
		{
			Name:       "{{.Name}}",
			StreamType: "{{if isUnary .}}UNARY{{else if isServerStreaming .}}SERVER_STREAMING{{else if isClientStreaming .}}CLIENT_STREAMING{{else}}BIDI_STREAMING{{end}}",
			InputType:  "{{.InputTypeName}}",
			OutputType: "{{.OutputTypeName}}",
			Topic:      "{{.Topic}}",
		},
{{end}}
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"service": "{{.Name}}",
		"methods": methods,
	})
}

type {{.Name}}InvokeRequest struct {
	Method  string            ` + "`" + `json:"method"` + "`" + `
	Payload json.RawMessage   ` + "`" + `json:"payload"` + "`" + `
	Headers map[string]string ` + "`" + `json:"headers"` + "`" + `
	Timeout int               ` + "`" + `json:"timeout"` + "`" + `
}

type {{.Name}}InvokeResponse struct {
	Success    bool              ` + "`" + `json:"success"` + "`" + `
	Response   json.RawMessage   ` + "`" + `json:"response,omitempty"` + "`" + `
	Error      string            ` + "`" + `json:"error,omitempty"` + "`" + `
	Headers    map[string]string ` + "`" + `json:"headers,omitempty"` + "`" + `
	DurationMs int64             ` + "`" + `json:"durationMs"` + "`" + `
}

// handleInvoke handles unary RPC invocations from the playground
func (pg *servicePlayground) handleInvoke(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	var req {{.Name}}InvokeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	
	// Route to the appropriate method handler
	switch req.Method {
{{range .Methods}}
{{if isUnary .}}
	case "{{.Name}}":
		pg.handle{{.Name}}Invoke(w, &req)
{{end}}
{{end}}
	default:
		http.Error(w, "Method not found", http.StatusNotFound)
	}
}

{{range .Methods}}
{{if isUnary .}}
// handle{{.Name}}Invoke executes the {{.Name}} method
func (pg *servicePlayground) handle{{.Name}}Invoke(w http.ResponseWriter, req *{{.ServiceName}}InvokeRequest) {
	// Create input message
	inputMsg := &{{.InputTypeName}}{}
	if err := protojson.Unmarshal(req.Payload, inputMsg); err != nil {
		json.NewEncoder(w).Encode(&{{.ServiceName}}InvokeResponse{
			Success: false,
			Error:   fmt.Sprintf("Invalid payload: %v", err),
		})
		return
	}

	// Set timeout
	timeout := time.Duration(req.Timeout) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Invoke RPC using generated client
	start := time.Now()
	outputMsg, err := pg.client.{{.Name}}(ctx, inputMsg)
	duration := time.Since(start)

	if err != nil {
		json.NewEncoder(w).Encode(&{{.ServiceName}}InvokeResponse{
			Success:    false,
			Error:      err.Error(),
			DurationMs: duration.Milliseconds(),
		})
		return
	}

	// Convert to JSON
	jsonData, err := protojson.Marshal(outputMsg)
	if err != nil {
		json.NewEncoder(w).Encode(&{{.ServiceName}}InvokeResponse{
			Success:    false,
			Error:      fmt.Sprintf("JSON conversion error: %v", err),
			DurationMs: duration.Milliseconds(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(&{{.ServiceName}}InvokeResponse{
		Success:    true,
		Response:   jsonData,
		DurationMs: duration.Milliseconds(),
	})
}
{{end}}
{{end}}

{{if hasServerStreaming .Methods}}
// handleStream handles streaming RPCs
func (pg *servicePlayground) handleStream(w http.ResponseWriter, r *http.Request) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}
	
	var req {{.Name}}InvokeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendSSE(w, flusher, "error", "Invalid request")
		return
	}
	
	// Route to appropriate stream handler
	switch req.Method {
{{range .Methods}}
{{if isServerStreaming .}}
	case "{{.Name}}":
		pg.handle{{.Name}}Stream(w, r, flusher, &req)
{{end}}
{{end}}
	default:
		sendSSE(w, flusher, "error", "Method not found or not a streaming method")
	}
}

{{range .Methods}}
{{if isServerStreaming .}}
// handle{{.Name}}Stream handles server streaming for {{.Name}}
func (pg *servicePlayground) handle{{.Name}}Stream(w http.ResponseWriter, r *http.Request, flusher http.Flusher, req *{{.ServiceName}}InvokeRequest) {
	// Create input message
	inputMsg := &{{.InputTypeName}}{}
	if err := protojson.Unmarshal(req.Payload, inputMsg); err != nil {
		sendSSE(w, flusher, "error", fmt.Sprintf("Invalid payload: %v", err))
		return
	}

	// Set timeout
	timeout := time.Duration(req.Timeout) * time.Second
	if timeout == 0 {
		timeout = 5 * time.Minute
	}

	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	// Call server streaming method using generated client
	stream, err := pg.client.{{.Name}}(ctx, inputMsg)
	if err != nil {
		sendSSE(w, flusher, "error", fmt.Sprintf("Stream initiation error: %v", err))
		return
	}
	defer stream.CloseSend()

	sendSSE(w, flusher, "started", "Stream started")

	// Receive messages from stream
	for {
		outputMsg, err := stream.Recv()
		if err == io.EOF {
			sendSSE(w, flusher, "close", "Stream completed")
			return
		}
		if err != nil {
			sendSSE(w, flusher, "error", err.Error())
			return
		}

		jsonData, err := protojson.Marshal(outputMsg)
		if err != nil {
			sendSSE(w, flusher, "error", fmt.Sprintf("JSON error: %v", err))
			return
		}

		sendSSE(w, flusher, "message", string(jsonData))
	}
}
{{end}}
{{end}}
{{end}}

// Helper functions
func sendSSE(w http.ResponseWriter, flusher http.Flusher, event, data string) {
	fmt.Fprintf(w, "event: %s\n", event)
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
}

func headerToMap(h nats.Header) map[string]string {
	m := make(map[string]string)
	for k, v := range h {
		if len(v) > 0 {
			m[k] = strings.Join(v, ", ")
		}
	}
	return m
}

{{if hasClientOrBidiStreaming .Methods}}
// Shared Streaming Types (used by both client and bidirectional streaming)

type {{.Name}}StreamSendRequest struct {
	StreamID string          ` + "`" + `json:"streamId"` + "`" + `
	Method   string          ` + "`" + `json:"method"` + "`" + `
	Payload  json.RawMessage ` + "`" + `json:"payload"` + "`" + `
}

type {{.Name}}StreamSendResponse struct {
	Success bool   ` + "`" + `json:"success"` + "`" + `
	Error   string ` + "`" + `json:"error,omitempty"` + "`" + `
}

type {{.Name}}StreamCloseRequest struct {
	StreamID string ` + "`" + `json:"streamId"` + "`" + `
	Method   string ` + "`" + `json:"method"` + "`" + `
}

type {{.Name}}StreamCloseResponse struct {
	Success  bool            ` + "`" + `json:"success"` + "`" + `
	Response json.RawMessage ` + "`" + `json:"response,omitempty"` + "`" + `
	Error    string          ` + "`" + `json:"error,omitempty"` + "`" + `
}
{{end}}

{{if hasClientStreaming .Methods}}
// Client Streaming Handlers

type {{.Name}}ClientStreamInitRequest struct {
	Method  string            ` + "`" + `json:"method"` + "`" + `
	Headers map[string]string ` + "`" + `json:"headers"` + "`" + `
	Timeout int               ` + "`" + `json:"timeout"` + "`" + `
}

type {{.Name}}ClientStreamInitResponse struct {
	Success  bool   ` + "`" + `json:"success"` + "`" + `
	StreamID string ` + "`" + `json:"streamId,omitempty"` + "`" + `
	Error    string ` + "`" + `json:"error,omitempty"` + "`" + `
}

func (pg *servicePlayground) handleClientStreamInit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req {{.Name}}ClientStreamInitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Route to appropriate init handler
	switch req.Method {
{{range .Methods}}
{{if isClientStreaming .}}
	case "{{.Name}}":
		pg.handle{{.Name}}ClientStreamInit(w, &req)
		return
{{end}}
{{end}}
	default:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(&{{.Name}}ClientStreamInitResponse{
			Success: false,
			Error:   "Method not found or not client streaming",
		})
	}
}

func (pg *servicePlayground) handleClientStreamSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req {{.Name}}StreamSendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	pg.mu.Lock()
	session, exists := pg.streamSessions[req.StreamID]
	pg.mu.Unlock()

	if !exists {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(&{{.Name}}StreamSendResponse{
			Success: false,
			Error:   "Stream not found",
		})
		return
	}

	// Route to appropriate send handler
	switch req.Method {
{{range .Methods}}
{{if isClientStreaming .}}
	case "{{.Name}}":
		pg.handle{{.Name}}ClientStreamSend(w, session, req.Payload)
		return
{{end}}
{{end}}
	default:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(&{{.Name}}StreamSendResponse{
			Success: false,
			Error:   "Method not found or not client streaming",
		})
	}
}

func (pg *servicePlayground) handleClientStreamClose(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req {{.Name}}StreamCloseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	pg.mu.Lock()
	session, exists := pg.streamSessions[req.StreamID]
	pg.mu.Unlock()

	if !exists {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(&{{.Name}}StreamCloseResponse{
			Success: false,
			Error:   "Stream not found",
		})
		return
	}

	// Cleanup session after handler completes
	defer func() {
		pg.mu.Lock()
		delete(pg.streamSessions, req.StreamID)
		pg.mu.Unlock()
	}()

	// Route to appropriate handler
	switch req.Method {
{{range .Methods}}
{{if isClientStreaming .}}
	case "{{.Name}}":
		pg.handle{{.Name}}ClientStreamClose(w, session)
		return
{{end}}
{{end}}
	default:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(&{{.Name}}StreamCloseResponse{
			Success: false,
			Error:   "Method not found or not client streaming",
		})
	}
}

{{range .Methods}}
{{if isClientStreaming .}}
// handle{{.Name}}ClientStreamInit initializes a client stream for {{.Name}}
func (pg *servicePlayground) handle{{.Name}}ClientStreamInit(w http.ResponseWriter, req *{{.ServiceName}}ClientStreamInitRequest) {
	// Set timeout
	timeout := time.Duration(req.Timeout) * time.Second
	if timeout == 0 {
		timeout = 5 * time.Minute
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	// Don't cancel immediately - store it in session for later cleanup

	// Create client stream using generated client
	stream, err := pg.client.{{.Name}}(ctx)
	if err != nil {
		cancel()
		json.NewEncoder(w).Encode(&{{.ServiceName}}ClientStreamInitResponse{
			Success: false,
			Error:   fmt.Sprintf("Stream initiation error: %v", err),
		})
		return
	}

	// Generate stream ID
	streamID := nats.NewInbox()

	// Create session to store the stream
	session := &streamSession{
		streamID:     streamID,
		method:       req.Method,
		clientStream: stream,
		closeCh:      make(chan struct{}),
		createdAt:    time.Now(),
	}

	pg.mu.Lock()
	pg.streamSessions[streamID] = session
	pg.mu.Unlock()

	// Store cancel function for cleanup
	go func() {
		<-session.closeCh
		cancel()
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(&{{.ServiceName}}ClientStreamInitResponse{
		Success:  true,
		StreamID: streamID,
	})
}

// handle{{.Name}}ClientStreamSend sends a message to the client stream
func (pg *servicePlayground) handle{{.Name}}ClientStreamSend(w http.ResponseWriter, session *streamSession, payload json.RawMessage) {
	// Get the stream from session
	stream, ok := session.clientStream.({{.ServiceName}}_{{.Name}}Client)
	if !ok {
		json.NewEncoder(w).Encode(&{{.ServiceName}}StreamSendResponse{
			Success: false,
			Error:   "Invalid stream type",
		})
		return
	}

	// Unmarshal JSON to proto
	inputMsg := &{{.InputTypeName}}{}
	if err := protojson.Unmarshal(payload, inputMsg); err != nil {
		json.NewEncoder(w).Encode(&{{.ServiceName}}StreamSendResponse{
			Success: false,
			Error:   fmt.Sprintf("Invalid payload: %v", err),
		})
		return
	}

	// Send message using generated client stream
	if err := stream.Send(inputMsg); err != nil {
		json.NewEncoder(w).Encode(&{{.ServiceName}}StreamSendResponse{
			Success: false,
			Error:   fmt.Sprintf("Send error: %v", err),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(&{{.ServiceName}}StreamSendResponse{
		Success: true,
	})
}

// handle{{.Name}}ClientStreamClose closes the client stream and receives final response
func (pg *servicePlayground) handle{{.Name}}ClientStreamClose(w http.ResponseWriter, session *streamSession) {
	// Get the stream from session
	stream, ok := session.clientStream.({{.ServiceName}}_{{.Name}}Client)
	if !ok {
		json.NewEncoder(w).Encode(&{{.ServiceName}}StreamCloseResponse{
			Success: false,
			Error:   "Invalid stream type",
		})
		return
	}

	// Close and receive final response
	outputMsg, err := stream.CloseAndRecv()
	if err != nil {
		json.NewEncoder(w).Encode(&{{.ServiceName}}StreamCloseResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	// Convert to JSON
	jsonData, err := protojson.Marshal(outputMsg)
	if err != nil {
		json.NewEncoder(w).Encode(&{{.ServiceName}}StreamCloseResponse{
			Success: false,
			Error:   fmt.Sprintf("JSON error: %v", err),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(&{{.ServiceName}}StreamCloseResponse{
		Success:  true,
		Response: jsonData,
	})

	// Signal cleanup
	close(session.closeCh)
}
{{end}}
{{end}}
{{end}}

{{if hasBidirectional .Methods}}
// Bidirectional Streaming Handlers

func (pg *servicePlayground) handleBidiStream(w http.ResponseWriter, r *http.Request) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	method := r.URL.Query().Get("method")
	if method == "" {
		sendSSE(w, flusher, "error", "Method required")
		return
	}

	// Generate stream ID
	streamID := nats.NewInbox()

	// Create session
	session := &streamSession{
		streamID:  streamID,
		method:    method,
		closeCh:   make(chan struct{}),
		createdAt: time.Now(),
	}

	pg.mu.Lock()
	pg.streamSessions[streamID] = session
	pg.mu.Unlock()

	defer func() {
		pg.mu.Lock()
		delete(pg.streamSessions, streamID)
		pg.mu.Unlock()
	}()

	// Route to appropriate handler
	switch method {
{{range .Methods}}
{{if isBidirectional .}}
	case "{{.Name}}":
		pg.handle{{.Name}}BidiStream(w, r, flusher, session)
		return
{{end}}
{{end}}
	}

	sendSSE(w, flusher, "error", "Method not found or not bidirectional")
}

{{range .Methods}}
{{if isBidirectional .}}
// handle{{.Name}}BidiStream handles bidirectional streaming for {{.Name}}
func (pg *servicePlayground) handle{{.Name}}BidiStream(w http.ResponseWriter, r *http.Request, flusher http.Flusher, session *streamSession) {
	// Set timeout
	timeout := 5 * time.Minute
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	// Create bidirectional stream using generated client
	stream, err := pg.client.{{.Name}}(ctx)
	if err != nil {
		sendSSE(w, flusher, "error", fmt.Sprintf("Stream initiation error: %v", err))
		return
	}
	defer stream.CloseSend()

	// Store stream in session
	session.bidiStream = stream

	// Send stream ID to client
	sendSSE(w, flusher, "started", session.streamID)

	// Start goroutine to receive messages from server
	go func() {
		for {
			outputMsg, err := stream.Recv()
			if err == io.EOF {
				sendSSE(w, flusher, "close", "Stream completed")
				close(session.closeCh)
				return
			}
			if err != nil {
				sendSSE(w, flusher, "error", err.Error())
				close(session.closeCh)
				return
			}

			jsonData, err := protojson.Marshal(outputMsg)
			if err != nil {
				sendSSE(w, flusher, "error", fmt.Sprintf("JSON error: %v", err))
				continue
			}

			sendSSE(w, flusher, "message", string(jsonData))
		}
	}()

	// Wait for close or timeout
	select {
	case <-session.closeCh:
	case <-r.Context().Done():
	case <-time.After(timeout):
		sendSSE(w, flusher, "error", "Timeout")
	}
}
{{end}}
{{end}}

func (pg *servicePlayground) handleBidiStreamSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req {{.Name}}StreamSendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	pg.mu.Lock()
	session, exists := pg.streamSessions[req.StreamID]
	pg.mu.Unlock()

	if !exists {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(&{{.Name}}StreamSendResponse{
			Success: false,
			Error:   "Stream not found",
		})
		return
	}

	// Route to appropriate handler
	switch req.Method {
{{range .Methods}}
{{if isBidirectional .}}
	case "{{.Name}}":
		pg.handle{{.Name}}BidiStreamSend(w, session, req.Payload)
		return
{{end}}
{{end}}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(&{{.Name}}StreamSendResponse{
		Success: false,
		Error:   "Method not found or not bidirectional",
	})
}

{{range .Methods}}
{{if isBidirectional .}}
// handle{{.Name}}BidiStreamSend sends a message to the bidirectional stream
func (pg *servicePlayground) handle{{.Name}}BidiStreamSend(w http.ResponseWriter, session *streamSession, payload json.RawMessage) {
	// Get the stream from session
	stream, ok := session.bidiStream.({{.ServiceName}}_{{.Name}}Client)
	if !ok {
		json.NewEncoder(w).Encode(&{{.ServiceName}}StreamSendResponse{
			Success: false,
			Error:   "Invalid stream type",
		})
		return
	}

	// Unmarshal JSON to proto
	inputMsg := &{{.InputTypeName}}{}
	if err := protojson.Unmarshal(payload, inputMsg); err != nil {
		json.NewEncoder(w).Encode(&{{.ServiceName}}StreamSendResponse{
			Success: false,
			Error:   fmt.Sprintf("Invalid payload: %v", err),
		})
		return
	}

	// Send message using generated client stream
	if err := stream.Send(inputMsg); err != nil {
		json.NewEncoder(w).Encode(&{{.ServiceName}}StreamSendResponse{
			Success: false,
			Error:   fmt.Sprintf("Send error: %v", err),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(&{{.ServiceName}}StreamSendResponse{
		Success: true,
	})
}
{{end}}
{{end}}

func (pg *servicePlayground) handleBidiStreamClose(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req {{.Name}}StreamCloseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	pg.mu.Lock()
	session, exists := pg.streamSessions[req.StreamID]
	pg.mu.Unlock()

	if !exists {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(&{{.Name}}StreamCloseResponse{
			Success: false,
			Error:   "Stream not found",
		})
		return
	}

	// Cleanup session after handler completes
	defer func() {
		pg.mu.Lock()
		delete(pg.streamSessions, req.StreamID)
		pg.mu.Unlock()
	}()

	// Route to appropriate handler
	switch req.Method {
{{range .Methods}}
{{if isBidirectional .}}
	case "{{.Name}}":
		pg.handle{{.Name}}BidiStreamClose(w, session)
		return
{{end}}
{{end}}
	default:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(&{{.Name}}StreamCloseResponse{
			Success: false,
			Error:   "Method not found or not bidirectional",
		})
	}
}

{{range .Methods}}
{{if isBidirectional .}}
// handle{{.Name}}BidiStreamClose closes a bidirectional stream
func (pg *servicePlayground) handle{{.Name}}BidiStreamClose(w http.ResponseWriter, session *streamSession) {
	// Get the stream from session
	stream, ok := session.bidiStream.({{.ServiceName}}_{{.Name}}Client)
	if !ok {
		json.NewEncoder(w).Encode(&{{.ServiceName}}StreamCloseResponse{
			Success: false,
			Error:   "Invalid stream type",
		})
		return
	}

	// Close the send side of the stream
	if err := stream.CloseSend(); err != nil {
		json.NewEncoder(w).Encode(&{{.ServiceName}}StreamCloseResponse{
			Success: false,
			Error:   fmt.Sprintf("Close error: %v", err),
		})
		return
	}

	// Signal the SSE handler to stop
	close(session.closeCh)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(&{{.ServiceName}}StreamCloseResponse{
		Success: true,
	})
}
{{end}}
{{end}}
{{end}}

{{template "playgroundHTML" .}}
`))

// Playground HTML template
var _ = template.Must(FileTemplate.New("playgroundHTML").Parse(fmt.Sprintf("const playgroundHTML = `%s`", playgroundHTML)))
