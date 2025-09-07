package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sashabaranov/go-openai"
)

// WebhookHandler handles Stream Chat webhook events
type WebhookHandler struct {
	chatGPTService      *ChatGPTService
	streamService       *StreamService
	authService         *AuthService
	processedWebhooks   map[string]bool // Track processed webhook IDs for deduplication
	pendingRecommendations map[string]*User // Track user recommendations pending confirmation
}

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler(chatGPTService *ChatGPTService, streamService *StreamService, authService *AuthService) *WebhookHandler {
	return &WebhookHandler{
		chatGPTService:         chatGPTService,
		streamService:          streamService,
		authService:            authService,
		processedWebhooks:      make(map[string]bool),
		pendingRecommendations: make(map[string]*User),
	}
}

// HandleStreamWebhook processes incoming Stream Chat webhook events
func (h *WebhookHandler) HandleStreamWebhook(c *gin.Context) {
	log.Printf("[WEBHOOK] Incoming webhook request from %s", c.ClientIP())
	
	// Extract webhook headers for validation as per Stream guidelines
	webhookID := c.GetHeader("X-Webhook-Id")
	apiKey := c.GetHeader("X-Api-Key")
	signature := c.GetHeader("X-Signature")
	
	log.Printf("[WEBHOOK] Headers - Webhook-Id: %s, Api-Key: %s, Signature present: %t", 
		webhookID, apiKey, signature != "")
	
	// Validate X-Api-Key header matches our Stream API key
	if apiKey != "" && apiKey != h.streamService.GetAPIKey() {
		log.Printf("[WEBHOOK] API key validation failed - received: %s, expected: %s", 
			apiKey, h.streamService.GetAPIKey())
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "invalid_api_key",
			Message: "API key validation failed",
		})
		return
	}

	// Check for duplicate webhook processing using X-Webhook-Id
	if webhookID != "" {
		if h.processedWebhooks[webhookID] {
			log.Printf("[WEBHOOK] Duplicate webhook detected - already processed: %s", webhookID)
			// Already processed this webhook, return success to avoid retries
			c.JSON(http.StatusOK, gin.H{"status": "already_processed"})
			return
		}
		// Mark as processed
		h.processedWebhooks[webhookID] = true
		log.Printf("[WEBHOOK] Marked webhook as processed: %s", webhookID)
	}

	// Read raw body for signature verification
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Printf("[WEBHOOK] Failed to read request body: %v", err)
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Failed to read request body",
		})
		return
	}
	
	log.Printf("[WEBHOOK] Request body length: %d bytes", len(body))

	// Verify webhook signature (required for security)
	if signature == "" {
		log.Printf("[WEBHOOK] Missing X-Signature header")
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "missing_signature",
			Message: "X-Signature header is required",
		})
		return
	}

	if !h.streamService.VerifyWebhook(body, signature) {
		log.Printf("[WEBHOOK] Signature verification failed")
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "invalid_signature",
			Message: "Webhook signature verification failed",
		})
		return
	}
	
	log.Printf("[WEBHOOK] Signature verification successful")

	// Parse webhook event
	var event StreamWebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		log.Printf("[WEBHOOK] Failed to parse JSON payload: %v", err)
		log.Printf("[WEBHOOK] Raw body: %s", string(body))
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_json",
			Message: "Failed to parse webhook payload",
		})
		return
	}

	log.Printf("[WEBHOOK] Event parsed successfully - Type: %s", event.Type)
	if event.Message != nil {
		log.Printf("[WEBHOOK] Message from user: %s, text: %s", 
			event.Message.User.ID, event.Message.Text)
	}
	if event.Channel != nil {
		log.Printf("[WEBHOOK] Channel: %s, CID: %s", event.Channel.ID, event.Channel.CID)
	}

	// Only process new messages
	if event.Type == "message.new" && event.Message != nil {
		log.Printf("[WEBHOOK] Processing new message event")
		h.handleNewMessage(event.Message, event.Channel)
	} else {
		log.Printf("[WEBHOOK] Skipping event - Type: %s, Message present: %t", 
			event.Type, event.Message != nil)
	}

	log.Printf("[WEBHOOK] Request processed successfully")
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// handleNewMessage processes new messages and generates GPT responses
func (h *WebhookHandler) handleNewMessage(message *StreamMessage, channel *StreamChannel) {
	log.Printf("[MESSAGE] Processing message from user: %s, role: %s", 
		message.User.ID, message.User.Role)
	log.Printf("[MESSAGE] Channel: %s, CID: %s", channel.ID, channel.CID)
	log.Printf("[MESSAGE] Message text: %s", message.Text)

	// Skip messages from bots to avoid loops
	if message.User.Role == "admin" || message.User.ID == "chatbot" || message.User.ID == "ai-assistant" {
		log.Printf("[MESSAGE] Skipping bot message from %s (role: %s)", 
			message.User.ID, message.User.Role)
		return
	}

	// Only respond in AI chat channels (channels with ID starting with "ai-chat-")
	if len(channel.ID) < 8 || channel.ID[:8] != "ai-chat-" {
		log.Printf("[MESSAGE] Skipping non-AI channel: %s", channel.ID)
		return
	}

	// Get user from database to check profile setup
	user, err := h.authService.GetUser(message.User.ID)
	if err != nil {
		log.Printf("[MESSAGE] Error getting user from database: %v", err)
		// Continue with default behavior
	} else if h.chatGPTService.NeedsProfileSetup(user) {
		log.Printf("[MESSAGE] User needs profile setup: %s", user.ID)
		
		// Convert Stream attachments to our format
		var attachments []StreamMessageAttachment
		for _, att := range message.Attachments {
			if att.Type == "image" && att.ImageURL != "" {
				attachments = append(attachments, StreamMessageAttachment{
					Type:     att.Type,
					ImageURL: att.ImageURL,
				})
			}
		}
		
		// Try to parse profile information from message
		profile, parseErr := h.chatGPTService.ParseProfileFromStreamMessage(message.Text, attachments)
		if parseErr != nil {
			log.Printf("[MESSAGE] Error parsing profile: %v", parseErr)
			// Send profile setup request
			response, genErr := h.chatGPTService.GenerateProfileSetupResponse(user)
			if genErr != nil {
				response = "Hi! Welcome to the chat! To get started, I need to set up your profile. Please share your name and upload a profile picture. What's your name?"
			}
			
			err = h.streamService.SendMessage(channel.CID, response, "ai-assistant")
			if err != nil {
				log.Printf("[MESSAGE] Error sending profile setup request: %v", err)
			} else {
				log.Printf("[MESSAGE] Profile setup request sent successfully")
			}
			return
		}
		
		// Validate parsed profile data
		if validateErr := h.chatGPTService.ValidateProfileData(profile); validateErr != nil {
			response := fmt.Sprintf("I need a bit more information to set up your profile. %s Please make sure to include your name and upload a profile picture!", validateErr.Error())
			
			err = h.streamService.SendMessage(channel.CID, response, "ai-assistant")
			if err != nil {
				log.Printf("[MESSAGE] Error sending profile validation error: %v", err)
			} else {
				log.Printf("[MESSAGE] Profile validation error sent successfully")
			}
			return
		}
		
		// If we have complete profile data, update the user
		if h.chatGPTService.IsProfileComplete(profile) {
			log.Printf("[MESSAGE] Updating user profile: Name=%s, PicURL=%s, Bio=%s", 
				profile.Name, profile.ProfilePicURL, profile.Bio)
			
			if updateErr := h.chatGPTService.UpdateUserProfileInDB(user.ID, profile, h.authService.supabaseService, h.streamService); updateErr != nil {
				log.Printf("[MESSAGE] Error updating user profile: %v", updateErr)
				response := "I'm sorry, there was an error setting up your profile. Please try again."
				h.streamService.SendMessage(channel.CID, response, "ai-assistant")
				return
			}
			
			// Generate confirmation message
			response := h.chatGPTService.GenerateProfileConfirmationMessage(profile)
			
			err = h.streamService.SendMessage(channel.CID, response, "ai-assistant")
			if err != nil {
				log.Printf("[MESSAGE] Error sending profile confirmation: %v", err)
			} else {
				log.Printf("[MESSAGE] Profile confirmation sent successfully")
			}
			return
		}
		
		// If profile is not complete, ask for more information
		response := "I still need a bit more information. Please make sure to share your name and upload a profile picture!"
		err = h.streamService.SendMessage(channel.CID, response, "ai-assistant")
		if err != nil {
			log.Printf("[MESSAGE] Error sending incomplete profile message: %v", err)
		} else {
			log.Printf("[MESSAGE] Incomplete profile message sent successfully")
		}
		return
	}

	log.Printf("[MESSAGE] Generating AI response for message: %s", message.Text)

	// Check if user is looking to meet someone (after profile is set up)
	if h.isMatchingRequest(message.Text) {
		log.Printf("[MESSAGE] Processing matching request from user: %s", message.User.ID)
		h.handleMatchingRequest(message.Text, message.User.ID, channel.CID)
		return
	}

	// Check if user is confirming they want to meet someone
	if h.isConfirmationMessage(message.Text) {
		log.Printf("[MESSAGE] Processing meeting confirmation from user: %s", message.User.ID)
		h.handleMeetingConfirmation(message.User.ID, channel.CID)
		return
	}

	// Generate GPT response
	aiResponse, err := h.chatGPTService.GenerateResponse(nil, message.Text, "gpt-3.5-turbo")
	if err != nil {
		log.Printf("[MESSAGE] Error generating AI response: %v", err)
		aiResponse = "I'm sorry, I'm having trouble processing your request right now."
	}

	log.Printf("[MESSAGE] Generated AI response: %s", aiResponse)

	// Send response back to Stream Chat
	err = h.streamService.SendMessage(channel.CID, aiResponse, "ai-assistant")
	if err != nil {
		log.Printf("[MESSAGE] Error sending AI response: %v", err)
	} else {
		log.Printf("[MESSAGE] AI response sent successfully to channel: %s", channel.CID)
	}
}

