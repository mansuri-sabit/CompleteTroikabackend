package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"

	"jevi-chat/config"
	"jevi-chat/models"
)

// GetSubscriptionStatus - Get comprehensive subscription status for a project
func GetSubscriptionStatus(c *gin.Context) {
	projectID := c.Param("projectId")

	project, err := getProjectByID(projectID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	// Calculate real-time status
	status := project.Status
	if time.Now().After(project.ExpiryDate) && status != "expired" {
		status = "expired"
		// Auto-update status in database
		updateProjectStatus(projectID, "expired")
	}

	// Calculate usage metrics
	usagePercent := float64(0)
	if project.MonthlyTokenLimit > 0 {
		usagePercent = float64(project.TotalTokensUsed) / float64(project.MonthlyTokenLimit) * 100
	}

	daysUntilExpiry := time.Until(project.ExpiryDate).Hours() / 24
	remainingTokens := project.MonthlyTokenLimit - project.TotalTokensUsed
	if remainingTokens < 0 {
		remainingTokens = 0
	}

	c.JSON(http.StatusOK, gin.H{
		"project_id":          projectID,
		"status":              status,
		"start_date":          project.StartDate,
		"expiry_date":         project.ExpiryDate,
		"total_tokens_used":   project.TotalTokensUsed,
		"monthly_token_limit": project.MonthlyTokenLimit,
		"remaining_tokens":    remainingTokens,
		"usage_percentage":    usagePercent,
		"days_until_expiry":   daysUntilExpiry,
		"is_active":           status == "active" && daysUntilExpiry > 0,
		"needs_renewal":       daysUntilExpiry <= 3,
	})
}

// RenewSubscription - Renew subscription for a project with flexible options
func RenewSubscription(c *gin.Context) {
	projectID := c.Param("projectId")

	var renewData struct {
		Months        int   `json:"months"`
		ResetTokens   bool  `json:"reset_tokens"`
		NewTokenLimit int64 `json:"new_token_limit"`
		ExtendFromNow bool  `json:"extend_from_now"`
	}

	if err := c.ShouldBindJSON(&renewData); err != nil {
		renewData.Months = 1
		renewData.ResetTokens = true
		renewData.ExtendFromNow = false
	}

	// Validate renewal period
	if renewData.Months <= 0 || renewData.Months > 12 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Months must be between 1 and 12"})
		return
	}

	// Get current project
	project, err := getProjectByID(projectID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	collection := config.GetProjectsCollection()

	// Calculate new expiry date
	var newExpiryDate time.Time
	if renewData.ExtendFromNow || time.Now().After(project.ExpiryDate) {
		// Extend from now if explicitly requested or if already expired
		newExpiryDate = time.Now().AddDate(0, renewData.Months, 0)
	} else {
		// Extend from current expiry date
		newExpiryDate = project.ExpiryDate.AddDate(0, renewData.Months, 0)
	}

	updateFields := bson.M{
		"expiry_date":   newExpiryDate,
		"status":        "active",
		"reminder_sent": false,
		"updated_at":    time.Now(),
	}

	// Reset token usage if requested
	if renewData.ResetTokens {
		updateFields["total_tokens_used"] = int64(0)
	}

	// Update token limit if provided
	if renewData.NewTokenLimit > 0 {
		updateFields["monthly_token_limit"] = renewData.NewTokenLimit
	}

	update := bson.M{"$set": updateFields}

	result, err := collection.UpdateOne(context.Background(),
		bson.M{"project_id": projectID}, update)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to renew subscription"})
		return
	}

	if result.ModifiedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	// Log renewal action
	config.LogNotification(project.ID, "renewal",
		fmt.Sprintf("Subscription renewed for %d month(s) for project: %s", renewData.Months, project.Name))

	log.Printf("✅ Subscription renewed: %s for %d month(s)", projectID, renewData.Months)

	c.JSON(http.StatusOK, gin.H{
		"message":      fmt.Sprintf("Subscription renewed for %d month(s)", renewData.Months),
		"new_expiry":   newExpiryDate,
		"status":       "active",
		"tokens_reset": renewData.ResetTokens,
		"new_limit":    renewData.NewTokenLimit,
	})
}

// SuspendSubscription - Suspend a project subscription with reason
func SuspendSubscription(c *gin.Context) {
	projectID := c.Param("projectId")

	var suspendData struct {
		Reason string `json:"reason"`
	}

	c.ShouldBindJSON(&suspendData)

	// Get project for logging
	project, err := getProjectByID(projectID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	err = updateProjectStatus(projectID, "suspended")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to suspend subscription"})
		return
	}

	// Log suspension
	logMessage := fmt.Sprintf("Subscription suspended for project: %s", project.Name)
	if suspendData.Reason != "" {
		logMessage += fmt.Sprintf(" (Reason: %s)", suspendData.Reason)
	}
	config.LogNotification(project.ID, "suspension", logMessage)

	log.Printf("⚠️ Subscription suspended: %s", projectID)

	c.JSON(http.StatusOK, gin.H{
		"message": "Subscription suspended successfully",
		"status":  "suspended",
		"reason":  suspendData.Reason,
	})
}

