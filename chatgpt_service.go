package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/sashabaranov/go-openai"
)

// ChatGPTService handles OpenAI ChatGPT integration
type ChatGPTService struct {
	client *openai.Client
}

// NewChatGPTService creates a new ChatGPT service instance
func NewChatGPTService(apiKey string) *ChatGPTService {
	client := openai.NewClient(apiKey)
	return &ChatGPTService{
		client: client,
	}
}

// GenerateResponse generates a ChatGPT response based on message history
func (s *ChatGPTService) GenerateResponse(messages []Message, userMessage string, model string) (string, error) {
	return s.GenerateResponseWithCustomSystem(messages, userMessage, "", model)
}

// GenerateResponseWithCustomSystem generates a response with custom system prompt
func (s *ChatGPTService) GenerateResponseWithCustomSystem(messages []Message, userMessage, systemPrompt, model string) (string, error) {
	// Default to GPT-3.5-turbo if no model specified
	if model == "" {
		model = openai.GPT3Dot5Turbo
	}

	// Use default system prompt if none provided
	if systemPrompt == "" {
		systemPrompt = "You are an AI meant to help people find new connections. You have access to the conversation history and can respond naturally to questions and participate in discussions. Be concise and helpful."
	}

	// Convert messages to OpenAI format
	var openAIMessages []openai.ChatCompletionMessage

	// Add system message
	systemMessage := openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleSystem,
		Content: systemPrompt,
	}
	openAIMessages = append(openAIMessages, systemMessage)

	// Add message history for context
	for _, msg := range messages {
		var role string
		switch msg.MessageType {
		case "assistant":
			role = openai.ChatMessageRoleAssistant
		case "system":
			role = openai.ChatMessageRoleSystem
		default:
			role = openai.ChatMessageRoleUser
		}

		// Format message with username for better context
		content := msg.MessageText
		if msg.SenderUsername != "" && msg.MessageType == "user" {
			content = fmt.Sprintf("%s: %s", msg.SenderUsername, msg.MessageText)
		}

		openAIMessages = append(openAIMessages, openai.ChatCompletionMessage{
			Role:    role,
			Content: content,
		})
	}

	// Add the new user message
	openAIMessages = append(openAIMessages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: userMessage,
	})

	// Set max tokens based on model
	maxTokens := DefaultMaxTokens
	if model == openai.GPT4 || model == openai.GPT4TurboPreview {
		maxTokens = GPT4MaxTokens
	}

	// Create completion request
	request := openai.ChatCompletionRequest{
		Model:       model,
		Messages:    openAIMessages,
		MaxTokens:   maxTokens,
		Temperature: 0.7,
	}

	// Generate response
	resp, err := s.client.CreateChatCompletion(context.Background(), request)
	if err != nil {
		return "", fmt.Errorf("failed to generate ChatGPT response: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response choices returned from ChatGPT")
	}

	return resp.Choices[0].Message.Content, nil
}

// NeedsProfileSetup checks if a user needs to set up their profile
func (s *ChatGPTService) NeedsProfileSetup(user *User) bool {
	return user.Name == "" || user.ProfilePicURL == ""
}

// GenerateProfileSetupResponse generates a response asking for profile information
func (s *ChatGPTService) GenerateProfileSetupResponse(user *User) (string, error) {
	systemPrompt := `You are a helpful AI assistant that needs to collect profile information from new users. 

The user needs to provide:
1. Their name (required)
2. A profile picture URL (required) 
3. A bio (optional)

Ask them politely to provide this information. Be conversational and friendly. Explain that you need this information to personalize their experience.

If they ask why you need this information, explain that it helps create a better user experience and allows other users to identify them in the chat.

Keep your response concise but friendly.`

	// Create a simple request to generate the profile setup message
	request := openai.ChatCompletionRequest{
		Model: openai.GPT3Dot5Turbo,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: systemPrompt,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: "I'm a new user who just joined the chat.",
			},
		},
		MaxTokens:   200,
		Temperature: 0.7,
	}

	resp, err := s.client.CreateChatCompletion(context.Background(), request)
	if err != nil {
		return "", fmt.Errorf("failed to generate profile setup response: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response choices returned")
	}

	return resp.Choices[0].Message.Content, nil
}

// ProfileSetupData represents parsed profile information
type ProfileSetupData struct {
	Name          string
	ProfilePicURL string
	Bio           string
}

// StreamMessageAttachment represents a message attachment
type StreamMessageAttachment struct {
	Type     string `json:"type"`
	ImageURL string `json:"image_url"`
}

