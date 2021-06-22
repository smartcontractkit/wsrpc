package main

import (
	"context"
	"crypto/ed25519"
	"errors"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/smartcontractkit/wsrpc"
	"github.com/smartcontractkit/wsrpc/credentials"
	"github.com/smartcontractkit/wsrpc/examples/simple/keys"
	pb "github.com/smartcontractkit/wsrpc/examples/simple/ping"
	"github.com/smartcontractkit/wsrpc/peer"
)

func main() {
	privKey := keys.FromHex(keys.ServerPrivKey)
	pubKeys := []ed25519.PublicKey{}
	clients := map[credentials.StaticSizedPublicKey]string{}

	for _, c := range keys.Clients {
		if c.RegisteredOnServer {
			clientPubKey := keys.FromHex(c.PubKey)

			staticClientPubKey, err := keys.ToStaticSizedBytes(clientPubKey)
			if err != nil {
				panic(err)
			}

			pubKeys = append(pubKeys, clientPubKey)
			clients[staticClientPubKey] = c.Name
		}
	}

	// Set up the wsrpc server
	lis, err := net.Listen("tcp", "127.0.0.1:1338")
	if err != nil {
		log.Fatalf("[MAIN] failed to listen: %v", err)
	}
	s := wsrpc.NewServer(wsrpc.Creds(privKey, pubKeys))

	// Register the ping server implementation with the wsrpc server
	pb.RegisterPingServer(s, &pingServer{
		clients: clients,
	})
	c := pb.NewGnipClient(s)

	// Start serving
	go s.Serve(lis)
	defer s.Stop()

	go pingClientsContinuously(c, clients)

	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigs
		done <- true
	}()

	<-done
}

// Not in use yet.
// TODO - Implement sending server RPC calls
// Sends messages to all registered clients. Clients may not have an active
// connection.
func pingClientsContinuously(c pb.GnipClient, clientIdentities map[credentials.StaticSizedPublicKey]string) {
	for {
		for pubKey, name := range clientIdentities {
			ctx := peer.NewCallContext(context.Background(), pubKey)
			res, err := c.Gnip(ctx, &pb.GnipRequest{Body: "Gnip"})
			if err != nil {
				if errors.Is(err, wsrpc.ErrNotConnected) {
					log.Printf("[RPC CALL] %s: %v", name, err)
				} else {
					log.Printf("[RPC CALL] Some error ocurred: %v", err)
				}

				continue
			}

			log.Printf("[RPC CALL] Gnip (%s) -> %s", name, res.GetBody())
		}

		time.Sleep(5 * time.Second)
	}
}

//------------------------------------------------------------------------------
// Implement the ping server handlers
//------------------------------------------------------------------------------

type pingServer struct {
	clients map[credentials.StaticSizedPublicKey]string
}

func (s *pingServer) Ping(ctx context.Context, req *pb.PingRequest) (*pb.PingResponse, error) {
	resBody := "pong"
	p, ok := peer.FromContext(ctx)
	if !ok {
		return nil, errors.New("could not extract public key")
	}
	name := s.clients[p.PublicKey]

	log.Printf("[RPC SERVICE HANDLER] %s (%s) -> %s", req.Body, name, resBody)

	return &pb.PingResponse{
		Body: resBody,
	}, nil
}
