package wsrpc

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"

	"github.com/smartcontractkit/wsrpc/internal/backoff"
	"github.com/smartcontractkit/wsrpc/internal/message"
	"github.com/smartcontractkit/wsrpc/internal/transport"
	"github.com/smartcontractkit/wsrpc/internal/wsrpcsync"
	"google.golang.org/grpc/connectivity"
)

var (
	// errConnClosing indicates that the connection is closing.
	errConnClosing = errors.New("wsrpc: the connection is closing")
)

// MethodCallHandler defines a handler which is called when the websocket
// message contains a response to an RPC call.
type MethodCallHandler func(*message.Response)

// ClientInterface defines the functions clients need to perform an RPC.
// It is implemented by *ClientConn and *Server.
type ClientInterface interface {
	Invoke(ctx context.Context, method string, args interface{}, reply interface{}) error
}

// ClientConn represents a virtual connection to a websocket endpoint, to
// perform and serve RPCs.
type ClientConn struct {
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.RWMutex
	wg     *sync.WaitGroup

	// The websocket address
	target string
	// A channel which receives updates when connectivity state changes
	stateCh <-chan connectivity.State
	// Manages the connectivity state.
	csMgr *connectivityStateManager

	dopts    dialOptions
	addrConn *addrConn

	// Contains all pending method call ids and a handler to call when a
	// response is received
	methodCalls map[string]MethodCallHandler

	// The RPC service definition
	service *serviceInfo
}

func Dial(target string, opts ...DialOption) (*ClientConn, error) {
	ctx := context.Background()
	return DialWithContext(ctx, target, opts...)
}

// Dial creates a client connection to the given target. By default, it's
// a non-blocking dial (the function won't wait for connections to be
// established, and connecting happens in the background). To make it a blocking
// dial, use WithBlock() dial option.
func DialWithContext(ctxCaller context.Context, target string, opts ...DialOption) (*ClientConn, error) {
	ctx, cancel := context.WithCancel(ctxCaller)

	cc := &ClientConn{
		ctx:         ctx,
		cancel:      cancel,
		wg:          &sync.WaitGroup{},
		target:      target,
		csMgr:       &connectivityStateManager{},
		dopts:       defaultDialOptions(),
		methodCalls: map[string]MethodCallHandler{},
	}

	for i, opt := range opts {
		err := opt.apply(&cc.dopts)
		if err != nil {
			return nil, fmt.Errorf("dial option %d failed: %w", i, err)
		}
	}

	// Set the backoff strategy. We may need to consider making this
	// customizable in the dial options.
	cc.dopts.bs = backoff.DefaultExponential

	addrConn := cc.newAddrConn(target)

	if err := addrConn.connect(); err != nil {
		return nil, fmt.Errorf("error connecting: %w", err)
	}
	cc.mu.Lock()
	cc.addrConn = addrConn
	cc.mu.Unlock()

	if cc.dopts.block {
		for {
			curState := cc.csMgr.getState()
			if curState == connectivity.Ready {
				break
			}

			// Wait for a state change to re run the for loop
			if !cc.WaitForStateChange(ctx, curState) {
				addrConn.cancel()

				return nil, ctx.Err()
			}
		}
	}

	return cc, nil
}

func (cc *ClientConn) WaitForStateChange(ctx context.Context, sourceState connectivity.State) bool {
	ch := cc.csMgr.getNotifyChan()
	if cc.csMgr.getState() != sourceState {
		return true
	}
	select {
	case <-ctx.Done():
		return false
	case <-ch:
		return true
	}
}

// WaitForReady waits until the state becomes Ready
// It returns true when that happens
// It returns false if the context is cancelled, or the conn is shut down
func (cc *ClientConn) WaitForReady(ctx context.Context) bool {
	ch := cc.csMgr.getNotifyChan()
	switch cc.csMgr.getState() {
	case connectivity.Ready:
		return true
	case connectivity.Shutdown:
		return false
	case connectivity.Idle, connectivity.Connecting, connectivity.TransientFailure:
		break
	}
	cc.dopts.logger.Debugf("Waiting for connection to be ready, current state: %s", cc.csMgr.getState())
	select {
	case <-ctx.Done():
		return false
	case <-ch:
		return cc.WaitForReady(ctx)
	}
}

// GetState gets the current connectivity state.
func (cc *ClientConn) GetState() connectivity.State {
	return cc.csMgr.getState()
}

