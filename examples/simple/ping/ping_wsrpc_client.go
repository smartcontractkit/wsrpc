package ping

import (
	"context"

	"github.com/smartcontractkit/wsrpc"
)

// PingClient is the client API for Ping service.
type PingClient interface {
	Ping(ctx context.Context, in *PingRequest) (*PingResponse, error)
}

type pingClient struct {
	cc wsrpc.ClientConnInterface
}

func NewPingClient(cc wsrpc.ClientConnInterface) PingClient {
	return &pingClient{cc}
}

func (c *pingClient) Ping(ctx context.Context, in *PingRequest) (*PingResponse, error) {
	out := new(PingResponse)
	err := c.cc.Invoke("Ping", in, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func RegisterPingClient(s wsrpc.ServiceRegistrar, c PingClient) {
	s.RegisterService(&PingClient_ServiceDesc, c)
}

func _PingClient_Ping_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error) (interface{}, error) {
	in := new(PingRequest)
	if err := dec(in); err != nil {
		return nil, err
	}

	return srv.(PingClient).Ping(ctx, in)
}

var PingClient_ServiceDesc = wsrpc.ServiceDesc{
	ServiceName: "ping.Ping",
	HandlerType: (*PingServer)(nil),
	Methods: []wsrpc.MethodDesc{
		{
			MethodName: "Ping",
			Handler:    _PingClient_Ping_Handler,
		},
	},
}
