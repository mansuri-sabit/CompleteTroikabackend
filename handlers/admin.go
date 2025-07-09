package handlers

import (
	"context"
	"fmt"
	"strings"
	"net/http"
	"strconv"
	"time"
	"os"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"github.com/ledongthuc/pdf"
	"jevi-chat/config"
	"jevi-chat/models"
	"github.com/sashabaranov/go-openai"
)

// AdminDashboard - Enhanced admin dashboard with comprehensive statistics
// AdminDashboard - Get dashboard statistics with project counts
func AdminDashboard(c *gin.Context) {
    userID := c.GetString("user_id")
    userRole := c.GetString("user_role")
    
    if userID == "" || userRole != "admin" {
        c.JSON(http.StatusUnauthorized, gin.H{
            "error": "Admin authentication required",
        })
        return
    }

    // Get project statistics
    projectStats, err := getProjectStatistics()
    if err != nil {
        log.Printf("❌ Failed to get project statistics: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "Failed to fetch dashboard data",
        })
        return
    }

    // Get recent projects
    recentProjects, err := getRecentProjects(5) // Get last 5 projects
    if err != nil {
        log.Printf("❌ Failed to get recent projects: %v", err)
        recentProjects = []models.Project{} // Empty array as fallback
    }

    // Calculate additional metrics
    totalRevenue := calculateTotalRevenue()
    apiCallsToday := calculateAPICallsToday()
    tokensUsedToday := calculateTokensUsedToday()

    c.JSON(http.StatusOK, gin.H{
        "stats": gin.H{
            "totalProjects":    projectStats.TotalProjects,
            "activeProjects":   projectStats.ActiveProjects,
            "suspendedProjects": projectStats.SuspendedProjects,
            "expiredProjects":  projectStats.ExpiredProjects,
            "monthlyRevenue":   totalRevenue,
            "apiCalls":         apiCallsToday,
            "tokensUsed":       tokensUsedToday,
        },
        "projects": recentProjects,
        "message": "Dashboard data fetched successfully",
    })
}

// ProjectStatistics - Structure for project statistics
type ProjectStatistics struct {
    TotalProjects     int64 `json:"total_projects"`
    ActiveProjects    int64 `json:"active_projects"`
    SuspendedProjects int64 `json:"suspended_projects"`
    ExpiredProjects   int64 `json:"expired_projects"`
}

// getProjectStatistics - Get comprehensive project statistics
func getProjectStatistics() (*ProjectStatistics, error) {
    collection := config.GetProjectsCollection()
    ctx := context.Background()

    // Count total projects (excluding deleted)
    totalProjects, err := collection.CountDocuments(ctx, bson.M{
        "status": bson.M{"$ne": "deleted"},
    })
    if err != nil {
        return nil, err
    }

    // Count active projects
    activeProjects, err := collection.CountDocuments(ctx, bson.M{
        "status": "active",
        "is_active": true,
    })
    if err != nil {
        return nil, err
    }

    // Count suspended projects
    suspendedProjects, err := collection.CountDocuments(ctx, bson.M{
        "status": "suspended",
    })
    if err != nil {
        return nil, err
    }

    // Count expired projects
    expiredProjects, err := collection.CountDocuments(ctx, bson.M{
        "expiry_date": bson.M{"$lt": time.Now()},
        "status": bson.M{"$ne": "deleted"},
    })
    if err != nil {
        return nil, err
    }

    return &ProjectStatistics{
        TotalProjects:     totalProjects,
        ActiveProjects:    activeProjects,
        SuspendedProjects: suspendedProjects,
        ExpiredProjects:   expiredProjects,
    }, nil
}

// getRecentProjects - Get recent projects for dashboard display
func getRecentProjects(limit int) ([]models.Project, error) {
    collection := config.GetProjectsCollection()
    ctx := context.Background()

    // Find recent projects, sorted by creation date
    cursor, err := collection.Find(ctx, bson.M{
        "status": bson.M{"$ne": "deleted"},
    }, &options.FindOptions{
        Sort:  bson.M{"created_at": -1},
        Limit: &[]int64{int64(limit)}[0],
    })
    if err != nil {
        return nil, err
    }
    defer cursor.Close(ctx)

    var projects []models.Project
    if err := cursor.All(ctx, &projects); err != nil {
        return nil, err
    }

    return projects, nil
}