// newAddrConn creates an addrConn for the addr and sets it to cc.conn.
func (cc *ClientConn) newAddrConn(addr string) *addrConn {
	stateCh := make(chan connectivity.State)
	ac := &addrConn{
		state:   connectivity.Idle,
		wg:      &sync.WaitGroup{},
		stateCh: stateCh,
		addr:    addr,
		dopts:   cc.dopts,
	}
	ac.ctx, ac.cancel = context.WithCancel(cc.ctx)
	cc.mu.Lock()

	cc.addrConn = ac
	cc.stateCh = stateCh
	cc.csMgr.getNotifyChan()
	cc.mu.Unlock()

	cc.wg.Add(1)
	go cc.listenForConnectivityChange()

	cc.wg.Add(1)
	go cc.listenForRead()

	return ac
}

// listenForConnectivityChange listens for the addrConn's connectivity to change
// and updates the ClientConn ConnectivityStateManager.
func (cc *ClientConn) listenForConnectivityChange() {
	defer cc.wg.Done()
	for {
		select {
		case <-cc.ctx.Done():
			return
		case s := <-cc.stateCh:
			cc.csMgr.updateState(s)
		}
	}
}

// listenForRead listens for the connectivity state to be ready and enables the
// read handler.
func (cc *ClientConn) listenForRead() {
	defer cc.wg.Done()

	var done chan struct{}
	for {
		select {
		case <-cc.ctx.Done():
			return
		case <-cc.csMgr.getNotifyChan():
			s := cc.csMgr.getState()

			if s == connectivity.Ready {
				if done == nil {
					done = make(chan struct{})
				}
				cc.wg.Add(1)
				go cc.handleRead(done)
			} else {
				if done != nil {
					close(done)
					done = nil
				}
			}
		}
	}
}

// handleRead listens to the transport read channel and passes the message to the
// readFn handler.
func (cc *ClientConn) handleRead(done <-chan struct{}) {
	defer cc.wg.Done()
	var tr transport.ClientTransport
	var conn *addrConn

	cc.mu.RLock()
	conn = cc.addrConn

	// if connection has been closed, then conn can be nil
	if conn == nil {
		cc.mu.RUnlock()

		return
	}

	conn.mu.RLock()
	tr = cc.addrConn.transport
	conn.mu.RUnlock()
	cc.mu.RUnlock()

	if nil == tr {
		return
	}

	for {
		select {
		case in := <-tr.Read():
			// Unmarshal the message
			msg := &message.Message{}
			if err := UnmarshalProtoMessage(in, msg); err != nil {
				continue
			}

			switch ex := msg.Exchange.(type) {
			case *message.Message_Request:
				cc.wg.Add(1)
				go cc.handleMessageRequest(ex.Request)
			case *message.Message_Response:
				cc.wg.Add(1)
				go cc.handleMessageResponse(ex.Response)
			default:
				cc.dopts.logger.Errorf("Invalid message type: %T", ex)
			}
		case <-cc.ctx.Done():
			return
		}
	}
}

// handleMessageRequest looks up the method matching the method name and calls
// the handler.
func (cc *ClientConn) handleMessageRequest(r *message.Request) {
	defer cc.wg.Done()
	methodName := r.GetMethod()
	if md, ok := cc.service.methods[methodName]; ok {
		// Create a decoder function to unmarshal the message
		dec := func(v interface{}) error {
			return UnmarshalProtoMessage(r.GetPayload(), v)
		}

		ctx := context.Background()
		v, herr := md.Handler(cc.service.serviceImpl, ctx, dec)

		msg, err := message.NewResponse(r.GetCallId(), v, herr)
		if err != nil {
			return
		}

		replyMsg, err := MarshalProtoMessage(msg)
		if err != nil {
			return
		}

		var tr transport.ClientTransport
		cc.mu.RLock()
		cc.addrConn.mu.RLock()
		tr = cc.addrConn.transport
		cc.addrConn.mu.RUnlock()
		cc.mu.RUnlock()

		if err := tr.Write(ctx, replyMsg); err != nil {
			cc.dopts.logger.Errorf("error writing to transport: %s", err)
		}
	}
}

// handleMessageResponse finds the call which matches the method call id of the
// response and sends the payload to the call channel.
func (cc *ClientConn) handleMessageResponse(r *message.Response) {
	defer cc.wg.Done()
	callID := r.GetCallId()

	cc.mu.Lock()
	handlerFunc, ok := cc.methodCalls[callID]
	cc.mu.Unlock()

	if ok {
		handlerFunc(r)
	}
}

// RegisterService registers a service and its implementation to the wsrpc
// server.
func (cc *ClientConn) RegisterService(sd *ServiceDesc, ss interface{}) {
	cc.register(sd, ss)
}