// ReactivateSubscription - Reactivate a suspended subscription
func ReactivateSubscription(c *gin.Context) {
	projectID := c.Param("projectId")

	project, err := getProjectByID(projectID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	// Check if subscription has expired
	if time.Now().After(project.ExpiryDate) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":        "Cannot reactivate expired subscription. Please renew first.",
			"expiry_date":  project.ExpiryDate,
			"days_expired": time.Since(project.ExpiryDate).Hours() / 24,
		})
		return
	}

	// Check current status
	if project.Status == "active" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Subscription is already active"})
		return
	}

	err = updateProjectStatus(projectID, "active")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reactivate subscription"})
		return
	}

	// Log reactivation
	config.LogNotification(project.ID, "reactivation",
		fmt.Sprintf("Subscription reactivated for project: %s", project.Name))

	log.Printf("✅ Subscription reactivated: %s", projectID)

	c.JSON(http.StatusOK, gin.H{
		"message":     "Subscription reactivated successfully",
		"status":      "active",
		"expiry_date": project.ExpiryDate,
	})
}

// GetSubscriptionUsage - Get detailed token usage and limits for a project
func GetSubscriptionUsage(c *gin.Context) {
	projectID := c.Param("projectId")
	days := c.DefaultQuery("days", "30")

	project, err := getProjectByID(projectID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	// Calculate usage metrics
	usagePercent := float64(0)
	if project.MonthlyTokenLimit > 0 {
		usagePercent = float64(project.TotalTokensUsed) / float64(project.MonthlyTokenLimit) * 100
	}

	remainingTokens := project.MonthlyTokenLimit - project.TotalTokensUsed
	if remainingTokens < 0 {
		remainingTokens = 0
	}

	daysUntilExpiry := time.Until(project.ExpiryDate).Hours() / 24
	estimatedCost := calculateEstimatedCost(project.TotalTokensUsed)

	// Get usage history if requested
	daysInt, _ := strconv.Atoi(days)
	usageHistory := getUsageHistory(projectID, daysInt)

	// Calculate daily average
	daysSinceStart := time.Since(project.StartDate).Hours() / 24
	dailyAverage := int64(0)
	if daysSinceStart > 0 {
		dailyAverage = int64(float64(project.TotalTokensUsed) / daysSinceStart)
	}

	c.JSON(http.StatusOK, gin.H{
		"project_id":        projectID,
		"tokens_used":       project.TotalTokensUsed,
		"token_limit":       project.MonthlyTokenLimit,
		"remaining_tokens":  remainingTokens,
		"usage_percentage":  usagePercent,
		"days_until_expiry": daysUntilExpiry,
		"estimated_cost":    estimatedCost,
		"daily_average":     dailyAverage,
		"status":            project.Status,
		"usage_history":     usageHistory,
		"warnings":          getUsageWarnings(usagePercent, daysUntilExpiry),
	})
}

// GetSubscriptionStats - Get comprehensive subscription statistics
func GetSubscriptionStats(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	collection := config.GetProjectsCollection()

	// Aggregate subscription statistics
	pipeline := []bson.M{
		{
			"$group": bson.M{
				"_id":          "$status",
				"count":        bson.M{"$sum": 1},
				"total_tokens": bson.M{"$sum": "$total_tokens_used"},
				"total_limit":  bson.M{"$sum": "$monthly_token_limit"},
			},
		},
	}

	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get subscription stats"})
		return
	}
	defer cursor.Close(ctx)

	var stats []bson.M
	if err := cursor.All(ctx, &stats); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse subscription stats"})
		return
	}

	// Get expiring soon count (next 7 days)
	expiringSoon, _ := collection.CountDocuments(ctx, bson.M{
		"expiry_date": bson.M{
			"$gte": time.Now(),
			"$lte": time.Now().AddDate(0, 0, 7),
		},
		"status": "active",
	})

	// Get high usage projects (>80%)
	highUsagePipeline := []bson.M{
		{
			"$addFields": bson.M{
				"usage_percentage": bson.M{
					"$multiply": []interface{}{
						bson.M{"$divide": []interface{}{"$total_tokens_used", "$monthly_token_limit"}},
						100,
					},
				},
			},
		},
		{
			"$match": bson.M{
				"usage_percentage": bson.M{"$gte": 80},
				"status":           "active",
			},
		},
		{
			"$count": "high_usage_count",
		},
	}

	highUsageCursor, _ := collection.Aggregate(ctx, highUsagePipeline)
	var highUsageResult []bson.M
	highUsageCursor.All(ctx, &highUsageResult)

	highUsageCount := int64(0)
	if len(highUsageResult) > 0 {
		if count, ok := highUsageResult[0]["high_usage_count"]; ok {
			highUsageCount = count.(int64)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"subscription_stats": stats,
		"expiring_soon":      expiringSoon,
		"high_usage_count":   highUsageCount,
		"timestamp":          time.Now(),
	})
}

