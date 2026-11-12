package app

import (
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = 25 * time.Second
	maxMessageSize = 512
	clientQueue    = 32
)

type Hub struct {
	mu      sync.RWMutex
	clients map[*client]struct{}
	origins map[string]struct{}
}

type client struct {
	conn *websocket.Conn
	send chan []byte
}

func NewHub(allowedOrigins []string) *Hub {
	origins := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		if origin = strings.TrimSpace(origin); origin != "" {
			origins[origin] = struct{}{}
		}
	}
	return &Hub{clients: make(map[*client]struct{}), origins: origins}
}

func (h *Hub) Broadcast(message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for connected := range h.clients {
		select {
		case connected.send <- message:
		default:
			log.Printf("dropping real-time alert for a slow WebSocket client")
		}
	}
}

func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     h.checkOrigin,
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("upgrade WebSocket connection: %v", err)
		return
	}
	connected := &client{conn: conn, send: make(chan []byte, clientQueue)}
	h.mu.Lock()
	h.clients[connected] = struct{}{}
	h.mu.Unlock()
	go connected.writePump()
	connected.readPump(h)
}

func (h *Hub) checkOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	if _, ok := h.origins["*"]; ok {
		return true
	}
	_, ok := h.origins[origin]
	return ok
}

func (h *Hub) unregister(connected *client) {
	h.mu.Lock()
	if _, exists := h.clients[connected]; exists {
		delete(h.clients, connected)
		close(connected.send)
	}
	h.mu.Unlock()
}

func (c *client) readPump(h *Hub) {
	defer func() {
		h.unregister(c)
		_ = c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (c *client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()
	for {
		select {
		case message, open := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !open {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
