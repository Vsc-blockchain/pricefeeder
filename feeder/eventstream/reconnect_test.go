package eventstream

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

// fakeWs is a wsI that never delivers a message, standing in for a socket whose
// subscription the node silently dropped.
type fakeWs struct {
	messages chan []byte
	resets   atomic.Int64
}

func (f *fakeWs) message() <-chan []byte { return f.messages }
func (f *fakeWs) reset()                 { f.resets.Add(1) }
func (f *fakeWs) close()                 {}

// A socket that stays open at the TCP level but stops delivering blocks used to
// wedge the feeder forever: nothing errored, so nothing ever re-dialed.
func TestStreamResetsWebsocketWhenBlocksStop(t *testing.T) {
	old := BlockTimeout
	BlockTimeout = 200 * time.Millisecond
	t.Cleanup(func() { BlockTimeout = old })

	ws := &fakeWs{messages: make(chan []byte)}
	s := newStream(ws, nil, zerolog.Nop())
	t.Cleanup(s.Close)

	require.Eventually(t, func() bool { return ws.resets.Load() >= 2 },
		3*time.Second, 20*time.Millisecond,
		"stream should keep re-dialing while no blocks arrive")
}

// The read loop must recover from a connection the server tears down, and must
// not give up permanently while the node is unreachable.
func TestWebsocketReconnectsAfterServerHangup(t *testing.T) {
	var upgrades atomic.Int64
	upgrader := websocket.Upgrader{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		n := upgrades.Add(1)
		// read the subscribe message the client sends on open
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
		if n == 1 {
			// first connection: hang up immediately
			conn.Close()
			return
		}
		_ = conn.WriteMessage(websocket.TextMessage, []byte("reconnected"))
		time.Sleep(2 * time.Second)
		conn.Close()
	}))
	t.Cleanup(server.Close)

	url := "ws" + strings.TrimPrefix(server.URL, "http")
	ws := NewWebsocket(url, []byte("subscribe"), zerolog.Nop())
	t.Cleanup(ws.close)

	select {
	case msg := <-ws.message():
		require.Equal(t, []byte("reconnected"), msg)
	case <-time.After(10 * time.Second):
		t.Fatal("websocket never reconnected after the server hung up")
	}
	require.EqualValues(t, 2, upgrades.Load())
}

// reset() must unblock a read loop that is parked in ReadMessage.
func TestWebsocketResetForcesRedial(t *testing.T) {
	var upgrades atomic.Int64
	upgrader := websocket.Upgrader{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		upgrades.Add(1)
		// go silent: never write anything, never hang up
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	t.Cleanup(server.Close)

	url := "ws" + strings.TrimPrefix(server.URL, "http")
	ws := NewWebsocket(url, []byte("subscribe"), zerolog.Nop())
	t.Cleanup(ws.close)

	require.Eventually(t, func() bool { return upgrades.Load() == 1 },
		5*time.Second, 20*time.Millisecond, "initial connection")

	ws.reset()

	require.Eventually(t, func() bool { return upgrades.Load() == 2 },
		5*time.Second, 20*time.Millisecond, "reset should trigger a re-dial")
}
