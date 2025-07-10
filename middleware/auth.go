package middleware

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/bcrypt"

	"jevi-chat/config"
	"jevi-chat/models"
)

// JWTClaims represents the JWT token claims
type JWTClaims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	Name   string `json:"name"`
	// Use RegisteredClaims instead of deprecated fields
	jwt.RegisteredClaims
}


// AuthMiddleware - Main authentication middleware for protected routes
// AuthMiddleware में admin के लिए special handling
func AuthMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        // Skip authentication for public routes
        if isPublicRoute(c.Request.URL.Path) {
            c.Next()
            return
        }

        // Extract token from header
        token := extractTokenFromHeader(c)
        if token == "" {
            log.Printf("❌ No token provided for protected route: %s", c.Request.URL.Path)
            c.JSON(http.StatusUnauthorized, gin.H{
                "error": "Authentication required",
                "code":  "NO_TOKEN",
            })
            c.Abort()
            return
        }

        // Validate and parse token
        claims, err := ValidateJWTToken(token)
        if err != nil {
            log.Printf("❌ Invalid token for route %s: %v", c.Request.URL.Path, err)
            c.JSON(http.StatusUnauthorized, gin.H{
                "error": "Invalid or expired token",
                "code":  "INVALID_TOKEN",
            })
            c.Abort()
            return
        }

        // ✅ Special handling for admin users
        if claims.Role == "admin" {
            // For admin users, set context directly without database lookup
            c.Set("user_id", claims.UserID)
            c.Set("user_email", claims.Email)
            c.Set("user_role", claims.Role)
            c.Set("user_name", claims.Name)
            
            log.Printf("✅ Admin authentication successful: %s (%s)", claims.Email, claims.Role)
            c.Next()
            return
        }

        // For regular users, verify user exists and is active
        user, err := getUserByID(claims.UserID)
        if err != nil {
            log.Printf("❌ User not found for token: %s", claims.UserID)
            c.JSON(http.StatusUnauthorized, gin.H{
                "error": "User not found or inactive",
                "code":  "USER_NOT_FOUND",
            })
            c.Abort()
            return
        }

        // Check if user is active
        if !user.IsActive {
            log.Printf("❌ Inactive user attempted access: %s", user.Email)
            c.JSON(http.StatusForbidden, gin.H{
                "error": "Account is deactivated",
                "code":  "ACCOUNT_DEACTIVATED",
            })
            c.Abort()
            return
        }

        // Add user info to context
        c.Set("user_id", claims.UserID)
        c.Set("user_email", claims.Email)
        c.Set("user_role", claims.Role)
        c.Set("user_name", claims.Name)
        c.Set("user", user)

        log.Printf("✅ Authentication successful for user: %s (%s)", user.Email, claims.Role)
        c.Next()
    }
}


// AdminMiddleware - Middleware to restrict access to admin-only routes
func AdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, exists := c.Get("user_role")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authentication required",
				"code":  "NO_AUTH",
			})
			c.Abort()
			return
		}

		role, ok := userRole.(string)
		if !ok || role != "admin" {
			userEmail, _ := c.Get("user_email")
			log.Printf("❌ Non-admin user attempted admin access: %s (role: %s)", userEmail, role)

			c.JSON(http.StatusForbidden, gin.H{
				"error": "Admin access required",
				"code":  "INSUFFICIENT_PRIVILEGES",
			})
			c.Abort()
			return
		}

		log.Printf("✅ Admin access granted for user: %s", c.GetString("user_email"))
		c.Next()
	}
}

// OptionalAuthMiddleware - Middleware for routes that work with or without authentication
func OptionalAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractTokenFromHeader(c)
		if token == "" {
			// No token provided, continue without authentication
			c.Next()
			return
		}

		// Validate token if provided
		claims, err := ValidateJWTToken(token)
		if err != nil {
			// Invalid token, continue without authentication
			log.Printf("⚠️ Invalid optional token: %v", err)
			c.Next()
			return
		}

		// Add user info to context if valid
		user, err := getUserByID(claims.UserID)
		if err == nil && user.IsActive {
			c.Set("user_id", claims.UserID)
			c.Set("user_email", claims.Email)
			c.Set("user_role", claims.Role)
			c.Set("user_name", claims.Name)
			c.Set("user", user)
		}

		c.Next()
	}
}

// CORSMiddleware - Enhanced CORS middleware with authentication support
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// Allow specific origins or all origins for development
		allowedOrigins := []string{
			"http://localhost:3000",
			"http://localhost:3001",
            "http://127.0.0.1:3000",
			"http://192.168.1.159:3000",
			"https://troikacompletefrontend.onrender.com",
			"https://troika-admin-dashborad.onrender.com/",
			"https://troika-admin-dashborad.onrender.com/api",
			"https://admin.troikatech.com",
		}

		// Check if origin is allowed
		isAllowed := false
		for _, allowedOrigin := range allowedOrigins {
			if origin == allowedOrigin {
				isAllowed = true
				break
			}
		}

		if isAllowed || os.Getenv("ENVIRONMENT") == "development" {
			c.Header("Access-Control-Allow-Origin", origin)
		}

		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Header("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// RateLimitMiddleware - Rate limiting middleware with user-based limits
func RateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := getClientIP(c)
		userID := c.GetString("user_id")

		// Different rate limits for authenticated vs anonymous users
		var identifier string
		var limit int
		var window time.Duration

		if userID != "" {
			// Authenticated user - higher limits
			identifier = fmt.Sprintf("user:%s", userID)
			limit = 1000 // 1000 requests per hour
			window = time.Hour
		} else {
			// Anonymous user - lower limits
			identifier = fmt.Sprintf("ip:%s", clientIP)
			limit = 100 // 100 requests per hour
			window = time.Hour
		}

		if !checkRateLimit(identifier, limit, window) {
			log.Printf("🚫 Rate limit exceeded for %s", identifier)
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "Rate limit exceeded",
				"code":        "RATE_LIMIT_EXCEEDED",
				"retry_after": 3600, // 1 hour in seconds
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// SecurityHeadersMiddleware - Add security headers
func SecurityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'")

		// Only add HSTS in production with HTTPS
		if os.Getenv("ENVIRONMENT") == "production" {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		c.Next()
	}
}

// Helper Functions

// extractTokenFromHeader - Extract JWT token from Authorization header
func extractTokenFromHeader(c *gin.Context) string {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		return ""
	}

	// Check for Bearer token format
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}

	return authHeader
}

