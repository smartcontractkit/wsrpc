package wsrpc

import (
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type ServerTransport interface {
	// Close tears down the transport. Once it is called, the transport
	// should not be accessed any more.
	Close() error

	// Read reads a message from the stream.
	Read() <-chan []byte

	// Write sends a message to the stream.
	Write(msg []byte) error
}

// ServerConfig consists of all the configurations to establish a server transport.
type ServerConfig struct{}

type WebsocketServer struct {
	mu sync.Mutex

	conn *websocket.Conn // underlying communication channel

	state transportState

	onClose func() // Callback function called when the transport is closed

	// Communication channels
	write chan []byte
	read  chan []byte

	done      chan struct{}
	interrupt chan struct{}
}

// NewWebsocket server upgrades an HTTP connection to a websocket connection
func NewWebsocketServer(c *websocket.Conn, config *ServerConfig, onClose func()) ServerTransport {
	s := &WebsocketServer{
		conn:      c,
		onClose:   onClose,
		write:     make(chan []byte),
		read:      make(chan []byte),
		done:      make(chan struct{}),
		interrupt: make(chan struct{}),
	}

	go s.start()

	return s
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

	log.Println("[Transport] Closing transport")

	s.state = closing

	// Close the write channel to stop the go routine
	close(s.interrupt)

	s.mu.Unlock()

	return nil
}

// Read returns a channel which provides the messages as they are read
func (c *WebsocketServer) Read() <-chan []byte {
	return c.read
}

// Write writes a message the websocket connection
func (s *WebsocketServer) Write(msg []byte) error {
	// Send the message to the channel
	s.write <- msg

	return nil
}

//
func (s *WebsocketServer) start() {
	defer func() {
		s.Close()
		s.onClose()
	}()

	// Set up reader
	go s.readPump()
	s.writePump()
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

	s.conn.SetPingHandler(func(string) error {
		s.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := s.conn.ReadMessage()
		// An error is provided when the websocket connection is closed,
		// allowing us to clean up the goroutine.
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[Transport] error: %v", err)
			}
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
			// channel so we can exit
			return
		case msg := <-s.write:
			s.conn.SetWriteDeadline(time.Now().Add(writeWait))
			err := s.conn.WriteMessage(websocket.BinaryMessage, msg)
			if err != nil {
				log.Printf("Some error ocurred writing: %v", err)

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
				log.Println("write close:", err)
				return
			}
			select {
			case <-s.done:
			case <-time.After(time.Second):
			}
			return
		}
	}
}
