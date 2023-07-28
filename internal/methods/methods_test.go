package methods_test

import (
	"crypto/ed25519"

	"testing"

	"github.com/smartcontractkit/wsrpc/credentials"
	"github.com/smartcontractkit/wsrpc/internal/message"
	"github.com/smartcontractkit/wsrpc/internal/methods"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMethodCalls(t *testing.T) {
	methodCalls := methods.NewMethodCalls()
	assert.NotNil(t, methodCalls.MethodCalls)
}

func TestPutMethodCallForPublicKey(t *testing.T) {
	methodCalls := methods.NewMethodCalls()
	_, pubKeyBytes, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	var pubKey credentials.StaticSizedPublicKey
	copy(pubKey[:], pubKeyBytes)

	id := "testID"
	ch := make(chan *message.Response)

	methodCalls.PutMethodCallForPublicKey(pubKey, id, ch)

	assert.NotNil(t, methodCalls.MethodCalls[pubKey])
	assert.NotNil(t, methodCalls.MethodCalls[pubKey].MethodCallsForPublicKey[id])
}

func TestGetMessageResponseChannelForPublicKey(t *testing.T) {
	methodCalls := methods.NewMethodCalls()
	_, pubKeyBytes, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	var pubKey credentials.StaticSizedPublicKey
	copy(pubKey[:], pubKeyBytes)

	id := "testID"
	ch := make(chan<- *message.Response)

	methodCalls.PutMethodCallForPublicKey(pubKey, id, ch)

	retrievedCh, err := methodCalls.GetMessageResponseChannelForPublicKey(pubKey, id)

	assert.Nil(t, err)
	assert.Equal(t, ch, retrievedCh)
}

func TestDeleteMethodCall(t *testing.T) {
	methodCalls := methods.NewMethodCalls()
	_, pubKeyBytes, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	var pubKey credentials.StaticSizedPublicKey
	copy(pubKey[:], pubKeyBytes)
	id := "testID"
	ch := make(chan *message.Response)

	methodCalls.PutMethodCallForPublicKey(pubKey, id, ch)
	methodCalls.DeleteMethodCall(pubKey, id)

	_, err = methodCalls.GetMessageResponseChannelForPublicKey(pubKey, id)

	assert.NotNil(t, err)
}

func TestNewMethodCallsForPublicKey(t *testing.T) {
	methodCallsForPubKey := methods.NewMethodCallsForPublicKey()
	assert.NotNil(t, methodCallsForPubKey.MethodCallsForPublicKey)
}

func TestPutMessageResponseChannel(t *testing.T) {
	methodCallsForPubKey := methods.NewMethodCallsForPublicKey()
	id := "testID"
	ch := make(chan *message.Response)

	methodCallsForPubKey.PutMessageResponseChannel(id, ch)

	assert.NotNil(t, methodCallsForPubKey.MethodCallsForPublicKey[id])
}

func TestGetMessageResponseChannel(t *testing.T) {
	methodCallsForPubKey := methods.NewMethodCallsForPublicKey()
	id := "testID"
	ch := make(chan<- *message.Response)

	methodCallsForPubKey.PutMessageResponseChannel(id, ch)

	retrievedCh, err := methodCallsForPubKey.GetMessageResponseChannel(id)

	assert.Nil(t, err)
	assert.Equal(t, ch, retrievedCh)
}

func TestDelete(t *testing.T) {
	methodCallsForPubKey := methods.NewMethodCallsForPublicKey()
	id := "testID"
	ch := make(chan *message.Response)

	methodCallsForPubKey.PutMessageResponseChannel(id, ch)
	methodCallsForPubKey.Delete(id)

	_, err := methodCallsForPubKey.GetMessageResponseChannel(id)

	assert.NotNil(t, err)
}
