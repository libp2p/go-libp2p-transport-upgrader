package stream_test

import (
	"github.com/libp2p/go-libp2p-core/connmgr"
	"github.com/libp2p/go-libp2p-core/control"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"

	ma "github.com/multiformats/go-multiaddr"
)

type testGater struct {
	// mutations are not thread-safe.
	BlockAccept, BlockSecured bool
}

var _ connmgr.ConnectionGater = (*testGater)(nil)

func (t *testGater) InterceptPeerDial(p peer.ID) (allow bool) {
	panic("not implemented")
}

func (t *testGater) InterceptAddrDial(id peer.ID, multiaddr ma.Multiaddr) (allow bool) {
	panic("not implemented")
}

func (t *testGater) InterceptAccept(multiaddrs network.ConnMultiaddrs) (allow bool) {
	return !t.BlockAccept
}

func (t *testGater) InterceptSecured(direction network.Direction, id peer.ID, multiaddrs network.ConnMultiaddrs) (allow bool) {
	return !t.BlockSecured
}

func (t *testGater) InterceptUpgraded(conn network.Conn) (allow bool, reason control.DisconnectReason) {
	panic("not implemented")
}
