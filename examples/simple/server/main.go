package main

import (
	"errors"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/smartcontractkit/wsrpc"
	"github.com/smartcontractkit/wsrpc/examples/simple/keys"
)

func main() {
	privKey := keys.FromHex(keys.ServerPrivKey)

	clientIdentities := map[wsrpc.StaticSizePubKey]string{}
	for _, c := range keys.Clients {
		if c.RegisteredOnServer {
			clientPubKey := keys.FromHex(c.PubKey)

			staticClientPubKey, err := keys.ToStaticSizedBytes(clientPubKey)
			if err != nil {
				panic(err)
			}

			clientIdentities[staticClientPubKey] = c.Name
		}
	}

	lis, err := net.Listen("tcp", "127.0.0.1:1337")
	if err != nil {
		log.Fatalf("[MAIN] failed to listen: %v", err)
	}
	s := wsrpc.NewServer(wsrpc.Creds(privKey, clientIdentities))
	// Register the handler
	handler := func(pubKey wsrpc.StaticSizePubKey, msg []byte) {
		name := clientIdentities[pubKey]

		log.Printf("[MAIN] recv: %s from %s", string(msg), name)
	}
	s.RegisterReadHandler(handler)

	go s.Serve(lis)
	go sendMessages(s, clientIdentities)
	defer s.Stop()

	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigs
		done <- true
	}()

	<-done
}

func receiveMessages(ch <-chan []byte) {
	for {
		message := <-ch
		log.Printf("[MAIN] received: %s", message)
	}
}

// Sends messages to all registered clients. Clients may not have an active
// connection.
func sendMessages(s *wsrpc.Server, clientIdentities map[wsrpc.StaticSizePubKey]string) {
	for {
		for pubKey, name := range clientIdentities {
			err := s.Send(pubKey, []byte("Pong"))
			if err != nil {
				if errors.Is(err, wsrpc.ErrNotConnected) {
					log.Printf("[MAIN] %s: %v", name, err)
				} else {
					log.Printf("[MAIN] Some error ocurred ponging: %v", err)
				}

				continue
			}

			log.Printf("[MAIN] Sent: Pong to %s", name)
		}

		time.Sleep(5 * time.Second)
	}
}
