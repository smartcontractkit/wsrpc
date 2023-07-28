package wsrpc

import (
	"crypto/ed25519"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/smartcontractkit/wsrpc/examples/simple/keys"
	"github.com/smartcontractkit/wsrpc/internal/message"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Server_UpdatePublicKeys(t *testing.T) {

	_, sPrivKey, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	c1PubKey, _, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	s := NewServer(
		WithCreds(sPrivKey, []ed25519.PublicKey{c1PubKey}),
	)

	require.Equal(t, []ed25519.PublicKey{c1PubKey}, s.opts.creds.PublicKeys.Keys())
	t.Run("valid_update", func(t *testing.T) {
		c2PubKey, _, err := ed25519.GenerateKey(nil)
		require.NoError(t, err)

		err = s.UpdatePublicKeys(c2PubKey)
		require.NoError(t, err)

		assert.Equal(t, []ed25519.PublicKey{c2PubKey}, s.opts.creds.PublicKeys.Keys())
	})

	t.Run("keys_not_32", func(t *testing.T) {
		shortKey := make([]byte, ed25519.PublicKeySize-1)
		longKey := make([]byte, ed25519.PublicKeySize+1)

		err := s.UpdatePublicKeys(shortKey)
		require.Error(t, err)

		err = s.UpdatePublicKeys(longKey)
		require.Error(t, err)

	})
}

func Test_Healthcheck(t *testing.T) {
	// Start the server
	privKey := keys.FromHex(keys.ServerPrivKey)
	pubKeys := []ed25519.PublicKey{}

	lis, err := net.Listen("tcp", "127.0.0.1:1338")
	require.NoError(t, err)
	s := NewServer(
		WithCreds(privKey, pubKeys),
		WithHealthcheck("127.0.0.1:1337"),
	)

	// Start serving
	go s.Serve(lis)
	t.Cleanup(s.Stop)

	// Test until the server boots
	assert.Eventually(t, func() bool {
		// Run a http call
		resp, err := http.Get("http://127.0.0.1:1337/healthz")
		if err != nil {
			return false
		}

		return assert.Equal(t, http.StatusOK, resp.StatusCode)
	}, 5*time.Second, 100*time.Millisecond)

}

func Test_Server_HTTPTimeout_Defaults(t *testing.T) {
	// Start the server
	privKey := keys.FromHex(keys.ServerPrivKey)
	pubKeys := []ed25519.PublicKey{}

	defaultServer := NewServer(
		WithCreds(privKey, pubKeys),
		WithHealthcheck("127.0.0.1:1337"),
	)

	assert.Equal(t, 5*time.Second, defaultServer.opts.healthcheckTimeout)
	assert.Equal(t, 10*time.Second, defaultServer.opts.wsTimeout)

	expectedTimeout := 1 * time.Nanosecond
	timeoutServer := NewServer(
		WithCreds(privKey, pubKeys),
		WithHealthcheck("127.0.0.1:1337"),
		WithHTTPReadTimeout(expectedTimeout, expectedTimeout*2),
	)
	assert.Equal(t, expectedTimeout, timeoutServer.opts.healthcheckTimeout)
	assert.Equal(t, expectedTimeout*2, timeoutServer.opts.wsTimeout)
}

func Test_Server_HTTPTimeout(t *testing.T) {
	// Start the server
	privKey := keys.FromHex(keys.ServerPrivKey)
	pubKeys := []ed25519.PublicKey{}

	lis, err := net.Listen("tcp", "127.0.0.1:1339")
	require.NoError(t, err)

	expectedTimeout := 1 * time.Nanosecond
	s := NewServer(
		WithCreds(privKey, pubKeys),
		WithHealthcheck("127.0.0.1:1336"),
		WithHTTPReadTimeout(expectedTimeout, expectedTimeout*2),
	)

	// Start serving
	go s.Serve(lis)
	t.Cleanup(s.Stop)

	// Test until the server boots
	assert.Eventually(t, func() bool {
		// Run a http call
		_, err := http.Get("http://127.0.0.1:1336/healthz")
		if err != nil {
			return strings.Contains(err.Error(), "EOF") // Check if the error contains "timeout"
		}

		return false
	}, 5*time.Second, 100*time.Millisecond)
}

func Test_Server_ValidateMessageRequest(t *testing.T) {
	s := &Server{
		service: &serviceInfo{
			methods: map[string]*MethodDesc{
				"TestMethod": {},
			},
		},
	}

	t.Run("valid_request", func(t *testing.T) {
		req := &message.Request{
			Method:  "TestMethod",
			CallId:  uuid.New().String(),
			Payload: []byte("test payload"),
		}

		err := s.validateMessageRequest(req)
		require.NoError(t, err)
	})

	t.Run("invalid_method", func(t *testing.T) {
		req := &message.Request{
			Method:  "InvalidMethod",
			CallId:  uuid.New().String(),
			Payload: []byte("test payload"),
		}

		err := s.validateMessageRequest(req)
		require.Error(t, err)
	})

	t.Run("invalid_call_id", func(t *testing.T) {
		req := &message.Request{
			Method:  "TestMethod",
			CallId:  "invalid uuid",
			Payload: []byte("test payload"),
		}

		err := s.validateMessageRequest(req)
		require.Error(t, err)
	})
}
