package main

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

// WebhookHandler handles Stream Chat webhook events
type WebhookHandler struct {
	chatGPTService *ChatGPTService
	streamService  *StreamService
}

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler(chatGPTService *ChatGPTService, streamService *StreamService) *WebhookHandler {
	return &WebhookHandler{
		chatGPTService: chatGPTService,
		streamService:  streamService,
	}
}

// HandleStreamWebhook processes incoming Stream Chat webhook events
func (h *WebhookHandler) HandleStreamWebhook(c *gin.Context) {
	// Read raw body for signature verification
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Failed to read request body",
		})
		return
	}

	// Verify webhook signature
	signature := c.GetHeader("X-Signature")
	if signature != "" {
		if !h.streamService.VerifyWebhook(body, signature) {
			c.JSON(http.StatusUnauthorized, ErrorResponse{
				Error:   "invalid_signature",
				Message: "Webhook signature verification failed",
			})
			return
		}
	}

	// Parse webhook event
	var event StreamWebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_json",
			Message: "Failed to parse webhook payload",
		})
		return
	}

	// Only process new messages
	if event.Type == "message.new" && event.Message != nil {
		h.handleNewMessage(event.Message, event.Channel)
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// handleNewMessage processes new messages and generates GPT responses
func (h *WebhookHandler) handleNewMessage(message *StreamMessage, channel *StreamChannel) {
	// Skip messages from bots to avoid loops
	if message.User.Role == "admin" || message.User.ID == "chatbot" || message.User.ID == "ai-assistant" {
		return
	}

	// Only respond in AI chat channels (channels with ID starting with "ai-chat-")
	if len(channel.ID) < 8 || channel.ID[:8] != "ai-chat-" {
		return
	}

	// Generate GPT response
	aiResponse, err := h.chatGPTService.GenerateResponse(nil, message.Text, "gpt-3.5-turbo")
	if err != nil {
		aiResponse = "I'm sorry, I'm having trouble processing your request right now."
	}

	// Send response back to Stream Chat
	h.streamService.SendMessage(channel.CID, aiResponse, "ai-assistant")
}