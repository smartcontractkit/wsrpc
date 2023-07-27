package transport

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewWebsocketServerWithConfig(t *testing.T) {
	mockConn := &mockWebSocketConn{}

	tests := []struct {
		name        string
		config      *ServerConfig
		wantTimeout time.Duration
		wantLimit   int64
	}{
		{
			name:        "Default values",
			config:      &ServerConfig{},
			wantTimeout: defaultWriteTimeout,
			wantLimit:   defaultReadLimit,
		},
		{
			name: "Custom WriteTimeout",
			config: &ServerConfig{
				WriteTimeout: 2 * time.Second,
			},
			wantTimeout: 2 * time.Second,
			wantLimit:   defaultReadLimit,
		},
		{
			name: "Custom ReadLimit",
			config: &ServerConfig{
				ReadLimit: 2048,
			},
			wantTimeout: defaultWriteTimeout,
			wantLimit:   2048,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newWebsocketServerWithConfig(mockConn, tt.config, nil)

			assert.Equal(t, tt.wantTimeout, server.writeTimeout, "Unexpected value for writeTimeout")
			assert.Equal(t, tt.wantLimit, mockConn.readLimit, "Unexpected value for readLimit")
		})
	}
}
