package stream

import (
	"fmt"

	"github.com/libp2p/go-libp2p-core/mux"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/transport"

	ma "github.com/multiformats/go-multiaddr"
)

type multiaddrConn struct {
	local, remote ma.Multiaddr
}

var _ network.ConnMultiaddrs = &multiaddrConn{}

func newMultiaddrConn(conn network.ConnMultiaddrs, proto ma.Protocol) network.ConnMultiaddrs {
	return &multiaddrConn{
		local:  addSecurityProtocol(conn.LocalMultiaddr(), proto),
		remote: addSecurityProtocol(conn.RemoteMultiaddr(), proto),
	}
}

func (c *multiaddrConn) LocalMultiaddr() ma.Multiaddr  { return c.local }
func (c *multiaddrConn) RemoteMultiaddr() ma.Multiaddr { return c.remote }

type transportConn struct {
	mux.MuxedConn
	network.ConnMultiaddrs
	network.ConnSecurity
	transport transport.Transport
	stat      network.Stat
}

func (t *transportConn) Transport() transport.Transport {
	return t.transport
}

func (t *transportConn) String() string {
	ts := ""
	if s, ok := t.transport.(fmt.Stringer); ok {
		ts = "[" + s.String() + "]"
	}
	return fmt.Sprintf(
		"<stream.Conn%s %s (%s) <-> %s (%s)>",
		ts,
		t.LocalMultiaddr(),
		t.LocalPeer(),
		t.RemoteMultiaddr(),
		t.RemotePeer(),
	)
}

func (t *transportConn) Stat() network.Stat {
	return t.stat
}

func addSecurityProtocol(addr ma.Multiaddr, proto ma.Protocol) ma.Multiaddr {
	return addr.Encapsulate(ma.Cast(proto.VCode))
}
