package main

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// StreamHandler handles Stream Chat-related HTTP requests
type StreamHandler struct {
	streamService *StreamService
	authService   *AuthService
}

// NewStreamHandler creates a new Stream handler
func NewStreamHandler(streamService *StreamService, authService *AuthService) *StreamHandler {
	return &StreamHandler{
		streamService: streamService,
		authService:   authService,
	}
}

// GenerateToken handles Stream token generation requests
// @Summary Generate Stream Chat token
// @Description Generate a Stream Chat token for a user
// @Tags Stream Chat
// @Accept json
// @Produce json
// @Param request body TokenRequest true "Token generation request"
// @Success 200 {object} TokenResponse "Successfully generated token"
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 404 {object} ErrorResponse "User not found"
// @Failure 500 {object} ErrorResponse "Token generation failed"
// @Router /stream/token [post]
func (h *StreamHandler) GenerateToken(c *gin.Context) {
	var req TokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
		return
	}

	// Validate user exists in our system
	_, err := h.authService.GetUser(req.UserID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "user_not_found",
			Message: "User does not exist in the system",
		})
		return
	}

	// Generate Stream token
	token, err := h.streamService.CreateToken(req.UserID, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "token_generation_failed",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, TokenResponse{
		Token:  token,
		UserID: req.UserID,
	})
}

// GenerateTokenWithExpiry handles Stream token generation with expiration
func (h *StreamHandler) GenerateTokenWithExpiry(c *gin.Context) {
	var req struct {
		UserID     string `json:"user_id" binding:"required"`
		ExpiryTime int64  `json:"expiry_time,omitempty"` // Unix timestamp
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
		return
	}

	// Validate user exists
	_, err := h.authService.GetUser(req.UserID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "user_not_found",
			Message: "User does not exist in the system",
		})
		return
	}

	var expiry *time.Time
	if req.ExpiryTime > 0 {
		t := time.Unix(req.ExpiryTime, 0)
		expiry = &t
	}

	// Generate Stream token
	token, err := h.streamService.CreateToken(req.UserID, expiry)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "token_generation_failed",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, TokenResponse{
		Token:  token,
		UserID: req.UserID,
	})
}

// CreateOrUpdateUser handles Stream user creation/update
// @Summary Create or update Stream user
// @Description Create or update a user in Stream Chat
// @Tags Stream Chat
// @Accept json
// @Produce json
// @Param request body StreamUserRequest true "User creation/update request"
// @Success 200 {object} object{message=string,user_id=string} "User created/updated successfully"
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 500 {object} ErrorResponse "Stream user creation failed"
// @Router /stream/user [post]
func (h *StreamHandler) CreateOrUpdateUser(c *gin.Context) {
	var req StreamUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
		return
	}

	// Create user object
	user := &User{
		ID:            req.ID,
		Username:      req.Username,
		Name:          req.Name,
		ProfilePicURL: req.Image,
	}

	// Create or update user in Stream
	if err := h.streamService.CreateOrUpdateUser(c.Request.Context(), user); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "stream_user_creation_failed",
			Message: err.Error(),
		})
		return
	}

	// Also update in local auth service if user exists
	if localUser, err := h.authService.GetUser(req.ID); err == nil {
		updates := map[string]interface{}{
			"name":     req.Name,
			"username": req.Username,
		}
		if req.Image != "" {
			updates["profile_pic_url"] = req.Image
		}
		
		h.authService.UpdateUser(req.ID, updates)
		_ = localUser // avoid unused variable warning
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "User created/updated successfully",
		"user_id": req.ID,
	})
}

// RevokeUserToken handles token revocation for a user
func (h *StreamHandler) RevokeUserToken(c *gin.Context) {
	var req struct {
		UserID     string `json:"user_id" binding:"required"`
		RevokeTime *int64 `json:"revoke_time,omitempty"` // Unix timestamp, null to undo revocation
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
		return
	}

	var revokeTime *time.Time
	if req.RevokeTime != nil {
		t := time.Unix(*req.RevokeTime, 0)
		revokeTime = &t
	}

	// Revoke token in Stream
	if err := h.streamService.RevokeUserToken(c.Request.Context(), req.UserID, revokeTime); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "token_revocation_failed",
			Message: err.Error(),
		})
		return
	}

	message := "Token revoked successfully"
	if revokeTime == nil {
		message = "Token revocation undone successfully"
	}

	c.JSON(http.StatusOK, gin.H{
		"message": message,
		"user_id": req.UserID,
	})
}