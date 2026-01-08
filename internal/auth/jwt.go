package auth

import (
	"fmt"
	"main/internal/config"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

type TokenValidator struct {
	config *config.Config
	logger *zap.Logger
}

func NewTokenValidator(cfg *config.Config, log *zap.Logger) *TokenValidator {
	return &TokenValidator{
		config: cfg,
		logger: log,
	}
}

// ValidateToken validates JWT token and returns claims
func (tv *TokenValidator) ValidateToken(tokenString string) (*Claims, error) {
	claims := &Claims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(tv.config.JWT.SecretKey), nil
	})

	if err != nil {
		tv.logger.Debug("Token parsing failed",
			zap.Error(err),
			zap.String("token_preview", tokenString[:20]+"..."),
		)
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("token is invalid")
	}

	// Verify claims
	if err := tv.verifyClaims(claims); err != nil {
		return nil, err
	}

	return claims, nil
}

func (tv *TokenValidator) verifyClaims(claims *Claims) error {
	now := time.Now().Unix()

	// Check expiration
	if claims.ExpiresAt != nil && claims.ExpiresAt.Unix() < now {
		return fmt.Errorf("token has expired")
	}

	// Check issuer if configured
	if tv.config.JWT.Issuer != "" && claims.Issuer != tv.config.JWT.Issuer {
		return fmt.Errorf("invalid issuer: expected %s, got %s", tv.config.JWT.Issuer, claims.Issuer)
	}

	// Check audience if configured
	if tv.config.JWT.Audience != "" {
		audiences := claims.Audience
		found := false
		for _, aud := range audiences {
			if aud == tv.config.JWT.Audience {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("invalid audience")
		}
	}

	return nil
}

// GenerateToken generates a new JWT token (for testing/internal use)
func (tv *TokenValidator) GenerateToken(userID, username, email, role string) (string, error) {
	claims := &Claims{
		UserID:   userID,
		Username: username,
		Email:    email,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(tv.config.JWT.ExpiresIn) * time.Second)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    tv.config.JWT.Issuer,
			Audience:  jwt.ClaimStrings{tv.config.JWT.Audience},
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(tv.config.JWT.SecretKey))
	if err != nil {
		tv.logger.Error("Token generation failed", zap.Error(err))
		return "", fmt.Errorf("failed to generate token: %w", err)
	}

	return tokenString, nil
}

// ExtractToken extracts token from Authorization header
func ExtractToken(authHeader string) (string, error) {
	if authHeader == "" {
		return "", fmt.Errorf("authorization header is empty")
	}

	const scheme = "Bearer "
	if len(authHeader) < len(scheme) || authHeader[:len(scheme)] != scheme {
		return "", fmt.Errorf("invalid authorization header format")
	}

	return authHeader[len(scheme):], nil
}

// RefreshToken generates a new token with extended expiration
func (tv *TokenValidator) RefreshToken(claims *Claims) (string, error) {
	claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(time.Duration(tv.config.JWT.ExpiresIn) * time.Second))
	claims.IssuedAt = jwt.NewNumericDate(time.Now())

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(tv.config.JWT.SecretKey))
	if err != nil {
		return "", fmt.Errorf("failed to refresh token: %w", err)
	}

	return tokenString, nil
}
