package ws

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/Allinost/go-backend-core/internal/pkg/logger"
)

// Client WebSocket 客户端连接，包含用户标识、房间和发送通道
type Client struct {
	ID     string          // 客户端唯一标识
	UserID string          // 用户 ID
	Rooms  map[string]bool // 已加入的房间集合
	Send   chan []byte     // 消息发送缓冲通道（容量 256）
	Close  func() error    // 关闭连接的函数
}

// Message WebSocket 消息结构
type Message struct {
	Room    string `json:"room"`              // 目标房间
	UserID  string `json:"user_id,omitempty"` // 发送者用户 ID
	Type    string `json:"type"`              // 消息类型
	Payload []byte `json:"payload"`           // 消息载荷
}

// Hub 客户端管理中心，管理注册、房间、广播和消息处理
type Hub struct {
	mu        sync.RWMutex
	clients   map[string]*Client            // 所有在线客户端（ID -> Client）
	rooms     map[string]map[string]*Client // 房间 -> 成员（ClientID -> Client）
	handlers  map[string]MessageHandler     // 消息类型 -> 处理函数
	history   []Message                     // 历史消息记录
	historyMu sync.RWMutex                  // 历史消息锁
}

// MessageHandler 消息处理函数类型
type MessageHandler func(ctx context.Context, client *Client, msg Message)

func NewHub() *Hub {
	return &Hub{
		clients:  make(map[string]*Client),
		rooms:    make(map[string]map[string]*Client),
		handlers: make(map[string]MessageHandler),
	}
}

// Register 注册客户端到 Hub
func (h *Hub) Register(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[client.ID] = client
	logger.Debug().Str("client_id", client.ID).Str("user_id", client.UserID).Msg("ws: client registered")
}

// Unregister 注销客户端，自动离开所有房间并关闭连接
func (h *Hub) Unregister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, client.ID)
	for room := range client.Rooms {
		if members, ok := h.rooms[room]; ok {
			delete(members, client.ID)
			if len(members) == 0 {
				delete(h.rooms, room)
			}
		}
	}
	if client.Close != nil {
		_ = client.Close()
	}
	logger.Debug().Str("client_id", client.ID).Msg("ws: client unregistered")
}

// JoinRoom 加入指定房间，自动初始化房间和客户端 Rooms
func (h *Hub) JoinRoom(room string, client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.rooms[room]; !ok {
		h.rooms[room] = make(map[string]*Client)
	}
	h.rooms[room][client.ID] = client
	if client.Rooms == nil {
		client.Rooms = make(map[string]bool)
	}
	client.Rooms[room] = true
}

// LeaveRoom 离开指定房间，房间无人时自动清理
func (h *Hub) LeaveRoom(room string, client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if members, ok := h.rooms[room]; ok {
		delete(members, client.ID)
		if len(members) == 0 {
			delete(h.rooms, room)
		}
	}
	delete(client.Rooms, room)
}

// BroadcastToRoom 向房间内所有客户端广播（可选排除列表）
func (h *Hub) BroadcastToRoom(room string, msg Message, exclude ...string) {
	h.mu.RLock()
	members := h.rooms[room]
	h.mu.RUnlock()
	excludeSet := make(map[string]bool)
	for _, id := range exclude {
		excludeSet[id] = true
	}
	for _, client := range members {
		if excludeSet[client.ID] {
			continue
		}
		select {
		case client.Send <- msg.Payload:
		default:
			logger.Warn().Str("client_id", client.ID).Msg("ws: client send buffer full")
		}
	}
}

// BroadcastAll 向所有注册客户端广播
func (h *Hub) BroadcastAll(msg Message) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, client := range h.clients {
		select {
		case client.Send <- msg.Payload:
		default:
			logger.Warn().Str("client_id", client.ID).Msg("ws: client send buffer full")
		}
	}
}

// SendToClient 向指定 ID 的客户端发送消息
func (h *Hub) SendToClient(clientID string, msg Message) bool {
	h.mu.RLock()
	client, ok := h.clients[clientID]
	h.mu.RUnlock()
	if !ok {
		return false
	}
	select {
	case client.Send <- msg.Payload:
		return true
	default:
		return false
	}
}

// SendToUser 向指定用户 ID 的所有客户端发送消息
func (h *Hub) SendToUser(userID string, msg Message) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	sent := false
	for _, client := range h.clients {
		if client.UserID == userID {
			select {
			case client.Send <- msg.Payload:
				sent = true
			default:
				logger.Warn().Str("client_id", client.ID).Msg("ws: client send buffer full")
			}
		}
	}
	return sent
}

// HandleMessageType 注册消息类型对应的处理函数
func (h *Hub) HandleMessageType(msgType string, handler MessageHandler) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.handlers[msgType] = handler
}

// OnMessage 处理收到的消息，查找对应的类型处理器执行
func (h *Hub) OnMessage(client *Client, msg Message) {
	h.mu.RLock()
	handler, ok := h.handlers[msg.Type]
	h.mu.RUnlock()
	if ok {
		handler(context.Background(), client, msg)
	}
}

// ClientCount 返回当前在线客户端数量
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// RoomCount 返回当前房间数量
func (h *Hub) RoomCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.rooms)
}

// RoomMemberCount 返回指定房间的成员数量
func (h *Hub) RoomMemberCount(room string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.rooms[room])
}

// AddHistory 添加历史消息，超过 1000 条自动截断
func (h *Hub) AddHistory(msg Message) {
	h.historyMu.Lock()
	defer h.historyMu.Unlock()
	h.history = append(h.history, msg)
	if len(h.history) > 1000 {
		h.history = h.history[len(h.history)-1000:]
	}
}

// SSEWriter 创建 SSE 事件写入函数（event + data 格式）
func SSEWriter(w io.Writer) func(event, data string) {
	return func(event, data string) {
		_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
	}
}

// SSETick 定时通过 SSE 推送数据，直到 context 取消
func SSETick(w io.Writer, interval time.Duration, ctx context.Context, dataFunc func() string) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if dataFunc != nil {
				SSEWriter(w)("message", dataFunc())
			}
		}
	}
}

// Logger 返回全局日志实例
func Logger() *logger.Logger {
	return logger.L
}
