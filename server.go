package wsrpc

// TODO - Dynamically Update TLS config to add more clients

import (
	"crypto/ed25519"
	"crypto/x509"
	"errors"
	"fmt"
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
	// Holds a list of the open connections mapped to a buffered channel of
	// outbound messages.
	conns map[StaticSizePubKey]ServerTransport
	// Parameters for upgrading a websocket connection
	upgrader websocket.Upgrader

	// Signals a quit event when the server wants to quit
	quit *Event
	// Signals a done event once the server has finished shutting down
	done *Event

	// readFn contains the registered handler for reading messages
	readFn func(pubKey StaticSizePubKey, message []byte)
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
		conns: map[StaticSizePubKey]ServerTransport{},
		quit:  NewEvent(),
		done:  NewEvent(),
	}

	return s
}

func (s *Server) Serve(lis net.Listener) {
	httpsrv := &http.Server{
		TLSConfig: s.opts.creds.Config,
	}
	http.HandleFunc("/", s.wshandler)
	go httpsrv.ServeTLS(lis, "", "")
	defer httpsrv.Close()

	<-s.done.Done()
}

func (s *Server) wshandler(w http.ResponseWriter, r *http.Request) {
	// Do not establish a new connection if quit has already been fired
	if s.quit.HasFired() {
		return
	}

	log.Println("[Server] Establishing Websocket connection")

	pubKey, err := s.ensureSingleClientConnection(r.TLS.PeerCertificates[0])
	if err != nil {
		log.Print("[Server] error: ", err)
		return
	}

	// Upgrade the websocket connection
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("[Server] error: upgrade", err)
		return
	}

	// A signal channel to close down running go routines (i.e receiving
	// messages) and then ensures the handler to returns
	done := make(chan struct{})

	config := &ServerConfig{}
	onClose := func() {
		s.mu.Lock()
		delete(s.conns, pubKey)

		close(done)
		s.mu.Unlock()
	}

	// Initialize the transport
	tr := NewWebsocketServer(conn, config, onClose)

	// Register the transport against the public key
	s.mu.Lock()
	s.conns[pubKey] = tr
	s.mu.Unlock()

	// Start the reader handler
	go s.handleRead(pubKey, done)

	select {
	case <-done:
		log.Println("Closing Handler: Connection dropped")
	case <-s.quit.Done():
		log.Println("Closing Handler: Shutdown")
	}
}

// Send writes the message to the connection which matches the public key.
func (s *Server) Send(pub [32]byte, msg []byte) error {
	// Find the transport matching the public key
	tr, ok := s.conns[pub]
	if !ok {
		return ErrNotConnected
	}

	tr.Write(msg)

	return nil
}

// handleRead listens to the transport read channel and passes the message to the
// readFn handler.
func (s *Server) handleRead(pubKey [ed25519.PublicKeySize]byte, done <-chan struct{}) {
	defer func() {
		fmt.Println("----> [Server] Reading goroutine closed")
	}()

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

// RegisterReadHandler registers a handler for incoming messages from the
// transport.
func (s *Server) RegisterReadHandler(handler func(pubKey StaticSizePubKey, message []byte)) {
	s.readFn = handler
}

// Stop stops the gRPC server. It immediately closes all open
// connections and listeners.
func (s *Server) Stop() {
	log.Println("[Server] Stopping Server")
	s.quit.Fire()
	defer func() {
		s.done.Fire()
	}()

	s.mu.Lock()
	conns := s.conns
	s.conns = nil
	s.mu.Unlock()

	for _, conn := range conns {
		conn.Close()
	}
}

// Ensure there is only a single connection per public key by checking the
// certificate's public key against the list of registered connections
func (s *Server) ensureSingleClientConnection(cert *x509.Certificate) ([ed25519.PublicKeySize]byte, error) {
	pubKey, err := pubKeyFromCert(cert)
	if err != nil {
		return pubKey, errors.New("could not extracting public key from certificate")
	}

	s.mu.Lock()
	if _, ok := s.conns[pubKey]; ok {
		return pubKey, errors.New("Only one connection allowed per client")
	}
	s.mu.Unlock()

	return pubKey, nil
}