// calculateTotalRevenue - Calculate total monthly revenue
func calculateTotalRevenue() float64 {
    // This is a placeholder - implement based on your pricing model
    // You might calculate based on active subscriptions, token usage, etc.
    collection := config.GetProjectsCollection()
    ctx := context.Background()

    // Example: Count active projects and multiply by subscription price
    activeCount, err := collection.CountDocuments(ctx, bson.M{
        "status": "active",
        "is_active": true,
    })
    if err != nil {
        return 0
    }

    // Assuming ₹500 per project per month
    return float64(activeCount) * 500
}

// calculateAPICallsToday - Calculate API calls for today
func calculateAPICallsToday() int64 {
    collection := config.GetCollection("chat_messages")
    ctx := context.Background()

    // Get today's date range
    today := time.Now()
    startOfDay := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
    endOfDay := startOfDay.Add(24 * time.Hour)

    count, err := collection.CountDocuments(ctx, bson.M{
        "created_at": bson.M{
            "$gte": startOfDay,
            "$lt":  endOfDay,
        },
    })
    if err != nil {
        return 0
    }

    return count
}

// calculateTokensUsedToday - Calculate tokens used today
func calculateTokensUsedToday() int64 {
    collection := config.GetCollection("chat_messages")
    ctx := context.Background()

    // Get today's date range
    today := time.Now()
    startOfDay := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
    endOfDay := startOfDay.Add(24 * time.Hour)

    // Aggregate tokens used today
    pipeline := []bson.M{
        {
            "$match": bson.M{
                "created_at": bson.M{
                    "$gte": startOfDay,
                    "$lt":  endOfDay,
                },
            },
        },
        {
            "$group": bson.M{
                "_id": nil,
                "total_tokens": bson.M{"$sum": "$tokens_used"},
            },
        },
    }

    cursor, err := collection.Aggregate(ctx, pipeline)
    if err != nil {
        return 0
    }
    defer cursor.Close(ctx)

    var result []bson.M
    if err := cursor.All(ctx, &result); err != nil || len(result) == 0 {
        return 0
    }

    if totalTokens, ok := result[0]["total_tokens"].(int64); ok {
        return totalTokens
    }

    return 0
}

// GetProjectsDashboard - Get all projects with enhanced filtering and pagination
func GetProjectsDashboard(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Parse query parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	status := c.Query("status")
	search := c.Query("search")
	sortBy := c.DefaultQuery("sort", "created_at")
	sortOrder := c.DefaultQuery("order", "desc")

	// Build filter
	filter := bson.M{}
	if status != "" {
		filter["status"] = status
	}
	if search != "" {
		filter["$or"] = []bson.M{
			{"name": bson.M{"$regex": search, "$options": "i"}},
			{"description": bson.M{"$regex": search, "$options": "i"}},
			{"project_id": bson.M{"$regex": search, "$options": "i"}},
		}
	}

	// Build sort
	sortDirection := 1
	if sortOrder == "desc" {
		sortDirection = -1
	}
	sort := bson.D{{sortBy, sortDirection}}

	// Calculate pagination
	skip := (page - 1) * limit

	collection := config.GetProjectsCollection()

	// Get total count
	totalCount, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count projects"})
		return
	}

	// Build aggregation pipeline for enhanced project data
	pipeline := []bson.M{
		{"$match": filter},
		{
			"$addFields": bson.M{
				"usage_percentage": bson.M{
					"$cond": bson.M{
						"if": bson.M{"$gt": []interface{}{"$monthly_token_limit", 0}},
						"then": bson.M{
							"$multiply": []interface{}{
								bson.M{"$divide": []interface{}{"$total_tokens_used", "$monthly_token_limit"}},
								100,
							},
						},
						"else": 0,
					},
				},
				"days_until_expiry": bson.M{
					"$divide": []interface{}{
						bson.M{"$subtract": []interface{}{"$expiry_date", "$$NOW"}},
						86400000, // milliseconds in a day
					},
				},
				"estimated_cost": bson.M{
					"$multiply": []interface{}{
						"$total_tokens_used",
						0.000005, // Approximate cost per token in INR
					},
				},
			},
		},
		{"$sort": sort},
		{"$skip": skip},
		{"$limit": limit},
	}

	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get projects"})
		return
	}
	defer cursor.Close(ctx)

	var projects []bson.M
	if err := cursor.All(ctx, &projects); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse projects"})
		return
	}

	// Calculate pagination info
	totalPages := (int(totalCount) + limit - 1) / limit

	c.JSON(http.StatusOK, gin.H{
		"projects": projects,
		"pagination": gin.H{
			"current_page": page,
			"total_pages":  totalPages,
			"total_count":  totalCount,
			"limit":        limit,
			"has_next":     page < totalPages,
			"has_prev":     page > 1,
		},
		"filters": gin.H{
			"status": status,
			"search": search,
			"sort":   sortBy,
			"order":  sortOrder,
		},
	})
}

