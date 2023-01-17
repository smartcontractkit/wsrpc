package wsrpc

import (
	"crypto/ed25519"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/smartcontractkit/wsrpc/credentials"
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

	assert.Equal(t, credentials.PublicKeys([]ed25519.PublicKey{c1PubKey}), *s.opts.creds.PublicKeys)

	c2PubKey, _, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	s.UpdatePublicKeys([]ed25519.PublicKey{c2PubKey})

	assert.Equal(t, credentials.PublicKeys([]ed25519.PublicKey{c2PubKey}), *s.opts.creds.PublicKeys)
}

func Test_Healthcheck(t *testing.T) {
	// Start the server
	privKey := keys.FromHex("c1afd224cec2ff6066746bf9b7cdf7f9f4694ab7ef2ca1692ff923a30df203483b0f149627adb7b6fafe1497a9dfc357f22295a5440786c3bc566dfdb0176808")
	pubKeys := []ed25519.PublicKey{}

	lis, err := net.Listen("tcp", "127.0.0.1:1338")
	require.NoError(t, err)
	s := NewServer(
		Creds(privKey, pubKeys),
		WithHealthcheck("127.0.0.1:1337"),
	)

	// Start serving
	go s.Serve(lis)
	defer s.Stop()

	// Test until the server boots
	assert.Eventually(t, func() bool {
		// Run a http call
		resp, err := http.Get("http://127.0.0.1:1337/healthz")
		if err != nil {
			return false
		}

		return assert.Equal(t, http.StatusOK, resp.StatusCode)
	}, 1*time.Second, 100*time.Millisecond)

}
