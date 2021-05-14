package wsrpc

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/smartcontractkit/wsrpc/credentials"
)

// ConnectOptions covers all relevant options for communicating with the server.
type ConnectOptions struct {
	// TransportCredentials stores the Authenticator required to setup a client
	// connection.
	TransportCredentials credentials.TransportCredentials
}

type ClientTransport interface {
	// Close tears down this transport. Once it returns, the transport
	// should not be accessed any more.
	Close() error

	// Write sends a message to the stream.
	Write(msg []byte) error

	// Read reads a message from the stream
	Read() <-chan []byte
}

// WebsocketClient implements the ClientTransport interface with websockets.
type WebsocketClient struct {
	ctx context.Context

	conn    *websocket.Conn // underlying communication channel
	onClose func()          // Called when the transport closes itself

	// Communication channels
	write chan []byte
	read  chan []byte

	// A signal channel called when the reader encounters a websocket close error
	done chan struct{}
	// A signal channel called when the transport is closed
	interrupt chan struct{}
}

// NewWebsocketClient establishes the transport with the required ConnectOptions
// and returns it to the caller.
func NewWebsocketClient(ctx context.Context, addr string, opts ConnectOptions, onClose func()) (_ *WebsocketClient, err error) {
	d := websocket.Dialer{
		TLSClientConfig:  opts.TransportCredentials.Config,
		HandshakeTimeout: 45 * time.Second,
	}

	url := fmt.Sprintf("wss://%s", addr)
	conn, _, err := d.DialContext(ctx, url, http.Header{})
	if err != nil {
		return nil, fmt.Errorf("[Transport] error while dialing %w", err)
	}

	c := &WebsocketClient{
		ctx:       ctx,
		conn:      conn,
		onClose:   onClose,
		write:     make(chan []byte), // Should this be buffered?
		read:      make(chan []byte), // Should this be buffered?
		done:      make(chan struct{}),
		interrupt: make(chan struct{}),
	}

	// Start go routines to establish the read/write channels
	go c.start()

	return c, nil
}

// Close closes the websocket connection and cleans up pump goroutines.
func (c *WebsocketClient) Close() error {
	close(c.interrupt)

	return nil
}

// Read returns a channel which provides the messages as they are read
func (c *WebsocketClient) Read() <-chan []byte {
	return c.read
}

// Write writes a message the websocket connection
func (c *WebsocketClient) Write(msg []byte) error {
	c.write <- msg

	return nil
}

// start run readPump in a goroutine and waits on writePump.
func (c WebsocketClient) start() {
	defer c.Close()
	defer c.onClose()

	// Set up reader
	go c.readPump()

	c.writePump()
}

// readPump pumps messages from the websocket connection. When a websocket
// connection closure is detected, it closes the done channel to shutdown
// writePump.
//
// The application runs readPump in a per-connection goroutine. This ensures
// that there is at most one reader on a connection by executing all reads from
// this goroutine.
func (c *WebsocketClient) readPump() {
	defer close(c.done)

	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[Transport] Unexpected Close Error: %v", err)
			}
			return
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
	defer ticker.Stop()

	// Pong Reply Handler
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		select {
		case <-c.done:
			// When the read detects a websocket closure, it will close the done
			// channel so we can exit
			return
		case msg := <-c.write:
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
		case <-c.interrupt:
			// Cleanly close the connection by sending a close message and then
			// waiting (with timeout) for the server to close the connection.
			//
			// TODO - This does not currently shutdown cleanly, as the caller does
			// not wait for this to complete.
			err := c.conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
			)
			if err != nil {
				log.Println("write close:", err)
				return
			}
			select {
			case <-c.done:
			case <-time.After(time.Second):
			}
			return
		}
	}
}