// GetProjectDetails - Get detailed project information with analytics
func GetProjectDetails(c *gin.Context) {
	projectID := c.Param("id")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get project by project_id or _id
	var project models.Project
	collection := config.GetProjectsCollection()

	// Try to find by project_id first, then by _id
	err := collection.FindOne(ctx, bson.M{"project_id": projectID}).Decode(&project)
	if err != nil {
		// Try by ObjectID
		if objID, parseErr := primitive.ObjectIDFromHex(projectID); parseErr == nil {
			err = collection.FindOne(ctx, bson.M{"_id": objID}).Decode(&project)
		}
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
			return
		}
	}

	// Get project analytics
	analytics := getProjectAnalytics(ctx, project.ProjectID)

	// Get recent chat messages
	recentChats := getRecentChats(ctx, project.ProjectID, 10)

	// Get usage history
	usageHistory := getUsageHistory(project.ProjectID, 30)

	// Calculate additional metrics
	usagePercent := float64(0)
	if project.MonthlyTokenLimit > 0 {
		usagePercent = float64(project.TotalTokensUsed) / float64(project.MonthlyTokenLimit) * 100
	}

	daysUntilExpiry := time.Until(project.ExpiryDate).Hours() / 24
	estimatedCost := float64(project.TotalTokensUsed) * 0.000005 // Approximate cost

	c.JSON(http.StatusOK, gin.H{
		"project":           project,
		"usage_percentage":  usagePercent,
		"days_until_expiry": daysUntilExpiry,
		"estimated_cost":    estimatedCost,
		"analytics":         analytics,
		"recent_chats":      recentChats,
		"usage_history":     usageHistory,
	})
}

// RenewProject - Renew project subscription
func RenewProject(c *gin.Context) {
	projectID := c.Param("id")

	var renewData struct {
		Months      int  `json:"months"`
		ResetTokens bool `json:"reset_tokens"`
	}

	if err := c.ShouldBindJSON(&renewData); err != nil {
		renewData.Months = 1
		renewData.ResetTokens = true
	}

	if renewData.Months <= 0 || renewData.Months > 12 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Months must be between 1 and 12"})
		return
	}

	collection := config.GetProjectsCollection()

	updateFields := bson.M{
		"expiry_date":   time.Now().AddDate(0, renewData.Months, 0),
		"status":        "active",
		"reminder_sent": false,
		"updated_at":    time.Now(),
	}

	if renewData.ResetTokens {
		updateFields["total_tokens_used"] = int64(0)
	}

	update := bson.M{"$set": updateFields}

	result, err := collection.UpdateOne(context.Background(),
		bson.M{"project_id": projectID}, update)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to renew project"})
		return
	}

	if result.ModifiedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	// Log renewal action
	config.LogNotification(primitive.NilObjectID, "renewal",
		fmt.Sprintf("Project %s renewed for %d month(s)", projectID, renewData.Months))

	c.JSON(http.StatusOK, gin.H{
		"message":    fmt.Sprintf("Project renewed for %d month(s)", renewData.Months),
		"new_expiry": time.Now().AddDate(0, renewData.Months, 0),
		"status":     "active",
	})
}

