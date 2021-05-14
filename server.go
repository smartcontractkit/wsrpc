package wsrpc

import (
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/smartcontractkit/wsrpc/credentials"
	"github.com/smartcontractkit/wsrpc/internal/message"
	"github.com/smartcontractkit/wsrpc/internal/wsrpcsync"
	"github.com/smartcontractkit/wsrpc/metadata"
)

var ErrNotConnected = errors.New("client not connected")

type methodHandler func(srv interface{}, ctx context.Context, dec func(interface{}) error) (interface{}, error)

// MethodDesc represents an RPC service's method specification.
type MethodDesc struct {
	MethodName string
	Handler    methodHandler
}

// ServiceDesc represents an RPC service's specification.
type ServiceDesc struct {
	ServiceName string

	HandlerType interface{}
	Methods     []MethodDesc
}

// serviceInfo wraps information about a service. It is very similar to
// ServiceDesc and is constructed from it for internal purposes.
type serviceInfo struct {
	// Contains the implementation for the methods in this service.
	serviceImpl interface{}
	methods     map[string]*MethodDesc
}
type Server struct {
	mu sync.RWMutex

	opts serverOptions
	// Holds a list of the open connections mapped to a buffered channel of
	// outbound messages.
	conns map[credentials.StaticSizedPublicKey]ServerTransport
	// Parameters for upgrading a websocket connection
	upgrader websocket.Upgrader
	service  *serviceInfo

	// Signals a quit event when the server wants to quit
	quit *wsrpcsync.Event
	// Signals a done event once the server has finished shutting down
	done *wsrpcsync.Event

	// readFn contains the registered handler for reading messages
	readFn func(pubKey credentials.StaticSizedPublicKey, message []byte) []byte
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
		conns: map[credentials.StaticSizedPublicKey]ServerTransport{},
		quit:  wsrpcsync.NewEvent(),
		done:  wsrpcsync.NewEvent(),
	}

	return s
}

// ServiceRegistrar wraps a single method that supports service registration. It
// enables users to pass concrete types other than wsrpc.Server to the service
// registration methods exported by the IDL generated code.
type ServiceRegistrar interface {
	// RegisterService registers a service and its implementation to the
	// concrete type implementing this interface.  It may not be called
	// once the server has started serving.
	// desc describes the service and its methods and handlers. impl is the
	// service implementation which is passed to the method handlers.
	RegisterService(desc *ServiceDesc, impl interface{})
}

// Serve accepts incoming connections on the listener lis, creating a new
// ServerTransport and service goroutine for each.
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
func (s *Server) sendMsg(pub [32]byte, msg []byte) error {
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
	s.mu.Lock()
	tr, ok := s.conns[pubKey]
	s.mu.Unlock()
	if !ok {
		return
	}

	for {
		select {
		case in := <-tr.Read():
			// Unmarshal the message
			msg := &message.Message{}
			if err := UnmarshalProtoMessage(in, msg); err != nil {
				log.Println("Failed to parse message:", err)

				continue
			}

			// Handle the message request or response
			switch msg.Exchange.(type) {
			case *message.Message_Request:
				methodName := msg.GetRequest().GetMethod()
				if md, ok := s.service.methods[methodName]; ok {
					// Create a decoder function to unmarshal the message
					dec := func(v interface{}) error {
						err := UnmarshalProtoMessage(msg.GetRequest().GetPayload(), v)
						if err != nil {
							return err
						}
						return nil
					}

					// Inject the public key into the context so the handler's can use it
					ctx := context.WithValue(context.Background(), metadata.PublicKeyCtxKey, pubKey)
					//----------------------
					// TODO - Handle errors by sending them in the message
					//----------------------
					v, _ := md.Handler(s.service.serviceImpl, ctx, dec)

					// Marshal the reply payload
					reply, err := MarshalProtoMessage(v)
					if err != nil {
						log.Println(err)
						return
					}

					// Construct the reply message
					msg := &message.Message{
						Exchange: &message.Message_Response{
							Response: &message.Response{
								CallId:  msg.GetRequest().GetCallId(),
								Payload: reply,
							},
						},
					}

					replyMsg, err := MarshalProtoMessage(msg)
					if err != nil {
						return
					}

					s.sendMsg(pubKey, replyMsg)
				}
			case *message.Message_Response:
				fmt.Println("This is response message")
			default:
				log.Println("Invalid message type")
			}
		case <-done:
			return
		}
	}
}

func (s *Server) RegisterService(sd *ServiceDesc, ss interface{}) {
	s.register(sd, ss)
}

func (s *Server) register(sd *ServiceDesc, ss interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	info := &serviceInfo{
		serviceImpl: ss,
		methods:     make(map[string]*MethodDesc),
	}
	for i := range sd.Methods {
		d := &sd.Methods[i]
		info.methods[d.MethodName] = d
	}
	s.service = info
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

	// TODO - Wait for the connections to close cleanly so we can perform a
	// graceful shutdown.
	for _, conn := range conns {
		conn.Close()
	}
}

// Ensure there is only a single connection per public key by checking the
// certificate's public key against the list of registered connections
func (s *Server) ensureSingleClientConnection(cert *x509.Certificate) ([ed25519.PublicKeySize]byte, error) {
	pubKey, err := credentials.PubKeyFromCert(cert)
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
