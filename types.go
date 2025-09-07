package main

import (
	"fmt"
	"time"
)

// Constants for validation and limits
const (
	// User validation limits
	MaxUsernameLength    = 24
	MaxNameLength       = 50
	MaxBioLength        = 500
	
	// ChatGPT token limits
	DefaultMaxTokens    = 500
	GPT4MaxTokens      = 1000
	
	// Pagination defaults
	DefaultMessageLimit = 50
	DefaultContextLimit = 20
	
	// JWT settings
	DefaultJWTSecret = "default-secret-key-change-in-production"
)

// ValidateUserFields validates user input fields
func ValidateUserFields(username, name, bio string) error {
	if len(username) > MaxUsernameLength {
		return fmt.Errorf("username too long (max %d characters)", MaxUsernameLength)
	}
	
	if len(name) > MaxNameLength {
		return fmt.Errorf("name too long (max %d characters)", MaxNameLength)
	}
	
	if len(bio) > MaxBioLength {
		return fmt.Errorf("bio too long (max %d characters)", MaxBioLength)
	}
	
	return nil
}

// User represents a user in the system
type User struct {
	ID            string    `json:"id" db:"id"`
	Username      string    `json:"username" db:"username"`
	Name          string    `json:"name" db:"name"`
	WalletAddress string    `json:"wallet_address,omitempty" db:"wallet_address"`
	ProfilePicURL string    `json:"profile_pic_url,omitempty" db:"profile_pic_url"`
	Bio           string    `json:"bio,omitempty" db:"bio"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
}

// LoginRequest represents the login request payload
type LoginRequest struct {
	WalletAddress string `json:"wallet_address" binding:"required"`
}

// RegisterRequest represents the registration request payload
type RegisterRequest struct {
	Name          string `json:"name,omitempty"`
	WalletAddress string `json:"wallet_address" binding:"required"`
	ProfilePicURL string `json:"profile_pic_url,omitempty"`
	Bio           string `json:"bio,omitempty"`
}

// TokenRequest represents the token generation request
type TokenRequest struct {
	UserID string `json:"user_id" binding:"required"`
}

// TokenResponse represents the token response
type TokenResponse struct {
	Token  string `json:"token"`
	UserID string `json:"user_id"`
}

// AuthResponse represents the authentication response
type AuthResponse struct {
	User        User   `json:"user"`
	Token       string `json:"token"`
	StreamToken string `json:"stream_token"`
}

// StreamUserRequest represents the Stream user creation/update request
type StreamUserRequest struct {
	ID       string `json:"id" binding:"required"`
	Username string `json:"username" binding:"required"`
	Name     string `json:"name" binding:"required"`
	Image    string `json:"image,omitempty"`
	Role     string `json:"role,omitempty"`
}

// Message represents a chat message in the system
type Message struct {
	ID              string     `json:"id" db:"id"`
	CreatedAt       time.Time  `json:"created_at" db:"created_at"`
	MessageText     string     `json:"message_text" db:"message_text"`
	SenderID        string     `json:"sender_id" db:"sender_id"`
	ChannelID       string     `json:"channel_id" db:"channel_id"`
	MessageType     string     `json:"message_type" db:"message_type"` // 'user', 'assistant', 'system'
	SenderUsername  string     `json:"sender_username" db:"sender_username"`
	Type            string     `json:"type" db:"type"` // 'text', 'image', etc.
	StreamMessageID *string    `json:"stream_message_id,omitempty" db:"stream_message_id"`
	ReplyToID       *string    `json:"reply_to_id,omitempty" db:"reply_to_id"`
}

// ChatbotRequest represents a request to the chatbot
type ChatbotRequest struct {
	ChannelID string `json:"channel_id" binding:"required"`
	Message   string `json:"message" binding:"required"`
	UserID    string `json:"user_id" binding:"required"`
	Model     string `json:"model,omitempty"` // "gpt-3.5-turbo" or "gpt-4", defaults to gpt-3.5-turbo
}

// ChatbotResponse represents a chatbot response
type ChatbotResponse struct {
	Response  string `json:"response"`
	MessageID string `json:"message_id,omitempty"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// StreamWebhookEvent represents a Stream Chat webhook event
type StreamWebhookEvent struct {
	Type        string                 `json:"type"`
	Message     *StreamMessage         `json:"message,omitempty"`
	Channel     *StreamChannel         `json:"channel,omitempty"`
	User        *StreamUser            `json:"user,omitempty"`
	CreatedAt   string                 `json:"created_at,omitempty"`
	CID         string                 `json:"cid,omitempty"`
	RequestInfo *StreamRequestInfo     `json:"request_info,omitempty"`
}

// StreamMessage represents a message from Stream Chat webhook
type StreamMessage struct {
	ID          string             `json:"id"`
	Text        string             `json:"text"`
	HTML        string             `json:"html,omitempty"`
	User        StreamUser         `json:"user"`
	ChannelID   string             `json:"channel_id,omitempty"`
	CID         string             `json:"cid,omitempty"`
	Attachments []StreamAttachment `json:"attachments,omitempty"`
	CreatedAt   string             `json:"created_at"`
	UpdatedAt   string             `json:"updated_at"`
	Type        string             `json:"type"`
	Command     string             `json:"command,omitempty"`
	Args        string             `json:"args,omitempty"`
}

// StreamUser represents a user from Stream Chat
type StreamUser struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Username string `json:"username,omitempty"`
	Image    string `json:"image,omitempty"`
	Role     string `json:"role,omitempty"`
	Online   bool   `json:"online,omitempty"`
}