// ParseProfileFromStreamMessage extracts profile info from Stream Chat message
func (s *ChatGPTService) ParseProfileFromStreamMessage(messageText string, attachments []StreamMessageAttachment) (*ProfileSetupData, error) {
	profile := &ProfileSetupData{}

	// Extract name from message text using ChatGPT
	namePrompt := fmt.Sprintf(`Extract the person's name from this message. Only return the name, nothing else. If no name is found, return "NONE".

Message: "%s"`, messageText)

	nameRequest := openai.ChatCompletionRequest{
		Model: openai.GPT3Dot5Turbo,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: namePrompt,
			},
		},
		MaxTokens:   50,
		Temperature: 0.1,
	}

	resp, err := s.client.CreateChatCompletion(context.Background(), nameRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to extract name: %w", err)
	}

	if len(resp.Choices) > 0 {
		extractedName := strings.TrimSpace(resp.Choices[0].Message.Content)
		if extractedName != "NONE" && extractedName != "" {
			profile.Name = extractedName
		}
	}

	// Extract bio/interests from message text
	bioPrompt := fmt.Sprintf(`Extract any bio/interests/personal information from this message (excluding the name). Return only the bio part or "NONE" if no bio found.

Message: "%s"`, messageText)

	bioRequest := openai.ChatCompletionRequest{
		Model: openai.GPT3Dot5Turbo,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: bioPrompt,
			},
		},
		MaxTokens:   100,
		Temperature: 0.1,
	}

	resp, err = s.client.CreateChatCompletion(context.Background(), bioRequest)
	if err == nil && len(resp.Choices) > 0 {
		extractedBio := strings.TrimSpace(resp.Choices[0].Message.Content)
		if extractedBio != "NONE" && extractedBio != "" {
			profile.Bio = extractedBio
		}
	}

	// Extract profile picture URL from attachments
	for _, attachment := range attachments {
		if attachment.Type == "image" && attachment.ImageURL != "" {
			profile.ProfilePicURL = attachment.ImageURL
			break
		}
	}

	return profile, nil
}

// ValidateProfileData validates the parsed profile information
func (s *ChatGPTService) ValidateProfileData(profile *ProfileSetupData) error {
	if strings.TrimSpace(profile.Name) == "" {
		return fmt.Errorf("name is required")
	}

	if strings.TrimSpace(profile.ProfilePicURL) == "" {
		return fmt.Errorf("profile picture is required")
	}

	// Validate name length and basic format
	name := strings.TrimSpace(profile.Name)
	if len(name) < 1 || len(name) > 50 {
		return fmt.Errorf("name must be between 1 and 50 characters")
	}

	// Validate bio length if provided
	if len(profile.Bio) > 500 {
		return fmt.Errorf("bio must be less than 500 characters")
	}

	return nil
}

// IsProfileComplete checks if we have all required information
func (s *ChatGPTService) IsProfileComplete(profile *ProfileSetupData) bool {
	return strings.TrimSpace(profile.Name) != "" && strings.TrimSpace(profile.ProfilePicURL) != ""
}

// GenerateProfileConfirmationMessage creates a confirmation message
func (s *ChatGPTService) GenerateProfileConfirmationMessage(profile *ProfileSetupData) string {
	msg := fmt.Sprintf("Perfect! I've got your profile set up:\n\n")
	msg += fmt.Sprintf("Name: %s\n", profile.Name)
	msg += "Profile Picture: ✓ Uploaded\n"

	if profile.Bio != "" {
		msg += fmt.Sprintf("Bio: %s\n", profile.Bio)
	}

	msg += "\nYour profile is now complete! Let's start matching you with new people! Who are you looking to meet?"

	return msg
}

// RecommendUser finds and returns a user recommendation based on preferences
func (s *ChatGPTService) RecommendUser(preferences string, currentUserID string, supabaseService *SupabaseService) (*User, error) {
	// Get all users except current user
	log.Printf("[CHATGPT] Fetching users excluding current user ID: %s", currentUserID)
	users, err := supabaseService.GetUsersExcluding(currentUserID, 20)
	if err != nil {
		return nil, fmt.Errorf("failed to get users: %w", err)
	}

	log.Printf("Found %d users for recommendation", len(users))
	if len(users) == 0 {
		return nil, fmt.Errorf("no other users found")
	}

	// print out all of the users
	for _, u := range users {
		log.Printf("User: ID=%s, Name=%s, Bio=%s", u.ID, u.Name, u.Bio)
	}

	// Simply return the first user
	return &users[0], nil
}

// GenerateMatchResponse creates a user recommendation message
func (s *ChatGPTService) GenerateMatchResponse(recommendedUser *User) string {
	bio := recommendedUser.Bio
	if bio == "" {
		bio = "They haven't shared much about themselves yet, but that could be a great conversation starter!"
	}

	return fmt.Sprintf(`Great! I found someone I think you'd like to meet:

**%s**

%s

Would you like me to connect you with %s? Just say "yes" and I'll create a chat between you two!`,
		recommendedUser.Name, bio, recommendedUser.Name)
}

// UpdateUserProfileInDB updates the user profile in Supabase with parsed information
func (s *ChatGPTService) UpdateUserProfileInDB(userID string, profile *ProfileSetupData, supabaseService *SupabaseService, streamService *StreamService) error {
	// Prepare update data
	updates := map[string]any{
		"name":            profile.Name,
		"profile_pic_url": profile.ProfilePicURL,
	}

	// Add bio if provided
	if profile.Bio != "" {
		updates["bio"] = profile.Bio
	}

	// Update user in database
	updatedUser, err := supabaseService.UpdateUser(userID, updates)
	if err != nil {
		return fmt.Errorf("failed to update user profile: %w", err)
	}

	// Update Stream Chat user to sync the profile changes
	err = streamService.CreateOrUpdateUser(context.Background(), updatedUser)
	if err != nil {
		// Log the error but don't fail the operation since Supabase update succeeded
		fmt.Printf("Warning: failed to sync profile with Stream Chat: %v", err)
	}

	return nil
}
