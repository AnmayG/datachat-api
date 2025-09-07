package main

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ChatbotHandler handles chatbot-related HTTP requests
type ChatbotHandler struct {
	messageService *MessageService
	chatGPTService *ChatGPTService
	authService    *AuthService
	streamService  *StreamService
}

// NewChatbotHandler creates a new chatbot handler
func NewChatbotHandler(messageService *MessageService, chatGPTService *ChatGPTService, authService *AuthService, streamService *StreamService) *ChatbotHandler {
	return &ChatbotHandler{
		messageService: messageService,
		chatGPTService: chatGPTService,
		authService:    authService,
		streamService:  streamService,
	}
}

// ChatWithBot handles chatbot interaction requests
// @Summary Chat with AI bot
// @Description Send a message to the AI chatbot and get a response based on channel history. Specify model in request body (gpt-3.5-turbo or gpt-4).
// @Tags Chatbot
// @Accept json
// @Produce json
// @Param request body ChatbotRequest true "Chatbot request"
// @Success 200 {object} ChatbotResponse "AI response generated"
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 404 {object} ErrorResponse "User not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /chatbot/chat [post]
func (h *ChatbotHandler) ChatWithBot(c *gin.Context) {
	var req ChatbotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
		return
	}

	// Get user info for message creation
	user, err := h.authService.GetUser(req.UserID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "user_not_found",
			Message: err.Error(),
		})
		return
	}

	// Check if user needs profile setup
	if h.chatGPTService.NeedsProfileSetup(user) {
		// Try to parse profile information from the message and attachments
		var attachments []StreamMessageAttachment
		// Note: You would need to get attachments from the request or Stream webhook
		// For now, we'll assume empty attachments and handle in webhook
		
		profile, err := h.chatGPTService.ParseProfileFromStreamMessage(req.Message, attachments)
		if err != nil {
			// If parsing fails, ask for profile setup
			response, err := h.chatGPTService.GenerateProfileSetupResponse(user)
			if err != nil {
				response = "Hi! Welcome to the chat! To get started, please share your name and upload a profile picture. What's your name?"
			}
			
			// Store the bot's profile setup request
			botMessage := &Message{
				MessageText:    response,
				SenderID:       "chatbot",
				SenderUsername: "AI Assistant",
				ChannelID:      req.ChannelID,
				MessageType:    "assistant",
				Type:           "text",
			}
			
			createdBotMessage, err := h.messageService.CreateMessage(botMessage)
			if err != nil {
				c.JSON(http.StatusInternalServerError, ErrorResponse{
					Error:   "failed_to_store_bot_response",
					Message: err.Error(),
				})
				return
			}
			
			c.JSON(http.StatusOK, ChatbotResponse{
				Response:  response,
				MessageID: createdBotMessage.ID,
			})
			return
		}
		
		// Validate the parsed profile
		if err := h.chatGPTService.ValidateProfileData(profile); err != nil {
			// If validation fails, ask for complete information
			response := fmt.Sprintf("I need a bit more information to set up your profile. %s Please share your name and upload a profile picture.", err.Error())
			
			botMessage := &Message{
				MessageText:    response,
				SenderID:       "chatbot", 
				SenderUsername: "AI Assistant",
				ChannelID:      req.ChannelID,
				MessageType:    "assistant",
				Type:           "text",
			}
			
			createdBotMessage, err := h.messageService.CreateMessage(botMessage)
			if err != nil {
				c.JSON(http.StatusInternalServerError, ErrorResponse{
					Error:   "failed_to_store_bot_response", 
					Message: err.Error(),
				})
				return
			}
			
			c.JSON(http.StatusOK, ChatbotResponse{
				Response:  response,
				MessageID: createdBotMessage.ID,
			})
			return
		}
		
		// If we have complete profile data, update the user
		if h.chatGPTService.IsProfileComplete(profile) {
			if err := h.chatGPTService.UpdateUserProfileInDB(user.ID, profile, h.authService.supabaseService, h.streamService); err != nil {
				c.JSON(http.StatusInternalServerError, ErrorResponse{
					Error:   "failed_to_update_profile",
					Message: err.Error(),
				})
				return
			}
			
			// Generate confirmation message
			response := h.chatGPTService.GenerateProfileConfirmationMessage(profile)
			
			botMessage := &Message{
				MessageText:    response,
				SenderID:       "chatbot",
				SenderUsername: "AI Assistant", 
				ChannelID:      req.ChannelID,
				MessageType:    "assistant",
				Type:           "text",
			}
			
			createdBotMessage, err := h.messageService.CreateMessage(botMessage)
			if err != nil {
				c.JSON(http.StatusInternalServerError, ErrorResponse{
					Error:   "failed_to_store_bot_response",
					Message: err.Error(),
				})
				return
			}
			
			c.JSON(http.StatusOK, ChatbotResponse{
				Response:  response,
				MessageID: createdBotMessage.ID,
			})
			return
		}
	}

	// Store the user's message first
	userMessage := &Message{
		MessageText:    req.Message,
		SenderID:       req.UserID,
		SenderUsername: user.Username,
		ChannelID:      req.ChannelID,
		MessageType:    "user",
		Type:           "text",
	}

	_, err = h.messageService.CreateMessage(userMessage)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "failed_to_store_message",
			Message: err.Error(),
		})
		return
	}

	// Get recent messages for context
	recentMessages, err := h.messageService.GetRecentChannelMessages(req.ChannelID, DefaultContextLimit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "failed_to_get_context",
			Message: err.Error(),
		})
		return
	}

	// Generate AI response with specified model
	aiResponse, err := h.chatGPTService.GenerateResponse(recentMessages, req.Message, req.Model)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "failed_to_generate_response",
			Message: err.Error(),
		})
		return
	}

	// Determine assistant name based on model
	assistantName := "AI Assistant"
	if req.Model == "gpt-4" {
		assistantName = "AI Assistant (GPT-4)"
	}

	// Store the AI's response
	botMessage := &Message{
		MessageText:    aiResponse,
		SenderID:       "chatbot",
		SenderUsername: assistantName,
		ChannelID:      req.ChannelID,
		MessageType:    "assistant",
		Type:           "text",
	}

	createdBotMessage, err := h.messageService.CreateMessage(botMessage)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "failed_to_store_bot_response",
			Message: err.Error(),
		})
		return
	}


	c.JSON(http.StatusOK, ChatbotResponse{
		Response:  aiResponse,
		MessageID: createdBotMessage.ID,
	})
}


// GetChannelMessages retrieves messages for a channel
// @Summary Get channel messages
// @Description Retrieve messages for a specific channel with pagination
// @Tags Messages
// @Accept json
// @Produce json
// @Param channel_id path string true "Channel ID"
// @Param limit query int false "Number of messages to retrieve" default(50)
// @Param offset query int false "Offset for pagination" default(0)
// @Success 200 {array} Message "Channel messages"
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /messages/channel/{channel_id} [get]
func (h *ChatbotHandler) GetChannelMessages(c *gin.Context) {
	channelID := c.Param("channel_id")
	if channelID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "missing_channel_id",
			Message: "Channel ID is required",
		})
		return
	}

	// Get query parameters with defaults
	limit := DefaultMessageLimit
	if l := c.Query("limit"); l != "" {
		if parsed, err := parseIntParam(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	offset := 0
	if o := c.Query("offset"); o != "" {
		if parsed, err := parseIntParam(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	messages, err := h.messageService.GetChannelMessages(channelID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "failed_to_get_messages",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, messages)
}

// parseIntParam is a helper function to parse integer parameters
func parseIntParam(s string) (int, error) {
	var result int
	_, err := fmt.Sscanf(s, "%d", &result)
	return result, err
}