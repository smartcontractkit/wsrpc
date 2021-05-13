package credentials

import (
	"crypto/tls"
)

// TransportCredentials defines the TLS configuration for establishing a
// connection
type TransportCredentials struct {
	Config *tls.Config
}

func NewTLS(config *tls.Config) TransportCredentials {
	return TransportCredentials{Config: config}
}
