package intgtest

import (
	"context"
	"crypto/ed25519"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/wsrpc"
	"github.com/smartcontractkit/wsrpc/connectivity"
	pb "github.com/smartcontractkit/wsrpc/intgtest/internal/rpcs"
)

func Test_SimpleCall(t *testing.T) {
	keypairs := generateKeys(t)
	pubKeys := []ed25519.PublicKey{keypairs.Client1.PubKey}

	// Start the server
	lis, s := setupServer(t,
		wsrpc.Creds(keypairs.Server.PrivKey, pubKeys),
	)

	// Register the ping server implementation with the wsrpc server
	pb.RegisterClientToServerServer(s, &clientToServerServer{})

	// Start serving
	go s.Serve(lis)
	t.Cleanup(s.Stop)

	// Start client
	conn, err := setupClientConn(t, 5*time.Second,
		wsrpc.WithTransportCreds(keypairs.Client1.PrivKey, keypairs.Server.PubKey),
	)
	require.NoError(t, err)
	t.Cleanup(conn.Close)

	c := pb.NewClientToServerClient(conn)

	// Wait for the connection to be established
	waitForReadyConnection(t, conn)

	resp, err := c.Echo(context.Background(), &pb.EchoRequest{
		Body: "bodyarg",
	})
	require.NoError(t, err)

	assert.Equal(t, "bodyarg", resp.Body)
}

func Test_ServerNotRunning(t *testing.T) {
	// Setup Keys
	keypairs := generateKeys(t)

	// Start client
	conn, err := setupClientConn(t, 5*time.Second,
		wsrpc.WithTransportCreds(keypairs.Client1.PrivKey, keypairs.Server.PubKey),
	)
	require.NoError(t, err)
	t.Cleanup(conn.Close)

	c := pb.NewClientToServerClient(conn)

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

	c := pb.NewClientToServerClient(conn)

	_, err = c.Echo(context.Background(), &pb.EchoRequest{
		Body: "bodyarg",
	})
	assert.Error(t, err, "connection is not ready")

	// Start the server
	lis, s := setupServer(t,
		wsrpc.Creds(keypairs.Server.PrivKey, pubKeys),
	)

	// Register the ping server implementation with the wsrpc server
	pb.RegisterClientToServerServer(s, &clientToServerServer{})

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

	pb.RegisterClientToServerServer(s, &clientToServerServer{})

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
	pb.RegisterClientToServerServer(s, &clientToServerServer{})

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

func Test_ConcurrentCalls(t *testing.T) {
	keypairs := generateKeys(t)
	pubKeys := []ed25519.PublicKey{keypairs.Client1.PubKey}

	// Start the server
	lis, s := setupServer(t,
		wsrpc.Creds(keypairs.Server.PrivKey, pubKeys),
	)

	// Register the ping server implementation with the wsrpc server
	pb.RegisterClientToServerServer(s, &clientToServerServer{})

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

	c := pb.NewClientToServerClient(conn)

	respCh := make(chan *pb.EchoResponse)
	defer close(respCh)

	processEchos(t, c,
		[]*echoReq{
			{message: &pb.EchoRequest{Body: "call1", DelayMs: 500}},
			{message: &pb.EchoRequest{Body: "call2"}, timeout: 200 * time.Millisecond},
		},
		respCh,
	)

	actual := waitForResponses(t, respCh, 2)

	assert.Equal(t, "call2", actual[0].Body)
	assert.Equal(t, "call1", actual[1].Body)
}

type echoReq struct {
	timeout time.Duration
	message *pb.EchoRequest
}

func processEchos(t *testing.T,
	c pb.ClientToServerClient,
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
