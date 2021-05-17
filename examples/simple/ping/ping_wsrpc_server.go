package ping

import (
	"context"

	"github.com/smartcontractkit/wsrpc"
	"github.com/smartcontractkit/wsrpc/credentials"
)

//------------------------------------------------------------------------------
// Server RPC Handler
//------------------------------------------------------------------------------

// PingServer is the server API for Ping service.
type PingServer interface {
	Ping(context.Context, *PingRequest) (*PingResponse, error)
}

func RegisterPingServer(s wsrpc.ServiceRegistrar, srv PingServer) {
	s.RegisterService(&PingServer_ServiceDesc, srv)
}

func _PingServer_Ping_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error) (interface{}, error) {
	in := new(PingRequest)
	if err := dec(in); err != nil {
		return nil, err
	}

	return srv.(PingServer).Ping(ctx, in)
}

var PingServer_ServiceDesc = wsrpc.ServiceDesc{
	ServiceName: "ping.Ping",
	HandlerType: (*PingServer)(nil),
	Methods: []wsrpc.MethodDesc{
		{
			MethodName: "Ping",
			Handler:    _PingServer_Ping_Handler,
		},
	},
}

//------------------------------------------------------------------------------
// Server RPC Caller
//------------------------------------------------------------------------------

// PingServerCaller is the server API for PingServer service.
type PingServerCaller interface {
	Ping(ctx context.Context, pubKey credentials.StaticSizedPublicKey, in *PingRequest) (*PingResponse, error)
}

type pingServerCaller struct {
	srv wsrpc.ServerCallerInterface
}

func NewPingServerCaller(sc wsrpc.ServerCallerInterface) PingServerCaller {
	return &pingServerCaller{sc}
}

func (c *pingServerCaller) Ping(ctx context.Context, pubKey credentials.StaticSizedPublicKey, in *PingRequest) (*PingResponse, error) {
	out := new(PingResponse)
	err := c.srv.Invoke(pubKey, "Ping", in, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}
