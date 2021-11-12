package wsrpc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/wsrpc/internal/message"
)

func Test_MarshalUnmarshalProtoMessage(t *testing.T) {
	t.Parallel()

	v := &message.Message{}

	b, err := MarshalProtoMessage(v)
	require.NoError(t, err)

	actual := &message.Message{}
	err = UnmarshalProtoMessage(b, actual)
	require.NoError(t, err)

	assert.Equal(t, v, actual)

	_, err = MarshalProtoMessage("anything")
	assert.Error(t, err)
}
