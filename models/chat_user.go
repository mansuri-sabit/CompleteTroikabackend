package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ChatUser represents an end-user who interacts with a project-specific widget.
type ChatUser struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"       json:"id"`
	UserID    string             `bson:"user_id,omitempty"   json:"user_id"` // Optional external ID
	ProjectID string             `bson:"project_id"          json:"project_id"`

	// Basic profile
	Name   string `bson:"name,omitempty"    json:"name"`
	Email  string `bson:"email,omitempty"   json:"email"`
	
	Password string `bson:"password,omitempty" json:"-"` 
	Avatar string `bson:"avatar,omitempty"  json:"avatar"`

	// Analytics
	TotalSessions  int       `bson:"total_sessions"   json:"total_sessions"`
	TotalMessages  int       `bson:"total_messages"   json:"total_messages"`
	TotalTokens    int64     `bson:"total_tokens"     json:"total_tokens"`
	LastSeenAt     time.Time `bson:"last_seen_at"     json:"last_seen_at"`
	CreatedAt      time.Time `bson:"created_at"       json:"created_at"`
	UpdatedAt      time.Time `bson:"updated_at"       json:"updated_at"`
	IsActive  bool   `bson:"is_active" json:"is_active"`   
	IsBlocked      bool      `bson:"is_blocked"       json:"is_blocked"`
	BlockingReason string    `bson:"blocking_reason,omitempty" json:"blocking_reason,omitempty"`
}

// Helper Methods

// IsValidUser checks if the chat user has valid required fields
func (cu *ChatUser) IsValidUser() bool {
    return cu.ProjectID != "" && cu.Email != ""
}

// CanChat checks if the user can participate in chat
func (cu *ChatUser) CanChat() bool {
    return cu.IsActive && !cu.IsBlocked
}

// IncrementSession increments the user's session count
func (cu *ChatUser) IncrementSession() {
    cu.TotalSessions++
    cu.LastSeenAt = time.Now()
    cu.UpdatedAt = time.Now()
}

// IncrementMessage increments the user's message count
func (cu *ChatUser) IncrementMessage(tokensUsed int64) {
    cu.TotalMessages++
    cu.TotalTokens += tokensUsed
    cu.LastSeenAt = time.Now()
    cu.UpdatedAt = time.Now()
}

// Block blocks the user from chatting
func (cu *ChatUser) Block() {
    cu.IsBlocked = true
    cu.UpdatedAt = time.Now()
}

// Unblock unblocks the user
func (cu *ChatUser) Unblock() {
    cu.IsBlocked = false
    cu.UpdatedAt = time.Now()
}

// Deactivate deactivates the user account
func (cu *ChatUser) Deactivate() {
    cu.IsActive = false
    cu.UpdatedAt = time.Now()
}

// Activate activates the user account
func (cu *ChatUser) Activate() {
    cu.IsActive = true
    cu.UpdatedAt = time.Now()
}