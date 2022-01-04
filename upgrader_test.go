package stream_test

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/mux"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/sec/insecure"
	"github.com/libp2p/go-libp2p-core/test"
	"github.com/libp2p/go-libp2p-core/transport"

	mplex "github.com/libp2p/go-libp2p-mplex"
	st "github.com/libp2p/go-libp2p-transport-upgrader"

	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"

	"github.com/stretchr/testify/require"
)

func createUpgrader(t *testing.T, opts ...st.Option) (peer.ID, transport.Upgrader) {
	return createUpgraderWithMuxer(t, &negotiatingMuxer{}, opts...)
}

func createUpgraderWithMuxer(t *testing.T, muxer mux.Multiplexer, opts ...st.Option) (peer.ID, transport.Upgrader) {
	priv, _, err := test.RandTestKeyPair(crypto.Ed25519, 256)
	require.NoError(t, err)
	id, err := peer.IDFromPrivateKey(priv)
	require.NoError(t, err)
	u, err := st.New(&MuxAdapter{tpt: insecure.NewWithIdentity(id, priv)}, muxer, opts...)
	require.NoError(t, err)
	return id, u
}

// negotiatingMuxer sets up a new mplex connection
// It makes sure that this happens at the same time for client and server.
type negotiatingMuxer struct{}

func (m *negotiatingMuxer) NewConn(c net.Conn, isServer bool) (mux.MuxedConn, error) {
	var err error
	// run a fake muxer negotiation
	if isServer {
		_, err = c.Write([]byte("setup"))
	} else {
		_, err = c.Read(make([]byte, 5))
	}
	if err != nil {
		return nil, err
	}
	return mplex.DefaultTransport.NewConn(c, isServer)
}

// blockingMuxer blocks the muxer negotiation until the contain chan is closed
type blockingMuxer struct {
	unblock chan struct{}
}

var _ mux.Multiplexer = &blockingMuxer{}

func newBlockingMuxer() *blockingMuxer {
	return &blockingMuxer{unblock: make(chan struct{})}
}

func (m *blockingMuxer) NewConn(c net.Conn, isServer bool) (mux.MuxedConn, error) {
	<-m.unblock
	return (&negotiatingMuxer{}).NewConn(c, isServer)
}

func (m *blockingMuxer) Unblock() {
	close(m.unblock)
}

// errorMuxer is a muxer that errors while setting up
type errorMuxer struct{}

var _ mux.Multiplexer = &errorMuxer{}

func (m *errorMuxer) NewConn(c net.Conn, isServer bool) (mux.MuxedConn, error) {
	return nil, errors.New("mux error")
}

func testConn(t *testing.T, clientConn, serverConn transport.CapableConn) {
	t.Helper()
	require := require.New(t)

	cstr, err := clientConn.OpenStream(context.Background())
	require.NoError(err)

	_, err = cstr.Write([]byte("foobar"))
	require.NoError(err)

	sstr, err := serverConn.AcceptStream()
	require.NoError(err)

	b := make([]byte, 6)
	_, err = sstr.Read(b)
	require.NoError(err)
	require.Equal([]byte("foobar"), b)
}

func dial(t *testing.T, upgrader transport.Upgrader, raddr ma.Multiaddr, p peer.ID) (transport.CapableConn, error) {
	t.Helper()

	macon, err := manet.Dial(raddr)
	if err != nil {
		return nil, err
	}
	return upgrader.Upgrade(context.Background(), nil, macon, network.DirOutbound, p)
}

func TestOutboundConnectionGating(t *testing.T) {
	require := require.New(t)

	id, upgrader := createUpgrader(t)
	ln := createListener(t, upgrader)
	defer ln.Close()

	testGater := &testGater{}
	_, dialUpgrader := createUpgrader(t, st.WithConnectionGater(testGater))
	conn, err := dial(t, dialUpgrader, ln.Multiaddr(), id)
	require.NoError(err)
	require.NotNil(conn)
	_ = conn.Close()

	// blocking accepts doesn't affect the dialling side, only the listener.
	testGater.BlockAccept(true)
	conn, err = dial(t, dialUpgrader, ln.Multiaddr(), id)
	require.NoError(err)
	require.NotNil(conn)
	_ = conn.Close()

	// now let's block all connections after being secured.
	testGater.BlockSecured(true)
	conn, err = dial(t, dialUpgrader, ln.Multiaddr(), id)
	require.Error(err)
	require.Contains(err.Error(), "gater rejected connection")
	require.Nil(conn)

}
