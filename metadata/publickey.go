package metadata

import (
	"context"
	"crypto/ed25519"
)

type publicKeyCtxKey string

var PublicKeyCtxKey = publicKeyCtxKey("publickey")

// PublicKeyFromContext extracts the public key from the context
func PublicKeyFromContext(ctx context.Context) ([ed25519.PublicKeySize]byte, bool) {
	pubKey, ok := ctx.Value(PublicKeyCtxKey).([ed25519.PublicKeySize]byte)

	return pubKey, ok
}
