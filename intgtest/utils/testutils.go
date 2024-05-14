package utils

import (
	"context"
	"crypto/ed25519"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/wsrpc"
	"github.com/smartcontractkit/wsrpc/credentials"
	pb "github.com/smartcontractkit/wsrpc/intgtest/internal/rpcs"
	"github.com/smartcontractkit/wsrpc/logger"
	"github.com/smartcontractkit/wsrpc/peer"
	"google.golang.org/grpc/connectivity"
)

const targetURI = "127.0.0.1:1338"

// Implements the ping server RPC call handlers
type EchoServer struct{}

// Echo echoes the request back to the client
func (s *EchoServer) Echo(ctx context.Context, req *pb.EchoRequest) (*pb.EchoResponse, error) {
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

func (kp keypair) StaticallySizedPublicKey(t *testing.T) credentials.StaticSizedPublicKey {
	t.Helper()

	pk, err := credentials.ToStaticallySizedPublicKey(kp.PubKey)
	require.NoError(t, err)

	return pk
}

type keys struct {
	Server  keypair
	Client1 keypair
	Client2 keypair
}

// GenerateKeys generates keypairs for the server and clients.
func GenerateKeys(t *testing.T) keys {
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

// SetupClientConnWithOptsAndTimeout is a convenience method to setup a client connection for most
// testing usecases.
func SetupClientConnWithOptsAndTimeout(t *testing.T, timeout time.Duration, opts ...wsrpc.DialOption) (*wsrpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	t.Cleanup(cancel)

	opts = append(opts, wsrpc.WithLogger(logger.Test(t)))

	return wsrpc.DialWithContext(ctx, targetURI, opts...)
}

// SetupServer is a convenience method to set up a server for most testing
// usecases.
func SetupServer(t *testing.T, opts ...wsrpc.ServerOption) (net.Listener, *wsrpc.Server) {
	// Attempt to reconnect to the port which the OS may not have had time
	// to clean up between tests.
	var (
		lis net.Listener
		err error
	)

	require.Eventually(t, func() bool {
		lis, err = net.Listen("tcp", targetURI)

		return err == nil
	}, 5*time.Second, 200*time.Millisecond)

	return lis, wsrpc.NewServer(opts...)
}

type EchoReq struct {
	// Sets the Timeout on the request context. Defaults to no Timeout
	Timeout time.Duration
	// Insert the client connection's public key into the context. This is
	// required for server to client calls, but optional for client to server
	// calls
	PubKey *credentials.StaticSizedPublicKey
	// The Message that will be sent in the request
	Message *pb.EchoRequest
}

func ProcessEchos(t *testing.T,
	c pb.EchoClient,
	reqs []EchoReq,
	ch chan<- *pb.EchoResponse,
) {
	wg := sync.WaitGroup{}
	for _, req := range reqs {
		wg.Add(1)
		go func(req EchoReq) {
			defer wg.Done()

			ctx := context.Background()
			if req.Timeout > 0 {
				tctx, cancel := context.WithTimeout(context.Background(), req.Timeout)
				defer cancel()

				ctx = tctx
			}

			if req.PubKey != nil {
				ctx = peer.NewCallContext(ctx, *req.PubKey)
			}

			resp, err := c.Echo(ctx, req.Message)
			require.NoError(t, err)

			ch <- resp
		}(req)
	}
	wg.Wait()
	close(ch)
}

func WaitForResponses(t *testing.T, ch <-chan *pb.EchoResponse, limit int) []*pb.EchoResponse {
	// Stores the calls in the order they were received.

	resps := []*pb.EchoResponse{}
	i := 0
	timer := time.After(5 * time.Second)

loop:
	for i < limit {
		select {
		case resp := <-ch:
			resps = append(resps, resp)
			i++
		case <-timer:
			break loop
		}
	}

	return resps
}

func WaitForReadyConnection(t *testing.T, conn *wsrpc.ClientConn) {
	t.Helper()

	require.Equal(t, false, conn.GetState() == connectivity.Shutdown)

	require.Eventually(t, func() bool {
		return conn.GetState() == connectivity.Ready
	}, 5*time.Second, 100*time.Millisecond)
}
