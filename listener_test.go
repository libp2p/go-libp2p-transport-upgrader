package stream_test

import (
	"context"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/sec/insecure"
	"github.com/libp2p/go-libp2p-core/transport"

	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr-net"

	st "github.com/libp2p/go-libp2p-transport-upgrader"

	"github.com/stretchr/testify/require"
)

func init() {
	transport.AcceptTimeout = 1 * time.Hour
}

func createListener(t *testing.T, upgrader *st.Upgrader) transport.Listener {
	t.Helper()
	require := require.New(t)

	addr, err := ma.NewMultiaddr("/ip4/0.0.0.0/tcp/0")
	require.NoError(err)

	ln, err := manet.Listen(addr)
	require.NoError(err)

	return upgrader.UpgradeListener(nil, ln)
}

func TestAcceptSingleConn(t *testing.T) {
	require := require.New(t)

	ln := createListener(t, defaultUpgrader)
	defer ln.Close()

	cconn, err := dial(t, defaultUpgrader, ln.Multiaddr(), peer.ID("1"))
	require.NoError(err)

	sconn, err := ln.Accept()
	require.NoError(err)

	testConn(t, cconn, sconn)
}

func TestAcceptMultipleConns(t *testing.T) {
	require := require.New(t)

	ln := createListener(t, defaultUpgrader)
	defer ln.Close()

	var toClose []io.Closer
	defer func() {
		for _, c := range toClose {
			_ = c.Close()
		}
	}()

	for i := 0; i < 10; i++ {
		cconn, err := dial(t, defaultUpgrader, ln.Multiaddr(), peer.ID("1"))
		require.NoError(err)
		toClose = append(toClose, cconn)

		sconn, err := ln.Accept()
		require.NoError(err)
		toClose = append(toClose, sconn)

		testConn(t, cconn, sconn)
	}
}

func TestConnectionsClosedIfNotAccepted(t *testing.T) {
	require := require.New(t)

	const timeout = 200 * time.Millisecond
	transport.AcceptTimeout = timeout
	defer func() { transport.AcceptTimeout = 1 * time.Hour }()

	ln := createListener(t, defaultUpgrader)
	defer ln.Close()

	conn, err := dial(t, defaultUpgrader, ln.Multiaddr(), peer.ID("2"))
	require.NoError(err)

	errCh := make(chan error)
	go func() {
		defer conn.Close()
		str, err := conn.OpenStream(context.Background())
		if err != nil {
			errCh <- err
			return
		}
		// start a Read. It will block until the connection is closed
		_, _ = str.Read([]byte{0})
		errCh <- nil
	}()

	time.Sleep(timeout / 2)
	select {
	case err := <-errCh:
		t.Fatalf("connection closed earlier than expected. expected nothing on channel, got: %v", err)
	default:
	}

	time.Sleep(timeout)
	require.Nil(<-errCh)
}

func TestFailedUpgradeOnListen(t *testing.T) {
	require := require.New(t)

	upgrader := &st.Upgrader{
		Secure: insecure.New(peer.ID("1")),
		Muxer:  &errorMuxer{},
	}

	ln := createListener(t, upgrader)
	defer ln.Close()

	errCh := make(chan error)
	go func() {
		_, err := ln.Accept()
		errCh <- err
	}()

	_, err := dial(t, defaultUpgrader, ln.Multiaddr(), peer.ID("2"))
	require.Error(err)

	// close the listener.
	ln.Close()
	require.Error(<-errCh)
}

func TestListenerClose(t *testing.T) {
	require := require.New(t)

	ln := createListener(t, defaultUpgrader)

	errCh := make(chan error)
	go func() {
		_, err := ln.Accept()
		errCh <- err
	}()

	select {
	case err := <-errCh:
		t.Fatalf("connection closed earlier than expected. expected nothing on channel, got: %v", err)
	case <-time.After(200 * time.Millisecond):
		// nothing in 200ms.
	}

	// unblocks Accept when it is closed.
	err := ln.Close()
	require.NoError(err)
	err = <-errCh
	require.Error(err)
	require.Contains(err.Error(), "use of closed network connection")

	// doesn't accept new connections when it is closed
	_, err = dial(t, defaultUpgrader, ln.Multiaddr(), peer.ID("1"))
	require.Error(err)
}

