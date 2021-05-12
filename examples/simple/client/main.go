package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/smartcontractkit/wsrpc"
	"github.com/smartcontractkit/wsrpc/examples/simple/keys"
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

	client := keys.Clients[cidx]

	privKey := make([]byte, hex.DecodedLen(len(client.PrivKey)))
	hex.Decode(privKey, []byte(client.PrivKey))

	serverPubKey := make([]byte, ed25519.PublicKeySize)
	hex.Decode(serverPubKey, []byte(keys.ServerPubKey))

	// Copy the pub key into a statically sized byte array
	var pubStaticServer [ed25519.PublicKeySize]byte
	if ed25519.PublicKeySize != copy(pubStaticServer[:], serverPubKey) {
		// assertion
		panic("copying public key failed")
	}

	conn, err := wsrpc.Dial("127.0.0.1:1337", wsrpc.WithTransportCreds(privKey, pubStaticServer))
	if err != nil {
		log.Fatalln(err)
	}
	defer conn.Close()

	go writeClientWS(conn)
	conn.RegisterHandler(readHandler)

	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigs
		done <- true
	}()

	<-done
}

func writeClientWS(c *wsrpc.ClientConn) {
	for {
		err := c.Send("Ping")
		if err != nil {
			log.Printf("[MAIN] Some error ocurred pinging: %v", err)
		} else {
			log.Println("[MAIN] Sent: Ping")
		}

		time.Sleep(5 * time.Second)
	}
}

func readHandler(msg []byte) {
	log.Printf("[MAIN] recv: %s", string(msg))
}
