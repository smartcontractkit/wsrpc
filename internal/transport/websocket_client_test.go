package transport

import (
	"context"
	"testing"
	"time"

	"github.com/smartcontractkit/wsrpc/logger"
	"github.com/stretchr/testify/assert"
)

func TestNewWebsocketClientConfig(t *testing.T) {
	ctx := context.Background()
	afterWritePump := func() {}
	mockLogger := logger.DefaultLogger
	mockConn := &mockWebSocketConn{}

	tests := []struct {
		name            string
		opts            ConnectOptions
		expectedTimeout time.Duration
		expectedLimit   int64
	}{
		{
			name:            "Default values",
			opts:            ConnectOptions{},
			expectedTimeout: defaultWriteTimeout,
			expectedLimit:   int64(defaultReadLimit),
		},
		{
			name: "Custom WriteTimeout",
			opts: ConnectOptions{
				WriteTimeout: 5 * time.Second,
			},
			expectedTimeout: 5 * time.Second,
			expectedLimit:   int64(defaultReadLimit),
		},
		{
			name: "Custom ReadLimit",
			opts: ConnectOptions{
				ReadLimit: 2048,
			},
			expectedTimeout: defaultWriteTimeout,
			expectedLimit:   2048,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newWebsocketClientConfig(ctx, mockLogger, "addr", tt.opts, afterWritePump, mockConn)
			assert.Equal(t, tt.expectedTimeout, client.writeTimeout)
			assert.Equal(t, tt.expectedLimit, mockConn.readLimit)
		})
	}
}
