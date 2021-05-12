package wsrpc

import (
	"crypto/tls"
)

// TransportCredentials defines the TLS configuration for establishing a
// connection
type TransportCredentials struct {
	Config *tls.Config
}

func NewTransportCredentials(config *tls.Config) TransportCredentials {
	return TransportCredentials{Config: config}
}
