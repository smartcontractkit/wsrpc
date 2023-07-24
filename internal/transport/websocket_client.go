package transport

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/smartcontractkit/wsrpc/logger"
)

// WebsocketClient implements the ClientTransport interface with websockets.
type WebsocketClient struct {
	ctx context.Context

	// Config
	writeTimeout time.Duration

	// Underlying communication channel
	conn WebSocketConn

	// Callback function called when the transport is closed
	afterWritePump func()

	wg sync.WaitGroup

	// Communication channels
	write chan []byte
	read  chan []byte

	// A signal channel called when the reader encounters a websocket close error
	closeWritePump chan struct{}
	// A signal channel called when the transport is closed
	closeConn chan struct{}

	log logger.Logger
}

// newWebsocketClient establishes the transport with the required ConnectOptions
// and returns it to the caller.
func newWebsocketClient(ctx context.Context, log logger.Logger, addr string, opts ConnectOptions, afterWritePump func()) (*WebsocketClient, error) {
	writeTimeout := defaultWriteTimeout
	if opts.WriteTimeout != 0 {
		writeTimeout = opts.WriteTimeout
	}

	readLimit := defaultReadLimit
	if opts.ReadLimit != 0 {
		readLimit = opts.ReadLimit
	}

	d := websocket.Dialer{
		TLSClientConfig:  opts.TransportCredentials.Config,
		HandshakeTimeout: 45 * time.Second,
	}

	url := fmt.Sprintf("wss://%s", addr)
	conn, _, err := d.DialContext(ctx, url, http.Header{})
	if err != nil {
		return nil, fmt.Errorf("[wsrpc] error while dialing %w", err)
	}

	conn.SetReadLimit(readLimit)

	c := &WebsocketClient{
		ctx:            ctx,
		writeTimeout:   writeTimeout,
		conn:           conn,
		afterWritePump: afterWritePump,
		write:          make(chan []byte),
		read:           make(chan []byte),
		closeWritePump: make(chan struct{}),
		closeConn:      make(chan struct{}),
		log:            log,
	}

	// Start go routines to establish the read/write channels
	c.Start()

	return c, nil
}

// Read returns a channel which provides the messages as they are read.
func (c *WebsocketClient) Read() <-chan []byte {
	return c.read
}

// Write writes a message the websocket connection.
func (c *WebsocketClient) Write(ctx context.Context, msg []byte) error {
	select {
	case <-c.closeWritePump:
		return fmt.Errorf("[wsrpc] could not write message, websocket is closed")
	case <-c.closeConn:
		return fmt.Errorf("[wsrpc] could not write message, transport is closed")
	case <-ctx.Done():
		return fmt.Errorf("[wsrpc] could not write message, context is done")
	case c.write <- msg:
		return nil
	}
}

// Close closes the websocket connection and cleans up pump goroutines.
func (c *WebsocketClient) Close() {
	close(c.closeConn)

	c.wg.Wait()
}

// Start runs readPump and writePump in goroutines.
func (c *WebsocketClient) Start() {
	// Set up reader
	c.wg.Add(1)
	go c.readPump()

	c.wg.Add(1)
	go c.writePump()
}

// readPump pumps messages from the websocket connection. When a websocket
// connection closure is detected through a read error, it closes the done
// channel to shutdown writePump.
//
// The application runs readPump in a per-connection goroutine. This ensures
// that there is at most one reader on a connection by executing all reads from
// this goroutine.
func (c *WebsocketClient) readPump() {
	defer func() {
		close(c.closeWritePump)
		c.wg.Done()
	}()

	//nolint:errcheck
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(handlePong(c.conn))

	for {
		_, msg, err := c.conn.ReadMessage()

		if err != nil {
			c.log.Errorw("[wsrpc] Read error", "err", err)
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
	defer func() {
		ticker.Stop()
		c.afterWritePump()
		c.wg.Done()
	}()

	for {
		select {
		case <-c.closeWritePump:
			// When the read detects a websocket closure, it will close the done
			// channel so we can exit
			return
		case msg := <-c.write: // Write the message
			// Any error due to a closed connection will be immediately picked
			// up in the subsequent network message read or write.
			//nolint:errcheck
			c.conn.SetWriteDeadline(time.Now().Add(c.writeTimeout))
			err := c.conn.WriteMessage(websocket.BinaryMessage, msg)
			if err != nil {
				c.log.Errorf("Write error: %v", err)

				c.conn.Close()

				return
			}
		case <-ticker.C:
			// Any error due to a closed connection will be immediately picked
			// up in the subsequent network message read or write.
			if err := c.conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(c.writeTimeout)); err != nil {
				c.conn.Close()

				return
			}
		case <-c.closeConn:
			// Cleanly close the connection by sending a close message and then
			// waiting (with timeout) for the server to close the connection.
			err := c.conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
			)
			if err != nil {
				return
			}
			c.conn.Close()
			select {
			case <-c.closeWritePump:
			case <-time.After(time.Second):
			}

			return
		}
	}
}