func (cc *ClientConn) register(sd *ServiceDesc, ss interface{}) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	info := &serviceInfo{
		serviceImpl: ss,
		methods:     make(map[string]*MethodDesc),
	}
	for i := range sd.Methods {
		d := &sd.Methods[i]
		info.methods[d.MethodName] = d
	}
	cc.service = info
}

// Close tears down the ClientConn and all underlying connections.
func (cc *ClientConn) Close() error {
	cc.cancel()
	cc.mu.Lock()
	addrConn := cc.addrConn
	cc.addrConn = nil
	cc.mu.Unlock()

	addrConn.teardown() //closes lower level

	cc.wg.Wait()
	return nil
}

// Invoke sends the RPC request on the wire and returns after response is
// received.
func (cc *ClientConn) Invoke(ctx context.Context, method string, args interface{}, reply interface{}) error {
	// Ensure the connection state is ready
	cc.mu.RLock()
	cc.addrConn.mu.RLock()
	state := cc.addrConn.state
	cc.addrConn.mu.RUnlock()
	cc.mu.RUnlock()

	if state != connectivity.Ready {
		return errors.New("connection is not ready")
	}

	callID := uuid.NewString()
	req, err := message.NewRequest(callID, method, args)
	if err != nil {
		return err
	}

	reqB, err := proto.Marshal(req)
	if err != nil {
		return err
	}

	// Register a method call for the callID.
	cc.mu.Lock()
	wait := cc.registerMethodCall(ctx, callID)
	cc.mu.Unlock()

	// Remove the method call once invoke has been completed.
	defer func() {
		cc.mu.Lock()
		cc.removeMethodCall(callID)
		cc.mu.Unlock()
	}()

	var tr transport.ClientTransport
	cc.mu.RLock()
	if cc.addrConn == nil {
		cc.mu.RUnlock()
		// Close() has been called
		return errors.New("client Close() called")
	}
	cc.addrConn.mu.RLock()
	tr = cc.addrConn.transport
	cc.addrConn.mu.RUnlock()
	cc.mu.RUnlock()

	if err := tr.Write(ctx, reqB); err != nil {
		return err
	}

	// Wait for the response
	select {
	case resp := <-wait:
		// Handle error
		if resp.Error != "" {
			return errors.New(resp.Error)
		}

		// Unmarshal the payload into the reply
		err := UnmarshalProtoMessage(resp.GetPayload(), reply)
		if err != nil {
			return err
		}
	case <-ctx.Done():
		return fmt.Errorf("call timeout: %w", ctx.Err())
	}

	return nil
}

// registerMethodCall registers a method call handler func.
//
// This requires a lock on cc.mu.
func (cc *ClientConn) registerMethodCall(ctx context.Context, id string) <-chan *message.Response {
	wait := make(chan *message.Response)

	cc.methodCalls[id] = func(r *message.Response) {
		select {
		case <-ctx.Done():
		case wait <- r:
		}
	}

	return wait
}

// removeMethodCall deregisters a method call to the method call map.
//
// This requires a lock on cc.mu.
func (cc *ClientConn) removeMethodCall(id string) {
	delete(cc.methodCalls, id)
}

// addrConn is a network connection to a given address.
type addrConn struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     *sync.WaitGroup

	addr  string
	dopts dialOptions

	// transport is set when there's a viable transport, and is reset
	// to nil when the current transport should no longer be used (e.g.
	// after transport is closed, ac has been torn down).
	transport transport.ClientTransport // The current transport.

	mu sync.RWMutex

	// Use updateConnectivityState for updating addrConn's connectivity state.
	state connectivity.State
	// Notifies this channel when the ConnectivityState changes
	stateCh chan connectivity.State
}

// connect starts creating a transport.
// It does nothing if the ac is not IDLE.
func (ac *addrConn) connect() error {
	ac.mu.Lock()
	if ac.state == connectivity.Shutdown {
		ac.mu.Unlock()

		return errConnClosing
	}

	if ac.state != connectivity.Idle {
		ac.mu.Unlock()

		return nil
	}

	// Update connectivity state within the lock to prevent subsequent or
	// concurrent calls from resetting the transport more than once.
	ac.updateConnectivityState(connectivity.Connecting)
	ac.mu.Unlock()

	// Start a goroutine connecting to the server asynchronously.
	ac.wg.Add(1)
	go ac.resetTransport()

	return nil
}

// Note: this requires a lock on ac.mu.
func (ac *addrConn) updateConnectivityState(s connectivity.State) {
	if ac.state == s {
		return
	}
	ac.state = s
	select {
	case ac.stateCh <- s:
		return
	case <-ac.ctx.Done():
		return
	}

}