// ValidateJWTToken - Validate and parse JWT token (exported for use in handlers)
func ValidateJWTToken(tokenString string) (*JWTClaims, error) {
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return nil, fmt.Errorf("JWT secret not configured")
	}

	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(jwtSecret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %v", err)
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	// Check if token is expired
	if claims.ExpiresAt != nil && claims.ExpiresAt.Time.Before(time.Now()) {
		return nil, fmt.Errorf("token has expired")
	}

	return claims, nil
}

// getUserByID - Get user by ID from database
func getUserByID(userID string) (*models.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	collection := config.GetCollection("users")

	objID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID format")
	}

	var user models.User
	err = collection.FindOne(ctx, bson.M{"_id": objID}).Decode(&user)
	if err != nil {
		return nil, fmt.Errorf("user not found: %v", err)
	}

	return &user, nil
}

// isPublicRoute - Check if route is public (doesn't require authentication)
func isPublicRoute(path string) bool {
	publicRoutes := []string{
		"/api/auth/login",
		"/api/auth/register",
		"/api/auth/logout",
		"/api/auth/verify",
		"/api/auth/forgot-password",
		"/api/auth/reset-password",
		"/api/public/",
		"/api/embed/",
		"/health",
		"/ping",
		"/widget.js",
		"/api/projects/", // Chat endpoints use project-based validation
	}

	for _, route := range publicRoutes {
		if strings.HasPrefix(path, route) {
			return true
		}
	}

	return false
}

// checkRateLimit - Basic rate limiting implementation
func checkRateLimit(identifier string, limit int, window time.Duration) bool {
	// This is a simplified implementation
	// In production, use Redis or similar for distributed rate limiting
	return true
}

// getClientIP - Get client IP address
func getClientIP(c *gin.Context) string {
	// Check for forwarded IP first
	if ip := c.GetHeader("X-Forwarded-For"); ip != "" {
		return strings.Split(ip, ",")[0]
	}
	if ip := c.GetHeader("X-Real-IP"); ip != "" {
		return ip
	}
	return c.ClientIP()
}

// GenerateJWTToken - Generate JWT token for user
func GenerateJWTToken(user *models.User) (string, error) {
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return "", fmt.Errorf("JWT secret not configured")
	}

	// Set token expiration (24 hours)
	expirationTime := time.Now().Add(24 * time.Hour)

	claims := &JWTClaims{
		UserID: user.ID.Hex(),
		Email:  user.Email,
		Role:   user.Role,
		Name:   user.Name,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "troika-tech",
			Subject:   user.ID.Hex(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(jwtSecret))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %v", err)
	}

	return tokenString, nil
}

// HashPassword - Hash password using bcrypt
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPasswordHash - Check if password matches hash
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// RefreshTokenMiddleware - Middleware to handle token refresh
func RefreshTokenMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractTokenFromHeader(c)
		if token == "" {
			c.Next()
			return
		}

		claims, err := ValidateJWTToken(token)
		if err != nil {
			c.Next()
			return
		}

		// Check if token expires within 1 hour
		if claims.ExpiresAt != nil && time.Until(claims.ExpiresAt.Time) < time.Hour {
			user, err := getUserByID(claims.UserID)
			if err == nil && user.IsActive {
				newToken, err := GenerateJWTToken(user)
				if err == nil {
					c.Header("X-New-Token", newToken)
					log.Printf("🔄 Token refreshed for user: %s", user.Email)
				}
			}
		}

		c.Next()
	}
}

// LoggingMiddleware - Enhanced logging middleware with authentication context
func LoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()

		// Process request
		c.Next()

		// Log request details
		duration := time.Since(startTime)
		userEmail := c.GetString("user_email")
		userRole := c.GetString("user_role")

		logEntry := fmt.Sprintf(
			"%s %s %d %v %s",
			c.Request.Method,
			c.Request.URL.Path,
			c.Writer.Status(),
			duration,
			getClientIP(c),
		)

		if userEmail != "" {
			logEntry += fmt.Sprintf(" [User: %s, Role: %s]", userEmail, userRole)
		}

		log.Printf("📝 %s", logEntry)
	}
}

// SessionValidationMiddleware - Validate user session and update last activity
func SessionValidationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		if userID == "" {
			c.Next()
			return
		}

		// Update user's last activity asynchronously
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			collection := config.GetCollection("users")
			objID, err := primitive.ObjectIDFromHex(userID)
			if err != nil {
				return
			}

			update := bson.M{
				"$set": bson.M{
					"last_login_at": time.Now(),
					"updated_at":    time.Now(),
				},
			}

			collection.UpdateOne(ctx, bson.M{"_id": objID}, update)
		}()

		c.Next()
	}
}



