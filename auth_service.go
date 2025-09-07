package main

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// AuthService handles authentication operations
type AuthService struct {
	jwtSecret       string
	supabaseService *SupabaseService
}

// NewAuthService creates a new authentication service
func NewAuthService(jwtSecret string, supabaseService *SupabaseService) *AuthService {
	if jwtSecret == "" {
		jwtSecret = DefaultJWTSecret
	}
	
	return &AuthService{
		jwtSecret:       jwtSecret,
		supabaseService: supabaseService,
	}
}

// JWTClaims represents JWT token claims
type JWTClaims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

// Login authenticates a user by wallet address only
func (a *AuthService) Login(req *LoginRequest) (*User, string, error) {
	if req.WalletAddress == "" {
		return nil, "", errors.New("wallet address required")
	}
	
	// Look up user by wallet address
	user, err := a.supabaseService.GetUserByWallet(req.WalletAddress)
	if err != nil {
		return nil, "", err
	}
	
	// If user doesn't exist, auto-create with wallet address
	if user == nil {
		newUser := &User{
			WalletAddress: req.WalletAddress,
			Name:          "User", // Simple default name
		}
		
		user, err = a.supabaseService.CreateUser(newUser)
		if err != nil {
			return nil, "", err
		}
	}

	token, err := a.GenerateJWT(user.ID)
	if err != nil {
		return nil, "", err
	}

	return user, token, nil
}

// Register creates a new user account
func (a *AuthService) Register(req *RegisterRequest) (*User, string, error) {
	if req.WalletAddress == "" {
		return nil, "", errors.New("wallet address required")
	}
	
	// Check if user already exists by wallet address only
	existingUser, err := a.supabaseService.GetUserByWallet(req.WalletAddress)
	if err != nil {
		return nil, "", err
	}
	
	if existingUser != nil {
		return nil, "", errors.New("user already exists")
	}

	// Set defaults
	name := req.Name
	if name == "" {
		name = "User"
	}

	user := &User{
		Name:          name,
		WalletAddress: req.WalletAddress,
		ProfilePicURL: req.ProfilePicURL,
		Bio:           req.Bio,
	}

	createdUser, err := a.supabaseService.CreateUser(user)
	if err != nil {
		return nil, "", err
	}

	token, err := a.GenerateJWT(createdUser.ID)
	if err != nil {
		return nil, "", err
	}

	return createdUser, token, nil
}

// GenerateJWT creates a JWT token for a user
func (a *AuthService) GenerateJWT(userID string) (string, error) {
	claims := &JWTClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(a.jwtSecret))
}

// ValidateJWT validates a JWT token and returns the user ID
func (a *AuthService) ValidateJWT(tokenString string) (string, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(a.jwtSecret), nil
	})

	if err != nil {
		return "", err
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims.UserID, nil
	}

	return "", errors.New("invalid token")
}

// GetUser retrieves a user by ID
func (a *AuthService) GetUser(userID string) (*User, error) {
	user, err := a.supabaseService.GetUserByID(userID)
	if err != nil {
		return nil, err
	}
	
	if user == nil {
		return nil, errors.New("user not found")
	}
	
	return user, nil
}

// UpdateUser updates user information
func (a *AuthService) UpdateUser(userID string, updates map[string]interface{}) (*User, error) {
	return a.supabaseService.UpdateUser(userID, updates)
}