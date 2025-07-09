package handlers

import (

	"context"

	"fmt"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
	"github.com/sashabaranov/go-openai"
	"jevi-chat/config"
	"jevi-chat/models"
)

// ProjectChatMessage - Enhanced chat handler with OpenAI GPT-4o and subscription validation
// ProjectChatMessage - Handle chat messages with PDF context
func ProjectChatMessage(c *gin.Context) {
    projectID := c.Param("projectId")
    
    var messageData struct {
        Message   string `json:"message" binding:"required"`
        SessionID string `json:"session_id"`
        UserID    string `json:"user_id"`
    }

    if err := c.ShouldBindJSON(&messageData); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid message data"})
        return
    }

    // Get project from database
    collection := config.GetProjectsCollection()
    var project models.Project
    err := collection.FindOne(context.Background(), bson.M{"project_id": projectID}).Decode(&project)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
        return
    }

    // ✅ Generate OpenAI response with PDF context
    response, tokenUsage, err := generateOpenAIResponse(messageData.Message, project.PDFContent, project.OpenAIModel)
    if err != nil {
        log.Printf("❌ OpenAI API error: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "Failed to generate response",
        })
        return
    }

    // Update token usage
    collection.UpdateOne(context.Background(),
        bson.M{"project_id": projectID},
        bson.M{"$inc": bson.M{"total_tokens_used": tokenUsage}},
    )

    // Save chat message to database
    chatMessage := models.ChatMessage{
        ID:        primitive.NewObjectID(),
        ProjectID: projectID,
        SessionID: messageData.SessionID,
        UserID:    messageData.UserID,
        Message:   messageData.Message,
        Response:  response,
        TokensUsed: tokenUsage,
        CreatedAt: time.Now(),
    }

    config.GetCollection("chat_messages").InsertOne(context.Background(), chatMessage)

    c.JSON(http.StatusOK, gin.H{
        "status":      "success",
        "response":    response,
        "tokens_used": tokenUsage,
        "usage": gin.H{
            "total_tokens": project.TotalTokensUsed + int64(tokenUsage),
            "limit":        project.MonthlyTokenLimit,
            "usage_percent": float64(project.TotalTokensUsed+int64(tokenUsage)) / float64(project.MonthlyTokenLimit) * 100,
        },
    })
}

// generateOpenAIResponse - Generate response using OpenAI with PDF context
func generateOpenAIResponse(userMessage, pdfContext, model string) (string, int, error) {
    client := openai.NewClient(os.Getenv("OPENAI_API_KEY"))
    
    // Create system message with PDF context
    systemMessage := fmt.Sprintf(`You are a helpful assistant. Use the following document content to answer user questions accurately:

Document Content:
%s

Instructions:
- Answer questions based on the provided document content
- If the question cannot be answered from the document, say so politely
- Be concise and helpful
- Cite relevant parts of the document when appropriate`, pdfContext)

    req := openai.ChatCompletionRequest{
        Model: model,
        Messages: []openai.ChatCompletionMessage{
            {
                Role:    openai.ChatMessageRoleSystem,
                Content: systemMessage,
            },
            {
                Role:    openai.ChatMessageRoleUser,
                Content: userMessage,
            },
        },
        MaxTokens:   500,
        Temperature: 0.7,
    }

    resp, err := client.CreateChatCompletion(context.Background(), req)
    if err != nil {
        return "", 0, err
    }

    if len(resp.Choices) == 0 {
        return "", 0, fmt.Errorf("no response generated")
    }

    return resp.Choices[0].Message.Content, resp.Usage.TotalTokens, nil
}


// IframeSendMessage - Legacy endpoint for backward compatibility
func IframeSendMessage(c *gin.Context) {
	// Redirect to new project-based endpoint
	ProjectChatMessage(c)
}

// GetChatHistory - Get chat history for a session
func GetChatHistory(c *gin.Context) {
	projectID := c.Param("projectId")
	sessionID := c.Query("session_id")
	limit := 50 // Default limit

	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 100 {
			limit = parsedLimit
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	collection := config.GetChatMessagesCollection()

	filter := bson.M{"project_id": projectID}
	if sessionID != "" {
		filter["session_id"] = sessionID
	}

	cursor, err := collection.Find(ctx, filter,
		options.Find().SetSort(bson.M{"timestamp": -1}).SetLimit(int64(limit)))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get chat history"})
		return
	}
	defer cursor.Close(ctx)

	var messages []bson.M
	if err := cursor.All(ctx, &messages); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse chat history"})
		return
	}

	// Reverse to get chronological order
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	c.JSON(http.StatusOK, gin.H{
		"messages":   messages,
		"count":      len(messages),
		"project_id": projectID,
		"session_id": sessionID,
	})
}

