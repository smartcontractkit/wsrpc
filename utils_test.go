package wsrpc

import (
	"testing"

	"github.com/smartcontractkit/wsrpc/internal/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_MarshalUnmarshalProtoMessage(t *testing.T) {
	t.Parallel()

	v := &message.Message{}

	b, err := MarshalProtoMessage(v)
	require.NoError(t, err)

	actual := &message.Message{}
	UnmarshalProtoMessage(b, actual)

	assert.Equal(t, v, actual)

	_, err = MarshalProtoMessage("anything")
	assert.Error(t, err)
}
