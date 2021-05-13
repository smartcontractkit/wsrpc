package wsrpc

// state of transport
type transportState int

const (
	reachable transportState = iota
	closing
)
