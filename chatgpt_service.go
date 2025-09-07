package main

import (
	"context"
	"fmt"

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
		systemPrompt = "You are a helpful AI assistant in a chat channel. You have access to the conversation history and can respond naturally to questions and participate in discussions. Be concise and helpful."
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