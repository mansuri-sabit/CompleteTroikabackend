package middleware

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
   
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"

	"jevi-chat/config"
	"jevi-chat/models"
)

// SubscriptionValidator - Middleware to validate project subscription status
func SubscriptionValidator() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract project ID from URL parameters
		projectID := c.Param("projectId")
		if projectID == "" {
			// Try alternative parameter names
			projectID = c.Param("id")
		}

		// Skip validation for non-project routes
		if projectID == "" {
			c.Next()
			return
		}

		// Skip validation for admin routes
		if isAdminRoute(c.Request.URL.Path) {
			c.Next()
			return
		}

		log.Printf("🔍 Validating subscription for project: %s", projectID)

		// Get project with subscription validation
		project, validationError := validateProjectSubscription(projectID)
		if validationError != nil {
			log.Printf("❌ Subscription validation failed for %s: %s", projectID, validationError.Error())

			c.JSON(http.StatusForbidden, gin.H{
				"error":      validationError.Error(),
				"status":     "subscription_blocked",
				"project_id": projectID,
				"timestamp":  time.Now(),
			})
			c.Abort()
			return
		}

		// Add project to context for use in handlers
		c.Set("project", project)
		c.Set("project_id", projectID)

		log.Printf("✅ Subscription validation passed for project: %s", projectID)
		c.Next()
	}
}

// TokenLimitValidator - Middleware to check token limits before processing
func TokenLimitValidator() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Only apply to chat endpoints
		if !isChatEndpoint(c.Request.URL.Path) {
			c.Next()
			return
		}

		projectInterface, exists := c.Get("project")
		if !exists {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Project not found in context"})
			c.Abort()
			return
		}

		project, ok := projectInterface.(*models.Project)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid project data in context"})
			c.Abort()
			return
		}

		// Check if project has reached token limit
		if project.TotalTokensUsed >= project.MonthlyTokenLimit {
			usagePercent := float64(project.TotalTokensUsed) / float64(project.MonthlyTokenLimit) * 100

			log.Printf("🚫 Token limit exceeded for project %s: %d/%d tokens (%.1f%%)",
				project.ProjectID, project.TotalTokensUsed, project.MonthlyTokenLimit, usagePercent)

			c.JSON(http.StatusOK, gin.H{
				"response": "Monthly usage limit reached. Please upgrade your plan or contact support.",
				"status":   "limit_exceeded",
				"usage": gin.H{
					"tokens_used":   project.TotalTokensUsed,
					"token_limit":   project.MonthlyTokenLimit,
					"usage_percent": usagePercent,
				},
			})
			c.Abort()
			return
		}

		// Check if approaching limit (90% threshold)
		usagePercent := float64(project.TotalTokensUsed) / float64(project.MonthlyTokenLimit) * 100
		if usagePercent >= 90 {
			log.Printf("⚠️ High token usage for project %s: %.1f%%", project.ProjectID, usagePercent)

			// Add warning header but continue processing
			c.Header("X-Usage-Warning", fmt.Sprintf("High usage: %.1f%% of monthly limit", usagePercent))
		}

		c.Next()
	}
}

// RateLimitValidator - Middleware for additional rate limiting protection
func RateLimitValidator() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Only apply to chat endpoints
		if !isChatEndpoint(c.Request.URL.Path) {
			c.Next()
			return
		}

		projectInterface, exists := c.Get("project")
		if !exists {
			c.Next()
			return
		}

		project, ok := projectInterface.(*models.Project)
		if !ok {
			c.Next()
			return
		}

		// Check rate limits based on project status
		if !checkProjectRateLimit(project, getClientIP(c)) {
			log.Printf("🚫 Rate limit exceeded for project %s from IP %s", project.ProjectID, getClientIP(c))

			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "Rate limit exceeded. Please wait before sending another message.",
				"status":      "rate_limited",
				"retry_after": 60, // seconds
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// ProjectAccessValidator - Middleware to validate project access permissions
func ProjectAccessValidator() gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("projectId")
		if projectID == "" {
			projectID = c.Param("id")
		}

		if projectID == "" {
			c.Next()
			return
		}

		// Check if project exists and is accessible
		project, err := getProjectForValidation(projectID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error":      "Project not found or access denied",
				"project_id": projectID,
			})
			c.Abort()
			return
		}

		// Check if project is soft deleted
		if project.Status == "deleted" || !project.IsActive {
			c.JSON(http.StatusGone, gin.H{
				"error":      "This project has been deleted or is no longer available",
				"project_id": projectID,
			})
			c.Abort()
			return
		}

		// Add project to context
		c.Set("project", project)
		c.Set("project_id", projectID)

		c.Next()
	}
}

// SubscriptionMaintenanceValidator - Middleware to handle automatic maintenance
func SubscriptionMaintenanceValidator() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Run maintenance checks asynchronously
		go func() {
			if err := performSubscriptionMaintenance(); err != nil {
				log.Printf("⚠️ Subscription maintenance error: %v", err)
			}
		}()

		c.Next()
	}
}

// Helper Functions

