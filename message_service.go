package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	supa "github.com/supabase-community/supabase-go"
)

// MessageService handles message database operations
type MessageService struct {
	client *supa.Client
}

// NewMessageService creates a new message service instance
func NewMessageService(supabaseClient *supa.Client) *MessageService {
	return &MessageService{
		client: supabaseClient,
	}
}

// CreateMessage creates a new message in the database
func (s *MessageService) CreateMessage(message *Message) (*Message, error) {
	// Generate UUID if not provided
	if message.ID == "" {
		message.ID = uuid.New().String()
	}
	
	// Set created_at if not provided
	if message.CreatedAt.IsZero() {
		message.CreatedAt = time.Now()
	}
	
	// Set default message type if not provided
	if message.MessageType == "" {
		message.MessageType = "user"
	}
	
	// Set default type if not provided
	if message.Type == "" {
		message.Type = "text"
	}
	
	messageJSON, err := json.Marshal(message)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message: %w", err)
	}
	
	result, _, err := s.client.From("messages").
		Insert(messageJSON, false, "", "", "").
		Execute()
	
	if err != nil {
		return nil, fmt.Errorf("failed to create message: %w", err)
	}
	
	var createdMessages []Message
	err = json.Unmarshal(result, &createdMessages)
	if err != nil {
		return nil, fmt.Errorf("failed to decode created message: %w", err)
	}
	
	if len(createdMessages) == 0 {
		return nil, fmt.Errorf("message creation failed")
	}
	
	return &createdMessages[0], nil
}

// GetChannelMessages retrieves messages for a specific channel, ordered by creation time
func (s *MessageService) GetChannelMessages(channelID string, limit int, offset int) ([]Message, error) {
	query := s.client.From("messages").
		Select("*", "", false).
		Eq("channel_id", channelID).
		Order("created_at", nil) // Order by created_at ascending
	
	if limit > 0 {
		query = query.Limit(limit, "")
	}
	
	if offset > 0 {
		query = query.Range(offset, offset+limit-1, "")
	}
	
	result, _, err := query.Execute()
	if err != nil {
		return nil, fmt.Errorf("failed to get channel messages: %w", err)
	}
	
	var messages []Message
	err = json.Unmarshal(result, &messages)
	if err != nil {
		return nil, fmt.Errorf("failed to decode messages: %w", err)
	}
	
	return messages, nil
}

// GetRecentChannelMessages retrieves the most recent messages for ChatGPT context
func (s *MessageService) GetRecentChannelMessages(channelID string, limit int) ([]Message, error) {
	if limit <= 0 {
		limit = DefaultMessageLimit // Default context window
	}
	
	result, _, err := s.client.From("messages").
		Select("*", "", false).
		Eq("channel_id", channelID).
		Order("created_at", nil). // Order by created_at descending  
		Limit(limit, "").
		Execute()
	
	if err != nil {
		return nil, fmt.Errorf("failed to get recent channel messages: %w", err)
	}
	
	var messages []Message
	err = json.Unmarshal(result, &messages)
	if err != nil {
		return nil, fmt.Errorf("failed to decode messages: %w", err)
	}
	
	// Reverse the order so oldest messages come first (for ChatGPT context)
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}
	
	return messages, nil
}

// GetMessageByID retrieves a specific message by ID
func (s *MessageService) GetMessageByID(messageID string) (*Message, error) {
	result, _, err := s.client.From("messages").
		Select("*", "", false).
		Eq("id", messageID).
		Execute()
	
	if err != nil {
		return nil, fmt.Errorf("failed to get message: %w", err)
	}
	
	var messages []Message
	err = json.Unmarshal(result, &messages)
	if err != nil {
		return nil, fmt.Errorf("failed to decode message: %w", err)
	}
	
	if len(messages) == 0 {
		return nil, nil // Message not found
	}
	
	return &messages[0], nil
}

// GetMessagesByStreamID retrieves messages by Stream Chat message ID
func (s *MessageService) GetMessagesByStreamID(streamMessageID string) ([]Message, error) {
	result, _, err := s.client.From("messages").
		Select("*", "", false).
		Eq("stream_message_id", streamMessageID).
		Execute()
	
	if err != nil {
		return nil, fmt.Errorf("failed to get messages by stream ID: %w", err)
	}
	
	var messages []Message
	err = json.Unmarshal(result, &messages)
	if err != nil {
		return nil, fmt.Errorf("failed to decode messages: %w", err)
	}
	
	return messages, nil
}

// DeleteMessage deletes a message by ID
func (s *MessageService) DeleteMessage(messageID string) error {
	_, _, err := s.client.From("messages").
		Delete("", "").
		Eq("id", messageID).
		Execute()
	
	if err != nil {
		return fmt.Errorf("failed to delete message: %w", err)
	}
	
	return nil
}

// UpdateMessage updates a message
func (s *MessageService) UpdateMessage(messageID string, updates map[string]interface{}) (*Message, error) {
	updatesJSON, err := json.Marshal(updates)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal updates: %w", err)
	}
	
	result, _, err := s.client.From("messages").
		Update(updatesJSON, "", "").
		Eq("id", messageID).
		Execute()
	
	if err != nil {
		return nil, fmt.Errorf("failed to update message: %w", err)
	}
	
	var updatedMessages []Message
	err = json.Unmarshal(result, &updatedMessages)
	if err != nil {
		return nil, fmt.Errorf("failed to decode updated message: %w", err)
	}
	
	if len(updatedMessages) == 0 {
		return nil, fmt.Errorf("message not found")
	}
	
	return &updatedMessages[0], nil
}