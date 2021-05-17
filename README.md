# Websockets RPC

Establishes a persistent bi-directional communication channel using mTLS and websockets.

## Usage

### Client to Server RPC

Implement handlers for the server
```go
type pingServer struct {}

func (s *pingServer) Ping(ctx context.Context, req *pb.PingRequest) (*pb.PingResponse, error) {
    // Extracts the connection client's public key. 
    // You can use this to identify the client
	pubKey, ok := metadata.PublicKeyFromContext(ctx)
	if !ok {
		return nil, errors.New("could not extract public key")
	}

    fmt.Println(pubKey)

	return &pb.PingResponse{
		Body: "Pong",
	}, nil
}
```

Initialize a server with the server's private key and a slice of all the allowable public keys.

```go
lis, err := net.Listen("tcp", "127.0.0.1:1337")
if err != nil {
    log.Fatalf("[MAIN] failed to listen: %v", err)
}
s := wsrpc.NewServer(wsrpc.Creds(privKey, pubKeys))
// Register the ping server implementation with the wsrpc server
pb.RegisterPingServer(s, &pingServer{})

s.Serve(lis)
```

Initialize a client with the client's private key and the server's public key

```go
conn, err := wsrpc.Dial("127.0.0.1:1338", wsrpc.WithTransportCreds(privKey, serverPubKey))
if err != nil {
    log.Fatalln(err)
}
defer conn.Close()

// Initialize a new wsrpc client caller
// This is used to called RPC methods on the server
c := pb.NewPingClientCaller(conn)

c.Ping(context.Background(), &pb.Ping{Body: "Ping"})
```

**Note: We do not currently have generators for protobuf service definitions so you will have to write them yourself. See examples/simple/ping** 

### Server to Client RPC

Implement handlers for the client

```go
type pingClient struct{}

func (c *pingClient) Ping(ctx context.Context, req *pb.PingRequest) (*pb.PingResponse, error) {
	return &pb.PingResponse{
		Body: "Pong",
	}, nil
}
```

Initialize a server with the server's private key and a slice of all the allowable public keys.

```go
lis, err := net.Listen("tcp", "127.0.0.1:1337")
if err != nil {
    log.Fatalf("[MAIN] failed to listen: %v", err)
}
s := wsrpc.NewServer(wsrpc.Creds(privKey, pubKeys))
caller := pb.NewPingServerCaller(s)

s.Serve(lis)

// Call the RPC method with the pub key so we know which connection to send it to
caller.Ping(context.Background(), pubKey, &pb.PingRequest{Body: "Ping"})
```

Initialize a client with the client's private key and the server's public key

```go
conn, err := wsrpc.Dial("127.0.0.1:1338", wsrpc.WithTransportCreds(privKey, serverPubKey))
if err != nil {
    log.Fatalln(err)
}
defer conn.Close()

// Initialize RPC call handlers on the client connection
pb.RegisterPingClientService(conn, &pingClient{})
```

**Note: We do not currently have generators for protobuf service definitions so you will have to write them yourself. See examples/simple/ping** 

## Example

You can run a simple example where both the client and server implement a Ping service, and perform RPC calls to each other every 5 seconds.

1. Run the server in `examples/simple/server` with `go run main.go`
2. Run a client (Alice) in `examples/simple/server` with `go run main.go 0` 
3. Run a client (Bob) in `examples/simple/server` with `go run main.go 1` 
4. Run a invalid client (Charlie) in `examples/simple/server` with `go run main.go 2`. The server will reject this connection. 

While the client's are connected, kill the server and see the client's enter a backoff retry loop. Start the server again and they will reconnect.

## TODO 

- [ ] Many Many Tests
- [x] Use Protobufs as the message format
- [x] Server to Node RPC calls
- [ ] Handle Read/Write Limits of the websocket connection
- [x] Dynamically Update TLS config to add more clients
- [x] Simple string error handling
- [ ] Response Status
- [ ] Service Definition Generator Plugin