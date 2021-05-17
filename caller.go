package wsrpc

import (
	"github.com/smartcontractkit/wsrpc/credentials"
)

type ServerCallerInterface interface {
	Invoke(pubKey credentials.StaticSizedPublicKey, method string, args interface{}, reply interface{}) error
}
