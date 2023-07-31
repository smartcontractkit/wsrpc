package transport

import (
	"time"
)

type mockWebSocketConn struct {
	readLimit     int64
	readDeadline  time.Time
	pongHandler   func(string) error
	writeDeadline time.Time
	messageType   int
	messageData   []byte
}

func (m *mockWebSocketConn) SetReadLimit(limit int64) {
	m.readLimit = limit
}

func (m *mockWebSocketConn) SetReadDeadline(t time.Time) error {
	m.readDeadline = t
	return nil
}

func (m *mockWebSocketConn) SetPongHandler(handler func(string) error) {
	m.pongHandler = handler
}

func (m *mockWebSocketConn) SetWriteDeadline(t time.Time) error {
	m.writeDeadline = t
	return nil
}

func (m *mockWebSocketConn) ReadMessage() (messageType int, p []byte, err error) {
	return m.messageType, m.messageData, nil
}

func (m *mockWebSocketConn) WriteMessage(messageType int, data []byte) error {
	return nil
}

func (m *mockWebSocketConn) WriteControl(messageType int, data []byte, deadline time.Time) error {
	return nil
}

func (m *mockWebSocketConn) Close() error {
	return nil
}
