package wsrpc

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/smartcontractkit/wsrpc/internal/backoff"
)

// TODO - Figure out how to deal with write/read deadlines

var (
	// errConnClosing indicates that the connection is closing.
	errConnClosing = errors.New("grpc: the connection is closing")
)

// ClientConn represents a virtual connection to a conceptual endpoint, to
// perform RPCs.
//
type ClientConn struct {
	ctx context.Context
	mu  sync.RWMutex

	target string
	csCh   <-chan ConnectivityState

	dopts dialOptions
	conn  *addrConn

	// readFn contains the registered handler for reading messages
	readFn func(message []byte)
}

// Dial creates a client connection to the given target.
func Dial(target string, opts ...DialOption) (*ClientConn, error) {
	cc := &ClientConn{
		ctx:    context.Background(),
		target: target,
		dopts:  defaultDialOptions(),
	}

	for _, opt := range opts {
		opt.apply(&cc.dopts)
	}

	// Set the backoff strategy. We may need to consider making this
	// customizable in the dial options.
	cc.dopts.bs = backoff.DefaultExponential

	addrConn, err := cc.newAddrConn(target)
	if err != nil {
		return nil, errors.New("Could not establish a connection")
	}

	addrConn.connect()
	cc.conn = addrConn

	return cc, nil
}

// newAddrConn creates an addrConn for the addr and sets it to cc.conn.
func (cc *ClientConn) newAddrConn(addr string) (*addrConn, error) {
	csCh := make(chan ConnectivityState)
	ac := &addrConn{
		state:   ConnectivityStateIdle,
		stateCh: csCh,
		cc:      cc,
		addr:    addr,
		dopts:   cc.dopts,

		// resetBackoff: make(chan struct{}),
	}
	ac.ctx, ac.cancel = context.WithCancel(cc.ctx)
	// Track ac in cc. This needs to be done before any getTransport(...) is called.
	cc.mu.Lock()
	// if cc.conn == nil {
	// 	cc.mu.Unlock()
	// 	return nil, ErrClientConnClosing
	// }

	cc.conn = ac
	cc.csCh = csCh
	cc.mu.Unlock()

	// Register handlers when the state changes
	go cc.listen()

	return ac, nil
}

// TODO - Rename
// listen for the connectivty state to be ready and enable the handler
// TODO - Shutdown the routine during when the client is closed
func (cc *ClientConn) listen() {
	for {
		s := <-cc.csCh

		var done chan struct{}

		if s == ConnectivityStateReady {
			done := make(chan struct{})
			go cc.startRead(done)
		} else {
			if done != nil {
				close(done)
			}
		}
	}
}

// TODO - Rename
func (cc *ClientConn) startRead(done <-chan struct{}) {
	for {
		select {

		case msg := <-cc.conn.transport.Read():
			cc.readFn(msg)
		case <-done:
			return
		}
	}
}

// Close tears down the ClientConn and all underlying connections.
func (cc *ClientConn) Close() {
	conn := cc.conn

	cc.mu.Lock()
	cc.conn = nil
	cc.mu.Unlock()

	conn.teardown()
}

// TODO - Figure out a way to return errors
func (cc *ClientConn) Send(message string) error {
	if cc.conn.state != ConnectivityStateReady {
		return errors.New("connection is not ready")
	}

	cc.conn.transport.Write([]byte(message))

	return nil
}

func (cc *ClientConn) RegisterHandler(handler func(message []byte)) {
	cc.readFn = handler
}

// addrConn is a network connection to a given address.
type addrConn struct {
	ctx    context.Context
	cancel context.CancelFunc

	cc *ClientConn

	addr  string
	dopts dialOptions

	// transport is set when there's a viable transport, and is reset
	// to nil when the current transport should no longer be used (e.g.
	// after transport is closed, ac has been torn down).
	transport ClientTransport // The current transport.

	mu sync.Mutex

	// Use updateConnectivityState for updating addrConn's connectivity state.
	state ConnectivityState
	// Notifies this channel when the ConnectivityState changes
	stateCh chan ConnectivityState

	// resetBackoff chan struct{}
}

func (ac *addrConn) connect() error {
	ac.mu.Lock()
	if ac.state == ConnectivityStateShutdown {
		ac.mu.Unlock()
		return errConnClosing
	}

	if ac.state != ConnectivityStateIdle {
		ac.mu.Unlock()
		return nil
	}

	// Update connectivity state within the lock to prevent subsequent or
	// concurrent calls from resetting the transport more than once.
	ac.updateConnectivityState(ConnectivityStateConnecting, nil)
	ac.mu.Unlock()

	// Start a goroutine connecting to the server asynchronously.
	go ac.resetTransport()

	return nil
}

// Note: this requires a lock on ac.mu.
func (ac *addrConn) updateConnectivityState(s ConnectivityState, lastErr error) {
	if ac.state == s {
		return
	}
	ac.state = s
	ac.stateCh <- s
	log.Printf("[AddrConn] Connectivity State: %s", s)
}

func (ac *addrConn) resetTransport() {
	for i := 0; ; i++ {
		ac.mu.Lock()
		if ac.state == ConnectivityStateShutdown {
			ac.mu.Unlock()
			return
		}

		backoffFor := ac.dopts.bs.NextBackOff()
		addr := ac.addr
		copts := ac.dopts.copts

		ac.updateConnectivityState(ConnectivityStateConnecting, nil)
		ac.transport = nil
		ac.mu.Unlock()

		newTr, reconnect, err := ac.createTransport(addr, copts)
		if err != nil {
			// After connection failure, the addrConn enters TRANSIENT_FAILURE.
			ac.mu.Lock()
			if ac.state == ConnectivityStateShutdown {
				ac.mu.Unlock()
				return
			}
			ac.updateConnectivityState(ConnectivityStateTransientFailure, err)
			ac.mu.Unlock()

			// Backoff.
			timer := time.NewTimer(backoffFor)
			log.Printf("[AddrConn] Waiting %s to reconnect", backoffFor)
			select {
			case <-timer.C:
			// case <-b:
			// 	timer.Stop()
			case <-ac.ctx.Done():
				timer.Stop()
				return
			}
			continue
		}

		// Close the transport if in a shutdown state
		ac.mu.Lock()
		if ac.state == ConnectivityStateShutdown {
			ac.mu.Unlock()
			newTr.Close()
			return
		}
		ac.transport = newTr
		ac.dopts.bs.Reset()

		ac.updateConnectivityState(ConnectivityStateReady, nil)

		ac.mu.Unlock()

		// Block until the created transport is down. When this happens, we
		// attempt to reconnect by starting again from the top
		<-reconnect.Done()
		// hcancel()
	}

}

func (ac *addrConn) createTransport(addr string, copts ConnectOptions) (ClientTransport, *Event, error) {
	reconnect := NewEvent()
	once := sync.Once{}

	// Called when the transport closes
	onClose := func() {
		ac.mu.Lock()
		once.Do(func() {
			if ac.state == ConnectivityStateReady {
				ac.updateConnectivityState(ConnectivityStateIdle, nil)
			}
		})
		ac.mu.Unlock()
		// close(onCloseCalled)
		reconnect.Fire()
	}

	tr, err := NewWebsocketClient(ac.cc.ctx, addr, copts, onClose)

	return tr, reconnect, err
}

// tearDown starts to tear down the addrConn.
func (ac *addrConn) teardown() {
	ac.mu.Lock()

	if ac.state == ConnectivityStateShutdown {
		ac.mu.Unlock()
		return
	}

	curTr := ac.transport
	ac.transport = nil

	ac.cancel()
	curTr.Close()

	ac.mu.Unlock()
}
