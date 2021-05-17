package message

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func Test_NewRequest(t *testing.T) {
	t.Parallel()

	var (
		callID = "id1"
		method = "ping"
	)

	payload := &Payload{Name: "test"}

	b, err := proto.Marshal(payload)
	require.NoError(t, err)

	exp := &Message{
		Exchange: &Message_Request{
			Request: &Request{
				CallId:  callID,
				Method:  method,
				Payload: b,
			},
		},
	}

	r, err := NewRequest(callID, method, payload)
	assert.NoError(t, err)
	assert.Equal(t, exp, r)

	r, err = NewRequest(callID, method, "not a proto message")
	assert.Error(t, err)
	assert.Nil(t, r)
}

func Test_NewResponse(t *testing.T) {
	var (
		callID = "id1"
		// payload = &Payload{Name: "test"}
	)

	testCases := []struct {
		name      string
		payload   *Payload
		rerr      error
		errString string
	}{
		{
			name:      "With response payload",
			payload:   &Payload{Name: "test"},
			rerr:      nil,
			errString: "",
		},
		{
			name:      "With response error",
			payload:   &Payload{Name: "test"},
			rerr:      errors.New("an error"),
			errString: "an error",
		},
		{
			name:      "marshal error",
			payload:   &Payload{Name: "test"},
			rerr:      errors.New("an error"),
			errString: "an error",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			b, err := proto.Marshal(tc.payload)
			require.NoError(t, err)

			exp := &Message{
				Exchange: &Message_Response{
					Response: &Response{
						CallId:  callID,
						Payload: b,
						Error:   tc.errString,
					},
				},
			}

			r, err := NewResponse(callID, tc.payload, tc.rerr)
			assert.NoError(t, err)
			assert.Equal(t, exp, r)
		})
	}

	// Test error marshalling the payload
	r, err := NewResponse(callID, "not a payload", nil)
	assert.Error(t, err)
	assert.Nil(t, r)
}
