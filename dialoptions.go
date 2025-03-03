package wsrpc

import (
	"crypto"
	"crypto/ed25519"
	"time"

	"github.com/smartcontractkit/wsrpc/credentials"
	"github.com/smartcontractkit/wsrpc/internal/backoff"
	"github.com/smartcontractkit/wsrpc/internal/transport"
	"github.com/smartcontractkit/wsrpc/logger"
)

// dialOptions configure a Dial call. dialOptions are set by the DialOption
// values passed to Dial.
type dialOptions struct {
	copts  transport.ConnectOptions
	bs     backoff.Strategy
	block  bool
	logger logger.Logger
}

// DialOption configures how we set up the connection.
type DialOption interface {
	apply(*dialOptions) error
}

// funcDialOption wraps a function that modifies dialOptions into an
// implementation of the DialOption interface.
type funcDialOption struct {
	f func(*dialOptions)
}

func (fdo *funcDialOption) apply(do *dialOptions) error {
	fdo.f(do)
	return nil
}

func newFuncDialOption(f func(*dialOptions)) *funcDialOption {
	return &funcDialOption{
		f: f,
	}
}

type funcDialOptionWithErr struct {
	funcWithErr func(*dialOptions) error
}

func (fdo *funcDialOptionWithErr) apply(do *dialOptions) error {
	return fdo.funcWithErr(do)
}

func newFuncDialOptionWithErr(f func(*dialOptions) error) *funcDialOptionWithErr {
	return &funcDialOptionWithErr{
		funcWithErr: f,
	}
}

// WithTransportCreds returns a DialOption which configures a connection level security credentials (e.g., TLS/SSL).
func WithTransportCreds(privKey ed25519.PrivateKey, serverPubKey ed25519.PublicKey) DialOption {
	return newFuncDialOptionWithErr(func(o *dialOptions) error {
		privKey, err := credentials.ValidPrivateKeyFromEd25519(privKey)
		if err != nil {
			return err
		}

		pubs, err := credentials.ValidPublicKeysFromEd25519(serverPubKey)
		if err != nil {
			return err
		}

		// Generate the TLS config for the client
		config, err := credentials.NewClientTLSConfig(privKey, pubs)
		if err != nil {
			return err
		}

		o.copts.TransportCredentials = credentials.NewTLS(config, pubs)
		return nil
	})
}

// WithTransportSigner returns a DialOption which configures a connection level security credentials (e.g., TLS/SSL).
func WithTransportSigner(signer crypto.Signer, serverPubKey ed25519.PublicKey) DialOption {
	return newFuncDialOptionWithErr(func(o *dialOptions) error {
		pubs, err := credentials.ValidPublicKeysFromEd25519(serverPubKey)
		if err != nil {
			return err
		}

		// Generate the TLS config for the client
		config, err := credentials.NewClientTLSSigner(signer, pubs)
		if err != nil {
			return err
		}

		o.copts.TransportCredentials = credentials.NewTLS(config, pubs)
		return nil
	})
}

// WithBlock returns a DialOption which makes caller of Dial blocks until the
// underlying connection is up. Without this, Dial returns immediately and
// connecting the server happens in background.
func WithBlock() DialOption {
	return newFuncDialOption(func(o *dialOptions) {
		o.block = true
	})
}

// WithWriteTimeout returns a DialOption which sets the write timeout for a
// message to be sent on the wire.
func WithWriteTimeout(d time.Duration) DialOption {
	return newFuncDialOption(func(o *dialOptions) {
		o.copts.WriteTimeout = d
	})
}

func WithReadLimit(size int64) DialOption {
	return newFuncDialOption(func(o *dialOptions) {
		o.copts.ReadLimit = size
	})
}

func WithLogger(lggr logger.Logger) DialOption {
	return newFuncDialOption(func(o *dialOptions) {
		o.logger = lggr
	})
}

func defaultDialOptions() dialOptions {
	return dialOptions{
		copts:  transport.ConnectOptions{},
		logger: logger.DefaultLogger,
	}
}
