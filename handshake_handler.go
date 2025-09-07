package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// HandshakeHandler handles handshake-related HTTP requests
type HandshakeHandler struct {
	handshakeService *HandshakeService
	pubsub          *PubSubService
	upgrader        websocket.Upgrader
}

// NewHandshakeHandler creates a new handshake handler
func NewHandshakeHandler(handshakeService *HandshakeService, pubsub *PubSubService) *HandshakeHandler {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins for simplicity
		},
	}
	
	return &HandshakeHandler{
		handshakeService: handshakeService,
		pubsub:          pubsub,
		upgrader:        upgrader,
	}
}

// SendHandshake handles sending a handshake event
// @Summary Send handshake
// @Description Send a handshake event to specific user or broadcast to all
// @Tags Handshake
// @Accept json
// @Produce json
// @Param uid query string true "User ID of sender"
// @Param request body HandshakeRequest true "Handshake request"
// @Success 200 {object} object{message=string} "Handshake sent successfully"
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Router /handshake/send [post]
func (hh *HandshakeHandler) SendHandshake(c *gin.Context) {
	uid := c.Query("uid")
	if uid == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "missing_uid",
			Message: "uid query parameter is required",
		})
		return
	}
	
	var request HandshakeRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "invalid_request",
			Message: err.Error(),
		})
		return
	}
	
	err := hh.handshakeService.SendHandshake(uid, request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: "send_failed",
			Message: err.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"message": "Handshake sent successfully",
	})
}

// WebSocketConnect handles WebSocket connections for real-time handshake events
// @Summary Connect to handshake WebSocket
// @Description Establish WebSocket connection to receive real-time handshake events
// @Tags Handshake
// @Param uid query string true "User ID"
// @Success 101 {string} string "Switching Protocols"
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Router /handshake/ws [get]
func (hh *HandshakeHandler) WebSocketConnect(c *gin.Context) {
	uid := c.Query("uid")
	if uid == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "missing_uid",
			Message: "uid query parameter is required",
		})
		return
	}
	
	conn, err := hh.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()
	
	// Subscribe user to handshake events
	hh.pubsub.Subscribe(uid, conn)
	defer hh.pubsub.Unsubscribe(uid, conn)
	
	log.Printf("WebSocket connection established for user: %s", uid)
	
	// Keep connection alive and handle disconnection
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			log.Printf("WebSocket connection closed for user %s: %v", uid, err)
			break
		}
	}
}

// GetActiveUsers returns list of users currently connected
// @Summary Get active users
// @Description Get list of users currently connected to handshake events
// @Tags Handshake
// @Produce json
// @Success 200 {object} object{users=[]string} "List of active users"
// @Router /handshake/active [get]
func (hh *HandshakeHandler) GetActiveUsers(c *gin.Context) {
	users := hh.handshakeService.GetActiveUsers()
	c.JSON(http.StatusOK, gin.H{
		"users": users,
	})
}