package main

import (
	"context"
	"errors"
	"time"

	stream "github.com/GetStream/stream-chat-go/v5"
)

// StreamService handles Stream Chat operations
type StreamService struct {
	client *stream.Client
	apiKey string
}

// NewStreamService creates a new Stream service instance
func NewStreamService(apiKey, secret string) *StreamService {
	client, err := stream.NewClient(apiKey, secret)
	if err != nil {
		panic("Failed to initialize Stream client: " + err.Error())
	}

	return &StreamService{
		client: client,
		apiKey: apiKey,
	}
}

// CreateToken generates a Stream Chat token for a user
func (s *StreamService) CreateToken(userID string, expiration *time.Time) (string, error) {
	if expiration != nil {
		token, err := s.client.CreateToken(userID, *expiration)
		return token, err
	}
	token, err := s.client.CreateToken(userID, time.Time{})
	return token, err
}

// CreateOrUpdateUser creates or updates a user in Stream Chat
func (s *StreamService) CreateOrUpdateUser(ctx context.Context, user *User) error {
	streamUser := &stream.User{
		ID:   user.ID,
		Name: user.Name,
		Role: "user",
	}

	if user.ProfilePicURL != "" {
		streamUser.Image = user.ProfilePicURL
	}

	_, err := s.client.UpsertUser(ctx, streamUser)
	return err
}

// GetUser retrieves a user from Stream Chat
func (s *StreamService) GetUser(ctx context.Context, userID string) (*stream.User, error) {
	users, err := s.client.QueryUsers(ctx, &stream.QueryOption{
		Filter: map[string]interface{}{
			"id": userID,
		},
	})
	if err != nil {
		return nil, err
	}

	if len(users.Users) == 0 {
		return nil, errors.New("user not found")
	}

	return users.Users[0], nil
}

// RevokeUserToken revokes all tokens for a user
func (s *StreamService) RevokeUserToken(ctx context.Context, userID string, revokeTime *time.Time) error {
	_, err := s.client.RevokeUserToken(ctx, userID, revokeTime)
	return err
}

// RevokeUsersTokens revokes tokens for multiple users
func (s *StreamService) RevokeUsersTokens(ctx context.Context, userIDs []string, revokeTime *time.Time) error {
	_, err := s.client.RevokeUsersTokens(ctx, userIDs, revokeTime)
	return err
}

// GetAPIKey returns the Stream API key
func (s *StreamService) GetAPIKey() string {
	return s.apiKey
}

// VerifyWebhook verifies webhook signature
func (s *StreamService) VerifyWebhook(body []byte, signature string) bool {
	return s.client.VerifyWebhook(body, []byte(signature))
}

// CreateAIChatChannel creates a private channel between user and AI chatbot
func (s *StreamService) CreateAIChatChannel(ctx context.Context, userID string) (string, error) {
	// Create AI bot user if doesn't exist
	botUser := &stream.User{
		ID:   "ai-assistant",
		Name: "AI Assistant",
		Role: "admin",
	}
	s.client.UpsertUser(ctx, botUser)
	
	// Create channel ID: ai-chat-{userID}
	channelID := "ai-chat-" + userID
	channel := s.client.Channel("messaging", channelID)
	
	// Create private channel with user and AI bot  
	_, err := channel.Update(ctx, map[string]interface{}{
		"members": []string{userID, "ai-assistant"},
	}, nil)
	if err != nil {
		return "", err
	}
	
	// Send welcome message
	welcomeMsg := &stream.Message{
		Text: "Hello! I'm your AI assistant. Feel free to ask me anything!",
		User: &stream.User{ID: "ai-assistant"},
	}
	
	channel.SendMessage(ctx, welcomeMsg, "ai-assistant")
	
	return channelID, nil
}

// SendMessage sends a message to a Stream Chat channel
func (s *StreamService) SendMessage(cid, text, senderID string) error {
	ctx := context.Background()
	
	// Get the channel
	channel := s.client.Channel("messaging", cid)
	
	// Create bot user if doesn't exist
	botUser := &stream.User{
		ID:   senderID,
		Name: senderID,
		Role: "admin",
	}
	s.client.UpsertUser(ctx, botUser)
	
	// Send message
	message := &stream.Message{
		Text: text,
		User: &stream.User{ID: senderID},
	}
	
	_, err := channel.SendMessage(ctx, message, senderID)
	return err
}