// resetTransport attempts to connect to the server. If the connection fails,
// it will continuously attempt reconnection with an exponential backoff.
func (ac *addrConn) resetTransport() {
	defer ac.wg.Done()
	for {
		ac.mu.Lock()
		if ac.state == connectivity.Shutdown {
			ac.mu.Unlock()
			return
		}

		backoffFor := ac.dopts.bs.NextBackOff()
		addr := ac.addr
		copts := ac.dopts.copts

		ac.transport = nil

		ac.updateConnectivityState(connectivity.Connecting)

		newTr, reconnect, err := ac.createTransport(addr, copts)

		ac.mu.Unlock()
		if err != nil {
			// After connection failure, the addrConn enters TRANSIENT_FAILURE.
			ac.mu.Lock()
			if ac.state == connectivity.Shutdown {
				ac.mu.Unlock()
				return
			}
			ac.dopts.logger.Errorf("failed to connect to server at %s, got: %v", addr, err)
			ac.updateConnectivityState(connectivity.TransientFailure)
			ac.mu.Unlock()

			// Reconnection backoff time
			ac.dopts.logger.Infof("attempting reconnection in %s", backoffFor)
			timer := time.NewTimer(backoffFor)

			select {
			case <-timer.C:
				// NOOP - This falls through to continue to retry connecting
			case <-ac.ctx.Done():
				timer.Stop()

				return
			}

			continue
		}

		// Close the transport early if in a SHUTDOWN state
		ac.mu.Lock()
		if ac.state == connectivity.Shutdown {
			ac.mu.Unlock()
			return
		}
		ac.transport = newTr
		ac.dopts.bs.Reset()

		ac.updateConnectivityState(connectivity.Ready)

		ac.mu.Unlock()

		ac.dopts.logger.Debugf("Connected to %s", ac.addr)

		// Block until the created transport is down. When this happens, we
		// attempt to reconnect by starting again from the top
		select {
		case <-ac.ctx.Done():
			return
		case <-reconnect.Done():
			ac.dopts.logger.Info("Reconnecting to server...")
		}
	}
}

// createTransport creates a new transport. If it fails to connect to the server,
// it returns an error which used to detect whether a retry is necessary. This
// also returns a reconnect event which is fired when the transport closes due
// to issues with the underlying connection.
func (ac *addrConn) createTransport(addr string, copts transport.ConnectOptions) (transport.ClientTransport, *wsrpcsync.Event, error) {
	reconnect := wsrpcsync.NewEvent()
	once := sync.Once{}

	// Called when the transport closes
	afterWritePump := func() {
		ac.mu.Lock()
		once.Do(func() {
			if connectivity.Ready == ac.state {
				ac.updateConnectivityState(connectivity.Idle)
			}
		})
		ac.mu.Unlock()
		reconnect.Fire()
	}

	tr, err := transport.NewClientTransport(ac.ctx, ac.dopts.logger, addr, copts, afterWritePump)

	return tr, reconnect, err
}

// tearDown starts to tear down the addrConn.
func (ac *addrConn) teardown() {
	ac.mu.Lock()
	ac.cancel()

	if connectivity.Shutdown == ac.state {
		ac.mu.Unlock()
		return
	}

	curTr := ac.transport
	ac.transport = nil

	ac.cancel()
	ac.updateConnectivityState(connectivity.Shutdown)
	ac.mu.Unlock()
	if curTr != nil {
		//syncronously closes lower level
		curTr.Close()
	}

	ac.wg.Wait()
}

// connectivityStateManager keeps the connectivity.State of ClientConn.
type connectivityStateManager struct {
	mu         sync.Mutex
	state      connectivity.State
	notifyChan chan struct{}
}

// updateState updates the connectivity.State of ClientConn.
// If there's a change it notifies goroutines waiting on state change to
// happen.
func (csm *connectivityStateManager) updateState(state connectivity.State) {
	csm.mu.Lock()
	defer csm.mu.Unlock()

	if csm.state == state {
		return
	}
	csm.state = state
	if csm.notifyChan != nil {
		// There are other goroutines waiting on this channel.
		notifyChan := csm.notifyChan
		csm.notifyChan = nil
		close(notifyChan)
	}
}

func (csm *connectivityStateManager) getState() connectivity.State {
	csm.mu.Lock()
	defer csm.mu.Unlock()

	return csm.state
}

func (csm *connectivityStateManager) getNotifyChan() <-chan struct{} {
	csm.mu.Lock()
	defer csm.mu.Unlock()

	if csm.notifyChan == nil {
		csm.notifyChan = make(chan struct{})
	}

	return csm.notifyChan
}
