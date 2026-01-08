package middleware

import (
	"main/internal/auth"
	"main/internal/models"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

// JWTErrorHandler handles JWT validation errors
func JWTErrorHandler(c *fiber.Ctx, err error) error {
	return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
		"error":  "invalid or expired token",
		"status": fiber.StatusUnauthorized,
	})
}

// RateLimitReachedFiber handles rate limit exceeded
func RateLimitReachedFiber(c *fiber.Ctx) error {
	return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
		"error":  "rate limit exceeded",
		"status": fiber.StatusTooManyRequests,
	})
}

// ValidateTokenFiber validates JWT token and extracts claims
func ValidateTokenFiber(validator *auth.TokenValidator, log *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Get JWT claims from context (set by jwtware middleware)
		user := c.Locals("user")
		if user == nil {
			return fiber.NewError(fiber.StatusUnauthorized, "missing user in context")
		}

		claims, ok := user.(*jwt.Token).Claims.(jwt.MapClaims)
		if !ok {
			log.Error("Failed to parse JWT claims")
			return fiber.NewError(fiber.StatusUnauthorized, "invalid token claims")
		}

		// Extract user information from claims
		userID := ""
		username := ""
		email := ""
		role := ""

		if v, exists := claims["user_id"]; exists {
			userID = v.(string)
		}
		if v, exists := claims["username"]; exists {
			username = v.(string)
		}
		if v, exists := claims["email"]; exists {
			email = v.(string)
		}
		if v, exists := claims["role"]; exists {
			role = v.(string)
		}

		// Add user information to headers for downstream services
		c.Set("X-User-ID", userID)
		c.Set("X-Username", username)
		c.Set("X-User-Email", email)
		c.Set("X-User-Role", role)

		// Store claims in context for later use
		c.Locals("claims", claims)

		log.Debug("Token validated",
			zap.String("user_id", userID),
			zap.String("username", username),
		)

		return c.Next()
	}
}

// RequestLogger logs all incoming requests (alternative to built-in logger)
func RequestLoggerFiber(log *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		log.Info("Request received",
			zap.String("method", c.Method()),
			zap.String("path", c.Path()),
			zap.String("ip", c.IP()),
		)

		return c.Next()
	}
}

// ErrorHandler is a global error handler for Fiber
func ErrorHandlerFiber(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError

	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
	}

	return c.Status(code).JSON(models.ErrorResponse{
		Error:  err.Error(),
		Status: code,
	})
}