// isMatchingRequest uses AI to determine if the user wants to meet someone
func (h *WebhookHandler) isMatchingRequest(text string) bool {
	systemPrompt := `You are an AI that determines if a user is asking to meet or connect with other people. 

Look for requests like:
- Wanting to meet someone with specific interests/qualities
- Looking for connections or introductions
- Asking for recommendations for people to talk to
- Expressing loneliness or desire for social connections
- Asking about finding friends, dates, or conversation partners

Respond with only "YES" if they want to meet someone, or "NO" if they don't.`

	request := openai.ChatCompletionRequest{
		Model: openai.GPT3Dot5Turbo,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: systemPrompt,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: fmt.Sprintf("User message: \"%s\"", text),
			},
		},
		MaxTokens:   10,
		Temperature: 0.1,
	}

	resp, err := h.chatGPTService.client.CreateChatCompletion(context.Background(), request)
	if err != nil {
		log.Printf("[MATCHING] Error checking if matching request: %v", err)
		return false
	}

	if len(resp.Choices) == 0 {
		return false
	}

	response := strings.ToUpper(strings.TrimSpace(resp.Choices[0].Message.Content))
	return response == "YES"
}

// isConfirmationMessage checks if the user is confirming they want to meet someone
func (h *WebhookHandler) isConfirmationMessage(text string) bool {
	text = strings.ToLower(strings.TrimSpace(text))
	confirmationWords := []string{"yes", "yeah", "yep", "sure", "okay", "ok", "connect", "meet them"}
	
	for _, word := range confirmationWords {
		if text == word || strings.HasPrefix(text, word+" ") || strings.HasSuffix(text, " "+word) {
			return true
		}
	}
	return false
}

