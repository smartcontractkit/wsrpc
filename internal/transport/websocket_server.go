package transport

import (
	"net"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type WebsocketServer struct {
	mu sync.Mutex

	// config
	writeTimeout time.Duration

	// Underlying communication channel
	conn *websocket.Conn

	// The current start of the server transport
	state transportState

	// Callback function called when the transport is closed
	onClose func()

	// Communication channels
	write chan []byte
	read  chan []byte

	// A signal channel called when the reader encounters a websocket close error
	done chan struct{}
	// A signal channel called when the transport is closed
	interrupt chan struct{}
}

// newWebsocketServer server upgrades an HTTP connection to a websocket connection.
func newWebsocketServer(c *websocket.Conn, config *ServerConfig, onClose func()) *WebsocketServer {
	writeTimeout := defaultWriteTimeout
	if config.WriteTimeout != 0 {
		writeTimeout = config.WriteTimeout
	}

	s := &WebsocketServer{
		writeTimeout: writeTimeout,
		conn:         c,
		onClose:      onClose,
		write:        make(chan []byte),
		read:         make(chan []byte),
		done:         make(chan struct{}),
		interrupt:    make(chan struct{}),
	}

	go s.start()

	return s
}

// Read returns a channel which provides the messages as they are read.
func (s *WebsocketServer) Read() <-chan []byte {
	return s.read
}

// Write writes a message the websocket connection.
func (s *WebsocketServer) Write(msg []byte) error {
	// Send the message to the channel
	s.write <- msg

	return nil
}

// Close closes the websocket connection and cleans up pump goroutines. Notifies
// the caller with the onClose callback.
func (s *WebsocketServer) Close() error {
	s.mu.Lock()
	// Make sure we only Close once.
	if s.state == closing {
		s.mu.Unlock()

		return nil
	}

	// log.Println("[wsrpc] closing transport")

	s.state = closing

	// Close the write channel to stop the go routine
	close(s.interrupt)

	s.mu.Unlock()

	return nil
}

// start runs readPump in a goroutine and waits on writePump.
func (s *WebsocketServer) start() {
	defer func() {
		s.Close()
		s.onClose()
	}()

	// Set up reader
	go s.readPump()
	s.writePump()
}

func (s *WebsocketServer) pingHandler(message string) error {
	if err := s.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		return err
	}

	err := s.conn.WriteControl(websocket.PongMessage, []byte(message), time.Now().Add(s.writeTimeout))
	if err == websocket.ErrCloseSent {
		return nil
	} else if e, ok := err.(net.Error); ok && e.Temporary() {
		return nil
	}
	return err
}

// readPump pumps messages from the websocket connection.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (s *WebsocketServer) readPump() {
	defer func() {
		defer close(s.done)
	}()

	s.conn.SetPingHandler(s.pingHandler)

	for {
		_, message, err := s.conn.ReadMessage()
		// An error is provided when the websocket connection is closed,
		// allowing us to clean up the goroutine.
		if err != nil {
			// Either remove this or implement better logging.
			// if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
			// log.Printf("[wsrpc] error: %v", err)
			// }
			break
		}
		s.read <- message
	}
}

// writePump pumps messages from the server to the websocket connection.
//
// A goroutine running writePump is started for each connection. The
// server ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
func (s *WebsocketServer) writePump() {
	for {
		select {
		case <-s.done:
			// When the read detects a websocket closure, it will close the done
			// channel so we can exit.
			return
		case msg := <-s.write:
			// Any error due to a closed connection will be immediately picked
			// up in the subsequent network message read or write.
			//nolint:errcheck
			s.conn.SetWriteDeadline(time.Now().Add(s.writeTimeout))
			err := s.conn.WriteMessage(websocket.BinaryMessage, msg)
			if err != nil {
				return
			}
		case <-s.interrupt:
			// Cleanly close the connection by sending a close message and then
			// waiting (with timeout) for the server to close the connection.
			//
			// TODO - This does not currently shutdown cleanly, as the caller does
			// not wait for this to complete.
			err := s.conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
			)
			if err != nil {
				// log.Println("[wsrpc] error:", err)
				return
			}
			s.conn.Close()
			select {
			case <-s.done:
			case <-time.After(time.Second):
			}

			return
		}
	}
}
