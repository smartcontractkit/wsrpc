package intgtest

import (
	"context"
	"crypto/ed25519"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/wsrpc"
	"github.com/smartcontractkit/wsrpc/connectivity"
	"github.com/smartcontractkit/wsrpc/credentials"
	pb "github.com/smartcontractkit/wsrpc/intgtest/internal/rpcs"
	"github.com/smartcontractkit/wsrpc/peer"
)

const targetURI = "127.0.0.1:1338"

// Implements the ping server RPC call handlers
type echoServer struct{}

// Echo echoes the request back to the client
func (s *echoServer) Echo(ctx context.Context, req *pb.EchoRequest) (*pb.EchoResponse, error) {
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

type echoReq struct {
	// Sets the timeout on the request context. Defaults to no timeout
	timeout time.Duration
	// Insert the client connection's public key into the context. This is
	// required for server to client calls, but optional for client to server
	// calls
	pubKey *credentials.StaticSizedPublicKey
	// The message that will be sent in the request
	message *pb.EchoRequest
}

func processEchos(t *testing.T,
	c pb.EchoClient,
	reqs []*echoReq,
	ch chan<- *pb.EchoResponse,
) {
	t.Helper()

	wg := sync.WaitGroup{}
	for _, req := range reqs {
		wg.Add(1)
		go func() {
			wg.Done()

			ctx := context.Background()
			if req.timeout > 0 {
				tctx, cancel := context.WithTimeout(context.Background(), req.timeout)
				defer cancel()

				ctx = tctx
			}

			if req.pubKey != nil {
				ctx = peer.NewCallContext(ctx, *req.pubKey)
			}

			resp, err := c.Echo(ctx, req.message)
			require.NoError(t, err)

			ch <- resp
		}()

		wg.Wait()
	}
}

func waitForResponses(t *testing.T, ch <-chan *pb.EchoResponse, limit int) []*pb.EchoResponse {
	// Stores the calls in the order they were received. Call 1 should arrive second
	// because of the delayed response.
	resps := []*pb.EchoResponse{}
	i := 0
loop:
	for {
		if i == limit {
			break
		}

		select {
		case resp := <-ch:
			resps = append(resps, resp)
		case <-time.After(3 * time.Second):
			break loop
		}

		i++
	}

	require.Len(t, resps, 2)

	return resps
}

func waitForReadyConnection(t *testing.T, conn *wsrpc.ClientConn) {
	t.Helper()

	require.Eventually(t, func() bool {
		return conn.GetState() == connectivity.Ready
	}, 5*time.Second, 100*time.Millisecond)
}
