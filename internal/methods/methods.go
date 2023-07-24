package methods

import (
	"fmt"

	"github.com/smartcontractkit/wsrpc/credentials"
	"github.com/smartcontractkit/wsrpc/internal/message"
)

type MethodCalls struct {
	MethodCalls map[credentials.StaticSizedPublicKey]*MethodCallsForPublicKey
}

func NewMethodCalls() *MethodCalls {
	return &MethodCalls{
		MethodCalls: make(map[credentials.StaticSizedPublicKey]*MethodCallsForPublicKey),
	}
}

func (m *MethodCalls) PutMethodCallForPublicKey(pubKey credentials.StaticSizedPublicKey, id string, ch chan<- *message.Response) {
	var methodCallsForPubKey *MethodCallsForPublicKey
	var ok bool
	if methodCallsForPubKey, ok = m.MethodCalls[pubKey]; !ok {
		methodCallsForPubKey = NewMethodCallsForPublicKey()
		m.MethodCalls[pubKey] = methodCallsForPubKey
	}
	methodCallsForPubKey.PutMessageResponseChannel(id, ch)
}

func (m *MethodCalls) GetMessageResponseChannelForPublicKey(pubKey credentials.StaticSizedPublicKey, id string) (chan<- *message.Response, error) {
	if methodCallsForPubKey, ok := m.MethodCalls[pubKey]; ok {
		return methodCallsForPubKey.GetMessageResponseChannel(id)
	}

	return nil, fmt.Errorf("public key not found: %v", pubKey)
}

func (m *MethodCalls) DeleteMethodCall(pubKey credentials.StaticSizedPublicKey, id string) {
	if methodCallsForPubKey, ok := m.MethodCalls[pubKey]; ok {
		methodCallsForPubKey.Delete(id)
	}
}

type MethodCallsForPublicKey struct {
	MethodCallsForPublicKey map[string]chan<- *message.Response
}

func NewMethodCallsForPublicKey() *MethodCallsForPublicKey {
	return &MethodCallsForPublicKey{
		MethodCallsForPublicKey: make(map[string]chan<- *message.Response),
	}
}

func (m *MethodCallsForPublicKey) PutMessageResponseChannel(id string, ch chan<- *message.Response) {
	m.MethodCallsForPublicKey[id] = ch
}

func (m *MethodCallsForPublicKey) GetMessageResponseChannel(id string) (chan<- *message.Response, error) {
	call, ok := m.MethodCallsForPublicKey[id]
	if !ok {
		return nil, fmt.Errorf("id not found: %v", id)
	}
	return call, nil
}

func (m *MethodCallsForPublicKey) Delete(id string) {
	delete(m.MethodCallsForPublicKey, id)
}
