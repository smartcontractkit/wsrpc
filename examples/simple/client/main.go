package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/smartcontractkit/wsrpc"
	"github.com/smartcontractkit/wsrpc/examples/simple/keys"
	pb "github.com/smartcontractkit/wsrpc/examples/simple/ping"
)

func main() {
	if len(os.Args[1:]) == 0 {
		log.Fatalf("Must provide the index of the client you wish to run")
	}
	// Run the client matching the array index
	arg1 := os.Args[1]
	cidx, err := strconv.Atoi(arg1)
	if err != nil {
		log.Fatalf("arg must be an int")
	}

	privKey := keys.FromHex(keys.Clients[cidx].PrivKey)
	serverPubKey := keys.FromHex(keys.ServerPubKey)

	conn, err := wsrpc.Dial("127.0.0.1:1338", wsrpc.WithTransportCreds(privKey, serverPubKey))
	if err != nil {
		log.Fatalln(err)
	}
	defer conn.Close()

	// Initialize a new wsrpc client caller
	// This is used to called RPC methods on the server
	c := pb.NewPingClientCaller(conn)

	// Initialize RPC call handlers on the client connection
	pb.RegisterPingClientService(conn, &pingClient{})

	// Call the Ping method
	go pingContinuously(c)

	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigs
		done <- true
	}()

	<-done
}

// pingContinuously sends a Ping RPC call to the server every 5 seconds
func pingContinuously(client pb.PingClient) {
	for {
		res, err := client.Ping(context.Background(), &pb.PingRequest{Body: "ping"})
		if err != nil {
			log.Printf("[RPC CALL] Some error ocurred pinging: %v", err)
		} else {
			log.Printf("[RPC CALL] CALL: Ping -> %s", res.GetBody())
		}

		time.Sleep(5 * time.Second)
	}
}

//--------------------

// Implements RPC call handlers for the ping client
type pingClient struct{}

func (c *pingClient) Ping(ctx context.Context, req *pb.PingRequest) (*pb.PingResponse, error) {
	resBody := "ping processed by client"

	log.Printf("[RPC SERVICE HANDLER] %s -> %s", req.Body, resBody)

	return &pb.PingResponse{
		Body: resBody,
	}, nil
}
