package ws

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHub_RegisterUnregister(t *testing.T) {
	h := NewHub()
	c := &Client{ID: "c1", Send: make(chan []byte, 10)}
	h.Register(c)
	assert.Equal(t, 1, h.ClientCount())
	h.Unregister(c)
	assert.Equal(t, 0, h.ClientCount())
}

func TestHub_JoinLeaveRoom(t *testing.T) {
	h := NewHub()
	c := &Client{ID: "c1", Rooms: make(map[string]bool), Send: make(chan []byte, 10)}
	h.Register(c)
	h.JoinRoom("room1", c)
	assert.Equal(t, 1, h.RoomMemberCount("room1"))

	h.LeaveRoom("room1", c)
	assert.Equal(t, 0, h.RoomMemberCount("room1"))
}

func TestHub_BroadcastToRoom(t *testing.T) {
	h := NewHub()
	c1 := &Client{ID: "c1", Rooms: make(map[string]bool), Send: make(chan []byte, 10)}
	c2 := &Client{ID: "c2", Rooms: make(map[string]bool), Send: make(chan []byte, 10)}
	h.Register(c1)
	h.Register(c2)
	h.JoinRoom("room1", c1)
	h.JoinRoom("room1", c2)

	h.BroadcastToRoom("room1", Message{Payload: []byte("hello")})

	select {
	case msg := <-c1.Send:
		assert.Equal(t, "hello", string(msg))
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for message")
	}
	select {
	case msg := <-c2.Send:
		assert.Equal(t, "hello", string(msg))
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func TestHub_BroadcastToRoomExclude(t *testing.T) {
	h := NewHub()
	c1 := &Client{ID: "c1", Rooms: make(map[string]bool), Send: make(chan []byte, 10)}
	c2 := &Client{ID: "c2", Rooms: make(map[string]bool), Send: make(chan []byte, 10)}
	h.Register(c1)
	h.Register(c2)
	h.JoinRoom("room1", c1)
	h.JoinRoom("room1", c2)

	h.BroadcastToRoom("room1", Message{Payload: []byte("hi")}, "c1")

	select {
	case <-c1.Send:
		t.Fatal("c1 should not receive message")
	default:
	}
	select {
	case msg := <-c2.Send:
		assert.Equal(t, "hi", string(msg))
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

func TestHub_SendToClient(t *testing.T) {
	h := NewHub()
	c := &Client{ID: "c1", Send: make(chan []byte, 10)}
	h.Register(c)

	ok := h.SendToClient("c1", Message{Payload: []byte("direct")})
	assert.True(t, ok)

	select {
	case msg := <-c.Send:
		assert.Equal(t, "direct", string(msg))
	default:
		t.Fatal("no message")
	}

	ok = h.SendToClient("nonexistent", Message{})
	assert.False(t, ok)
}

func TestHub_SendToUser(t *testing.T) {
	h := NewHub()
	c1 := &Client{ID: "c1", UserID: "u1", Send: make(chan []byte, 10)}
	c2 := &Client{ID: "c2", UserID: "u2", Send: make(chan []byte, 10)}
	h.Register(c1)
	h.Register(c2)

	sent := h.SendToUser("u1", Message{Payload: []byte("user-msg")})
	assert.True(t, sent)

	select {
	case msg := <-c1.Send:
		assert.Equal(t, "user-msg", string(msg))
	default:
		t.Fatal("no message")
	}
}

func TestHub_HandleMessageType(t *testing.T) {
	h := NewHub()
	handled := false
	h.HandleMessageType("ping", func(ctx context.Context, client *Client, msg Message) {
		handled = true
	})
	h.OnMessage(&Client{ID: "c1"}, Message{Type: "ping"})
	assert.True(t, handled)
}

func TestHub_BroadcastAll(t *testing.T) {
	h := NewHub()
	c1 := &Client{ID: "c1", Send: make(chan []byte, 10)}
	c2 := &Client{ID: "c2", Send: make(chan []byte, 10)}
	h.Register(c1)
	h.Register(c2)

	h.BroadcastAll(Message{Payload: []byte("all")})

	assert.Equal(t, "all", string(<-c1.Send))
	assert.Equal(t, "all", string(<-c2.Send))
}

func TestSSEWriter(t *testing.T) {
	var buf strings.Builder
	write := SSEWriter(&buf)
	write("update", "test-data")
	assert.Contains(t, buf.String(), "event: update")
	assert.Contains(t, buf.String(), "data: test-data")
}

func TestHub_AddHistory(t *testing.T) {
	h := NewHub()
	h.AddHistory(Message{Type: "chat", Payload: []byte("hi")})
	h.AddHistory(Message{Type: "chat", Payload: []byte("hello")})
	h.historyMu.RLock()
	assert.Len(t, h.history, 2)
	h.historyMu.RUnlock()
}

func TestHub_RoomCount(t *testing.T) {
	h := NewHub()
	c := &Client{ID: "c1", Rooms: make(map[string]bool), Send: make(chan []byte, 10)}
	h.Register(c)
	h.JoinRoom("r1", c)
	h.JoinRoom("r2", c)
	assert.Equal(t, 2, h.RoomCount())
}

func TestHub_UnregisterCleansRooms(t *testing.T) {
	h := NewHub()
	c := &Client{ID: "c1", Rooms: make(map[string]bool), Send: make(chan []byte, 10)}
	h.Register(c)
	h.JoinRoom("r1", c)
	h.Unregister(c)
	assert.Equal(t, 0, h.RoomCount())
	assert.Equal(t, 0, h.ClientCount())
}