// RateMessage - Rate a chat message (thumbs up/down)
func RateMessage(c *gin.Context) {
	projectID := c.Param("projectId")
	messageID := c.Param("messageId")

	var ratingData struct {
		Rating   string `json:"rating" binding:"required"` // "positive" or "negative"
		Feedback string `json:"feedback"`
	}

	if err := c.ShouldBindJSON(&ratingData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid rating data"})
		return
	}

	if ratingData.Rating != "positive" && ratingData.Rating != "negative" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Rating must be 'positive' or 'negative'"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	collection := config.GetChatMessagesCollection()

	objID, err := primitive.ObjectIDFromHex(messageID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid message ID"})
		return
	}

	update := bson.M{
		"$set": bson.M{
			"rating":   ratingData.Rating,
			"feedback": ratingData.Feedback,
			"rated_at": time.Now(),
		},
	}

	result, err := collection.UpdateOne(ctx,
		bson.M{"_id": objID, "project_id": projectID}, update)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save rating"})
		return
	}

	if result.ModifiedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Message not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Rating saved successfully",
		"rating":  ratingData.Rating,
	})
}

// Helper Functions

// getProjectWithValidation - Get project with comprehensive subscription validation
func getProjectWithValidation(projectID string) (*models.Project, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	collection := config.GetProjectsCollection()

	var project models.Project
	err := collection.FindOne(ctx, bson.M{"project_id": projectID}).Decode(&project)
	if err != nil {
		return nil, fmt.Errorf("Project not found or invalid.")
	}

	// Check if project is active
	if project.Status != "active" {
		switch project.Status {
		case "expired":
			return nil, fmt.Errorf("Your subscription has expired. Please renew to continue.")
		case "suspended":
			return nil, fmt.Errorf("Your account is suspended. Please contact support.")
		case "deleted":
			return nil, fmt.Errorf("This project has been deleted.")
		default:
			return nil, fmt.Errorf("Your account is inactive. Please contact support.")
		}
	}

	// Check expiry date
	if time.Now().After(project.ExpiryDate) {
		// Auto-update status to expired
		updateProjectStatus(projectID, "expired")
		return nil, fmt.Errorf("Your subscription has expired. Please renew to continue.")
	}

	// Check token limit
	if project.TotalTokensUsed >= project.MonthlyTokenLimit {
		return nil, fmt.Errorf("Monthly usage limit reached. Please upgrade your plan.")
	}

	return &project, nil
}



// updateProjectTokenUsage - Update project token usage with notification triggers
func updateProjectTokenUsage(projectID string, tokensUsed int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	collection := config.GetProjectsCollection()

	// Get current project state
	var project models.Project
	err := collection.FindOne(ctx, bson.M{"project_id": projectID}).Decode(&project)
	if err != nil {
		return fmt.Errorf("failed to get project: %v", err)
	}

	// Calculate new usage
	newTotalUsage := project.TotalTokensUsed + int64(tokensUsed)
	usagePercent := float64(newTotalUsage) / float64(project.MonthlyTokenLimit) * 100

	// Update token usage in database
	update := bson.M{
		"$inc": bson.M{"total_tokens_used": int64(tokensUsed)},
		"$set": bson.M{"updated_at": time.Now()},
	}

	_, err = collection.UpdateOne(ctx, bson.M{"project_id": projectID}, update)
	if err != nil {
		return err
	}

	// Trigger notifications asynchronously
	go func() {
		// Update project with new usage for notifications
		project.TotalTokensUsed = newTotalUsage

		// Check if monthly limit reached (100%)
		if usagePercent >= 100 {
			recentlySent, err := config.WasNotificationRecentlySent(project.ID, "monthly_limit", 24)
			if err == nil && !recentlySent {
				message := fmt.Sprintf("Monthly token limit reached for project: %s", project.Name)
				config.LogNotification(project.ID, "monthly_limit", message)
				log.Printf("🚨 Monthly limit notification logged for project: %s", project.Name)
			}
		} else if usagePercent >= 80 {
			// Send warning if approaching limit (80%)
			recentlySent, err := config.WasNotificationRecentlySent(project.ID, "usage_warning", 12)
			if err == nil && !recentlySent {
				message := fmt.Sprintf("Token usage warning (%.1f%%) for project: %s", usagePercent, project.Name)
				config.LogNotification(project.ID, "usage_warning", message)
				log.Printf("⚠️ Usage warning notification logged for project: %s", project.Name)
			}
		}
	}()

	return nil
}

