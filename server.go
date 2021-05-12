package wsrpc

// TODO - Dynamically Update TLS config to add more clients

import (
	"crypto/ed25519"
	"errors"
	"log"
	"net"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var ErrNotConnected = errors.New("client not connected")

type Server struct {
	mu sync.RWMutex

	opts serverOptions
	// Inbound messages from the clients.
	broadcast chan []byte
	// Holds a list of the open connections mapped to a buffered channel of
	// outbound messages.
	conns map[[ed25519.PublicKeySize]byte]ServerTransport
	// Parameters for upgrading a websocket connection
	upgrader websocket.Upgrader

	// readFn contains the registered handler for reading messages
	readFn func(pubKey [ed25519.PublicKeySize]byte, message []byte)
}

func NewServer(opt ...ServerOption) *Server {
	opts := defaultServerOptions
	for _, o := range opt {
		o.apply(&opts)
	}

	s := &Server{
		opts: opts,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  opts.readBufferSize,
			WriteBufferSize: opts.writeBufferSize,
		},
		conns:     map[[ed25519.PublicKeySize]byte]ServerTransport{},
		broadcast: make(chan []byte),
	}

	return s
}

func (s *Server) Serve(lis net.Listener) {
	httpsrv := &http.Server{
		TLSConfig: s.opts.creds.Config,
	}
	http.HandleFunc("/", s.wshandler)
	httpsrv.ServeTLS(lis, "", "")
}

func (s *Server) wshandler(w http.ResponseWriter, r *http.Request) {
	log.Println("[Server] Establishing Websocket connection")

	// Ensure there is only a single connection per public key
	pk, err := pubKeyFromCert(r.TLS.PeerCertificates[0])
	if err != nil {
		log.Print("[Server] error: ", err)
		return
	}

	if _, ok := s.conns[pk]; ok {
		log.Println("[Server] error: Only one connection allowed per client")

		return
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}

	config := &ServerConfig{}

	onClose := func() {
		s.mu.Lock()
		delete(s.conns, pk)
		s.mu.Unlock()
	}

	tr := NewWebsocketServer(conn, config, onClose)

	defer tr.Close()

	// Register the transport against the public key
	s.mu.Lock()
	s.conns[pk] = tr
	s.mu.Unlock()

	// Start the reader
	// TODO - Make this more generic so that closing the transport will kill the read
	done := make(chan struct{})
	go s.startRead(pk, done)
	defer close(done)

	select {}
}

func (s *Server) Send(pub [32]byte, msg []byte) error {
	// Find the transport
	tr, ok := s.conns[pub]
	if !ok {
		return ErrNotConnected
	}

	tr.Write(msg)

	return nil
}

// TODO - Rename
func (s *Server) startRead(pubKey [ed25519.PublicKeySize]byte, done <-chan struct{}) {
	s.mu.Lock()
	tr, ok := s.conns[pubKey]
	s.mu.Unlock()
	if !ok {
		return
	}

	for {
		select {

		case msg := <-tr.Read():
			if s.readFn != nil {
				s.readFn(pubKey, msg)
			}
		case <-done:
			return
		}
	}
}

func (s *Server) RegisterReadHandler(handler func(pubKey [ed25519.PublicKeySize]byte, message []byte)) {
	s.readFn = handler
}
