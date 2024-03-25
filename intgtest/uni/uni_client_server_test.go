package uni_test

import (
	"context"
	"crypto/ed25519"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/wsrpc"
	pb "github.com/smartcontractkit/wsrpc/intgtest/internal/rpcs"
	"github.com/smartcontractkit/wsrpc/intgtest/utils"
)

func Test_ClientServer_SimpleCall(t *testing.T) {
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

	// Start client
	conn, err := utils.SetupClientConnWithOptsAndTimeout(t, 5*time.Second,
		wsrpc.WithTransportCreds(keypairs.Client1.PrivKey, keypairs.Server.PubKey),
	)
	require.NoError(t, err)
	t.Cleanup(func() {conn.Close()})

	c := pb.NewEchoClient(conn)

	// Wait for the connection to be established
	utils.WaitForReadyConnection(t, conn)

	resp, err := c.Echo(context.Background(), &pb.EchoRequest{
		Body: "bodyarg",
	})
	require.NoError(t, err)

	assert.Equal(t, "bodyarg", resp.Body)
}

func Test_ClientServer_ConcurrentCalls(t *testing.T) {
	keypairs := utils.GenerateKeys(t)
	pubKeys := []ed25519.PublicKey{keypairs.Client1.PubKey}

	// Start the server
	lis, s := utils.SetupServer(t,
		wsrpc.WithCreds(keypairs.Server.PrivKey, pubKeys),
	)

	// Register the echo server implementation with the wsrpc server
	pb.RegisterEchoServer(s, &utils.EchoServer{})

	// Start serving
	go s.Serve(lis)
	t.Cleanup(s.Stop)

	// Start client
	conn, err := utils.SetupClientConnWithOptsAndTimeout(t, 5*time.Second,
		wsrpc.WithTransportCreds(keypairs.Client1.PrivKey, keypairs.Server.PubKey),
		wsrpc.WithBlock(),
	)
	require.NoError(t, err)
	t.Cleanup(func() {conn.Close()})

	c := pb.NewEchoClient(conn)

	respCh := make(chan *pb.EchoResponse)
	doneCh := make(chan []*pb.EchoResponse)

	reqs := []utils.EchoReq{
		{Message: &pb.EchoRequest{Body: "call1", DelayMs: 500}},
		{Message: &pb.EchoRequest{Body: "call2"}, Timeout: 200 * time.Millisecond},
	}

	go func() {
		doneCh <- utils.WaitForResponses(t, respCh, len(reqs))
	}()

	utils.ProcessEchos(t, c, reqs, respCh)

	actual := <-doneCh

	assert.Equal(t, len(reqs), len(actual))
	assert.Equal(t, "call2", actual[0].Body)
	assert.Equal(t, "call1", actual[1].Body)
}
