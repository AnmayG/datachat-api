package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"
	supa "github.com/supabase-community/supabase-go"
)

// SupabaseService handles Supabase database operations
type SupabaseService struct {
	client *supa.Client
	url    string
	key    string
}

// NewSupabaseService creates a new Supabase service instance
func NewSupabaseService(url, key string) (*SupabaseService, error) {
	client, err := supa.NewClient(url, key, &supa.ClientOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create Supabase client: %w", err)
	}

	return &SupabaseService{
		client: client,
		url:    url,
		key:    key,
	}, nil
}

// queryUsersByField retrieves users by a specific field using direct HTTP
func (s *SupabaseService) queryUsersByField(field, value string) (*User, error) {
	url := fmt.Sprintf("%s/rest/v1/users?%s=eq.%s", s.url, field, value)
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("apikey", s.key)
	req.Header.Set("Authorization", "Bearer "+s.key)
	req.Header.Set("Content-Type", "application/json")
	
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}
	
	var users []User
	err = json.Unmarshal(body, &users)
	if err != nil {
		return nil, fmt.Errorf("failed to decode user: %w", err)
	}
	
	if len(users) == 0 {
		return nil, nil // User not found
	}
	
	return &users[0], nil
}

// GetUserByUsername retrieves a user by username
func (s *SupabaseService) GetUserByUsername(username string) (*User, error) {
	return s.queryUsersByField("username", username)
}

// GetUserByWallet retrieves a user by wallet address
func (s *SupabaseService) GetUserByWallet(walletAddress string) (*User, error) {
	return s.queryUsersByField("wallet_address", walletAddress)
}

// GetUserByID retrieves a user by ID
func (s *SupabaseService) GetUserByID(id string) (*User, error) {
	return s.queryUsersByField("id", id)
}

// CreateUserWithUpsert creates a new user or returns existing one if wallet exists
func (s *SupabaseService) CreateUserWithUpsert(user *User) (*User, error) {
	// First check if user with wallet address already exists
	if user.WalletAddress != "" {
		existingUser, err := s.GetUserByWallet(user.WalletAddress)
		if err != nil {
			// Continue with creation attempt
		} else if existingUser != nil {
			return existingUser, nil
		}
	}
	
	return s.createUserInternal(user)
}

// CreateUser creates a new user (keeping original function for compatibility)
func (s *SupabaseService) CreateUser(user *User) (*User, error) {
	return s.CreateUserWithUpsert(user)
}

// createUserInternal handles the actual user creation logic using direct HTTP
func (s *SupabaseService) createUserInternal(user *User) (*User, error) {
	// Validate user fields
	if err := ValidateUserFields(user.Username, user.Name, user.Bio); err != nil {
		return nil, err
	}
	
	// Generate UUID if not provided
	if user.ID == "" {
		user.ID = uuid.New().String()
	}
	
	// Create user data matching the actual database schema
	userData := map[string]interface{}{
		"id":       user.ID,
		"username": user.Username,
	}
	
	// Only add optional fields if they have values
	if user.Name != "" {
		userData["name"] = user.Name
	}
	if user.WalletAddress != "" {
		userData["wallet_address"] = user.WalletAddress
	}
	if user.ProfilePicURL != "" {
		userData["profile_pic_url"] = user.ProfilePicURL
	}
	if user.Bio != "" {
		userData["bio"] = user.Bio
	}
	
	userDataJSON, err := json.Marshal(userData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal user data: %w", err)
	}
	
	// Direct HTTP POST to Supabase like the working Python test
	url := fmt.Sprintf("%s/rest/v1/users", s.url)
	
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(userDataJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("apikey", s.key)
	req.Header.Set("Authorization", "Bearer "+s.key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=representation")
	
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("insert failed with status %d: %s", resp.StatusCode, string(body))
	}
	
	var createdUsers []User
	err = json.Unmarshal(body, &createdUsers)
	if err != nil {
		return nil, fmt.Errorf("failed to decode created user: %w", err)
	}
	
	if len(createdUsers) == 0 {
		return nil, fmt.Errorf("user creation failed - no data returned")
	}
	
	return &createdUsers[0], nil
}

// UpdateUser updates an existing user using direct HTTP
func (s *SupabaseService) UpdateUser(id string, updates map[string]interface{}) (*User, error) {
	// Validate updated fields
	username, _ := updates["username"].(string)
	name, _ := updates["name"].(string)
	bio, _ := updates["bio"].(string)
	
	if err := ValidateUserFields(username, name, bio); err != nil {
		return nil, err
	}
	
	updatesJSON, err := json.Marshal(updates)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal updates: %w", err)
	}
	
	url := fmt.Sprintf("%s/rest/v1/users?id=eq.%s", s.url, id)
	
	req, err := http.NewRequest("PATCH", url, bytes.NewBuffer(updatesJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("apikey", s.key)
	req.Header.Set("Authorization", "Bearer "+s.key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=representation")
	
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("update failed with status %d: %s", resp.StatusCode, string(body))
	}
	
	var updatedUsers []User
	err = json.Unmarshal(body, &updatedUsers)
	if err != nil {
		return nil, fmt.Errorf("failed to decode updated user: %w", err)
	}
	
	if len(updatedUsers) == 0 {
		return nil, fmt.Errorf("user not found")
	}
	
	return &updatedUsers[0], nil
}

// GetUsersExcluding gets all users except the specified user ID, with pagination
func (s *SupabaseService) GetUsersExcluding(excludeUserID string, limit int) ([]User, error) {
	if limit <= 0 {
		limit = 10 // Default limit
	}
	
	url := fmt.Sprintf("%s/rest/v1/users?id=neq.%s&limit=%d", s.url, excludeUserID, limit)
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("apikey", s.key)
	req.Header.Set("Authorization", "Bearer "+s.key)
	req.Header.Set("Content-Type", "application/json")
	
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}
	
	var users []User
	err = json.Unmarshal(body, &users)
	if err != nil {
		return nil, fmt.Errorf("failed to decode users: %w", err)
	}
	
	return users, nil
}

// UserExists checks if a user exists by wallet address only (usernames are not unique)
func (s *SupabaseService) UserExists(username, walletAddress string) (bool, error) {
	// Only check wallet address for uniqueness, not username
	if walletAddress != "" {
		user, err := s.GetUserByWallet(walletAddress)
		if err != nil {
			return false, err
		}
		if user != nil {
			return true, nil
		}
		return false, nil
	}
	
	// If no wallet address provided, user doesn't exist (we need wallet for uniqueness)
	return false, nil
}