// UpdateProjectStatus - Update project status (active, suspended, expired)
func UpdateProjectStatus(c *gin.Context) {
	projectID := c.Param("id")

	var statusData struct {
		Status string `json:"status" binding:"required"`
		Reason string `json:"reason"`
	}

	if err := c.ShouldBindJSON(&statusData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status data"})
		return
	}

	if !isValidStatus(statusData.Status) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid status. Must be: active, suspended, expired, or deleted",
		})
		return
	}

	collection := config.GetProjectsCollection()

	update := bson.M{
		"$set": bson.M{
			"status":     statusData.Status,
			"updated_at": time.Now(),
		},
	}

	result, err := collection.UpdateOne(context.Background(),
		bson.M{"project_id": projectID}, update)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update status"})
		return
	}

	if result.ModifiedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	// Log status change
	logMessage := fmt.Sprintf("Project %s status changed to %s", projectID, statusData.Status)
	if statusData.Reason != "" {
		logMessage += fmt.Sprintf(" (Reason: %s)", statusData.Reason)
	}

	config.LogNotification(primitive.NilObjectID, "status_change", logMessage)

	c.JSON(http.StatusOK, gin.H{
		"message": "Project status updated successfully",
		"status":  statusData.Status,
	})
}

// GetProjectUsage - Get detailed usage statistics
func GetProjectUsage(c *gin.Context) {
	projectID := c.Param("id")
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get project
	var project models.Project
	collection := config.GetProjectsCollection()
	err := collection.FindOne(ctx, bson.M{"project_id": projectID}).Decode(&project)
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
	daysUntilExpiry := time.Until(project.ExpiryDate).Hours() / 24
	estimatedCost := float64(project.TotalTokensUsed) * 0.000005

	// Get usage history
	usageHistory := getUsageHistory(project.ProjectID, days)

	// Get chat statistics
	chatStats := getChatStatistics(ctx, projectID)

	c.JSON(http.StatusOK, gin.H{
		"project_id":        projectID,
		"tokens_used":       project.TotalTokensUsed,
		"token_limit":       project.MonthlyTokenLimit,
		"remaining_tokens":  remainingTokens,
		"usage_percentage":  usagePercent,
		"days_until_expiry": daysUntilExpiry,
		"estimated_cost":    estimatedCost,
		"usage_history":     usageHistory,
		"chat_statistics":   chatStats,
		"status":            project.Status,
		"last_updated":      project.UpdatedAt,
	})
}

// GetNotificationHistory - Get notification history
func GetNotificationHistory(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	notificationType := c.Query("type")
	projectID := c.Query("project_id")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	collection := config.GetNotificationsCollection()

	// Build filter
	filter := bson.M{}
	if notificationType != "" {
		filter["type"] = notificationType
	}
	if projectID != "" {
		if objID, err := primitive.ObjectIDFromHex(projectID); err == nil {
			filter["project_id"] = objID
		}
	}

	// Calculate pagination
	skip := (page - 1) * limit

	// Get total count
	totalCount, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count notifications"})
		return
	}

	// Get notifications
	cursor, err := collection.Find(ctx, filter,
		options.Find().SetSort(bson.M{"sent_at": -1}).SetSkip(int64(skip)).SetLimit(int64(limit)))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get notifications"})
		return
	}
	defer cursor.Close(ctx)

	var notifications []bson.M
	if err := cursor.All(ctx, &notifications); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse notifications"})
		return
	}

	totalPages := (int(totalCount) + limit - 1) / limit

	c.JSON(http.StatusOK, gin.H{
		"notifications": notifications,
		"pagination": gin.H{
			"current_page": page,
			"total_pages":  totalPages,
			"total_count":  totalCount,
			"limit":        limit,
		},
	})
}

// GetProjectNotifications - Get notifications for specific project
func GetProjectNotifications(c *gin.Context) {
	projectID := c.Param("id")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Convert project_id to ObjectID for notification lookup
	var project models.Project
	collection := config.GetProjectsCollection()
	err := collection.FindOne(ctx, bson.M{"project_id": projectID}).Decode(&project)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	notificationsCol := config.GetNotificationsCollection()

	filter := bson.M{
		"project_id": project.ID,
		"sent_at": bson.M{
			"$gte": time.Now().AddDate(0, 0, -30), // Last 30 days
		},
	}

	cursor, err := notificationsCol.Find(ctx, filter,
		options.Find().SetSort(bson.M{"sent_at": -1}))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get project notifications"})
		return
	}
	defer cursor.Close(ctx)

	var notifications []bson.M
	if err := cursor.All(ctx, &notifications); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse notifications"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"project_id":    projectID,
		"notifications": notifications,
		"count":         len(notifications),
	})
}

