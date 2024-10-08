package jsonrpc

import (
	"context"
	"fmt"
	"io"

	"github.com/TobiasYin/go-lsp/logs"
	jsoniter "github.com/json-iterator/go"
)

type ReaderWriter interface {
	io.Reader
	io.Writer
	io.Closer
}
type CloserReader interface {
	io.Reader
	io.Closer
}
type fakeCloseReader struct {
	io.Reader
}

func (f *fakeCloseReader) Close() error {
	return nil
}

func NewFakeCloserReader(r io.Reader) CloserReader {
	return &fakeCloseReader{r}
}

type CloserWriter interface {
	io.Writer
	io.Closer
}

type fakeCloseWriter struct {
	io.Writer
}

func (f *fakeCloseWriter) Close() error {
	return nil
}

func NewFakeCloserWriter(w io.Writer) CloserWriter {
	return &fakeCloseWriter{w}
}

// The connection of rpc, not limited to net.Conn
type Conn struct {
	reader CloserReader
	writer CloserWriter
}

func NewNotCloseConn(reader io.Reader, writer io.Writer) *Conn {
	return &Conn{reader: NewFakeCloserReader(reader), writer: NewFakeCloserWriter(writer)}
}

func NewConn(reader CloserReader, writer CloserWriter) *Conn {
	// logs.Println("Making Connection: ", reader, writer)
	return &Conn{reader: reader, writer: writer}
}

func (c *Conn) Write(p []byte) (n int, err error) {
	return c.writer.Write(p)
}
func (c *Conn) Read(p []byte) (n int, err error) {
	return c.reader.Read(p)
}

func (c *Conn) Close() error {
	var err1 = c.reader.Close()
	var err2 = c.reader.Close()
	if err1 == nil && err2 == nil {
		return nil
	}
	if err1 == nil {
		return err2
	}
	if err2 == nil {
		return err1
	}
	return fmt.Errorf("two errors, err1: %v, err2: %v", err1, err2)
}

func (s *Conn) Notify(ctx context.Context, method string, params interface{}) error {

	// Create the notification message
	notifyMsg := NotificationMessage{
		BaseMessage: BaseMessage{
			Jsonrpc: "2.0",
		},
		Method: method,
	}
	// logs.Println("Notify Message: ", notifyMsg)
	// Marshal the params into JSON
	if params != nil {
		jsonParams, err := jsoniter.Marshal(params)
		if err != nil {
			return err
		}
		notifyMsg.Params = jsonParams
	}

	// Convert the notification message to JSON
	notifyData, err := jsoniter.Marshal(notifyMsg)
	if err != nil {
		return err
	}
	// logs.Println("Notify Data: ", notifyData)
	
	// Calculate the content length
	totalLen := len(notifyData)
	
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", totalLen)
	message := []byte(header + string(notifyData))

	// Send the message
	_, err = s.Write(message)
	if err != nil {
		return err
	}

	logs.Println("Message sent successfully.")
	return nil
}
