package handlers

import (
	"context"
	"fmt"

	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"jevi-chat/config"
	"jevi-chat/models"
	"jevi-chat/middleware"
)

// EmbedChat - GET /embed/:projectId
func EmbedChat(c *gin.Context) {
	projectID := c.Param("projectId")

	userToken := c.Query("token")
	if userToken == "" {
		// No token, show pre-auth UI
		c.HTML(http.StatusOK, "prechat.html", gin.H{
			"project_id": projectID,
			"api_url":    os.Getenv("APP_URL"),
		})
		return
	}

	// Validate project ID
	objID, err := primitive.ObjectIDFromHex(projectID)
	if err != nil {
		c.HTML(http.StatusOK, "error.html", gin.H{"error": "Invalid project ID"})
		return
	}

	// Fetch project from DB
	projectCollection := config.GetCollection("projects")
	var project models.Project
	err = projectCollection.FindOne(context.Background(), bson.M{"_id": objID}).Decode(&project)
	if err != nil || !project.IsActive {
		c.HTML(http.StatusOK, "error.html", gin.H{"error": "Project not found or inactive"})
		return
	}

	// Validate token using middleware function
	claims, err := validateUserToken(userToken)
	if err != nil {
		c.Redirect(http.StatusFound, fmt.Sprintf("/embed/%s", projectID))
		return
	}

	// Extract userID from claims
	userID := claims.UserID

	// Fetch user
	userCollection := config.GetCollection("chat_users")
	var user models.ChatUser
	userObjID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		c.Redirect(http.StatusFound, fmt.Sprintf("/embed/%s", projectID))
		return
	}

	err = userCollection.FindOne(context.Background(), bson.M{"_id": userObjID}).Decode(&user)
	if err != nil {
		c.Redirect(http.StatusFound, fmt.Sprintf("/embed/%s", projectID))
		return
	}

	// Render chat UI
	c.HTML(http.StatusOK, "chat.html", gin.H{
		"project":    project,
		"project_id": projectID,
		"api_url":    os.Getenv("APP_URL"),
		"user":       user,
		"user_token": userToken,
	})
}

// EmbedAuth - POST /embed/:projectId/auth
func EmbedAuth(c *gin.Context) {
	projectID := c.Param("projectId")

	var authData struct {
		Mode     string `json:"mode"`
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := c.ShouldBindJSON(&authData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid data"})
		return
	}

	// Validate project
	objID, err := primitive.ObjectIDFromHex(projectID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid project"})
		return
	}

	projectCollection := config.GetCollection("projects")
	var project models.Project
	if err := projectCollection.FindOne(context.Background(), bson.M{"_id": objID}).Decode(&project); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Project not found"})
		return
	}

	userCollection := config.GetCollection("chat_users")

	if authData.Mode == "register" {
		// Check if user exists
		var existingUser models.ChatUser
		err := userCollection.FindOne(context.Background(), bson.M{
			"project_id": projectID,
			"email":      authData.Email,
		}).Decode(&existingUser)
		if err == nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Email already registered"})
			return
		}

		// Hash password using middleware function
		hashedPassword, err := middleware.HashPassword(authData.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to process password"})
			return
		}

		// Create new user
		user := models.ChatUser{
			ProjectID:     projectID,
			Name:          authData.Name,
			Email:         authData.Email,
			Password:      hashedPassword,
			IsActive:      true,
			TotalSessions: 0,
			TotalMessages: 0,
			TotalTokens:   0,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}

		result, err := userCollection.InsertOne(context.Background(), user)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to create user"})
			return
		}

		user.ID = result.InsertedID.(primitive.ObjectID)
		
		// Create a temporary User object for token generation
		tempUser := &models.User{
			ID:    user.ID,
			Name:  user.Name,
			Email: user.Email,
			Role:  "chat_user",
		}
		
		token, err := middleware.GenerateJWTToken(tempUser)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to generate token"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"user": gin.H{
				"id":    user.ID.Hex(),
				"name":  user.Name,
				"email": user.Email,
			},
			"token": token,
		})
		return
	}

	// Login
	var user models.ChatUser
	err = userCollection.FindOne(context.Background(), bson.M{
		"project_id": projectID,
		"email":      authData.Email,
	}).Decode(&user)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Invalid credentials"})
		return
	}

	// Verify password using middleware function
	if !middleware.CheckPasswordHash(authData.Password, user.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Invalid credentials"})
		return
	}

	if !user.IsActive {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Account deactivated"})
		return
	}

	// Create a temporary User object for token generation
	tempUser := &models.User{
		ID:    user.ID,
		Name:  user.Name,
		Email: user.Email,
		Role:  "chat_user",
	}
	
	token, err := middleware.GenerateJWTToken(tempUser)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"user": gin.H{
			"id":    user.ID.Hex(),
			"name":  user.Name,
			"email": user.Email,
		},
		"token": token,
	})
}

// IframeChatInterface - GET /embed/:projectId/chat
func IframeChatInterface(c *gin.Context) {
	projectID := c.Param("projectId")

	objID, err := primitive.ObjectIDFromHex(projectID)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid project ID")
		return
	}

	var project models.Project
	err = config.GetCollection("projects").FindOne(context.Background(), bson.M{"_id": objID}).Decode(&project)
	if err != nil {
		c.String(http.StatusNotFound, "Project not found")
		return
	}

	// Check if project is active
	if !project.IsActive {
		c.String(http.StatusForbidden, "Project is inactive")
		return
	}

	// Render the chat.html template
	c.HTML(http.StatusOK, "embed/chat.html", gin.H{
		"project":    project,
		"project_id": project.ID.Hex(),
		"api_url":    os.Getenv("APP_URL"),
	})
}

// EmbedHealth - Simple health check
func EmbedHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"service":   "troika-tech-embed",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// ShowEmbedAuth - GET /embed/:projectId/auth
func ShowEmbedAuth(c *gin.Context) {
	projectID := c.Param("projectId")

	// Validate project ID
	objID, err := primitive.ObjectIDFromHex(projectID)
	if err != nil {
		c.HTML(http.StatusOK, "error.html", gin.H{"error": "Invalid project ID"})
		return
	}

	// Get project details
	collection := config.GetCollection("projects")
	var project models.Project
	err = collection.FindOne(context.Background(), bson.M{"_id": objID}).Decode(&project)
	if err != nil {
		c.HTML(http.StatusOK, "error.html", gin.H{"error": "Project not found"})
		return
	}

	// Check if project is active
	if !project.IsActive {
		c.HTML(http.StatusOK, "error.html", gin.H{"error": "Project is inactive"})
		return
	}

	// Render authentication page
	c.HTML(http.StatusOK, "embed/auth.html", gin.H{
		"project":    project,
		"project_id": projectID,
		"api_url":    os.Getenv("APP_URL"),
	})
}

// validateUserToken - Validates JWT token and returns claims (uses middleware validation)
func validateUserToken(tokenString string) (*middleware.JWTClaims, error) {
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return nil, fmt.Errorf("JWT secret not configured")
	}

	// Use the same validation logic as middleware but return the claims directly
	claims, err := middleware.ValidateJWTToken(tokenString)
	if err != nil {
		return nil, err
	}

	return claims, nil
}