// TestNotification - Send test notification
func TestNotification(c *gin.Context) {
	projectID := c.Param("id")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get project
	var project models.Project
	collection := config.GetProjectsCollection()
	err := collection.FindOne(ctx, bson.M{"project_id": projectID}).Decode(&project)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	// Send test notification
	message := fmt.Sprintf("Test notification for project: %s", project.Name)
	err = config.LogNotification(project.ID, "test", message)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send test notification"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Test notification sent successfully",
		"project": project.Name,
	})
}

// GetSystemStats - Get comprehensive system statistics
func GetSystemStats(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stats := getDashboardStats(ctx)
	dbStats := config.GetDatabaseStats()

	c.JSON(http.StatusOK, gin.H{
		"system_stats":   stats,
		"database_stats": dbStats,
		"timestamp":      time.Now(),
	})
}

// Helper Functions

// getDashboardStats - Get comprehensive dashboard statistics
func getDashboardStats(ctx context.Context) map[string]interface{} {
	stats := make(map[string]interface{})

	// Projects statistics
	projectsCol := config.GetProjectsCollection()

	totalProjects, _ := projectsCol.CountDocuments(ctx, bson.M{})
	activeProjects, _ := projectsCol.CountDocuments(ctx, bson.M{"status": "active"})
	expiredProjects, _ := projectsCol.CountDocuments(ctx, bson.M{"status": "expired"})
	suspendedProjects, _ := projectsCol.CountDocuments(ctx, bson.M{"status": "suspended"})

	// Token usage statistics
	pipeline := []bson.M{
		{
			"$group": bson.M{
				"_id":          nil,
				"total_tokens": bson.M{"$sum": "$total_tokens_used"},
				"avg_tokens":   bson.M{"$avg": "$total_tokens_used"},
				"total_limit":  bson.M{"$sum": "$monthly_token_limit"},
			},
		},
	}

	cursor, err := projectsCol.Aggregate(ctx, pipeline)
	var tokenStats bson.M
	if err == nil {
		defer cursor.Close(ctx)
		if cursor.Next(ctx) {
			cursor.Decode(&tokenStats)
		}
	}

	// Recent activity
	recentProjects, _ := projectsCol.CountDocuments(ctx, bson.M{
		"created_at": bson.M{"$gte": time.Now().AddDate(0, 0, -7)},
	})

	stats["projects"] = map[string]interface{}{
		"total":     totalProjects,
		"active":    activeProjects,
		"expired":   expiredProjects,
		"suspended": suspendedProjects,
		"recent":    recentProjects,
	}

	if tokenStats != nil {
		stats["tokens"] = tokenStats
	}

	return stats
}

// getRecentActivity - Get recent system activity
func getRecentActivity(ctx context.Context) []map[string]interface{} {
	var activities []map[string]interface{}

	// Get recent projects
	projectsCol := config.GetProjectsCollection()
	cursor, err := projectsCol.Find(ctx, bson.M{},
		options.Find().SetSort(bson.M{"created_at": -1}).SetLimit(5))
	if err == nil {
		defer cursor.Close(ctx)
		for cursor.Next(ctx) {
			var project bson.M
			if cursor.Decode(&project) == nil {
				activities = append(activities, map[string]interface{}{
					"type":        "project_created",
					"description": fmt.Sprintf("Project '%s' was created", project["name"]),
					"timestamp":   project["created_at"],
				})
			}
		}
	}

	return activities
}

// getSystemHealth - Get system health status
func getSystemHealth() map[string]interface{} {
	health := make(map[string]interface{})

	// Database health
	if err := config.HealthCheck(); err != nil {
		health["database"] = "unhealthy"
		health["database_error"] = err.Error()
	} else {
		health["database"] = "healthy"
	}

	// System status
	health["status"] = "operational"
	health["uptime"] = time.Now().Format(time.RFC3339)

	return health
}

