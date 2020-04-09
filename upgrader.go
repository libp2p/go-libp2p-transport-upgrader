package stream

import (
	"context"
	"errors"
	"fmt"
	"github.com/libp2p/go-libp2p-core/network"
	"net"

	"github.com/libp2p/go-libp2p-core/connmgr"
	"github.com/libp2p/go-libp2p-core/mux"
	"github.com/libp2p/go-libp2p-core/peer"
	ipnet "github.com/libp2p/go-libp2p-core/pnet"
	"github.com/libp2p/go-libp2p-core/sec"
	"github.com/libp2p/go-libp2p-core/transport"
	"github.com/libp2p/go-libp2p-pnet"

	manet "github.com/multiformats/go-multiaddr-net"
)

// ErrNilPeer is returned when attempting to upgrade an outbound connection
// without specifying a peer ID.
var ErrNilPeer = errors.New("nil peer")

// AcceptQueueLength is the number of connections to fully setup before not accepting any new connections
var AcceptQueueLength = 16

// Upgrader is a multistream upgrader that can upgrade an underlying connection
// to a full transport connection (secure and multiplexed).
type Upgrader struct {
	PSK       ipnet.PSK
	Secure    sec.SecureTransport
	Muxer     mux.Multiplexer
	ConnGater connmgr.ConnectionGater
}

// UpgradeListener upgrades the passed multiaddr-net listener into a full libp2p-transport listener.
func (u *Upgrader) UpgradeListener(t transport.Transport, list manet.Listener) transport.Listener {
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

// UpgradeOutbound upgrades the given outbound multiaddr-net connection into a
// full libp2p-transport connection.
func (u *Upgrader) UpgradeOutbound(ctx context.Context, t transport.Transport, maconn manet.Conn, p peer.ID) (transport.CapableConn, error) {
	if p == "" {
		return nil, ErrNilPeer
	}
	return u.upgrade(ctx, t, maconn, p, network.DirOutbound)
}

// UpgradeInbound upgrades the given inbound multiaddr-net connection into a
// full libp2p-transport connection.
func (u *Upgrader) UpgradeInbound(ctx context.Context, t transport.Transport, maconn manet.Conn) (transport.CapableConn, error) {
	// We should ALSO do this in the transport, but dosen't hurt to have this here.
	if u.ConnGater != nil && !u.ConnGater.InterceptAccept(maconn) {
		return nil, processInterceptFailed(maconn, network.DirInbound, "accepted", "")
	}

	return u.upgrade(ctx, t, maconn, "", network.DirInbound)
}

func (u *Upgrader) upgrade(ctx context.Context, t transport.Transport, maconn manet.Conn, p peer.ID, dir network.Direction) (transport.CapableConn, error) {
	var conn net.Conn = maconn
	if u.PSK != nil {
		pconn, err := pnet.NewProtectedConn(u.PSK, conn)
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("failed to setup private network protector: %s", err)
		}
		conn = pconn
	} else if ipnet.ForcePrivateNetwork {
		log.Error("tried to dial with no Private Network Protector but usage" +
			" of Private Networks is forced by the enviroment")
		return nil, ipnet.ErrNotInPrivateNetwork
	}
	sconn, err := u.setupSecurity(ctx, conn, p)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to negotiate security protocol: %s", err)
	}
	// should we gate the secured connection
	if u.ConnGater != nil && !u.ConnGater.InterceptSecured(dir, sconn.RemotePeer(), maconn) {
		return nil, processInterceptFailed(maconn, dir, "secured", sconn.RemotePeer())
	}

	smconn, err := u.setupMuxer(ctx, sconn, p)
	if err != nil {
		sconn.Close()
		return nil, fmt.Errorf("failed to negotiate stream multiplexer: %s", err)
	}

	tc := &transportConn{
		MuxedConn:      smconn,
		ConnMultiaddrs: maconn,
		ConnSecurity:   sconn,
		transport:      t,
	}

	// Gater function for the upgraded connection will be applied in the Swarm as
	// we need to send a disconnect message for it.

	return tc, nil
}

func processInterceptFailed(c manet.Conn, dir network.Direction, state string, p peer.ID) error {
	errStr := fmt.Sprintf("gater blocked connection with peer %s and Addr %s with direction %d in state %s",
		p.Pretty(), c.RemoteMultiaddr().String(), dir, state)

	log.Debug(errStr)
	if err := c.Close(); err != nil {
		log.Errorf("failed to close connection with peerID %s and Addr %s, err=%s",
			p.Pretty(), c.RemoteMultiaddr().String(), err)
	}
	return errors.New(errStr)
}

func (u *Upgrader) setupSecurity(ctx context.Context, conn net.Conn, p peer.ID) (sec.SecureConn, error) {
	if p == "" {
		return u.Secure.SecureInbound(ctx, conn)
	}
	return u.Secure.SecureOutbound(ctx, conn, p)
}

func (u *Upgrader) setupMuxer(ctx context.Context, conn net.Conn, p peer.ID) (mux.MuxedConn, error) {
	// TODO: The muxer should take a context.
	done := make(chan struct{})

	var smconn mux.MuxedConn
	var err error
	go func() {
		defer close(done)
		smconn, err = u.Muxer.NewConn(conn, p == "")
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
