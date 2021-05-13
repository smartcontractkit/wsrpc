# Websockets RPC

Establishes a persistent bidirectional communication channel using mTLS and websockets.

## Example

You can run the example in `examples/simple` by:

1. Open a terminal to `examples/simple/server` and run `go run main.go`. The server will attempt to send a `Pong` message to all registered clients (These can be viewed in `examples/simple/keys`) every 5 second. Because the client is not active, you will receive `client not connected` messages.
2. Open a separate terminal (Client 0) to `examples/simple/client` and run `go run main.go 0`. This will connect to the server as registered client `Alice` and sends a `Ping` message every 5 seconds. You should now start to see ping/pong messages in both terminals.
3. Open another terminal (Client 1) to `examples/simple/client` and run `go run main.go 1`. This will connect to the server as registered client `Bob` and sends a `Ping` message every 5 seconds. The server terminal will now print out the messages being sent to both `Alice` and `Bob`
4. Quit Bob's client (Client 1). You should now see that Bob is not longer connected on the server terminal. 
5. Quit the server and Alice's client will close the connection and attempt to reconnect to the server with a backoff.
6. Start the server again and you will see Alice reconnect and start sending messages.
7. To test a non registered key run `go run main.go 2`

## TODO 

[ ] Many Many Tests
[ ] Use Protobufs as the message format
[ ] Handle Read/Write Limits of the websocket connection