func TestListenerCloseClosesQueued(t *testing.T) {
	require := require.New(t)

	ln := createListener(t, defaultUpgrader)

	var conns []transport.CapableConn
	for i := 0; i < 10; i++ {
		conn, err := dial(t, defaultUpgrader, ln.Multiaddr(), peer.ID(fmt.Sprintf("%d", i)))
		require.NoError(err)
		conns = append(conns, conn)
	}

	// wait for all the dials to happen.
	time.Sleep(500 * time.Millisecond)

	// all the connections are opened.
	for _, c := range conns {
		require.False(c.IsClosed())
	}

	// expect that all the connections will be closed.
	err := ln.Close()
	require.NoError(err)

	// all the connections are closed.
	require.Eventually(func() bool {
		for _, c := range conns {
			if !c.IsClosed() {
				return false
			}
		}
		return true
	}, 3*time.Second, 100*time.Millisecond)

	for _, c := range conns {
		_ = c.Close()
	}
}

func TestConcurrentAccept(t *testing.T) {
	var (
		require       = require.New(t)
		num           = 3 * st.AcceptQueueLength
		blockingMuxer = newBlockingMuxer()
		upgrader      = &st.Upgrader{
			Secure: insecure.New(peer.ID("1")),
			Muxer:  blockingMuxer,
		}
	)

	ln := createListener(t, upgrader)
	defer ln.Close()

	accepted := make(chan transport.CapableConn, num)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
			accepted <- conn
		}
	}()

	// start num dials, which all block while setting up the muxer
	errCh := make(chan error, num)
	var wg sync.WaitGroup
	for i := 0; i < num; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			conn, err := dial(t, defaultUpgrader, ln.Multiaddr(), peer.ID("2"))
			if err != nil {
				errCh <- err
				return
			}
			defer conn.Close()

			_, err = conn.AcceptStream() // wait for conn to be accepted.
			errCh <- err
		}()
	}

	time.Sleep(200 * time.Millisecond)
	// the dials are still blocked, so we shouldn't have any connection available yet
	require.Empty(accepted)
	blockingMuxer.Unblock() // make all dials succeed
	require.Eventually(func() bool { return len(accepted) == num }, 3*time.Second, 100*time.Millisecond)
	wg.Wait()
}

func TestAcceptQueueBacklogged(t *testing.T) {
	require := require.New(t)

	ln := createListener(t, defaultUpgrader)
	defer ln.Close()

	// setup AcceptQueueLength connections, but don't accept any of them
	errCh := make(chan error, st.AcceptQueueLength+1)
	doDial := func() {
		conn, err := dial(t, defaultUpgrader, ln.Multiaddr(), peer.ID("2"))
		errCh <- err
		if conn != nil {
			_ = conn.Close()
		}
	}

	for i := 0; i < st.AcceptQueueLength; i++ {
		go doDial()
	}

	require.Eventually(func() bool { return len(errCh) == st.AcceptQueueLength }, 2*time.Second, 100*time.Millisecond)

	// dial a new connection. This connection should not complete setup, since the queue is full
	go doDial()

	time.Sleep(500 * time.Millisecond)
	require.Len(errCh, st.AcceptQueueLength)

	// accept a single connection. Now the new connection should be set up, and fill the queue again
	conn, err := ln.Accept()
	require.NoError(err)
	_ = conn.Close()

	require.Eventually(func() bool { return len(errCh) == st.AcceptQueueLength+1 }, 2*time.Second, 100*time.Millisecond)
}

func TestListenerConnectionGater(t *testing.T) {
	require := require.New(t)

	testGater := &testGater{}
	upgrader := *defaultUpgrader
	upgrader.ConnGater = testGater

	ln := createListener(t, &upgrader)
	defer ln.Close()

	// no gating.
	conn, err := dial(t, defaultUpgrader, ln.Multiaddr(), peer.ID("0"))
	require.NoError(err)
	require.False(conn.IsClosed())
	_ = conn.Close()

	// rejecting after handshake.
	testGater.BlockSecured(true)
	testGater.BlockAccept(false)
	conn, err = dial(t, defaultUpgrader, ln.Multiaddr(), peer.ID("0"))
	require.Error(err)
	require.Nil(conn)

	// rejecting on accept will trigger first.
	testGater.BlockSecured(true)
	testGater.BlockAccept(true)
	conn, err = dial(t, defaultUpgrader, ln.Multiaddr(), peer.ID("0"))
	require.Error(err)
	require.Nil(conn)

	// rejecting only on acceptance.
	testGater.BlockSecured(false)
	testGater.BlockAccept(true)
	conn, err = dial(t, defaultUpgrader, ln.Multiaddr(), peer.ID("0"))
	require.Error(err)
	require.Nil(conn)

	// back to normal
	testGater.BlockSecured(false)
	testGater.BlockAccept(false)
	conn, err = dial(t, defaultUpgrader, ln.Multiaddr(), peer.ID("0"))
	require.NoError(err)
	require.False(conn.IsClosed())
	_ = conn.Close()
}
