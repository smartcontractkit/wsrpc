package bi_test

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/wsrpc"
	pb "github.com/smartcontractkit/wsrpc/intgtest/internal/rpcs"
	"github.com/smartcontractkit/wsrpc/intgtest/utils"
	"github.com/smartcontractkit/wsrpc/peer"
)

func Test_Bidirectional_ConcurrentCalls(t *testing.T) {
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
	sClient := pb.NewEchoClient(s)

	// Start client
	conn, err := utils.SetupClientConnWithOptsAndTimeout(t, 5*time.Second,
		wsrpc.WithTransportCreds(keypairs.Client1.PrivKey, keypairs.Server.PubKey),
		wsrpc.WithBlock(),
	)
	require.NoError(t, err)
	t.Cleanup(func() {conn.Close()})

	cClient := pb.NewEchoClient(conn)
	// Register the handlers on the wsrpc client
	pb.RegisterEchoServer(conn, &utils.EchoServer{})

	// Make a client to server call
	resp, err := cClient.Echo(context.Background(), &pb.EchoRequest{
		Body: "bodyarg",
	})
	require.NoError(t, err)

	assert.Equal(t, "bodyarg", resp.Body)

	// Make a server to client call
	ctx := peer.NewCallContext(context.Background(), keypairs.Client1.StaticallySizedPublicKey(t))
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	resp, err = sClient.Echo(ctx, &pb.EchoRequest{
		Body: "bodyarg",
	})
	require.NoError(t, err)

	assert.Equal(t, "bodyarg", resp.Body)
}

// Tests that calls can be multiplexed
//
// 1. Server makes a call to the client
// 2. Client makes a call back to the server in the handler
// 3. Server returns the response from the client as the echo
func Test_Bidirectional_MultiplexCalls(t *testing.T) {
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
	sClient := pb.NewEchoClient(s)

	// Start client
	conn, err := utils.SetupClientConnWithOptsAndTimeout(t, 5*time.Second,
		wsrpc.WithTransportCreds(keypairs.Client1.PrivKey, keypairs.Server.PubKey),
		wsrpc.WithBlock(),
	)
	require.NoError(t, err)
	t.Cleanup(func() {conn.Close()})

	cClient := pb.NewEchoClient(conn)
	// Register the handlers on the wsrpc client
	pb.RegisterEchoServer(conn, &multiplexEchoServer{
		client:     cClient,
		echoPrefix: "interleaved_echo",
	})

	// Make a server to client call
	ctx := peer.NewCallContext(context.Background(), keypairs.Client1.StaticallySizedPublicKey(t))
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	resp, err := sClient.Echo(ctx, &pb.EchoRequest{
		Body: "bodyarg",
	})
	require.NoError(t, err)

	assert.Equal(t, "interleaved_echo_bodyarg", resp.Body)
}

// multiplexEchoServer
type multiplexEchoServer struct {
	client     pb.EchoClient
	echoPrefix string
}

// Echo echoes the request back to the client
func (s *multiplexEchoServer) Echo(ctx context.Context, req *pb.EchoRequest) (*pb.EchoResponse, error) {
	if req.DelayMs > 0 {
		time.Sleep(time.Duration(req.DelayMs) * time.Millisecond)
	}

	resp, err := s.client.Echo(context.Background(), &pb.EchoRequest{
		Body: fmt.Sprintf("%s_%s", s.echoPrefix, req.Body),
	})
	if err != nil {
		return nil, err
	}

	return &pb.EchoResponse{
		Body: resp.Body,
	}, nil
}
