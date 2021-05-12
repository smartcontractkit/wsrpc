package wsrpc

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// ConnectOptions covers all relevant options for communicating with the server.
type ConnectOptions struct {
	// TransportCredentials stores the Authenticator required to setup a client
	// connection. Only one of TransportCredentials and CredsBundle is non-nil.
	TransportCredentials TransportCredentials
	// WriteBufferSize sets the size of write buffer which in turn determines how much data can be batched before it's written on the wire.
	WriteBufferSize int
	// ReadBufferSize sets the size of read buffer, which in turn determines how much data can be read at most for one read syscall.
	ReadBufferSize int
}

type ClientTransport interface {
	// Close tears down this transport. Once it returns, the transport
	// should not be accessed any more. The caller must make sure this
	// is called only once.
	Close() error

	// Write sends a message to the stream.
	Write(msg []byte) error

	// Read reads a message from the stream
	Read() <-chan []byte

	// Error returns a channel that is closed when some I/O error
	// happens. Typically the caller should have a goroutine to monitor
	// this in order to take action (e.g., close the current transport
	// and create a new one) in error case. It should not return nil
	// once the transport is initiated.
	Error() <-chan struct{}
}

// WebsocketClient implements the ClientTransport interface with websockets.
type WebsocketClient struct {
	ctx    context.Context
	cancel context.CancelFunc

	conn    *websocket.Conn // underlying communication channel
	onClose func()

	write chan []byte
	read  chan []byte
}

// NewWebsocketClient establishes the transport with the required ConnectOptions
// and returns it to the caller.
func NewWebsocketClient(ctx context.Context, addr string, opts ConnectOptions, onClose func()) (_ *WebsocketClient, err error) {
	ctx, cancel := context.WithCancel(ctx)
	defer func() {
		if err != nil {
			cancel()
		}
	}()

	d := websocket.Dialer{
		TLSClientConfig:  opts.TransportCredentials.Config,
		HandshakeTimeout: 45 * time.Second,
	}

	url := fmt.Sprintf("wss://%s", addr)
	conn, _, err := d.Dial(url, http.Header{})
	if err != nil {
		fmt.Println("error dialing", err)
		return nil, fmt.Errorf("transport: error while dialing %v", err)
	}

	c := &WebsocketClient{
		ctx:     ctx,
		cancel:  cancel,
		conn:    conn,
		onClose: onClose,
		write:   make(chan []byte), // Should this be buffered?
		read:    make(chan []byte), // Should this be buffered?
	}

	// Start go routines to establish the read/write channels
	go c.readPump()
	go c.writePump()

	return c, nil
}

func (c *WebsocketClient) Close() error {
	fmt.Println("Calling websocket client close")
	close(c.write)

	return c.conn.Close()
}

func (c *WebsocketClient) Write(msg []byte) error {
	c.write <- msg

	return nil
}

func (c *WebsocketClient) Error() <-chan struct{} {
	return c.ctx.Done()
}

func (c *WebsocketClient) Read() <-chan []byte {
	return c.read
}

// readPump pumps messages from the websocket connection.

// The application runs readPump in a per-connection goroutine. This ensures
// that there is at most one reader on a connection by executing all reads from
// this goroutine.
func (c *WebsocketClient) readPump() {
	defer func() {
		c.cancel()
	}()
	// Put this back in with confiugration
	// c.conn.SetReadLimit(maxMessageSize)
	// c.conn.SetReadDeadline(time.Now().Add(pongWait))
	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[Transport] Unexpected Close Error: %v", err)
			}
			break
		}
		c.read <- msg
	}
}

// writePump pumps messages from the client to the websocket connection.
//
// A goroutine running writePump is started for each connection. This ensures
// that there is at most one writer to a connection by executing all writes
// from this goroutine.
func (c *WebsocketClient) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		c.onClose()
		ticker.Stop()
		c.cancel()
	}()

	// Pong Reply Handler
	c.conn.SetPongHandler(func(string) error {
		fmt.Println("Pong Handler")
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		select {
		case msg, ok := <-c.write:
			// Closed the channel.
			if !ok {
				log.Println("[Transport] Sending close message to server")
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// Write the message
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			err := c.conn.WriteMessage(websocket.BinaryMessage, msg)
			if err != nil {
				log.Printf("[Transport] Error ocurred writing: %v", err)
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("[Transport] Error ocurred pinging: %v", err)
				return
			}
		}
	}
}
