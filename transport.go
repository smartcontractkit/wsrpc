package wsrpc

import (
	"crypto/tls"
)

// state of transport
type transportState int

const (
	reachable transportState = iota
	closing
)

// TransportCredentials defines the TLS configuration for establishing a
// connection
type TransportCredentials struct {
	Config *tls.Config
}

func NewTransportCredentials(config *tls.Config) TransportCredentials {
	return TransportCredentials{Config: config}
}
