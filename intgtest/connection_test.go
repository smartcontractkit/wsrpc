package intgtest

import (
	"context"
	"crypto/ed25519"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/wsrpc"
	"github.com/smartcontractkit/wsrpc/connectivity"
	pb "github.com/smartcontractkit/wsrpc/intgtest/internal/rpcs"
)

func Test_ServerNotRunning(t *testing.T) {
	// Setup Keys
	keypairs := generateKeys(t)

	// Start client
	conn, err := setupClientConn(t, 5*time.Second,
		wsrpc.WithTransportCreds(keypairs.Client1.PrivKey, keypairs.Server.PubKey),
	)
	require.NoError(t, err)
	t.Cleanup(conn.Close)

	c := pb.NewEchoClient(conn)

	_, err = c.Echo(context.Background(), &pb.EchoRequest{
		Body: "bodyarg",
	})
	assert.Error(t, err, "connection is not ready")
}

func Test_AutomatedConnectionRetry(t *testing.T) {
	// Setup Keys
	keypairs := generateKeys(t)
	pubKeys := []ed25519.PublicKey{keypairs.Client1.PubKey}

	// Start client
	conn, err := setupClientConn(t, 1000*time.Millisecond,
		wsrpc.WithTransportCreds(keypairs.Client1.PrivKey, keypairs.Server.PubKey),
	)
	require.NoError(t, err)
	t.Cleanup(conn.Close)

	c := pb.NewEchoClient(conn)

	_, err = c.Echo(context.Background(), &pb.EchoRequest{
		Body: "bodyarg",
	})
	assert.Error(t, err, "connection is not ready")

	// Start the server
	lis, s := setupServer(t,
		wsrpc.Creds(keypairs.Server.PrivKey, pubKeys),
	)

	// Register the ping server implementation with the wsrpc server
	pb.RegisterEchoServer(s, &echoServer{})

	// Start serving
	go s.Serve(lis)
	t.Cleanup(s.Stop)

	// Wait for the connection
	waitForReadyConnection(t, conn)

	resp, err := c.Echo(context.Background(), &pb.EchoRequest{
		Body: "bodyarg",
	})
	require.NoError(t, err)

	assert.Equal(t, "bodyarg", resp.Body)
}

func Test_BlockingDial(t *testing.T) {
	// Setup Keys
	keypairs := generateKeys(t)
	pubKeys := []ed25519.PublicKey{keypairs.Client1.PubKey}

	unblocked := make(chan *wsrpc.ClientConn)

	go func() {
		conn, err := setupClientConn(t, 5*time.Second,
			wsrpc.WithTransportCreds(keypairs.Client1.PrivKey, keypairs.Server.PubKey),
			wsrpc.WithBlock(),
		)
		require.NoError(t, err)

		unblocked <- conn
	}()

	// Start the server in a goroutine. We wait to start up the server so we can
	// test the blocking mechanism.
	lis, s := setupServer(t,
		wsrpc.Creds(keypairs.Server.PrivKey, pubKeys),
	)

	pb.RegisterEchoServer(s, &echoServer{})

	time.Sleep(500 * time.Millisecond)
	go s.Serve(lis)
	t.Cleanup(s.Stop)

	// Wait for the connection
	select {
	case conn := <-unblocked:
		t.Cleanup(conn.Close)
	case <-time.After(3 * time.Second):
		assert.Fail(t, "did not connect")
	}
}

func Test_BlockingDialTimeout(t *testing.T) {
	// Setup Keys
	keypairs := generateKeys(t)

	// Start client
	_, err := setupClientConn(t, 50*time.Millisecond,
		wsrpc.WithTransportCreds(keypairs.Client1.PrivKey, keypairs.Server.PubKey),
		wsrpc.WithBlock(),
	)

	require.Error(t, err, "context deadline exceeded")
}

func Test_InvalidCredentials(t *testing.T) {
	keypairs := generateKeys(t)
	pubKeys := []ed25519.PublicKey{keypairs.Client2.PubKey}

	// Start the server
	lis, s := setupServer(t,
		wsrpc.Creds(keypairs.Server.PrivKey, pubKeys),
	)

	// Register the ping server implementation with the wsrpc server
	pb.RegisterEchoServer(s, &echoServer{})

	// Start serving
	go s.Serve(lis)
	t.Cleanup(s.Stop)

	// Start client
	conn, err := setupClientConn(t, 100*time.Millisecond,
		wsrpc.WithTransportCreds(keypairs.Client1.PrivKey, keypairs.Server.PubKey),
	)
	require.NoError(t, err)
	t.Cleanup(conn.Close)

	// Test that it fails to connect
	assert.Eventually(t, func() bool {
		return conn.GetState() == connectivity.TransientFailure
	}, 5*time.Second, 100*time.Millisecond)

	// Update the servers allowed list of public keys to include the client's
	s.UpdatePublicKeys([]ed25519.PublicKey{keypairs.Client1.PubKey})

	waitForReadyConnection(t, conn)
}
