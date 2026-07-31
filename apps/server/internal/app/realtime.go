package app

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	enginetypes "github.com/zishang520/engine.io/v2/types"
	"github.com/zishang520/socket.io/v2/socket"
)

type Realtime struct {
	server  *socket.Server
	handler http.Handler
	db      *sql.DB
	mu      sync.RWMutex
	users   map[string]map[*socket.Socket]struct{}
	stores  map[string]map[*socket.Socket]struct{}
}

type socketUser struct {
	UserID   string
	Role     string
	StoreID  string
	Username string
}

func newRealtime(db *sql.DB, cfg Config) *Realtime {
	secret := cfg.JWTSecret
	options := socket.DefaultServerOptions()
	options.SetServeClient(false)
	options.SetCors(&enginetypes.Cors{
		Origin:         socketOriginConfig(cfg.CORSOrigins),
		AllowedHeaders: []string{"Authorization", "Content-Type"},
		Credentials:    true,
	})
	io := socket.NewServer(nil, options)
	realtime := &Realtime{
		server: io,
		users:  make(map[string]map[*socket.Socket]struct{}),
		stores: make(map[string]map[*socket.Socket]struct{}),
		db:     db,
	}
	realtime.handler = io.ServeHandler(nil)

	io.On("connection", func(args ...any) {
		if len(args) == 0 {
			return
		}
		client, ok := args[0].(*socket.Socket)
		if !ok {
			return
		}
		token := socketToken(client)
		claims, err := parseToken(secret, token)
		if err != nil {
			client.Disconnect(true)
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		claims, err = activeAuthClaims(ctx, realtime.db, claims)
		cancel()
		if err != nil {
			client.Disconnect(true)
			return
		}
		user := socketUser{UserID: claims.Subject, Role: claims.Role, StoreID: claims.StoreID, Username: claims.Username}
		client.SetData(user)
		realtime.addClient(client, user)
		client.On("disconnect", func(...any) {
			realtime.removeClient(client, user)
		})
		client.Join(socket.Room("store:"+claims.StoreID), socket.Room("user:"+claims.Subject))
		_ = client.Emit("connected", map[string]any{"ok": true, "userId": claims.Subject})
		client.On("ping", func(values ...any) {
			payload := any(map[string]any{"time": time.Now().UnixMilli()})
			if len(values) > 0 && values[0] != nil {
				payload = values[0]
			}
			_ = client.Emit("pong", payload)
		})
	})
	return realtime
}

func socketToken(client *socket.Socket) string {
	if handshake := client.Handshake(); handshake != nil {
		switch auth := handshake.Auth.(type) {
		case map[string]any:
			if token, ok := auth["token"].(string); ok {
				return token
			}
		case map[string]string:
			return auth["token"]
		}
		for key, values := range handshake.Headers {
			if strings.EqualFold(key, "Authorization") && len(values) > 0 {
				return bearerToken(values[0])
			}
		}
	}
	return ""
}

func socketOriginConfig(origins []string) any {
	allowed := make([]any, 0, len(origins))
	for _, origin := range origins {
		if origin == "*" {
			return true
		}
		allowed = append(allowed, origin)
	}
	return allowed
}

func (r *Realtime) Handler() http.Handler {
	return r.handler
}

func (r *Realtime) EmitToStore(storeID, event string, payload any) {
	for _, client := range r.clientsForStore(storeID) {
		if err := client.Emit(event, payload); err != nil {
			log.Printf("realtime store event %s failed: %v", event, err)
		}
	}
}

func (r *Realtime) EmitToUser(userID, event string, payload any) {
	r.mu.RLock()
	clients := make([]*socket.Socket, 0, len(r.users[userID]))
	for client := range r.users[userID] {
		clients = append(clients, client)
	}
	r.mu.RUnlock()
	for _, client := range clients {
		if err := client.Emit(event, payload); err != nil {
			log.Printf("realtime user event %s failed: %v", event, err)
		}
	}
}

func (r *Realtime) DisconnectUser(userID string) {
	r.mu.RLock()
	clients := make([]*socket.Socket, 0, len(r.users[userID]))
	for client := range r.users[userID] {
		clients = append(clients, client)
	}
	r.mu.RUnlock()
	for _, client := range clients {
		_ = client.Disconnect(true)
	}
}

func (r *Realtime) addClient(client *socket.Socket, user socketUser) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.users[user.UserID] == nil {
		r.users[user.UserID] = make(map[*socket.Socket]struct{})
	}
	r.users[user.UserID][client] = struct{}{}
	if r.stores[user.StoreID] == nil {
		r.stores[user.StoreID] = make(map[*socket.Socket]struct{})
	}
	r.stores[user.StoreID][client] = struct{}{}
}

func (r *Realtime) removeClient(client *socket.Socket, user socketUser) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if clients := r.users[user.UserID]; clients != nil {
		delete(clients, client)
		if len(clients) == 0 {
			delete(r.users, user.UserID)
		}
	}
	if clients := r.stores[user.StoreID]; clients != nil {
		delete(clients, client)
		if len(clients) == 0 {
			delete(r.stores, user.StoreID)
		}
	}
}

func (r *Realtime) clientsForStore(storeID string) []*socket.Socket {
	r.mu.RLock()
	clients := make([]*socket.Socket, 0, len(r.stores[storeID]))
	for client := range r.stores[storeID] {
		clients = append(clients, client)
	}
	r.mu.RUnlock()
	return clients
}

func (r *Realtime) Close() {
	r.server.Close(nil)
}
