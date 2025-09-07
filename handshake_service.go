package main

import (
	"time"
)

// HandshakeService handles handshake-related business logic
type HandshakeService struct {
	pubsub *PubSubService
}

// NewHandshakeService creates a new handshake service
func NewHandshakeService(pubsub *PubSubService) *HandshakeService {
	return &HandshakeService{
		pubsub: pubsub,
	}
}

// SendHandshake processes and broadcasts a handshake event
func (hs *HandshakeService) SendHandshake(fromUID string, request HandshakeRequest) error {
	event := HandshakeEvent{
		Type:      request.Type,
		FromUID:   fromUID,
		ToUID:     request.ToUID,
		Message:   request.Message,
		Timestamp: time.Now(),
	}
	
	// Broadcast the handshake event
	hs.pubsub.PublishHandshake(event)
	
	return nil
}

// GetActiveUsers returns a list of users currently connected
func (hs *HandshakeService) GetActiveUsers() []string {
	return hs.pubsub.GetActiveUsers()
}