package main

import (
	"log"
	"sync"

	"github.com/gorilla/websocket"
)

// PubSubService handles simple pub/sub functionality for handshake events
type PubSubService struct {
	subscribers map[string][]*websocket.Conn // uid -> list of connections
	mutex       sync.RWMutex
}

// NewPubSubService creates a new pub/sub service
func NewPubSubService() *PubSubService {
	return &PubSubService{
		subscribers: make(map[string][]*websocket.Conn),
	}
}

// Subscribe adds a WebSocket connection for a user
func (ps *PubSubService) Subscribe(uid string, conn *websocket.Conn) {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()
	
	if ps.subscribers[uid] == nil {
		ps.subscribers[uid] = make([]*websocket.Conn, 0)
	}
	
	ps.subscribers[uid] = append(ps.subscribers[uid], conn)
	log.Printf("User %s subscribed to handshake events", uid)
}

// Unsubscribe removes a WebSocket connection for a user
func (ps *PubSubService) Unsubscribe(uid string, conn *websocket.Conn) {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()
	
	connections := ps.subscribers[uid]
	for i, c := range connections {
		if c == conn {
			// Remove this connection
			ps.subscribers[uid] = append(connections[:i], connections[i+1:]...)
			break
		}
	}
	
	// Clean up empty slices
	if len(ps.subscribers[uid]) == 0 {
		delete(ps.subscribers, uid)
	}
	
	log.Printf("User %s unsubscribed from handshake events", uid)
}

// PublishHandshake broadcasts a handshake event
func (ps *PubSubService) PublishHandshake(event HandshakeEvent) {
	ps.mutex.RLock()
	defer ps.mutex.RUnlock()
	
	// If ToUID is specified, send only to that user
	if event.ToUID != "" {
		ps.sendToUser(event.ToUID, event)
		return
	}
	
	// Otherwise, broadcast to all users except the sender
	for uid, connections := range ps.subscribers {
		if uid != event.FromUID {
			ps.sendToConnections(connections, event, uid)
		}
	}
}

// sendToUser sends an event to a specific user
func (ps *PubSubService) sendToUser(uid string, event HandshakeEvent) {
	connections := ps.subscribers[uid]
	if connections != nil {
		ps.sendToConnections(connections, event, uid)
	}
}

// sendToConnections sends an event to a list of connections
func (ps *PubSubService) sendToConnections(connections []*websocket.Conn, event HandshakeEvent, uid string) {
	deadConnections := make([]*websocket.Conn, 0)
	
	for _, conn := range connections {
		err := conn.WriteJSON(event)
		if err != nil {
			log.Printf("Error sending handshake event to user %s: %v", uid, err)
			deadConnections = append(deadConnections, conn)
		}
	}
	
	// Remove dead connections
	for _, deadConn := range deadConnections {
		ps.Unsubscribe(uid, deadConn)
	}
}

// GetActiveUsers returns a list of currently subscribed users
func (ps *PubSubService) GetActiveUsers() []string {
	ps.mutex.RLock()
	defer ps.mutex.RUnlock()
	
	users := make([]string, 0, len(ps.subscribers))
	for uid := range ps.subscribers {
		users = append(users, uid)
	}
	return users
}
