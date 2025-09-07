package main

import (
	"log"
	"os"

	_ "social-messenger-backend/docs" // Import generated docs

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title Social Messenger API
// @version 1.0
// @description Go API backend for Stream Chat integration with user authentication and Supabase database
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io

// @license.name MIT
// @license.url https://opensource.org/licenses/MIT

// @host localhost:8080
// @BasePath /
// @securityDefinitions.apikey Bearer
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

// handleDatabaseTest creates a handler for database testing
func handleDatabaseTest(supabaseService *SupabaseService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Test 1: Try a simple select first
		selectResult, selectCount, selectErr := supabaseService.client.From("users").
			Select("*", "", false).
			Execute()
		
		// Check what's in the environment
		supabaseURL := os.Getenv("SUPABASE_URL")
		supabaseKey := os.Getenv("SUPABASE_SERVICE_KEY")
		
		c.JSON(200, gin.H{
			"select_result": string(selectResult),
			"select_count": selectCount,
			"select_error": func() string {
				if selectErr != nil {
					return selectErr.Error()
				}
				return ""
			}(),
			"message": "Direct insert methods removed - use auth endpoints for user creation",
			"supabase_url": supabaseURL,
			"key_length": len(supabaseKey),
			"key_prefix": func() string {
				if len(supabaseKey) > 20 {
					return supabaseKey[:20] + "..."
				}
				return supabaseKey
			}(),
		})
	}
}

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Initialize Supabase service
	supabaseService, err := NewSupabaseService(
		os.Getenv("SUPABASE_URL"),
		os.Getenv("SUPABASE_SERVICE_KEY"),
	)
	if err != nil {
		log.Fatal("Failed to initialize Supabase service:", err)
	}

	// Initialize Stream client
	streamService := NewStreamService(
		os.Getenv("STREAM_API_KEY"),
		os.Getenv("STREAM_SECRET"),
	)

	// Initialize message service
	messageService := NewMessageService(supabaseService.client)

	// Initialize ChatGPT service
	chatGPTService := NewChatGPTService(os.Getenv("OPENAI_API_KEY"))

	// Initialize auth service with Supabase
	authService := NewAuthService(os.Getenv("JWT_SECRET"), supabaseService)

	// Initialize pub/sub service for handshakes
	pubsubService := NewPubSubService()

	// Initialize handshake service
	handshakeService := NewHandshakeService(pubsubService)

	// Initialize handlers
	authHandler := NewAuthHandler(authService, streamService)
	streamHandler := NewStreamHandler(streamService, authService)
	chatbotHandler := NewChatbotHandler(messageService, chatGPTService, authService, streamService)
	webhookHandler := NewWebhookHandler(chatGPTService, streamService)
	handshakeHandler := NewHandshakeHandler(handshakeService, pubsubService)

	// Setup router
	r := gin.Default()

	// Configure CORS
	config := cors.DefaultConfig()
	config.AllowAllOrigins = true
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"Origin", "Content-Length", "Content-Type", "Authorization", "Accept", "X-Requested-With"}
	config.ExposeHeaders = []string{"Content-Length", "Authorization"}
	config.AllowCredentials = true
	r.Use(cors.New(config))

	// Swagger documentation
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Health check
	// @Summary Health check
	// @Description Check if the server is running
	// @Tags Health
	// @Produce json
	// @Success 200 {object} object{status=string} "Server is healthy"
	// @Router /health [get]
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Database test endpoint
	// @Summary Test database connection
	// @Description Test inserting data into the users table
	// @Tags Testing
	// @Produce json
	// @Success 200 {object} object{test_data=string,result=string,count=int,error=string} "Database test result"
	// @Router /test-db [get]
	r.GET("/test-db", handleDatabaseTest(supabaseService))

	// Auth routes
	r.POST("/auth/login", authHandler.Login)
	r.POST("/auth/register", authHandler.Register)

	// Handshake routes
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
	r.POST("/handshake/send", handshakeHandler.SendHandshake)

	// @Summary Connect to handshake WebSocket
	// @Description Establish WebSocket connection to receive real-time handshake events
	// @Tags Handshake
	// @Param uid query string true "User ID"
	// @Success 101 {string} string "Switching Protocols"
	// @Failure 400 {object} ErrorResponse "Invalid request"
	// @Router /handshake/ws [get]
	r.GET("/handshake/ws", handshakeHandler.WebSocketConnect)

	// @Summary Get active users
	// @Description Get list of users currently connected to handshake events
	// @Tags Handshake
	// @Produce json
	// @Success 200 {object} object{users=[]string} "List of active users"
	// @Router /handshake/active [get]
	r.GET("/handshake/active", handshakeHandler.GetActiveUsers)

	// Stream token routes
	// @Summary Generate Stream token
	// @Description Generate a Stream Chat token for authenticated user
	// @Tags Stream
	// @Accept json
	// @Produce json
	// @Security Bearer
	// @Param request body TokenRequest true "Token request"
	// @Success 200 {object} TokenResponse "Token generated successfully"
	// @Failure 401 {object} ErrorResponse "Unauthorized"
	// @Router /stream/token [post]
	r.POST("/stream/token", streamHandler.GenerateToken)
	
	// @Summary Create or update Stream user
	// @Description Create or update user in Stream Chat
	// @Tags Stream
	// @Accept json
	// @Produce json
	// @Security Bearer
	// @Param request body StreamUserRequest true "Stream user data"
	// @Success 200 {object} object{message=string} "User created/updated successfully"
	// @Failure 401 {object} ErrorResponse "Unauthorized"
	// @Router /stream/user [post]
	r.POST("/stream/user", streamHandler.CreateOrUpdateUser)
	
	// @Summary Get user channels
	// @Description Get all channels that a user is a member of
	// @Tags Stream
	// @Produce json
	// @Security Bearer
	// @Param user_id path string true "User ID"
	// @Success 200 {array} StreamChannel "User channels"
	// @Failure 401 {object} ErrorResponse "Unauthorized"
	// @Failure 404 {object} ErrorResponse "User not found"
	// @Failure 500 {object} ErrorResponse "Failed to retrieve channels"
	// @Router /stream/channels/{user_id} [get]
	r.GET("/stream/channels/:user_id", streamHandler.GetUserChannels)

	// Chatbot routes
	// @Summary Chat with bot
	// @Description Send a message to the chatbot and get a response
	// @Tags Chatbot
	// @Accept json
	// @Produce json
	// @Security Bearer
	// @Param request body ChatbotRequest true "Chat request"
	// @Success 200 {object} ChatbotResponse "Bot response"
	// @Failure 400 {object} ErrorResponse "Invalid request"
	// @Failure 401 {object} ErrorResponse "Unauthorized"
	// @Router /chatbot/chat [post]
	r.POST("/chatbot/chat", chatbotHandler.ChatWithBot)

	// Message routes
	// @Summary Get channel messages
	// @Description Retrieve messages from a specific channel
	// @Tags Messages
	// @Produce json
	// @Security Bearer
	// @Param channel_id path string true "Channel ID"
	// @Success 200 {array} Message "Channel messages"
	// @Failure 401 {object} ErrorResponse "Unauthorized"
	// @Router /messages/channel/{channel_id} [get]
	r.GET("/messages/channel/:channel_id", chatbotHandler.GetChannelMessages)

	// Webhook routes
	// @Summary Handle Stream webhook
	// @Description Handle incoming webhooks from Stream Chat
	// @Tags Webhooks
	// @Accept json
	// @Produce json
	// @Param request body StreamWebhookEvent true "Webhook event"
	// @Success 200 {object} WebhookResponse "Webhook processed"
	// @Failure 400 {object} ErrorResponse "Invalid request"
	// @Router /webhooks/stream [post]
	r.POST("/webhooks/stream", webhookHandler.HandleStreamWebhook)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s", port)
	r.Run(":" + port)
}
