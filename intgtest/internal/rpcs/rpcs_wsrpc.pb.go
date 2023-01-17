// Code generated by protoc-gen-go-wsrpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-wsrpc v0.0.1
// - protoc             v3.21.7

package rpcs

import (
	context "context"
	wsrpc "github.com/smartcontractkit/wsrpc"
)

// ClientToServerClient is the client API for ClientToServer service.
//
type ClientToServerClient interface {
	Echo(ctx context.Context, in *EchoRequest) (*EchoResponse, error)
}

type clientToServerClient struct {
	cc wsrpc.ClientInterface
}

func NewClientToServerClient(cc wsrpc.ClientInterface) ClientToServerClient {
	return &clientToServerClient{cc}
}

func (c *clientToServerClient) Echo(ctx context.Context, in *EchoRequest) (*EchoResponse, error) {
	out := new(EchoResponse)
	err := c.cc.Invoke(ctx, "Echo", in, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ClientToServerServer is the server API for ClientToServer service.
type ClientToServerServer interface {
	Echo(context.Context, *EchoRequest) (*EchoResponse, error)
}

func RegisterClientToServerServer(s wsrpc.ServiceRegistrar, srv ClientToServerServer) {
	s.RegisterService(&ClientToServer_ServiceDesc, srv)
}

func _ClientToServer_Echo_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error) (interface{}, error) {
	in := new(EchoRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	return srv.(ClientToServerServer).Echo(ctx, in)
}

// ClientToServer_ServiceDesc is the wsrpc.ServiceDesc for ClientToServer service.
// It's only intended for direct use with wsrpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var ClientToServer_ServiceDesc = wsrpc.ServiceDesc{
	ServiceName: "rpcs.ClientToServer",
	HandlerType: (*ClientToServerServer)(nil),
	Methods: []wsrpc.MethodDesc{
		{
			MethodName: "Echo",
			Handler:    _ClientToServer_Echo_Handler,
		},
	},
}
