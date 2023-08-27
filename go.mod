module github.com/smartcontractkit/wsrpc

go 1.16

require (
	github.com/cenkalti/backoff v2.2.1+incompatible
	github.com/google/uuid v1.2.0
	github.com/gorilla/websocket v1.4.2
	github.com/pkg/errors v0.9.1
	github.com/stretchr/testify v1.8.0
	go.uber.org/zap v1.24.0
	google.golang.org/protobuf v1.26.0
)

replace golang.org/x/text => golang.org/x/text v0.11.0

replace golang.org/x/net => golang.org/x/net v0.14.0
