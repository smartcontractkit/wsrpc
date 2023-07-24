package transport

import (
	"testing"
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

func TestNewWebsocketServerWithConfig(t *testing.T) {
	mockConn := &mockWebSocketConn{}

	tests := []struct {
		name        string
		config      *ServerConfig
		wantTimeout time.Duration
		wantLimit   int64
	}{
		{
			name:        "Defaults",
			config:      &ServerConfig{},
			wantTimeout: defaultWriteTimeout,
			wantLimit:   defaultReadLimit,
		},
		{
			name: "Custom",
			config: &ServerConfig{
				WriteTimeout: 2 * time.Second,
				ReadLimit:    2048,
			},
			wantTimeout: 2 * time.Second,
			wantLimit:   2048,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newWebsocketServerWithConfig(mockConn, tt.config, nil)

			if server.writeTimeout != tt.wantTimeout {
				t.Errorf("newWebsocketServerWithConfig().writeTimeout = %v, want %v", server.writeTimeout, tt.wantTimeout)
			}

			if server.readLimit != tt.wantLimit {
				t.Errorf("newWebsocketServerWithConfig().readLimit = %v, want %v", server.readLimit, tt.wantLimit)
			}

			if mockConn.readLimit != tt.wantLimit {
				t.Errorf("mockConn.readLimit = %v, want %v", mockConn.readLimit, tt.wantLimit)
			}
		})
	}
}
