package wsrpc

import (
	"context"
	"crypto/tls"
	"errors"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/wsrpc/internal/message"
	"github.com/smartcontractkit/wsrpc/mocks"
)

func testRespMsg(t *testing.T, payload []byte) []byte {
	resp, err := message.NewResponse(
		"1",
		&message.Response{Payload: payload},
		nil)
	require.NoError(t, err)
	respBytes, err := MarshalProtoMessage(resp)
	require.NoError(t, err)
	return respBytes
}

func TestUniClient(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		conn := new(mocks.Conn)
		lggr := new(mocks.Logger)
		uc := UniClientConn{
			conn: conn,
			lggr: lggr,
		}
		resp := message.Response{}
		conn.On("WriteMessage", mock.Anything, mock.Anything).Return(nil).Once()
		conn.On("ReadMessage").Return(websocket.BinaryMessage, testRespMsg(t, []byte("hello")), nil).Once()
		err := uc.Invoke(context.Background(), "Method", &message.Request{}, &resp)
		require.NoError(t, err)
		assert.Equal(t, "hello", string(resp.Payload))
		conn.AssertExpectations(t)
	})

	t.Run("reconnect on write", func(t *testing.T) {
		conn := new(mocks.Conn)
		lggr := new(mocks.Logger)
		uc := UniClientConn{
			conn: conn,
			lggr: lggr,
		}
		resp := message.Response{}
		uc.connector = func(ctx context.Context, target string, tlsConfig *tls.Config) (Conn, error) {
			return conn, nil
		}
		conn.On("WriteMessage", mock.Anything, mock.Anything).Return(errors.New("oh no")).Once()
		// Should cause a reconnect, so we write again
		lggr.On("Warnf", mock.Anything, mock.Anything)
		conn.On("WriteMessage", mock.Anything, mock.Anything).Return(nil).Once()
		conn.On("ReadMessage").Return(websocket.BinaryMessage, testRespMsg(t, []byte("hello")), nil)
		err := uc.Invoke(context.Background(), "Method", &message.Request{}, &resp)
		require.NoError(t, err)
		assert.Equal(t, "hello", string(resp.Payload))
		conn.AssertExpectations(t)
	})

	t.Run("reconnect on read", func(t *testing.T) {
		conn := new(mocks.Conn)
		lggr := new(mocks.Logger)
		uc := UniClientConn{
			conn: conn,
			lggr: lggr,
		}
		resp := new(message.Response)
		uc.connector = func(ctx context.Context, target string, tlsConfig *tls.Config) (Conn, error) {
			return conn, nil
		}
		conn.On("WriteMessage", mock.Anything, mock.Anything).Return(nil).Once()
		conn.On("ReadMessage").Return(websocket.BinaryMessage, nil, errors.New("oh no")).Once()
		// Should cause a reconnect, so we write again
		lggr.On("Warnf", mock.Anything, mock.Anything)
		conn.On("WriteMessage", mock.Anything, mock.Anything).Return(nil).Once()
		conn.On("ReadMessage").Return(websocket.BinaryMessage, testRespMsg(t, []byte("hello")), nil).Once()
		err := uc.Invoke(context.Background(), "Method", &message.Request{}, resp)
		require.NoError(t, err)
		assert.Equal(t, "hello", string(resp.Payload))
		conn.AssertExpectations(t)
	})

	t.Run("cancel", func(t *testing.T) {
		conn := new(mocks.Conn)
		lggr := new(mocks.Logger)
		uc := UniClientConn{
			conn: conn,
			lggr: lggr,
		}
		resp := new(message.Response)
		req := new(message.Request)
		// Connection always errors, but we can cancel
		uc.connector = func(ctx context.Context, target string, tlsConfig *tls.Config) (Conn, error) {
			return conn, errors.New("oh no")
		}
		conn.On("WriteMessage", mock.Anything, mock.Anything).Return(errors.New("oh no"))
		lggr.On("Warnf", mock.Anything, mock.Anything)

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		var err error
		go func() {
			err = uc.Invoke(ctx, "Method", req, resp)
			done <- struct{}{}
		}()
		cancel()
		select {
		case <-done:
		case <-time.After(1 * time.Second):
			t.Error("failed to cancel")
		}
		require.Error(t, err)
		assert.Contains(t, "context canceled", err.Error())
		conn.AssertExpectations(t)
	})
}