// UpdateTokenLimit - Update monthly token limit for a project
func UpdateTokenLimit(c *gin.Context) {
	projectID := c.Param("projectId")

	var limitData struct {
		NewLimit int64 `json:"new_limit" binding:"required"`
	}

	if err := c.ShouldBindJSON(&limitData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid limit data"})
		return
	}

	if limitData.NewLimit <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Token limit must be greater than 0"})
		return
	}

	collection := config.GetProjectsCollection()

	update := bson.M{
		"$set": bson.M{
			"monthly_token_limit": limitData.NewLimit,
			"updated_at":          time.Now(),
		},
	}

	result, err := collection.UpdateOne(context.Background(),
		bson.M{"project_id": projectID}, update)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update token limit"})
		return
	}

	if result.ModifiedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	// Get project for logging
	project, _ := getProjectByID(projectID)
	if project != nil {
		config.LogNotification(project.ID, "limit_update",
			fmt.Sprintf("Token limit updated to %d for project: %s", limitData.NewLimit, project.Name))
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Token limit updated successfully",
		"new_limit": limitData.NewLimit,
	})
}

// ResetTokenUsage - Reset token usage for a project
func ResetTokenUsage(c *gin.Context) {
	projectID := c.Param("projectId")

	collection := config.GetProjectsCollection()

	update := bson.M{
		"$set": bson.M{
			"total_tokens_used": int64(0),
			"updated_at":        time.Now(),
		},
	}

	result, err := collection.UpdateOne(context.Background(),
		bson.M{"project_id": projectID}, update)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reset token usage"})
		return
	}

	if result.ModifiedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	// Get project for logging
	project, _ := getProjectByID(projectID)
	if project != nil {
		config.LogNotification(project.ID, "usage_reset",
			fmt.Sprintf("Token usage reset for project: %s", project.Name))
	}

	log.Printf("✅ Token usage reset: %s", projectID)

	c.JSON(http.StatusOK, gin.H{
		"message":     "Token usage reset successfully",
		"tokens_used": 0,
	})
}

// Helper Functions

// getProjectByID - Get project by project ID
func getProjectByID(projectID string) (*models.Project, error) {
	collection := config.GetProjectsCollection()

	var project models.Project
	err := collection.FindOne(context.Background(),
		bson.M{"project_id": projectID}).Decode(&project)
	if err != nil {
		return nil, err
	}

	return &project, nil
}

// calculateEstimatedCost - Calculate estimated cost based on token usage
func calculateEstimatedCost(tokensUsed int64) float64 {
	// OpenAI GPT-4o pricing (approximate)
	inputCostPerToken := 0.000002  // $0.000002 per input token
	outputCostPerToken := 0.000008 // $0.000008 per output token

	// Assume 60% input, 40% output tokens
	inputTokens := float64(tokensUsed) * 0.6
	outputTokens := float64(tokensUsed) * 0.4

	totalCostUSD := (inputTokens * inputCostPerToken) + (outputTokens * outputCostPerToken)
	totalCostINR := totalCostUSD * 83 // Convert to INR

	return totalCostINR
}

// getUsageHistory - Get token usage history for specified days
func getUsageHistory(projectID string, days int) []map[string]interface{} {
	// This would typically query usage logs collection
	// For now, return empty array - implement based on your logging structure
	return []map[string]interface{}{}
}

// getUsageWarnings - Get usage warnings based on percentage and expiry
func getUsageWarnings(usagePercent, daysUntilExpiry float64) []string {
	var warnings []string

	if usagePercent >= 100 {
		warnings = append(warnings, "Monthly token limit exceeded")
	} else if usagePercent >= 90 {
		warnings = append(warnings, "Approaching monthly token limit (90%+)")
	} else if usagePercent >= 80 {
		warnings = append(warnings, "High token usage (80%+)")
	}

	if daysUntilExpiry <= 0 {
		warnings = append(warnings, "Subscription has expired")
	} else if daysUntilExpiry <= 3 {
		warnings = append(warnings, "Subscription expires soon (3 days or less)")
	} else if daysUntilExpiry <= 7 {
		warnings = append(warnings, "Subscription expires within a week")
	}

	return warnings
}