// handleMatchingRequest processes user's request to meet someone
func (h *WebhookHandler) handleMatchingRequest(preferences, userID, channelCID string) {
	log.Printf("[MATCHING] Processing matching request for user %s with preferences: %s", userID, preferences)
	
	// Get recommendation from ChatGPT service
	recommendedUser, err := h.chatGPTService.RecommendUser(preferences, userID, h.authService.supabaseService)
	if err != nil {
		log.Printf("[MATCHING] Error getting recommendation: %v", err)
		response := "I'm sorry, I couldn't find anyone matching your preferences right now. There might not be other users available, or you might want to try describing what you're looking for differently."
		h.streamService.SendMessage(channelCID, response, "ai-assistant")
		return
	}
	
	// Store the recommendation for later confirmation
	h.pendingRecommendations[userID] = recommendedUser
	
	// Generate and send recommendation message
	response := h.chatGPTService.GenerateMatchResponse(recommendedUser)
	err = h.streamService.SendMessage(channelCID, response, "ai-assistant")
	if err != nil {
		log.Printf("[MATCHING] Error sending recommendation: %v", err)
	} else {
		log.Printf("[MATCHING] Sent recommendation for user %s: %s", recommendedUser.Name, recommendedUser.ID)
	}
}

// handleMeetingConfirmation processes user's confirmation to meet someone
func (h *WebhookHandler) handleMeetingConfirmation(userID, channelCID string) {
	log.Printf("[MATCHING] Processing meeting confirmation for user %s", userID)
	
	// Get the pending recommendation
	recommendedUser, exists := h.pendingRecommendations[userID]
	if !exists {
		log.Printf("[MATCHING] No pending recommendation found for user %s", userID)
		response := "I don't have any pending introductions for you. Try asking me to find someone for you to meet!"
		h.streamService.SendMessage(channelCID, response, "ai-assistant")
		return
	}
	
	// Create a new channel between the two users
	matchChannelID, err := h.streamService.CreateUserMatchChannel(context.Background(), userID, recommendedUser.ID)
	if err != nil {
		log.Printf("[MATCHING] Error creating match channel: %v", err)
		response := "I'm sorry, there was an error creating your chat. Please try again later."
		h.streamService.SendMessage(channelCID, response, "ai-assistant")
		return
	}
	
	// Get current user info for the introduction message
	currentUser, err := h.authService.GetUser(userID)
	if err != nil {
		log.Printf("[MATCHING] Error getting current user info: %v", err)
		currentUser = &User{ID: userID, Name: "Unknown"}
	}
	
	// Send introduction message to the new channel
	introMessage := fmt.Sprintf(`Hi! I'm Oliver, and I've connected you two because I thought you might hit it off!

👋 %s, meet %s
👋 %s, meet %s

Feel free to introduce yourselves and start chatting. Have fun getting to know each other!`, 
		currentUser.Name, recommendedUser.Name,
		recommendedUser.Name, currentUser.Name)
	
	matchChannelCID := fmt.Sprintf("messaging:%s", matchChannelID)
	err = h.streamService.SendMessage(matchChannelCID, introMessage, "ai-assistant")
	if err != nil {
		log.Printf("[MATCHING] Error sending introduction message: %v", err)
	}
	
	// Send confirmation to the original AI chat
	confirmationResponse := fmt.Sprintf("Perfect! I've created a chat between you and %s. Check your channels to start the conversation!", recommendedUser.Name)
	err = h.streamService.SendMessage(channelCID, confirmationResponse, "ai-assistant")
	if err != nil {
		log.Printf("[MATCHING] Error sending confirmation: %v", err)
	} else {
		log.Printf("[MATCHING] Successfully connected users %s and %s", userID, recommendedUser.ID)
	}
	
	// Clean up the pending recommendation
	delete(h.pendingRecommendations, userID)
}