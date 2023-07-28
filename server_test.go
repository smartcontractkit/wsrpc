package wsrpc

import (
	"crypto/ed25519"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/smartcontractkit/wsrpc/examples/simple/keys"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Server_UpdatePublicKeys(t *testing.T) {
	_, sPrivKey, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	c1PubKey, _, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	s := NewServer(
		Creds(sPrivKey, []ed25519.PublicKey{c1PubKey}),
	)

	assert.Equal(t, []ed25519.PublicKey{c1PubKey}, s.opts.creds.PublicKeys.Keys())

	c2PubKey, _, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	s.UpdatePublicKeys([]ed25519.PublicKey{c2PubKey})

	assert.Equal(t, []ed25519.PublicKey{c2PubKey}, s.opts.creds.PublicKeys.Keys())
}

func Test_Healthcheck(t *testing.T) {
	// Start the server
	privKey := keys.FromHex(keys.ServerPrivKey)
	pubKeys := []ed25519.PublicKey{}

	lis, err := net.Listen("tcp", "127.0.0.1:1338")
	require.NoError(t, err)
	s := NewServer(
		Creds(privKey, pubKeys),
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
		Creds(privKey, pubKeys),
		WithHealthcheck("127.0.0.1:1337"),
	)

	assert.Equal(t, 5*time.Second, defaultServer.opts.healthcheckTimeout)
	assert.Equal(t, 10*time.Second, defaultServer.opts.wsTimeout)

	expectedTimeout := 1 * time.Nanosecond
	timeoutServer := NewServer(
		Creds(privKey, pubKeys),
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
		Creds(privKey, pubKeys),
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
