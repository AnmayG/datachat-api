package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// AuthHandler handles authentication-related HTTP requests
type AuthHandler struct {
	authService   *AuthService
	streamService *StreamService
}

// NewAuthHandler creates a new authentication handler
func NewAuthHandler(authService *AuthService, streamService *StreamService) *AuthHandler {
	return &AuthHandler{
		authService:   authService,
		streamService: streamService,
	}
}

// createAuthResponse creates a complete authentication response with Stream token
func (h *AuthHandler) createAuthResponse(c *gin.Context, user *User, token string, statusCode int) {
	// Create Stream Chat token
	streamToken, err := h.streamService.CreateToken(user.ID, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "stream_token_error",
			Message: err.Error(),
		})
		return
	}

	// Create or update user in Stream
	if err := h.streamService.CreateOrUpdateUser(c.Request.Context(), user); err != nil {
		// Log error but don't fail the request
		c.Header("X-Stream-Warning", "Failed to sync user with Stream Chat")
	}
	
	// For new registrations, create AI chat channel
	if statusCode == http.StatusCreated {
		if _, err := h.streamService.CreateAIChatChannel(c.Request.Context(), user.ID); err != nil {
			// Log error but don't fail the request
			c.Header("X-Stream-Warning", "Failed to create AI chat channel")
		}
	}

	c.JSON(statusCode, AuthResponse{
		User:        *user,
		Token:       token,
		StreamToken: streamToken,
	})
}

// Login handles user login
// @Summary User login
// @Description Authenticate user by username or wallet address
// @Tags Authentication
// @Accept json
// @Produce json
// @Param request body LoginRequest true "Login credentials"
// @Success 200 {object} AuthResponse "Successfully authenticated"
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 401 {object} ErrorResponse "Authentication failed"
// @Failure 500 {object} ErrorResponse "Stream token error"
// @Router /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
		return
	}
	
	// Debug logging
	println("Login request - Username:", req.Username, "WalletAddress:", req.WalletAddress)

	// Authenticate user
	user, token, err := h.authService.Login(&req)
	if err != nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "authentication_failed",
			Message: err.Error(),
		})
		return
	}

	h.createAuthResponse(c, user, token, http.StatusOK)
}

// Register handles user registration
// @Summary User registration
// @Description Create a new user account
// @Tags Authentication
// @Accept json
// @Produce json
// @Param request body RegisterRequest true "Registration data"
// @Success 201 {object} AuthResponse "Successfully registered"
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 409 {object} ErrorResponse "Registration failed"
// @Failure 500 {object} ErrorResponse "Stream token error"
// @Router /auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
		return
	}

	// Create user account
	user, token, err := h.authService.Register(&req)
	if err != nil {
		c.JSON(http.StatusConflict, ErrorResponse{
			Error:   "registration_failed",
			Message: err.Error(),
		})
		return
	}

	h.createAuthResponse(c, user, token, http.StatusCreated)
}

// AuthMiddleware validates JWT tokens
func (h *AuthHandler) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, ErrorResponse{
				Error: "missing_authorization_header",
			})
			c.Abort()
			return
		}

		tokenString := authHeader
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			tokenString = authHeader[7:]
		}

		userID, err := h.authService.ValidateJWT(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, ErrorResponse{
				Error:   "invalid_token",
				Message: err.Error(),
			})
			c.Abort()
			return
		}

		c.Set("user_id", userID)
		c.Next()
	}
}