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

// Login authenticates a user (simplified for demo)
func (a *AuthService) Login(req *LoginRequest) (*User, string, error) {
	var user *User
	var err error
	
	// Try to find user by username or wallet address
	if req.Username != "" {
		user, err = a.supabaseService.GetUserByUsername(req.Username)
	} else if req.WalletAddress != "" {
		user, err = a.supabaseService.GetUserByWallet(req.WalletAddress)
	} else {
		return nil, "", errors.New("username or wallet address required")
	}
	
	if err != nil {
		return nil, "", err
	}
	
	// If user doesn't exist, auto-create for demo purposes
	if user == nil {
		// Generate defaults if not provided
		username := req.Username
		if username == "" && req.WalletAddress != "" {
			username = "user_" + req.WalletAddress[:8]
		} else if username == "" {
			return nil, "", errors.New("username or wallet address required")
		}
		
		name := username
		if req.WalletAddress != "" {
			name = "Algorand User (" + req.WalletAddress[:8] + "...)"
		}

		newUser := &User{
			Username:      username,
			Name:          name,
			WalletAddress: req.WalletAddress,
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
	// Generate defaults if not provided
	username := req.Username
	if username == "" && req.WalletAddress != "" {
		username = "user_" + req.WalletAddress[:8]
	} else if username == "" {
		return nil, "", errors.New("username or wallet address required")
	}
	
	name := req.Name
	if name == "" {
		if req.WalletAddress != "" {
			name = "Algorand User (" + req.WalletAddress[:8] + "...)"
		} else {
			name = username
		}
	}

	// Check if user already exists
	exists, err := a.supabaseService.UserExists(username, req.WalletAddress)
	if err != nil {
		return nil, "", err
	}
	
	if exists {
		return nil, "", errors.New("user already exists")
	}

	user := &User{
		Username:      username,
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