// MIT License
//
// Copyright (c) 2026 GoAkt Team
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
//

package ws

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// wsTransport implements [sdkmcp.Transport] by returning an already-established
// WebSocket connection. This adapter bridges the gorilla/websocket connection
// into the MCP SDK's Transport/Connection model.
type wsTransport struct {
	conn *wsConn
}

// wsConn implements [sdkmcp.Connection] over a gorilla/websocket connection.
type wsConn struct {
	conn      *websocket.Conn
	sessionID string

	// writeMu serializes writes to the WebSocket connection. gorilla/websocket
	// supports concurrent readers but requires serialized writes.
	writeMu sync.Mutex

	closeOnce sync.Once
	done      chan struct{}
}

// Compile-time check that wsTransport satisfies the Transport interface.
var _ sdkmcp.Transport = (*wsTransport)(nil)

// Connect returns the pre-established WebSocket connection.
func (t *wsTransport) Connect(_ context.Context) (sdkmcp.Connection, error) {
	return t.conn, nil
}

// newWSConn wraps a WebSocket connection as an MCP Connection.
func newWSConn(conn *websocket.Conn, sessionID string) *wsConn {
	return &wsConn{
		conn:      conn,
		sessionID: sessionID,
		done:      make(chan struct{}),
	}
}

// SessionID returns the session identifier for this connection.
func (c *wsConn) SessionID() string {
	return c.sessionID
}

// Read blocks until a JSON-RPC message arrives on the WebSocket or the context
// is cancelled. Returns io.EOF when the connection is closed.
func (c *wsConn) Read(ctx context.Context) (jsonrpc.Message, error) {
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-c.done:
			return nil, io.EOF
		default:
		}

		// Short deadline so we can re-check ctx and done.
		_ = c.conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				return nil, io.EOF
			}
			if isTimeout(err) {
				continue
			}
			select {
			case <-c.done:
				return nil, io.EOF
			default:
			}
			return nil, err
		}

		msg, err := jsonrpc.DecodeMessage(data)
		if err != nil {
			return nil, err
		}
		return msg, nil
	}
}

// Write sends a JSON-RPC message over the WebSocket. Safe for concurrent use.
func (c *wsConn) Write(ctx context.Context, msg jsonrpc.Message) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	data, err := jsonrpc.EncodeMessage(msg)
	if err != nil {
		return err
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	// Re-check both conditions under the lock to avoid writing to a closed or
	// cancelled connection after encoding but before acquiring the mutex.
	if ctx.Err() != nil {
		return ctx.Err()
	}
	select {
	case <-c.done:
		return io.EOF
	default:
	}

	return c.conn.WriteMessage(websocket.TextMessage, data)
}

// Close terminates the WebSocket connection. Safe to call multiple times.
func (c *wsConn) Close() error {
	var closeErr error
	c.closeOnce.Do(func() {
		close(c.done)
		_ = c.conn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
			time.Now().Add(time.Second),
		)
		closeErr = c.conn.Close()
	})
	return closeErr
}

// isTimeout reports whether the error is a net.Error timeout.
func isTimeout(err error) bool {
	type timeouter interface {
		Timeout() bool
	}
	if te, ok := err.(timeouter); ok {
		return te.Timeout()
	}
	return false
}
