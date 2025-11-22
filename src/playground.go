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

// servicePlayground provides a web interface for testing {{.Name}} methods
type servicePlayground struct {
	nc     *nats.Conn
	server *http.Server
	mu     sync.Mutex
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
		nc: nc,
	}
	
	mux := http.NewServeMux()
	mux.HandleFunc("/", pg.handleUI)
	mux.HandleFunc("/api/methods", pg.handleListMethods)
	mux.HandleFunc("/api/invoke", pg.handleInvoke)
	mux.HandleFunc("/api/stream", pg.handleStream)
	
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
			StreamType: "{{if isUnary .}}UNARY{{else if isServerStreaming .}}SERVER_STREAMING{{else if isClientStreaming .}}CLIENT_STREAMING{{else}}BIDIRECTIONAL{{end}}",
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
	
	// Marshal to protobuf
	data, err := proto.Marshal(inputMsg)
	if err != nil {
		json.NewEncoder(w).Encode(&{{.ServiceName}}InvokeResponse{
			Success: false,
			Error:   fmt.Sprintf("Marshal error: %v", err),
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
	
	// Create NATS message with headers
	msg := &nats.Msg{
		Subject: "{{.Topic}}",
		Data:    data,
		Header:  make(nats.Header),
	}
	
	for k, v := range req.Headers {
		msg.Header.Set(k, v)
	}
	
	// Invoke RPC
	start := time.Now()
	respMsg, err := pg.nc.RequestMsgWithContext(ctx, msg)
	duration := time.Since(start)
	
	if err != nil {
		json.NewEncoder(w).Encode(&{{.ServiceName}}InvokeResponse{
			Success:    false,
			Error:      err.Error(),
			DurationMs: duration.Milliseconds(),
		})
		return
	}
	
	// Check for error in headers
	if errMsg := respMsg.Header.Get("X-Error"); errMsg != "" {
		json.NewEncoder(w).Encode(&{{.ServiceName}}InvokeResponse{
			Success:    false,
			Error:      errMsg,
			Headers:    headerToMap(respMsg.Header),
			DurationMs: duration.Milliseconds(),
		})
		return
	}
	
	// Unmarshal response
	outputMsg := &{{.OutputTypeName}}{}
	if err := proto.Unmarshal(respMsg.Data, outputMsg); err != nil {
		json.NewEncoder(w).Encode(&{{.ServiceName}}InvokeResponse{
			Success:    false,
			Error:      fmt.Sprintf("Unmarshal error: %v", err),
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
		Headers:    headerToMap(respMsg.Header),
		DurationMs: duration.Milliseconds(),
	})
}
{{end}}
{{end}}

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
	
	data, err := proto.Marshal(inputMsg)
	if err != nil {
		sendSSE(w, flusher, "error", fmt.Sprintf("Marshal error: %v", err))
		return
	}
	
	// Create unique inbox
	inbox := nats.NewInbox()
	
	// Subscribe to responses
	sub, err := pg.nc.Subscribe(inbox, func(msg *nats.Msg) {
		// Check for EOF
		if msg.Header.Get("Stream-EOF") == "true" {
			sendSSE(w, flusher, "close", "Stream completed")
			return
		}
		
		// Check for error
		if errMsg := msg.Header.Get("X-Error"); errMsg != "" {
			sendSSE(w, flusher, "error", errMsg)
			return
		}
		
		// Unmarshal response
		outputMsg := &{{.OutputTypeName}}{}
		if err := proto.Unmarshal(msg.Data, outputMsg); err != nil {
			sendSSE(w, flusher, "error", fmt.Sprintf("Unmarshal error: %v", err))
			return
		}
		
		jsonData, err := protojson.Marshal(outputMsg)
		if err != nil {
			sendSSE(w, flusher, "error", fmt.Sprintf("JSON error: %v", err))
			return
		}
		
		sendSSE(w, flusher, "message", string(jsonData))
	})
	
	if err != nil {
		sendSSE(w, flusher, "error", fmt.Sprintf("Subscribe error: %v", err))
		return
	}
	defer sub.Unsubscribe()
	
	// Send request
	msg := &nats.Msg{
		Subject: "{{.Topic}}",
		Reply:   inbox,
		Data:    data,
		Header:  make(nats.Header),
	}
	
	for k, v := range req.Headers {
		msg.Header.Set(k, v)
	}
	
	if err := pg.nc.PublishMsg(msg); err != nil {
		sendSSE(w, flusher, "error", fmt.Sprintf("Publish error: %v", err))
		return
	}
	
	sendSSE(w, flusher, "started", "Stream started")
	
	// Wait for client disconnect or timeout
	timeout := time.Duration(req.Timeout) * time.Second
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	
	select {
	case <-r.Context().Done():
	case <-time.After(timeout):
		sendSSE(w, flusher, "error", "Timeout")
	}
}
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

{{template "playgroundHTML" .}}
`))

// Playground HTML template
var _ = template.Must(FileTemplate.New("playgroundHTML").Parse(fmt.Sprintf("const playgroundHTML = `%s`", playgroundHTML)))