// saveChatMessage - Save chat message to database
func saveChatMessage(projectID, sessionID, userMessage, aiResponse string, tokensUsed int, clientIP, userAgent, userID, userName, userEmail string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	collection := config.GetChatMessagesCollection()

	messageID := primitive.NewObjectID()

	chatMessage := bson.M{
		"_id":          messageID,
		"project_id":   projectID,
		"session_id":   sessionID,
		"user_message": userMessage,
		"ai_response":  aiResponse,
		"tokens_used":  tokensUsed,
		"timestamp":    time.Now(),
		"client_ip":    clientIP,
		"user_agent":   userAgent,
		"user_id":      userID,
		"user_name":    userName,
		"user_email":   userEmail,
		"rating":       "",
		"feedback":     "",
	}

	_, err := collection.InsertOne(ctx, chatMessage)
	if err != nil {
		log.Printf("❌ Failed to save chat message: %v", err)
		return ""
	}

	return messageID.Hex()
}

// updateWidgetSession - Update or create widget session
func updateWidgetSession(projectID, sessionID, clientIP, userAgent string, tokensUsed int) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	collection := config.GetWidgetSessionsCollection()

	filter := bson.M{"session_id": sessionID}
	update := bson.M{
		"$set": bson.M{
			"project_id":    projectID,
			"last_activity": time.Now(),
			"is_active":     true,
		},
		"$inc": bson.M{
			"message_count": 1,
			"tokens_used":   int64(tokensUsed),
		},
		"$setOnInsert": bson.M{
			"session_id": sessionID,
			"ip_address": clientIP,
			"user_agent": userAgent,
			"started_at": time.Now(),
		},
	}

	opts := options.Update().SetUpsert(true)
	_, err := collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		log.Printf("❌ Failed to update widget session: %v", err)
	}
}

// logOpenAIUsage - Log OpenAI API usage for analytics
func logOpenAIUsage(projectID, sessionID, userMessage, aiResponse string, inputTokens, outputTokens int, model string, success bool, errorMessage string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	collection := config.GetOpenAIUsageLogsCollection()

	usageLog := bson.M{
		"project_id":    projectID,
		"session_id":    sessionID,
		"user_message":  userMessage,
		"ai_response":   aiResponse,
		"input_tokens":  inputTokens,
		"output_tokens": outputTokens,
		"total_tokens":  inputTokens + outputTokens,
		"model":         model,
		"success":       success,
		"error_message": errorMessage,
		"timestamp":     time.Now(),
		"cost":          calculateCost(inputTokens, outputTokens),
	}

	_, err := collection.InsertOne(ctx, usageLog)
	if err != nil {
		log.Printf("❌ Failed to log OpenAI usage: %v", err)
	}
}

// calculateCost - Calculate cost based on OpenAI pricing
func calculateCost(inputTokens, outputTokens int) float64 {
	// OpenAI GPT-4o pricing (as of 2024)
	inputCostPerToken := 0.000002  // $0.000002 per input token
	outputCostPerToken := 0.000008 // $0.000008 per output token

	inputCost := float64(inputTokens) * inputCostPerToken
	outputCost := float64(outputTokens) * outputCostPerToken
	totalCostUSD := inputCost + outputCost

	// Convert to INR (approximate rate)
	totalCostINR := totalCostUSD * 83

	return totalCostINR
}

// checkRateLimit - Check rate limiting for additional protection
func checkRateLimit(projectID, clientIP string) bool {
	// Implement rate limiting logic here
	// For now, return true (no rate limiting)
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

// generateSessionID - Generate unique session ID
func generateSessionID() string {
	return fmt.Sprintf("sess_%d_%s", time.Now().Unix(), generateRandomString(8))
}

// getErrorResponse - Get user-friendly error response
func getErrorResponse(err error) string {
	errStr := err.Error()

	if strings.Contains(errStr, "quota") || strings.Contains(errStr, "rate limit") {
		return "I'm experiencing high demand right now. Please try again in a moment."
	}
	if strings.Contains(errStr, "authentication") || strings.Contains(errStr, "unauthorized") {
		return "I'm having authentication issues. Please contact support."
	}
	if strings.Contains(errStr, "timeout") {
		return "I'm taking longer than usual to respond. Please try a shorter question."
	}
	if strings.Contains(errStr, "network") || strings.Contains(errStr, "connection") {
		return "I'm having connectivity issues. Please try again later."
	}

	return "I'm having trouble answering just now. Please try again later."
}

// updateProjectStatus - Update project status
func updateProjectStatus(projectID, status string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	collection := config.GetProjectsCollection()

	update := bson.M{
		"$set": bson.M{
			"status":     status,
			"updated_at": time.Now(),
		},
	}

	_, err := collection.UpdateOne(ctx, bson.M{"project_id": projectID}, update)
	return err
}
