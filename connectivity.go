// Package connectivity defines connectivity semantics.
package wsrpc

import (
	"log"
)

// State indicates the state of connectivity.
// It can be the state of a ClientConn or SubConn.
type ConnectivityState int

func (s ConnectivityState) String() string {
	switch s {
	case ConnectivityStateIdle:
		return "IDLE"
	case ConnectivityStateConnecting:
		return "CONNECTING"
	case ConnectivityStateReady:
		return "READY"
	case ConnectivityStateTransientFailure:
		return "TRANSIENT_FAILURE"
	case ConnectivityStateShutdown:
		return "SHUTDOWN"
	default:
		log.Printf("unknown connectivity state: %d", s)
		return "Invalid-State"
	}
}

const (
	// Idle indicates the ClientConn is idle.
	ConnectivityStateIdle ConnectivityState = iota
	// Connecting indicates the ClientConn is connecting.
	ConnectivityStateConnecting
	// Ready indicates the ClientConn is ready for work.
	ConnectivityStateReady
	// TransientFailure indicates the ClientConn has seen a failure but expects to recover.
	ConnectivityStateTransientFailure
	// Shutdown indicates the ClientConn has started shutting down.
	ConnectivityStateShutdown
)
