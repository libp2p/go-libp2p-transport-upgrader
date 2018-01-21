package stream

import (
	"context"
	"fmt"
	"sync"

	logging "github.com/ipfs/go-log"
	tec "github.com/jbenet/go-temp-err-catcher"
	transport "github.com/libp2p/go-libp2p-transport"
	manet "github.com/multiformats/go-multiaddr-net"
)

var log = logging.Logger("stream-upgrader")

type connErr struct {
	conn transport.Conn
	err  error
}

type listener struct {
	manet.Listener

	transport transport.Transport
	upgrader  *Upgrader

	incoming chan transport.Conn
	err      error

	ticket chan struct{}

	ctx    context.Context
	cancel func()
}

// Close closes the listener.
func (l *listener) Close() error {
	// Do this first to try to get any relevent errors.
	err := l.Listener.Close()

	l.cancel()
	// Drain and wait.
	for c := range l.incoming {
		c.Close()
	}
	return err
}

func (l *listener) handleIncoming() {
	var wg sync.WaitGroup
	defer func() {
		// make sure we're closed
		l.Listener.Close()
		if l.err == nil {
			l.err = fmt.Errorf("listener closed")
		}

		wg.Wait()
		close(l.incoming)
	}()

	var catcher tec.TempErrCatcher
	for {

		select {
		case <-l.ticket:
		case <-l.ctx.Done():
			return
		}

		maconn, err := l.Listener.Accept()
		if err != nil {
			if catcher.IsTemporary(err) {
				log.Infof("temporary accept error: %s", err)
				continue
			}
			l.err = err
			return
		}

		log.Debugf("listener %s got connection: %s <---> %s",
			l,
			maconn.LocalMultiaddr(),
			maconn.RemoteMultiaddr())

		wg.Add(1)
		go func() {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(l.ctx, transport.AcceptTimeout)
			defer cancel()

			conn, err := l.upgrader.UpgradeInbound(ctx, l.transport, maconn)
			if err != nil {
				// Don't bother bubbling this up. We just failed
				// to completely negotiate the connection.
				log.Debugf("accept upgrade error: %s (%s <--> %s)",
					err,
					maconn.LocalMultiaddr(),
					maconn.RemoteMultiaddr())
				return
			}

			log.Debugf("listener %s accepted connection: %s", l, conn)

			// Wait on the context with a timeout. This way,
			// if we stop accepting connections for some reason,
			// we'll eventually close all the open ones
			// instead of hanging onto them.
			select {
			case l.incoming <- conn:
			case <-ctx.Done():
				if l.ctx.Err() == context.DeadlineExceeded {
					// Listener *not* closed but the accept timeout expired.
					log.Warningf("listener dropped connection due to slow accept")
				}
				conn.Close()
			}
		}()
	}
}

// Accept accepts a connection.
func (l *listener) Accept() (transport.Conn, error) {
	for {
		select {
		case l.ticket <- struct{}{}:
		case c, ok := <-l.incoming:
			if !ok {
				return nil, l.err
			}
			return c, nil
		}
	}
}

func (l *listener) String() string {
	if s, ok := l.transport.(fmt.Stringer); ok {
		return fmt.Sprintf("<stream.Listener[%s] %s>", s, l.Multiaddr())
	}
	return fmt.Sprintf("<stream.Listener %s>", l.Multiaddr())
}

var _ transport.Listener = (*listener)(nil)
