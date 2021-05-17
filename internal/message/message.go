package message

import (
	"fmt"

	"google.golang.org/protobuf/proto"
)

// New constructs a new message response with v as the payload, and rerr as the
// response error
func NewResponse(callID string, v interface{}, rerr error) (*Message, error) {
	payload, err := marshalProtoMessage(v)
	if err != nil {
		return nil, err
	}

	resp := &Response{
		CallId:  callID,
		Payload: payload,
	}
	if rerr != nil {
		resp.Error = rerr.Error()
	}

	return &Message{
		Exchange: &Message_Response{
			Response: resp,
		},
	}, nil
}

func NewRequest(callID string, method string, v interface{}) (*Message, error) {
	payload, err := marshalProtoMessage(v)
	if err != nil {
		return nil, err
	}

	return &Message{
		Exchange: &Message_Request{
			Request: &Request{
				CallId:  callID,
				Method:  method,
				Payload: payload,
			},
		},
	}, nil
}

// marshalProtoMessage returns the protobuf message wire format of v
func marshalProtoMessage(v interface{}) ([]byte, error) {
	vv, ok := v.(proto.Message)
	if !ok {
		return nil, fmt.Errorf("failed to marshal, message is %T, want proto.Message", v)
	}
	return proto.Marshal(vv)
}
