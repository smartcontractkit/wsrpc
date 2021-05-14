package ping

import (
	"context"

	"github.com/smartcontractkit/wsrpc"
)

// PingServer is the server API for Ping service.
type PingServer interface {
	Ping(context.Context, *PingRequest) (*PingResponse, error)
}

func RegisterPingServer(s wsrpc.ServiceRegistrar, srv PingServer) {
	s.RegisterService(&Ping_ServiceDesc, srv)
}

func _Ping_Ping_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error) (interface{}, error) {
	in := new(PingRequest)
	if err := dec(in); err != nil {
		return nil, err
	}

	return srv.(PingServer).Ping(ctx, in)
}

var Ping_ServiceDesc = wsrpc.ServiceDesc{
	ServiceName: "ping.Ping",
	HandlerType: (*PingServer)(nil),
	Methods: []wsrpc.MethodDesc{
		{
			MethodName: "Ping",
			Handler:    _Ping_Ping_Handler,
		},
	},
}