// validateProjectSubscription - Comprehensive project subscription validation
func validateProjectSubscription(projectID string) (*models.Project, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	collection := config.GetProjectsCollection()

	var project models.Project
	err := collection.FindOne(ctx, bson.M{"project_id": projectID}).Decode(&project)
	if err != nil {
		return nil, fmt.Errorf("Project not found or invalid")
	}

	// Check if project is active
	if project.Status != "active" {
		switch project.Status {
		case "expired":
			return nil, fmt.Errorf("Your subscription has expired. Please renew to continue")
		case "suspended":
			return nil, fmt.Errorf("Your account is suspended. Please contact support")
		case "deleted":
			return nil, fmt.Errorf("This project has been deleted")
		default:
			return nil, fmt.Errorf("Your account is inactive. Please contact support")
		}
	}

	// Check expiry date
	if time.Now().After(project.ExpiryDate) {
		// Auto-update status to expired
		go updateProjectStatusAsync(projectID, "expired")
		return nil, fmt.Errorf("Your subscription has expired. Please renew to continue")
	}

	// Check if project is soft deleted
	if !project.IsActive {
		return nil, fmt.Errorf("This project is no longer available")
	}

	return &project, nil
}

// getProjectForValidation - Get project for basic validation
func getProjectForValidation(projectID string) (*models.Project, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	collection := config.GetProjectsCollection()

	var project models.Project
	err := collection.FindOne(ctx, bson.M{"project_id": projectID}).Decode(&project)
	if err != nil {
		return nil, err
	}

	return &project, nil
}

// updateProjectStatusAsync - Asynchronously update project status
func updateProjectStatusAsync(projectID, status string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	collection := config.GetProjectsCollection()

	update := bson.M{
		"$set": bson.M{
			"status":     status,
			"updated_at": time.Now(),
		},
	}

	_, err := collection.UpdateOne(ctx, bson.M{"project_id": projectID}, update)
	if err != nil {
		log.Printf("❌ Failed to update project status: %v", err)
	} else {
		log.Printf("✅ Project status updated to %s: %s", status, projectID)
	}
}



func checkProjectRateLimit(project *models.Project, clientIP string) bool {
    // Define identifier using project ID and client IP
    identifier := fmt.Sprintf("%s:%s", project.ProjectID, clientIP)
    
    // Example rate limits by project status:
    switch project.Status {
    case "active":
        return !checkRateLimit(identifier, 60, time.Minute) // 60 requests per minute
    case "suspended":
        return false // No access for suspended projects
    default:
        return !checkRateLimit(identifier, 30, time.Minute) // 30 requests per minute
    }
}


// performSubscriptionMaintenance - Perform automatic subscription maintenance
func performSubscriptionMaintenance() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	collection := config.GetProjectsCollection()

	// Find and update expired projects
	filter := bson.M{
		"expiry_date": bson.M{"$lt": time.Now()},
		"status":      bson.M{"$ne": "expired"},
	}

	update := bson.M{
		"$set": bson.M{
			"status":     "expired",
			"updated_at": time.Now(),
		},
	}

	result, err := collection.UpdateMany(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update expired projects: %v", err)
	}

	if result.ModifiedCount > 0 {
		log.Printf("🔄 Marked %d projects as expired during maintenance", result.ModifiedCount)
	}

	return nil
}

// isAdminRoute - Check if the route is an admin route
func isAdminRoute(path string) bool {
	adminPrefixes := []string{
		"/api/admin",
		"/admin",
		"/dashboard",
	}

	for _, prefix := range adminPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}

	return false
}

// isChatEndpoint - Check if the route is a chat endpoint
func isChatEndpoint(path string) bool {
	chatPaths := []string{
		"/api/chat",
		"/api/projects/",
		"/chat",
		"/message",
	}

	for _, chatPath := range chatPaths {
		if strings.Contains(path, chatPath) {
			return true
		}
	}

	return false
}

// SubscriptionMetrics - Middleware to collect subscription metrics
func SubscriptionMetrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()

		c.Next()

		// Collect metrics after request processing
		duration := time.Since(startTime)

		projectInterface, exists := c.Get("project")
		if exists {
			if project, ok := projectInterface.(*models.Project); ok {
				go recordSubscriptionMetrics(project, c.Request.Method, c.Request.URL.Path, duration, c.Writer.Status())
			}
		}
	}
}

// recordSubscriptionMetrics - Record subscription-related metrics
func recordSubscriptionMetrics(project *models.Project, method, path string, duration time.Duration, statusCode int) {
	// This would typically send metrics to your monitoring system
	// For now, just log the metrics
	log.Printf("📊 Metrics - Project: %s, Method: %s, Path: %s, Duration: %v, Status: %d, Usage: %.1f%%",
		project.ProjectID,
		method,
		path,
		duration,
		statusCode,
		float64(project.TotalTokensUsed)/float64(project.MonthlyTokenLimit)*100,
	)
}

// SubscriptionLogger - Middleware to log subscription-related activities
func SubscriptionLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("projectId")
		if projectID == "" {
			projectID = c.Param("id")
		}

		if projectID != "" {
			log.Printf("🔍 Subscription activity - Project: %s, Method: %s, Path: %s, IP: %s",
				projectID,
				c.Request.Method,
				c.Request.URL.Path,
				getClientIP(c),
			)
		}

		c.Next()
	}
}

// SubscriptionHeaders - Middleware to add subscription-related headers
func SubscriptionHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		projectInterface, exists := c.Get("project")
		if exists {
			if project, ok := projectInterface.(*models.Project); ok {
				// Add subscription info to response headers
				c.Header("X-Subscription-Status", project.Status)
				c.Header("X-Token-Usage", fmt.Sprintf("%d/%d", project.TotalTokensUsed, project.MonthlyTokenLimit))
				c.Header("X-Days-Until-Expiry", fmt.Sprintf("%.1f", time.Until(project.ExpiryDate).Hours()/24))

				usagePercent := float64(project.TotalTokensUsed) / float64(project.MonthlyTokenLimit) * 100
				c.Header("X-Usage-Percentage", fmt.Sprintf("%.1f", usagePercent))
			}
		}

		c.Next()
	}
}
