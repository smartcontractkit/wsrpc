package wsrpc

import (
	"fmt"
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
}

// NewWebsocket server upgrades an HTTP connection to a websocket connection
func NewWebsocketServer(c *websocket.Conn, config *ServerConfig, onClose func()) ServerTransport {
	s := &WebsocketServer{
		conn:    c,
		onClose: onClose,
		write:   make(chan []byte),
		read:    make(chan []byte),
	}

	go s.writePump()
	go s.readPump()

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

	// Close the websocket conn
	err := s.conn.Close()
	if err != nil {
		return nil
	}

	// Close the write channel to stop the go routine
	close(s.write)

	// Notify the caller that the underlying conn is closed
	s.onClose()
	s.mu.Unlock()

	return err
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

// writePump pumps messages from the server to the websocket connection.
//
// A goroutine running writePump is started for each connection. The
// server ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
func (s *WebsocketServer) writePump() {
	defer func() {
		fmt.Println("----> [Transport] Closing write pump goroutine")
		s.Close()
	}()

	for {
		select {
		case msg, ok := <-s.write:
			s.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Closed the channel.
				s.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			err := s.conn.WriteMessage(websocket.BinaryMessage, msg)
			if err != nil {
				log.Printf("Some error ocurred writing: %v", err)

				return
			}
		}
	}
}

// readPump pumps messages from the websocket connection.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (s *WebsocketServer) readPump() {
	defer func() {
		s.Close()
		fmt.Println("----> [Transport] Closing read pump goroutine")
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
