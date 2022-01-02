package stream

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/libp2p/go-libp2p-core/connmgr"
	"github.com/libp2p/go-libp2p-core/mux"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	ipnet "github.com/libp2p/go-libp2p-core/pnet"
	"github.com/libp2p/go-libp2p-core/sec"
	"github.com/libp2p/go-libp2p-core/transport"

	pnet "github.com/libp2p/go-libp2p-pnet"
	manet "github.com/multiformats/go-multiaddr/net"
)

// ErrNilPeer is returned when attempting to upgrade an outbound connection
// without specifying a peer ID.
var ErrNilPeer = errors.New("nil peer")

// AcceptQueueLength is the number of connections to fully setup before not accepting any new connections
var AcceptQueueLength = 16

const defaultAcceptTimeout = 15 * time.Second

type Option func(*upgrader) error

func WithPSK(psk ipnet.PSK) Option {
	return func(u *upgrader) error {
		u.psk = psk
		return nil
	}
}

func WithAcceptTimeout(t time.Duration) Option {
	return func(u *upgrader) error {
		u.acceptTimeout = t
		return nil
	}
}

func WithConnectionGater(g connmgr.ConnectionGater) Option {
	return func(u *upgrader) error {
		u.connGater = g
		return nil
	}
}

// Upgrader is a multistream upgrader that can upgrade an underlying connection
// to a full transport connection (secure and multiplexed).
type upgrader struct {
	secure sec.SecureMuxer
	muxer  mux.Multiplexer

	psk       ipnet.PSK
	connGater connmgr.ConnectionGater

	// AcceptTimeout is the maximum duration an Accept is allowed to take.
	// This includes the time between accepting the raw network connection,
	// protocol selection as well as the handshake, if applicable.
	//
	// If unset, the default value (15s) is used.
	acceptTimeout time.Duration
}

var _ transport.Upgrader = &upgrader{}

func NewUpgrader(secureMuxer sec.SecureMuxer, muxer mux.Multiplexer, opts ...Option) (transport.Upgrader, error) {
	u := &upgrader{
		secure:        secureMuxer,
		muxer:         muxer,
		acceptTimeout: defaultAcceptTimeout,
	}
	for _, opt := range opts {
		if err := opt(u); err != nil {
			return nil, err
		}
	}
	return u, nil
}

func (u *upgrader) UpgradeListener(t transport.Transport, list manet.Listener) transport.Listener {
	ctx, cancel := context.WithCancel(context.Background())
	l := &listener{
		Listener:  list,
		upgrader:  u,
		transport: t,
		threshold: newThreshold(AcceptQueueLength),
		incoming:  make(chan transport.CapableConn),
		cancel:    cancel,
		ctx:       ctx,
	}
	go l.handleIncoming()
	return l
}

func (u *upgrader) UpgradeOutbound(ctx context.Context, t transport.Transport, maconn manet.Conn, p peer.ID) (transport.CapableConn, error) {
	return u.Upgrade(ctx, t, maconn, network.DirOutbound, p)
}

func (u *upgrader) UpgradeInbound(ctx context.Context, t transport.Transport, maconn manet.Conn) (transport.CapableConn, error) {
	return u.Upgrade(ctx, t, maconn, network.DirInbound, "")
}

func (u *upgrader) Upgrade(ctx context.Context, t transport.Transport, maconn manet.Conn, dir network.Direction, p peer.ID) (transport.CapableConn, error) {
	if dir == network.DirOutbound && p == "" {
		return nil, ErrNilPeer
	}
	var stat network.ConnStats
	if cs, ok := maconn.(network.ConnStat); ok {
		stat = cs.Stat()
	}

	var conn net.Conn = maconn
	if u.psk != nil {
		pconn, err := pnet.NewProtectedConn(u.psk, conn)
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("failed to setup private network protector: %s", err)
		}
		conn = pconn
	} else if ipnet.ForcePrivateNetwork {
		log.Error("tried to dial with no Private Network Protector but usage of Private Networks is forced by the environment")
		return nil, ipnet.ErrNotInPrivateNetwork
	}

	sconn, server, err := u.setupSecurity(ctx, conn, p, dir)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to negotiate security protocol: %s", err)
	}

	// call the connection gater, if one is registered.
	if u.connGater != nil && !u.connGater.InterceptSecured(dir, sconn.RemotePeer(), maconn) {
		if err := maconn.Close(); err != nil {
			log.Errorf("failed to close connection with peer %s and addr %s; err: %s",
				p.Pretty(), maconn.RemoteMultiaddr(), err)
		}
		return nil, fmt.Errorf("gater rejected connection with peer %s and addr %s with direction %d",
			sconn.RemotePeer().Pretty(), maconn.RemoteMultiaddr(), dir)
	}

	smconn, err := u.setupMuxer(ctx, sconn, server)
	if err != nil {
		sconn.Close()
		return nil, fmt.Errorf("failed to negotiate stream multiplexer: %s", err)
	}

	tc := &transportConn{
		MuxedConn:      smconn,
		ConnMultiaddrs: maconn,
		ConnSecurity:   sconn,
		transport:      t,
		stat:           stat,
	}
	return tc, nil
}

func (u *upgrader) setupSecurity(ctx context.Context, conn net.Conn, p peer.ID, dir network.Direction) (sec.SecureConn, bool, error) {
	if dir == network.DirInbound {
		return u.secure.SecureInbound(ctx, conn, p)
	}
	return u.secure.SecureOutbound(ctx, conn, p)
}

func (u *upgrader) setupMuxer(ctx context.Context, conn net.Conn, server bool) (mux.MuxedConn, error) {
	// TODO: The muxer should take a context.
	done := make(chan struct{})

	var smconn mux.MuxedConn
	var err error
	go func() {
		defer close(done)
		smconn, err = u.muxer.NewConn(conn, server)
	}()

	select {
	case <-done:
		return smconn, err
	case <-ctx.Done():
		// interrupt this process
		conn.Close()
		// wait to finish
		<-done
		return nil, ctx.Err()
	}
}
