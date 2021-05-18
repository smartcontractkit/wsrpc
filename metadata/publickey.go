package metadata

import (
	"context"

	"github.com/smartcontractkit/wsrpc/credentials"
)

type publicKeyCtxKey string

var PublicKeyCtxKey = publicKeyCtxKey("publickey")

// PublicKeyFromContext extracts the public key from the context
func PublicKeyFromContext(ctx context.Context) (credentials.StaticSizedPublicKey, bool) {
	pubKey, ok := ctx.Value(PublicKeyCtxKey).(credentials.StaticSizedPublicKey)

	return pubKey, ok
}
