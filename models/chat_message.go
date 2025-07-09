package models

import (
    "time"
    "go.mongodb.org/mongo-driver/bson/primitive"
)

// ChatMessage represents a chat message between user and AI
type ChatMessage struct {
    ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
    ProjectID string             `bson:"project_id" json:"project_id"`
    SessionID string             `bson:"session_id" json:"session_id"`
    UserID    string             `bson:"user_id,omitempty" json:"user_id"`
    UserName  string             `bson:"user_name,omitempty" json:"user_name"`
    
    // Message content
    Message   string `bson:"message" json:"message"`
    Response  string `bson:"response" json:"response"`
    
    // AI processing details
    TokensUsed    int    `bson:"tokens_used" json:"tokens_used"`
    Model         string `bson:"model,omitempty" json:"model"`
    ProcessingTime int64 `bson:"processing_time,omitempty" json:"processing_time"` // milliseconds
    
    // User feedback
    Rating    string `bson:"rating,omitempty" json:"rating"` // positive, negative, neutral
    Feedback  string `bson:"feedback,omitempty" json:"feedback"`
    
    // Metadata
    IPAddress string    `bson:"ip_address,omitempty" json:"ip_address"`
    UserAgent string    `bson:"user_agent,omitempty" json:"user_agent"`
    CreatedAt time.Time `bson:"created_at" json:"created_at"`
    UpdatedAt time.Time `bson:"updated_at" json:"updated_at"`
}

// ChatSession represents a chat session
type ChatSession struct {
    ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
    SessionID string             `bson:"session_id" json:"session_id"`
    ProjectID string             `bson:"project_id" json:"project_id"`
    UserID    string             `bson:"user_id,omitempty" json:"user_id"`
    
    // Session details
    StartTime     time.Time `bson:"start_time" json:"start_time"`
    EndTime       time.Time `bson:"end_time,omitempty" json:"end_time"`
    MessageCount  int       `bson:"message_count" json:"message_count"`
    TotalTokens   int       `bson:"total_tokens" json:"total_tokens"`
    
    // Session metadata
    IPAddress string    `bson:"ip_address,omitempty" json:"ip_address"`
    UserAgent string    `bson:"user_agent,omitempty" json:"user_agent"`
    CreatedAt time.Time `bson:"created_at" json:"created_at"`
    UpdatedAt time.Time `bson:"updated_at" json:"updated_at"`
}

// DefaultChatMessage creates a new chat message with default values
func DefaultChatMessage(projectID, sessionID, userID, message string) *ChatMessage {
    return &ChatMessage{
        ID:        primitive.NewObjectID(),
        ProjectID: projectID,
        SessionID: sessionID,
        UserID:    userID,
        Message:   message,
        CreatedAt: time.Now(),
        UpdatedAt: time.Now(),
    }
}

// SetResponse sets the AI response and related metadata
func (cm *ChatMessage) SetResponse(response string, tokensUsed int, model string, processingTime int64) {
    cm.Response = response
    cm.TokensUsed = tokensUsed
    cm.Model = model
    cm.ProcessingTime = processingTime
    cm.UpdatedAt = time.Now()
}

// SetRating sets user feedback rating
func (cm *ChatMessage) SetRating(rating, feedback string) {
    cm.Rating = rating
    cm.Feedback = feedback
    cm.UpdatedAt = time.Now()
}
