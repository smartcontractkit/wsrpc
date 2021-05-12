package wsrpc

import (
	"fmt"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

type ServerTransport interface {
	// Close tears down the transport. Once it is called, the transport
	// should not be accessed any more. All the pending streams and their
	// handlers will be terminated asynchronously.
	Close() error

	// Write sends a message to the stream.
	Write(msg []byte) error

	// Read reads a message from the stream
	Read() <-chan []byte
}

type ServerConfig struct {
	// WriteBufferSize int
	// ReadBufferSize  int
}

type WebsocketServer struct {
	conn *websocket.Conn // underlying communication channel

	onClose func()

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

func (s *WebsocketServer) Close() error {
	return s.conn.Close()
}

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
	// ticker := time.NewTicker(pingPeriod)
	defer func() {
		// ticker.Stop()
		s.conn.Close()
		s.onClose()
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
			// case <-ticker.C:
			// 	conn.SetWriteDeadline(time.Now().Add(writeWait))
			// 	if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
			// 		return
			// 	}
		}
	}
}

func (c *WebsocketServer) Read() <-chan []byte {
	return c.read
}

// readPump pumps messages from the websocket connection.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (s *WebsocketServer) readPump() {
	defer func() {
		s.conn.Close()
		s.onClose()
	}()
	// c.conn.SetReadLimit(maxMessageSize)
	// c.conn.SetReadDeadline(time.Now().Add(pongWait))
	// conn.SetPongHandler(func(string) error {
	// 	fmt.Println("Received Ping")
	// 	conn.SetReadDeadline(time.Now().Add(pongWait))
	// 	return nil
	// })

	s.conn.SetPingHandler(func(string) error {
		s.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := s.conn.ReadMessage()
		if err != nil {
			fmt.Println("Closing", err)
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[Transport] error: %v", err)
			}
			break
		}

		s.read <- message
	}
}
