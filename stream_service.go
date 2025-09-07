package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
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

	service := &StreamService{
		client: client,
		apiKey: apiKey,
	}

	// Configure webhook on initialization
	service.configureWebhook()

	return service
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
		ID:     user.ID,
		Name:   user.Name,
		Role:   "user",
		Online: true, // Set user as online when they login
	}

	if user.ProfilePicURL != "" {
		streamUser.Image = user.ProfilePicURL
	}

	// Add wallet address to extra data
	if user.WalletAddress != "" {
		streamUser.ExtraData = map[string]interface{}{
			"wallet_address": user.WalletAddress,
		}
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
	_, err := s.client.UpsertUser(ctx, botUser)
	if err != nil {
		return "", fmt.Errorf("failed to create bot user: %w", err)
	}

	// Create channel ID: ai-chat-{userID}
	channelID := "ai-chat-" + userID

	// Create the channel with both user and AI assistant as members
	_, err = s.client.CreateChannel(ctx, "messaging", channelID, userID, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create channel: %w", err)
	}

	// Get channel reference
	channel := s.client.Channel("messaging", channelID)

	// Add both user and AI assistant as members
	_, err = channel.AddMembers(ctx, []string{userID, "ai-assistant"})
	if err != nil {
		return "", fmt.Errorf("failed to add members to channel: %w", err)
	}

	// Send profile setup message
	welcomeMsg := &stream.Message{
		Text: `Hi! I'm Oliver, here to help you meet people in your community.

To help others recognize and find you, I'll need a few details:

1. **Your name** - What should I call you?
2. **Profile picture** - Share a photo (upload an image)
3. **Bio** - Tell me a bit about yourself!

You can share this info in any format. For example:
"Hi! I'm John, and I love coding!"

Just include your name and upload a picture. What would you like to share?`,
		User: &stream.User{ID: "ai-assistant"},
	}

	_, err = channel.SendMessage(ctx, welcomeMsg, "ai-assistant")
	if err != nil {
		return "", fmt.Errorf("failed to send welcome message: %w", err)
	}

	return channelID, nil
}

// SendMessage sends a message to a Stream Chat channel
func (s *StreamService) SendMessage(cid, text, senderID string) error {
	ctx := context.Background()

	// Parse CID to extract channel type and ID
	// CID format is "type:id" (e.g., "messaging:ai-chat-uuid")
	var channelType, channelID string
	if colonIndex := len(cid); colonIndex > 0 {
		for i, r := range cid {
			if r == ':' {
				channelType = cid[:i]
				channelID = cid[i+1:]
				break
			}
		}
	}

	// Default to messaging if no type found
	if channelType == "" {
		channelType = "messaging"
		channelID = cid
	}

	log.Printf("[STREAM] Sending message to channel type: %s, ID: %s", channelType, channelID)

	// Get the channel
	channel := s.client.Channel(channelType, channelID)

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
	if err != nil {
		log.Printf("[STREAM] Failed to send message: %v", err)
	} else {
		log.Printf("[STREAM] Message sent successfully to %s:%s", channelType, channelID)
	}
	return err
}

// GetUserChannels retrieves all channels that a user is a member of
func (s *StreamService) GetUserChannels(ctx context.Context, userID string) ([]StreamChannel, error) {
	// Query channels where the user is a member
	channels, err := s.client.QueryChannels(ctx, &stream.QueryOption{
		Filter: map[string]interface{}{
			"members": map[string]interface{}{
				"$in": []string{userID},
			},
		},
	})
	if err != nil {
		return nil, err
	}

	// Convert to our StreamChannel type
	var userChannels []StreamChannel
	for _, channel := range channels.Channels {
		streamChannel := StreamChannel{
			ID:   channel.ID,
			Type: channel.Type,
			CID:  channel.CID,
		}

		// Add members if available
		if len(channel.Members) > 0 {
			for _, member := range channel.Members {
				streamChannel.Members = append(streamChannel.Members, StreamUser{
					ID:     member.User.ID,
					Name:   member.User.Name,
					Image:  member.User.Image,
					Role:   member.User.Role,
					Online: member.User.Online,
				})
			}
		}

		userChannels = append(userChannels, streamChannel)
	}

	return userChannels, nil
}

// HasAIChannel checks if a user has any AI chat channels
func (s *StreamService) HasAIChannel(ctx context.Context, userID string) (bool, error) {
	// Try to query the specific AI channel for this user
	aiChannelID := "ai-chat-" + userID
	channels, err := s.client.QueryChannels(ctx, &stream.QueryOption{
		Filter: map[string]interface{}{
			"id": aiChannelID,
			"members": map[string]interface{}{
				"$in": []string{userID},
			},
		},
	})
	if err != nil {
		return false, err
	}

	return len(channels.Channels) > 0, nil
}

// CreateUserMatchChannel creates a private channel between two users
func (s *StreamService) CreateUserMatchChannel(ctx context.Context, user1ID, user2ID string) (string, error) {
	// Create channel ID: match-{user1ID}-{user2ID} (sorted for consistency)
	var channelID string
	if user1ID < user2ID {
		channelID = "match-" + user1ID + "-" + user2ID
	} else {
		channelID = "match-" + user2ID + "-" + user1ID
	}

	// Create the channel with both users as members
	_, err := s.client.CreateChannel(ctx, "messaging", channelID, user1ID, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create match channel: %w", err)
	}

	// Get channel reference
	channel := s.client.Channel("messaging", channelID)

	// Add both users as members
	_, err = channel.AddMembers(ctx, []string{user1ID, user2ID})
	if err != nil {
		return "", fmt.Errorf("failed to add members to match channel: %w", err)
	}

	return channelID, nil
}

// configureWebhook configures the webhook URL in Stream Chat app settings
func (s *StreamService) configureWebhook() {
	webhookBaseURL := os.Getenv("WEBHOOK_BASE_URL")
	if webhookBaseURL == "" {
		log.Println("Warning: WEBHOOK_BASE_URL not set, skipping webhook configuration")
		return
	}

	ctx := context.Background()
	webhookURL := webhookBaseURL + "/webhooks/stream"

	// Configure webhook using Stream's app settings
	settings := &stream.AppSettings{
		WebhookURL: webhookURL,
	}
	_, err := s.client.UpdateAppSettings(ctx, settings)
	if err != nil {
		log.Printf("Failed to configure webhook URL %s: %v", webhookURL, err)
		log.Println("Note: Some tunnel URLs (like trycloudflare.com) may not be accepted by Stream Chat")
		log.Println("For development, try using ngrok instead: https://ngrok.com/")
		log.Println("For production, use a proper domain with valid SSL certificate")
		return
	}

	log.Printf("Successfully configured webhook URL: %s", webhookURL)
}
