package handlers

import (
	"context"
	"log"
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

// Login - Unified admin and user login handler
// Login function में admin handling को update करें
func Login(c *gin.Context) {
    var loginData struct {
        Email    string `json:"email" binding:"required,email"`
        Password string `json:"password" binding:"required"`
    }

    if err := c.ShouldBindJSON(&loginData); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{
            "error": "Invalid request data",
            "details": err.Error(),
        })
        return
    }

    // Check for admin login first
    adminEmail := os.Getenv("ADMIN_EMAIL")
    adminPassword := os.Getenv("ADMIN_PASSWORD")

    if loginData.Email == adminEmail && loginData.Password == adminPassword {
        // ✅ Create proper admin user object for token generation
        adminUser := &models.User{
            ID:    primitive.NewObjectID(),
            Name:  "Super Admin",
            Email: adminEmail,
            Role:  "admin",
            IsActive: true, // ✅ Important: Set IsActive to true
        }

        // ✅ Generate JWT token using middleware function
        token, err := middleware.GenerateJWTToken(adminUser)
        if err != nil {
            log.Printf("❌ Failed to generate admin token: %v", err)
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
            return
        }

        log.Printf("✅ Admin login successful: %s, Token: %s", adminEmail, token[:20]+"...")

        c.JSON(http.StatusOK, gin.H{
            "message": "Admin login successful",
            "token":   token,
            "user": gin.H{
                "id":    "admin",
                "name":  "Super Admin",
                "email": adminEmail,
                "role":  "admin",
            },
        })
        return
    }

    // Regular user login logic...
}


// Register - User registration handler
func Register(c *gin.Context) {
	var registerData struct {
		Name     string `json:"name" binding:"required"`
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required,min=8"`
	}

	if err := c.ShouldBindJSON(&registerData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request data",
			"details": err.Error(),
		})
		return
	}

	collection := config.GetCollection("users")

	// Check if user already exists
	var existingUser models.User
	err := collection.FindOne(context.Background(), bson.M{"email": registerData.Email}).Decode(&existingUser)
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "User with this email already exists"})
		return
	}

	// Hash password using middleware function
	hashedPassword, err := middleware.HashPassword(registerData.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process password"})
		return
	}

	// Create new user
	user := models.User{
		ID:            primitive.NewObjectID(),
		Name:          registerData.Name,
		Email:         registerData.Email,
		Password:      hashedPassword,
		Role:          "user",
		IsActive:      true,
		EmailVerified: false,
		NotificationPrefs: models.DefaultUserNotificationPrefs,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	result, err := collection.InsertOne(context.Background(), user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	user.ID = result.InsertedID.(primitive.ObjectID)

	// Generate JWT token using middleware function
	token, err := middleware.GenerateJWTToken(&user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	log.Printf("✅ User registered successfully: %s", user.Email)

	c.JSON(http.StatusCreated, gin.H{
		"message": "Registration successful",
		"token":   token,
		"user": gin.H{
			"id":    user.ID.Hex(),
			"name":  user.Name,
			"email": user.Email,
			"role":  user.Role,
		},
	})
}

// GetUserProfile - Get current user profile
func GetUserProfile(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	collection := config.GetCollection("users")
	var user models.User

	objID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	err = collection.FindOne(context.Background(), bson.M{"_id": objID}).Decode(&user)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id":                 user.ID.Hex(),
			"name":               user.Name,
			"email":              user.Email,
			"role":               user.Role,
			"email_verified":     user.EmailVerified,
			"notification_prefs": user.NotificationPrefs,
			"created_at":         user.CreatedAt,
			"last_login_at":      user.LastLoginAt,
		},
	})
}

// UpdateUserProfile - Update user profile
func UpdateUserProfile(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var updateData struct {
		Name              string                        `json:"name"`
		Company           string                        `json:"company"`
		Phone             string                        `json:"phone"`
		Timezone          string                        `json:"timezone"`
		Language          string                        `json:"language"`
		NotificationPrefs models.UserNotificationPrefs `json:"notification_prefs"`
	}

	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid update data"})
		return
	}

	collection := config.GetCollection("users")
	objID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	update := bson.M{
		"$set": bson.M{
			"updated_at": time.Now(),
		},
	}

	// Update fields if provided
	if updateData.Name != "" {
		update["$set"].(bson.M)["name"] = updateData.Name
	}
	if updateData.Company != "" {
		update["$set"].(bson.M)["company"] = updateData.Company
	}
	if updateData.Phone != "" {
		update["$set"].(bson.M)["phone"] = updateData.Phone
	}
	if updateData.Timezone != "" {
		update["$set"].(bson.M)["timezone"] = updateData.Timezone
	}
	if updateData.Language != "" {
		update["$set"].(bson.M)["language"] = updateData.Language
	}

	result, err := collection.UpdateOne(context.Background(), bson.M{"_id": objID}, update)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile"})
		return
	}

	if result.ModifiedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Profile updated successfully",
	})
}

// ChangePassword - Change user password
func ChangePassword(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var passwordData struct {
		CurrentPassword string `json:"current_password" binding:"required"`
		NewPassword     string `json:"new_password" binding:"required,min=8"`
	}

	if err := c.ShouldBindJSON(&passwordData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	collection := config.GetCollection("users")
	objID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Get current user
	var user models.User
	err = collection.FindOne(context.Background(), bson.M{"_id": objID}).Decode(&user)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Verify current password using middleware function
	if !middleware.CheckPasswordHash(passwordData.CurrentPassword, user.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current password is incorrect"})
		return
	}

	// Hash new password using middleware function
	hashedPassword, err := middleware.HashPassword(passwordData.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process new password"})
		return
	}

	// Update password
	_, err = collection.UpdateOne(context.Background(),
		bson.M{"_id": objID},
		bson.M{"$set": bson.M{
			"password":   hashedPassword,
			"updated_at": time.Now(),
		}},
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update password"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Password changed successfully",
	})
}

// Logout - User logout
func Logout(c *gin.Context) {
	// In a stateless JWT system, logout is handled client-side by removing the token
	c.JSON(http.StatusOK, gin.H{
		"message": "Logged out successfully",
	})
}

// VerifyToken - Verify JWT token validity
func VerifyToken(c *gin.Context) {
	userID := c.GetString("user_id")
	userEmail := c.GetString("user_email")
	userRole := c.GetString("user_role")

	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"valid": true,
		"user": gin.H{
			"id":    userID,
			"email": userEmail,
			"role":  userRole,
		},
	})
}
