package eventstream

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

const (
	// pongWait bounds how long we sit inside ReadMessage without hearing anything
	// at all from the node. CometBFT pings its websocket clients every ~30s, so a
	// longer silence means the socket is a black hole and has to be re-dialed.
	// Without a deadline a half-open TCP connection parks ReadMessage forever.
	pongWait = 70 * time.Second
	// writeWait bounds the pong and subscribe writes so a stuck socket cannot
	// wedge the read loop.
	writeWait = 10 * time.Second
	// maxDialBackoff caps the reconnect backoff so a long node outage does not
	// turn into an hours-long sleep.
	maxDialBackoff = 30 * time.Second
	// readBuffer lets the read loop keep draining the socket while the feeder is
	// busy posting prices. If we blocked here instead, the node would eventually
	// drop us as a slow consumer.
	readBuffer = 16
)

type dianFn func() (*websocket.Conn, error)

type ws struct {
	logger           zerolog.Logger
	stopSignal       chan struct{} // external signal to stop the ws
	done             chan struct{} // internal signal to wait for the ws to execute its shutdown operations
	read             chan []byte
	dial             dianFn
	connMutex        sync.Mutex
	connection       *websocket.Conn
	connectionClosed *atomic.Bool
}

func NewWebsocket(url string, onOpenMsg []byte, logger zerolog.Logger) *ws {
	dialFunction := func() (*websocket.Conn, error) {
		conn, _, err := websocket.DefaultDialer.Dial(url, nil)
		if err != nil {
			return nil, err
		}

		// Control frames refresh the read deadline, so an idle-but-healthy
		// connection stays up while a dead one trips it.
		if err := conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
			conn.Close()
			return nil, err
		}
		conn.SetPongHandler(func(string) error {
			return conn.SetReadDeadline(time.Now().Add(pongWait))
		})
		conn.SetPingHandler(func(appData string) error {
			if err := conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
				return err
			}
			if err := conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				return err
			}
			if err := conn.WriteMessage(websocket.PongMessage, []byte(appData)); err != nil && err != websocket.ErrCloseSent {
				return err
			}
			return nil
		})

		if err := conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
			conn.Close()
			return nil, err
		}
		if err := conn.WriteMessage(websocket.BinaryMessage, onOpenMsg); err != nil {
			conn.Close()
			return nil, err
		}
		return conn, nil
	}

	ws := &ws{
		logger:           logger.With().Str("component", "websocket").Logger(),
		stopSignal:       make(chan struct{}),
		done:             make(chan struct{}),
		read:             make(chan []byte, readBuffer),
		dial:             dialFunction,
		connection:       nil,
		connectionClosed: new(atomic.Bool),
	}

	go ws.loop()
	return ws
}

func (w *ws) loop() {
	defer close(w.done)

	// read messages and also handles reconnection.
	for {
		conn := w.conn()
		if conn == nil {
			if w.stopped() {
				return
			}
			w.connect()
			continue
		}

		if err := conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
			w.reset()
			continue
		}

		_, bytes, err := conn.ReadMessage()
		if err != nil {
			if w.stopped() {
				// if the connection was closed, then we exit
				return
			}

			// otherwise we attempt to reconnect
			// we don't care if it fails, because if it does on ReadMessage we will receive an error
			// and then attempt to reconnect again.
			w.logger.Err(err).Msg("disconnected from websocket, attempting to reconnect")
			w.connect()
			continue
		}

		// no error, forward the msg
		select {
		case w.read <- bytes:
			w.logger.Debug().Str("payload", string(bytes)).Msg("message received")
		case <-w.stopSignal:
			w.logger.Warn().Str("payload", string(bytes)).Msg("message dropped due to shutdown")
			return
		}
	}
}

// attemps to dial the websocket using binary exponential backoff
func (w *ws) connect() {
	w.logger.Debug().Msg("connecting")

	// Drop whatever we were holding first: a half-open socket would otherwise
	// leak its file descriptor for the lifetime of the process.
	w.closeConn()

	delay := 1 * time.Second
	for attempt := 1; ; attempt++ {
		if w.stopped() {
			return
		}

		connection, err := w.dial()
		if err == nil {
			w.setConn(connection)
			w.logger.Info().Int("attempt", attempt).Msg("connected to websocket")
			return
		}

		// Keep retrying indefinitely. Giving up here used to kill the process,
		// but a feeder that dies during a node restart never comes back, and a
		// capped backoff costs nothing.
		w.logger.Err(err).Int("attempt", attempt).Dur("retry-in", delay).Msg("failed to connect to websocket, retrying")

		select {
		case <-time.After(delay):
		case <-w.stopSignal:
			return
		}

		if delay *= 2; delay > maxDialBackoff {
			delay = maxDialBackoff
		}
	}
}

// reset tears down the current connection so the read loop re-dials. Safe to
// call from any goroutine; used when the socket is healthy at the TCP level but
// the subscription behind it has gone silent.
func (w *ws) reset() {
	w.closeConn()
}

func (w *ws) conn() *websocket.Conn {
	w.connMutex.Lock()
	defer w.connMutex.Unlock()
	return w.connection
}

func (w *ws) setConn(conn *websocket.Conn) {
	w.connMutex.Lock()
	defer w.connMutex.Unlock()
	w.connection = conn
}

func (w *ws) closeConn() {
	w.connMutex.Lock()
	defer w.connMutex.Unlock()
	if w.connection != nil {
		if err := w.connection.Close(); err != nil {
			w.logger.Debug().Err(err).Msg("close error")
		}
		w.connection = nil
	}
}

func (w *ws) stopped() bool {
	if w.connectionClosed.Load() {
		return true
	}
	select {
	case <-w.stopSignal:
		return true
	default:
		return false
	}
}

func (w *ws) message() <-chan []byte {
	return w.read
}

func (w *ws) close() {
	w.connectionClosed.Store(true)
	close(w.stopSignal)
	w.closeConn()
	<-w.done
}
