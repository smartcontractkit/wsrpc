package wsrpc

import (
	"crypto"
	"crypto/ed25519"
	"time"

	"github.com/smartcontractkit/wsrpc/credentials"
)

// A ServerOption sets options such as credentials, codec and keepalive parameters, etc.
type ServerOption interface {
	apply(*serverOptions)
}

type serverOptions struct {
	// Buffer
	writeBufferSize int
	readBufferSize  int

	// Transport Credentials
	creds credentials.TransportCredentials

	// The address that the healthcheck will run on
	healthcheckAddr string

	// The HTTP ReadTimeout the healthcheck will use. Set to 0 for no timeout
	healthcheckTimeout time.Duration

	// The HTTP ReadTimeout the ws server will use. Set to 0 for no timeout
	wsTimeout time.Duration

	// The request size limit the ws server will use in bytes. Defaults to 10MB.
	wsReadLimit int64
}

// funcServerOption wraps a function that modifies serverOptions into an
// implementation of the ServerOption interface.
type funcServerOption struct {
	f func(*serverOptions)
}

func newFuncServerOption(f func(*serverOptions)) *funcServerOption {
	return &funcServerOption{
		f: f,
	}
}

func (fdo *funcServerOption) apply(do *serverOptions) {
	fdo.f(do)
}

// returns a ServerOption that sets the healthcheck HTTP read timeout and the server HTTP read timeout
func WithHTTPReadTimeout(hctime time.Duration, wstime time.Duration) ServerOption {
	return newFuncServerOption(func(o *serverOptions) {
		o.healthcheckTimeout = hctime
		o.wsTimeout = wstime
	})
}

func WithWSReadLimit(numBytes int64) ServerOption {
	return newFuncServerOption(func(o *serverOptions) {
		o.wsReadLimit = numBytes
	})
}

// WithCreds returns a ServerOption that sets credentials for server connections.
func WithCreds(privKey ed25519.PrivateKey, pubKeys []ed25519.PublicKey) ServerOption {
	return newFuncServerOption(func(o *serverOptions) {
		privKey, err := credentials.ValidPrivateKeyFromEd25519(privKey)
		if err != nil {
			return
		}

		pubs, err := credentials.ValidPublicKeysFromEd25519(pubKeys...)
		if err != nil {
			return
		}

		config, err := credentials.NewServerTLSConfig(privKey, pubs)
		if err != nil {
			return
		}

		o.creds = credentials.NewTLS(config, pubs)
	})
}

// WithSigner returns a ServerOption that sets credentials for server connections.
func WithSigner(signer crypto.Signer, pubKeys []ed25519.PublicKey) ServerOption {
	return newFuncServerOption(func(o *serverOptions) {
		pubs, err := credentials.ValidPublicKeysFromEd25519(pubKeys...)
		if err != nil {
			return
		}

		config, err := credentials.NewServerTLSSigner(signer, pubs)
		if err != nil {
			return
		}

		o.creds = credentials.NewTLS(config, pubs)
	})
}

// WriteBufferSize specifies the I/O write buffer size in bytes. If a buffer
// size is zero, then a useful default size is used.
func WriteBufferSize(s int) ServerOption {
	return newFuncServerOption(func(o *serverOptions) {
		o.writeBufferSize = s
	})
}

// WriteBufferSize specifies the I/O read buffer size in bytes. If a buffer
// size is zero, then a useful default size is used.
func ReadBufferSize(s int) ServerOption {
	return newFuncServerOption(func(o *serverOptions) {
		o.readBufferSize = s
	})
}

var defaultServerOptions = serverOptions{
	writeBufferSize:    4096,
	readBufferSize:     4096,
	healthcheckTimeout: 5 * time.Second,
	wsTimeout:          10 * time.Second,
	wsReadLimit:        int64(10_000_000),
}

// WithHealthcheck specifies whether to run a healthcheck endpoint. If a url
// is not provided, a healthcheck endpoint is not started.
func WithHealthcheck(addr string) ServerOption {
	return newFuncServerOption(func(o *serverOptions) {
		o.healthcheckAddr = addr
	})
}
