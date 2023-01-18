package intgtest

import (
	"context"
	"crypto/ed25519"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/wsrpc"
	pb "github.com/smartcontractkit/wsrpc/intgtest/internal/rpcs"
)

const targetURI = "127.0.0.1:1338"

// Implements the ping server RPC call handlers
type clientToServerServer struct{}

// Echo echoes the request back to the client
func (s *clientToServerServer) Echo(ctx context.Context, req *pb.EchoRequest) (*pb.EchoResponse, error) {
	if req.DelayMs > 0 {
		time.Sleep(time.Duration(req.DelayMs) * time.Millisecond)
	}

	return &pb.EchoResponse{
		Body: req.Body,
	}, nil
}

type keypair struct {
	PubKey  ed25519.PublicKey
	PrivKey ed25519.PrivateKey
}

type keys struct {
	Server  keypair
	Client1 keypair
	Client2 keypair
}

// generateKeys generates keypairs for the server and clients.
func generateKeys(t *testing.T) keys {
	t.Helper()

	// Setup Keys
	sPubKey, sPrivKey, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	c1PubKey, c1PrivKey, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	c2PubKey, c2PrivKey, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	return keys{
		Server:  keypair{PubKey: sPubKey, PrivKey: sPrivKey},
		Client1: keypair{PubKey: c1PubKey, PrivKey: c1PrivKey},
		Client2: keypair{PubKey: c2PubKey, PrivKey: c2PrivKey},
	}
}

// setupClientConn is a convenience method to setup a client connection for most
// testing usecases.
func setupClientConn(t *testing.T, timeout time.Duration, opts ...wsrpc.DialOption) (*wsrpc.ClientConn, error) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	t.Cleanup(cancel)

	return wsrpc.DialWithContext(ctx, targetURI, opts...)
}

// setupServer is a convenience method to set up a server for most testing
// usecases.
func setupServer(t *testing.T, opts ...wsrpc.ServerOption) (net.Listener, *wsrpc.Server) {
	lis, err := net.Listen("tcp", targetURI)
	require.NoError(t, err)

	return lis, wsrpc.NewServer(opts...)
}
