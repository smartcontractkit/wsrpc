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
