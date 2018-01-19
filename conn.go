package stream

import (
	transport "github.com/libp2p/go-libp2p-transport"
	smux "github.com/libp2p/go-stream-muxer"
)

type transportConn struct {
	smux.Conn
	transport.ConnMultiaddrs
	transport.ConnSecurity
	transport transport.Transport
}

func (t *transportConn) Transport() transport.Transport {
	return t.transport
}
