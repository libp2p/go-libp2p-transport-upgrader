// Deprecated: This package has moved into go-libp2p as a sub-package: github.com/libp2p/go-libp2p/p2p/net/upgrader.
package upgrader

import (
	"errors"
	"time"

	"github.com/libp2p/go-libp2p-core/connmgr"
	"github.com/libp2p/go-libp2p-core/network"
	ipnet "github.com/libp2p/go-libp2p-core/pnet"
	"github.com/libp2p/go-libp2p-core/sec"
	"github.com/libp2p/go-libp2p-core/transport"

	"github.com/libp2p/go-libp2p/p2p/net/upgrader"
)

// ErrNilPeer is returned when attempting to upgrade an outbound connection
// without specifying a peer ID.
// Deprecated: use github.com/libp2p/go-libp2p/p2p/net/upgrader.ErrNilPeer instead.
var ErrNilPeer = errors.New("nil peer")

// AcceptQueueLength is the number of connections to fully setup before not accepting any new connections
// Deprecated: use github.com/libp2p/go-libp2p/p2p/net/upgrader.AcceptQueueLength instead.
var AcceptQueueLength = 16

// Deprecated: use github.com/libp2p/go-libp2p/p2p/net/upgrader.Option instead.
type Option = upgrader.Option

// Deprecated: use github.com/libp2p/go-libp2p/p2p/net/upgrader.WithPSK instead.
func WithPSK(psk ipnet.PSK) Option {
	return upgrader.WithPSK(psk)
}

// Deprecated: use github.com/libp2p/go-libp2p/p2p/net/upgrader.WithAcceptTimeout instead.
func WithAcceptTimeout(t time.Duration) Option {
	return upgrader.WithAcceptTimeout(t)
}

// Deprecated: use github.com/libp2p/go-libp2p/p2p/net/upgrader.WithConnectionGater instead.
func WithConnectionGater(g connmgr.ConnectionGater) Option {
	return upgrader.WithConnectionGater(g)
}

// Deprecated: use github.com/libp2p/go-libp2p/p2p/net/upgrader.WithResourceManager instead.
func WithResourceManager(m network.ResourceManager) Option {
	return upgrader.WithResourceManager(m)
}

// Deprecated: use github.com/libp2p/go-libp2p/p2p/net/upgrader.New instead.
func New(secureMuxer sec.SecureMuxer, muxer network.Multiplexer, opts ...Option) (transport.Upgrader, error) {
	return upgrader.New(secureMuxer, muxer, opts...)
}
