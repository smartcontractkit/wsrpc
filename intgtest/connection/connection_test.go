package connection_test

import (
	"context"
	"crypto/ed25519"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/wsrpc"
	"github.com/smartcontractkit/wsrpc/connectivity"
	"github.com/smartcontractkit/wsrpc/credentials"
	pb "github.com/smartcontractkit/wsrpc/intgtest/internal/rpcs"
	"github.com/smartcontractkit/wsrpc/intgtest/utils"
)

func Test_ServerNotRunning(t *testing.T) {
	// Setup Keys
	keypairs := utils.GenerateKeys(t)

	// Start client
	conn, err := utils.SetupClientConnWithOptsAndTimeout(t, 5*time.Second,
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
	keypairs := utils.GenerateKeys(t)
	pubKeys := []ed25519.PublicKey{keypairs.Client1.PubKey}

	// Start client
	conn, err := utils.SetupClientConnWithOptsAndTimeout(t, 5*time.Second,
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
	lis, s := utils.SetupServer(t,
		wsrpc.WithCreds(keypairs.Server.PrivKey, pubKeys),
	)

	// Register the ping server implementation with the wsrpc server
	pb.RegisterEchoServer(s, &utils.EchoServer{})

	// Start serving
	go s.Serve(lis)
	t.Cleanup(s.Stop)

	// Wait for the connection
	utils.WaitForReadyConnection(t, conn)

	resp, err := c.Echo(context.Background(), &pb.EchoRequest{
		Body: "bodyarg",
	})
	require.NoError(t, err)

	assert.Equal(t, "bodyarg", resp.Body)
}

func Test_BlockingDial(t *testing.T) {
	// Setup Keys
	keypairs := utils.GenerateKeys(t)
	pubKeys := []ed25519.PublicKey{keypairs.Client1.PubKey}

	unblocked := make(chan *wsrpc.ClientConn)

	go func() {
		conn, err := utils.SetupClientConnWithOptsAndTimeout(t, 5*time.Second,
			wsrpc.WithTransportCreds(keypairs.Client1.PrivKey, keypairs.Server.PubKey),
			wsrpc.WithBlock(),
		)
		require.NoError(t, err)

		unblocked <- conn
	}()

	// Start the server in a goroutine. We wait to start up the server so we can
	// test the blocking mechanism.
	lis, s := utils.SetupServer(t,
		wsrpc.WithCreds(keypairs.Server.PrivKey, pubKeys),
	)

	pb.RegisterEchoServer(s, &utils.EchoServer{})

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
	keypairs := utils.GenerateKeys(t)

	// Start client
	_, err := utils.SetupClientConnWithOptsAndTimeout(t, 50*time.Millisecond,
		wsrpc.WithTransportCreds(keypairs.Client1.PrivKey, keypairs.Server.PubKey),
		wsrpc.WithBlock(),
	)

	require.Error(t, err, "context deadline exceeded")
}

func Test_InvalidCredentials(t *testing.T) {
	keypairs := utils.GenerateKeys(t)
	pubKeys := []ed25519.PublicKey{keypairs.Client2.PubKey}

	// Start the server
	lis, s := utils.SetupServer(t,
		wsrpc.WithCreds(keypairs.Server.PrivKey, pubKeys),
	)

	// Register the ping server implementation with the wsrpc server
	pb.RegisterEchoServer(s, &utils.EchoServer{})

	// Start serving
	go s.Serve(lis)
	t.Cleanup(s.Stop)

	// Start client
	conn, err := utils.SetupClientConnWithOptsAndTimeout(t, 5*time.Second,
		wsrpc.WithTransportCreds(keypairs.Client1.PrivKey, keypairs.Server.PubKey),
	)
	require.NoError(t, err)
	t.Cleanup(conn.Close)

	// Test that it fails to connect
	require.Eventually(t, func() bool {
		return conn.GetState() == connectivity.TransientFailure
	}, 5*time.Second, 100*time.Millisecond)

	// Update the servers allowed list of public keys to include the client's
	err = s.UpdatePublicKeys(keypairs.Client1.PubKey)
	require.NoError(t, err)

	utils.WaitForReadyConnection(t, conn)
}

func Test_InvalidKeyLengthCredentials(t *testing.T) {
	keypairs := utils.GenerateKeys(t)
	pubKeys := []ed25519.PublicKey{keypairs.Client2.PubKey}

	// Start the server
	lis, s := utils.SetupServer(t,
		wsrpc.WithCreds(keypairs.Server.PrivKey, pubKeys),
	)

	// Register the ping server implementation with the wsrpc server
	pb.RegisterEchoServer(s, &utils.EchoServer{})

	// Start serving
	go s.Serve(lis)
	t.Cleanup(s.Stop)

	// Start client
	_, err := utils.SetupClientConnWithOptsAndTimeout(t, 5*time.Second,
		wsrpc.WithTransportCreds(keypairs.Client1.PrivKey[:ed25519.PublicKeySize-1], keypairs.Server.PubKey),
	)
	require.Error(t, err)
}

func Test_GetConnectedPeerPublicKeys(t *testing.T) {
	keypairs := utils.GenerateKeys(t)
	pubKeys := []ed25519.PublicKey{keypairs.Client1.PubKey}

	// Start the server
	lis, s := utils.SetupServer(t,
		wsrpc.WithCreds(keypairs.Server.PrivKey, pubKeys),
	)

	// Register the ping server implementation with the wsrpc server
	pb.RegisterEchoServer(s, &utils.EchoServer{})

	// Start serving
	go s.Serve(lis)
	t.Cleanup(s.Stop)

	require.Empty(t, s.GetConnectedPeerPublicKeys())

	// Start client
	conn, err := utils.SetupClientConnWithOptsAndTimeout(t, 5*time.Second,
		wsrpc.WithTransportCreds(keypairs.Client1.PrivKey, keypairs.Server.PubKey),
	)
	require.NoError(t, err)
	t.Cleanup(conn.Close)

	utils.WaitForReadyConnection(t, conn)

	connectedKeys := s.GetConnectedPeerPublicKeys()
	require.Len(t, s.GetConnectedPeerPublicKeys(), 1)
	actualKey, err := credentials.ToStaticallySizedPublicKey(keypairs.Client1.PubKey)
	require.NoError(t, err)
	require.Equal(t, actualKey, connectedKeys[0])
}

func Test_GetNotificationChan(t *testing.T) {
	keypairs := utils.GenerateKeys(t)
	pubKeys := []ed25519.PublicKey{keypairs.Client1.PubKey}

	// Start the server
	lis, s := utils.SetupServer(t,
		wsrpc.WithCreds(keypairs.Server.PrivKey, pubKeys),
	)

	// Register the ping server implementation with the wsrpc server
	pb.RegisterEchoServer(s, &utils.EchoServer{})

	// Start serving
	go s.Serve(lis)
	t.Cleanup(s.Stop)

	notifyChan := s.GetConnectionNotifyChan()

	// Start client
	conn, err := utils.SetupClientConnWithOptsAndTimeout(t, 100*time.Millisecond,
		wsrpc.WithTransportCreds(keypairs.Client1.PrivKey, keypairs.Server.PubKey),
	)
	require.NoError(t, err)
	t.Cleanup(conn.Close)

	utils.WaitForReadyConnection(t, conn)

	// Wait for connection notification
	select {
	case <-notifyChan:
		t.Log("received notification")
	case <-time.After(3 * time.Second):
		assert.Fail(t, "did not notify")
	}
}

func Test_ServerOpenConnections(t *testing.T) {
	keypairs := utils.GenerateKeys(t)
	pubKeys := []ed25519.PublicKey{keypairs.Client1.PubKey}

	// Start the server
	lis, s := utils.SetupServer(t,
		wsrpc.WithCreds(keypairs.Server.PrivKey, pubKeys),
	)

	// Register the ping server implementation with the wsrpc server
	pb.RegisterEchoServer(s, &utils.EchoServer{})

	// Start serving
	go s.Serve(lis)
	t.Cleanup(s.Stop)

	require.Equal(t, s.OpenConnections(), 0)

	// Start client
	conn, err := utils.SetupClientConnWithOptsAndTimeout(t, 5*time.Second,
		wsrpc.WithTransportCreds(keypairs.Client1.PrivKey, keypairs.Server.PubKey),
	)
	require.NoError(t, err)
	t.Cleanup(conn.Close)

	utils.WaitForReadyConnection(t, conn)

	require.Equal(t, s.OpenConnections(), 1)
}
