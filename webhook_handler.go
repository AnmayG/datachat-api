package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// WebhookHandler handles Stream Chat webhook events
type WebhookHandler struct {
	chatGPTService    *ChatGPTService
	streamService     *StreamService
	processedWebhooks map[string]bool // Track processed webhook IDs for deduplication
}

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler(chatGPTService *ChatGPTService, streamService *StreamService) *WebhookHandler {
	return &WebhookHandler{
		chatGPTService:    chatGPTService,
		streamService:     streamService,
		processedWebhooks: make(map[string]bool),
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

	log.Printf("[MESSAGE] Generating AI response for message: %s", message.Text)

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