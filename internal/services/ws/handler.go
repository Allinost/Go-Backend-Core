package ws

import (
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	gorilla "github.com/gorilla/websocket"
)

// upgrader WebSocket 升级器，允许所有来源
var upgrader = gorilla.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// WSHandler WebSocket 请求处理器，管理升级、认证和连接生命周期
type WSHandler struct {
	Hub   *Hub                                          // Hub 实例
	GenID func() string                                 // 生成客户端 ID 的函数
	Auth  func(c *gin.Context) (userID string, ok bool) // 认证函数
}

// NewWSHandler 创建 WebSocket 处理器，设置默认 ID 生成和放行认证
func NewWSHandler(hub *Hub) *WSHandler {
	return &WSHandler{
		Hub: hub,
		GenID: func() string {
			return fmt.Sprintf("ws_%d", time.Now().UnixNano())
		},
		Auth: func(c *gin.Context) (string, bool) {
			return "", true
		},
	}
}

// Upgrade 将 HTTP 连接升级为 WebSocket，创建 Client 并启动读写协程
func (h *WSHandler) Upgrade(c *gin.Context) {
	userID, ok := h.Auth(c)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	client := &Client{
		ID:     h.GenID(),
		UserID: userID,
		Rooms:  make(map[string]bool),
		Send:   make(chan []byte, 256),
		Close:  func() error { return conn.Close() },
	}

	h.Hub.Register(client)

	go h.writePump(client, conn)
	go h.readPump(client, conn)
}

// writePump 从 client.Send 通道读取消息并写入 WebSocket 连接，每隔 30s 发送 Ping
func (h *WSHandler) writePump(client *Client, conn *gorilla.Conn) {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		h.Hub.Unregister(client)
	}()
	for {
		select {
		case message, ok := <-client.Send:
			if !ok {
				conn.WriteMessage(gorilla.CloseMessage, []byte{})
				return
			}
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(gorilla.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(gorilla.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// readPump 从 WebSocket 连接读取消息，限制 4096 字节，交由 Hub.OnMessage 处理
func (h *WSHandler) readPump(client *Client, conn *gorilla.Conn) {
	defer func() {
		h.Hub.Unregister(client)
	}()
	conn.SetReadLimit(4096)
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			break
		}
		h.Hub.OnMessage(client, Message{
			UserID:  client.UserID,
			Type:    "message",
			Payload: message,
		})
	}
}

// SSEHandler 创建 SSE（Server-Sent Events）处理函数，用于实时推送
func SSEHandler(hub *Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("X-Accel-Buffering", "no")

		clientID := fmt.Sprintf("sse_%d", time.Now().UnixNano())
		ch := make(chan []byte, 64)

		hub.mu.Lock()
		hub.clients[clientID] = &Client{
			ID:    clientID,
			Send:  ch,
			Close: func() error { return nil },
		}
		hub.mu.Unlock()

		defer func() {
			hub.mu.Lock()
			delete(hub.clients, clientID)
			hub.mu.Unlock()
		}()

		c.Stream(func(w io.Writer) bool {
			select {
			case msg, ok := <-ch:
				if !ok {
					return false
				}
				fmt.Fprintf(w, "data: %s\n\n", string(msg))
				return true
			case <-c.Request.Context().Done():
				return false
			}
		})
	}
}

var (
	_ = gorilla.TextMessage
	_ sync.Mutex
)