// StreamChannel represents a channel from Stream Chat
type StreamChannel struct {
	ID      string                 `json:"id"`
	Type    string                 `json:"type"`
	CID     string                 `json:"cid"`
	Config  map[string]interface{} `json:"config,omitempty"`
	Members []StreamUser           `json:"members,omitempty"`
}

// StreamRequestInfo represents request information from webhook
type StreamRequestInfo struct {
	Type      string `json:"type"`
	IP        string `json:"ip,omitempty"`
	UserAgent string `json:"user_agent,omitempty"`
	SDK       string `json:"sdk,omitempty"`
	Ext       string `json:"ext,omitempty"`
}

// StreamAttachment represents an attachment with actions
type StreamAttachment struct {
	Type       string        `json:"type"`
	Title      string        `json:"title,omitempty"`
	Text       string        `json:"text,omitempty"`
	TitleLink  string        `json:"title_link,omitempty"`
	ThumbURL   string        `json:"thumb_url,omitempty"`
	Actions    []StreamAction `json:"actions,omitempty"`
	Fields     []StreamField  `json:"fields,omitempty"`
	Color      string        `json:"color,omitempty"`
	Fallback   string        `json:"fallback,omitempty"`
	ImageURL   string        `json:"image_url,omitempty"`
	AssetURL   string        `json:"asset_url,omitempty"`
	OgScrapeURL string       `json:"og_scrape_url,omitempty"`
}

// StreamAction represents a button action
type StreamAction struct {
	Name  string `json:"name"`
	Text  string `json:"text"`
	Type  string `json:"type"`
	Value string `json:"value"`
	Style string `json:"style,omitempty"`
}

// StreamField represents a form field
type StreamField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short,omitempty"`
}

// BotMessageRequest represents a structured bot message with attachments
type BotMessageRequest struct {
	ChannelID   string            `json:"channel_id" binding:"required"`
	Text        string            `json:"text"`
	Attachments []StreamAttachment `json:"attachments,omitempty"`
	UserID      string            `json:"user_id,omitempty"`
	SkipPush    bool              `json:"skip_push,omitempty"`
}

// WebhookResponse represents the response to a webhook
type WebhookResponse struct {
	Message     *BotMessageRequest `json:"message,omitempty"`
	Processed   bool               `json:"processed"`
	Action      string             `json:"action,omitempty"`
	Description string             `json:"description,omitempty"`
}

// HandshakeEvent represents a handshake event to be broadcast
type HandshakeEvent struct {
	Type      string    `json:"type"`       // "handshake_sent", "handshake_received", etc.
	FromUID   string    `json:"from_uid"`   // User who initiated the handshake
	ToUID     string    `json:"to_uid,omitempty"` // Target user (optional for broadcasts)
	Message   string    `json:"message,omitempty"` // Optional message
	Timestamp time.Time `json:"timestamp"`
}

// HandshakeRequest represents a request to send a handshake
type HandshakeRequest struct {
	Type    string `json:"type" binding:"required"`    // "wave", "high_five", "fist_bump", etc.
	ToUID   string `json:"to_uid,omitempty"`           // Specific user or empty for broadcast
	Message string `json:"message,omitempty"`          // Optional message
}