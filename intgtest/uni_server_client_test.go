package intgtest

import (
	"context"
	"crypto/ed25519"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/wsrpc"
	pb "github.com/smartcontractkit/wsrpc/intgtest/internal/rpcs"
	"github.com/smartcontractkit/wsrpc/peer"
)

func Test_ServerClient_SimpleCall(t *testing.T) {
	keypairs := generateKeys(t)
	pubKeys := []ed25519.PublicKey{keypairs.Client1.PubKey}

	// Start the server
	lis, s := setupServer(t,
		wsrpc.Creds(keypairs.Server.PrivKey, pubKeys),
	)

	// Start serving
	go s.Serve(lis)
	t.Cleanup(s.Stop)
	// Create an RPC client for the server
	c := pb.NewEchoClient(s)

	// Start client
	conn, err := setupClientConn(t, 5*time.Second,
		wsrpc.WithTransportCreds(keypairs.Client1.PrivKey, keypairs.Server.PubKey),
	)
	require.NoError(t, err)
	t.Cleanup(conn.Close)

	// Register the handlers on the wsrpc client
	pb.RegisterEchoServer(conn, &serverToClientServer{})

	// Wait for the connection to be established
	waitForReadyConnection(t, conn)

	ctx := peer.NewCallContext(context.Background(), keypairs.Client1.StaticallySizedPublicKey(t))
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	resp, err := c.Echo(ctx, &pb.EchoRequest{Body: "bodyarg"})
	require.NoError(t, err)

	assert.Equal(t, "bodyarg", resp.Body)
}

func Test_ServerClient_ConcurrentCalls(t *testing.T) {
	keypairs := generateKeys(t)
	pubKeys := []ed25519.PublicKey{keypairs.Client1.PubKey}

	// Start the server
	lis, s := setupServer(t,
		wsrpc.Creds(keypairs.Server.PrivKey, pubKeys),
	)

	// Register the ping server implementation with the wsrpc server
	pb.RegisterEchoServer(s, &echoServer{})
	// Create an RPC client for the server
	c := pb.NewEchoClient(s)

	// Start serving
	go s.Serve(lis)
	t.Cleanup(s.Stop)

	// Start client
	conn, err := setupClientConn(t, 5*time.Second,
		wsrpc.WithTransportCreds(keypairs.Client1.PrivKey, keypairs.Server.PubKey),
		wsrpc.WithBlock(),
	)
	require.NoError(t, err)
	t.Cleanup(conn.Close)

	// Register the handlers on the wsrpc client
	pb.RegisterEchoServer(conn, &serverToClientServer{})

	respCh := make(chan *pb.EchoResponse)
	defer close(respCh)

	pk := keypairs.Client1.StaticallySizedPublicKey(t)
	processEchos(t, c,
		[]*echoReq{
			{message: &pb.EchoRequest{Body: "call1", DelayMs: 500}, pubKey: &pk},
			{message: &pb.EchoRequest{Body: "call2"}, timeout: 200 * time.Millisecond, pubKey: &pk},
		},
		respCh,
	)

	actual := waitForResponses(t, respCh, 2)

	assert.Equal(t, "call2", actual[0].Body)
	assert.Equal(t, "call1", actual[1].Body)
}