// getProjectAnalytics - Get project analytics
func getProjectAnalytics(ctx context.Context, projectID string) map[string]interface{} {
	analytics := make(map[string]interface{})

	// Get chat message count
	chatCol := config.GetChatMessagesCollection()
	messageCount, _ := chatCol.CountDocuments(ctx, bson.M{"project_id": projectID})

	// Get recent message count (last 7 days)
	recentCount, _ := chatCol.CountDocuments(ctx, bson.M{
		"project_id": projectID,
		"timestamp":  bson.M{"$gte": time.Now().AddDate(0, 0, -7)},
	})

	analytics["total_messages"] = messageCount
	analytics["recent_messages"] = recentCount

	return analytics
}

// getRecentChats - Get recent chat messages
func getRecentChats(ctx context.Context, projectID string, limit int) []bson.M {
	var chats []bson.M

	chatCol := config.GetChatMessagesCollection()
	cursor, err := chatCol.Find(ctx, bson.M{"project_id": projectID},
		options.Find().SetSort(bson.M{"timestamp": -1}).SetLimit(int64(limit)))
	if err == nil {
		defer cursor.Close(ctx)
		cursor.All(ctx, &chats)
	}

	return chats
}

// getChatStatistics - Get chat statistics
func getChatStatistics(ctx context.Context, projectID string) map[string]interface{} {
	stats := make(map[string]interface{})

	chatCol := config.GetChatMessagesCollection()

	// Total messages
	totalMessages, _ := chatCol.CountDocuments(ctx, bson.M{"project_id": projectID})

	// Messages today
	today := time.Now().Truncate(24 * time.Hour)
	todayMessages, _ := chatCol.CountDocuments(ctx, bson.M{
		"project_id": projectID,
		"timestamp":  bson.M{"$gte": today},
	})

	// Messages this week
	weekStart := today.AddDate(0, 0, -int(today.Weekday()))
	weekMessages, _ := chatCol.CountDocuments(ctx, bson.M{
		"project_id": projectID,
		"timestamp":  bson.M{"$gte": weekStart},
	})

	stats["total_messages"] = totalMessages
	stats["today_messages"] = todayMessages
	stats["week_messages"] = weekMessages

	return stats
}

// isValidStatus - Validate project status
func isValidStatus(status string) bool {
	validStatuses := []string{"active", "suspended", "expired", "deleted"}
	for _, validStatus := range validStatuses {
		if status == validStatus {
			return true
		}
	}
	return false
}

// extractPDFContent - Extract text content from PDF file
func extractPDFContent(filePath string) (string, error) {
    file, err := os.Open(filePath)
    if err != nil {
        return "", err
    }
    defer file.Close()

    // Get file info
    info, err := file.Stat()
    if err != nil {
        return "", err
    }

    // Read PDF content using pdf library
    reader, err := pdf.NewReader(file, info.Size())
    if err != nil {
        return "", err
    }

    var content strings.Builder
    
    // Extract text from each page
    for i := 1; i <= reader.NumPage(); i++ {
        page := reader.Page(i)
        if page.V.IsNull() {
            continue
        }
        
        // ✅ Fix: GetPlainText requires font map parameter
        text, err := page.GetPlainText(nil)
        if err != nil {
            log.Printf("⚠️ Failed to extract text from page %d: %v", i, err)
            continue
        }
        
        content.WriteString(text)
        content.WriteString("\n")
    }

    return content.String(), nil
}

func generateOpenAIEmbeddings(content string) ([]float64, error) {
    apiKey := os.Getenv("OPENAI_API_KEY")
    if apiKey == "" {
        return nil, fmt.Errorf("OpenAI API key not configured")
    }

    client := openai.NewClient(apiKey)
    
    // Truncate content if too long (OpenAI has token limits)
    if len(content) > 8000 {
        content = content[:8000]
    }
    
    // Create embedding request
    req := openai.EmbeddingRequest{
        Input: []string{content},
        Model: openai.AdaEmbeddingV2,
    }
    
    resp, err := client.CreateEmbeddings(context.Background(), req)
    if err != nil {
        return nil, fmt.Errorf("failed to create embeddings: %v", err)
    }
    
    if len(resp.Data) == 0 {
        return nil, fmt.Errorf("no embeddings generated")
    }

    // Convert []float32 to []float64
    embedding32 := resp.Data[0].Embedding
    embedding64 := make([]float64, len(embedding32))
    for i, v := range embedding32 {
        embedding64[i] = float64(v)
    }
    return embedding64, nil
}
