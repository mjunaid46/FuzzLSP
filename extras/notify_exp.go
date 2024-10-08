package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/sourcegraph/jsonrpc2"
)

// Server represents the language server with the connection
type Server struct {
	conn *jsonrpc2.Conn
}

// Start initializes the server and sets up the connection
func (s *Server) Start(ctx context.Context) error {
	// Using os.Stdin and os.Stdout for the connection
	rwc := &stdioReadWriteCloser{os.Stdin, os.Stdout}

	// Create a new JSON-RPC 2 connection
	stream := jsonrpc2.NewBufferedStream(rwc, jsonrpc2.VSCodeObjectCodec{})
	s.conn = jsonrpc2.NewConn(ctx, stream, s) // Server is the handler
	return nil
}

// NotifyGeneratedCode sends a notification to the client
func (s *Server) NotifyGeneratedCode(ctx context.Context, generatedCode string) error {
	if s.conn == nil {
		return fmt.Errorf("connection is nil, cannot send notification")
	}

	// Send notification to the client
	err := s.conn.Notify(ctx, "someMethod", generatedCode)
	if err != nil {
		return fmt.Errorf("failed to send generated code notification: %v", err)
	}

	return nil
}

// Handle is required to implement the jsonrpc2.Handler interface
// It processes incoming requests and notifications from the client
func (s *Server) Handle(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
	// Here, you can handle incoming requests or notifications from the client
	// For example, you can log the request or respond to specific methods

	if req.Method == "someMethod" {
		// Handle the method (e.g., send a response or perform an action)
		fmt.Println("Received a request for someMethod")
	}

	// You can handle other methods or notifications as needed
}

// stdioReadWriteCloser implements io.ReadWriteCloser for stdin/stdout
type stdioReadWriteCloser struct {
	io.Reader
	io.Writer
}

func (stdioReadWriteCloser) Close() error {
	return nil // No close needed for stdio
}

func main() {
	server := &Server{}
	ctx := context.Background()

	// Start the language server
	if err := server.Start(ctx); err != nil {
		fmt.Printf("Error starting server: %v\n", err)
	}

	// Example notification (triggered elsewhere in a real-world scenario)
	generatedCode := "func hello() { fmt.Println(\"Hello, world!\") }"
	if err := server.NotifyGeneratedCode(ctx, generatedCode); err != nil {
		fmt.Printf("Error sending notification: %v\n", err)
	}
}
