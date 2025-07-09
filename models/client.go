package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

// Client represents a client entity in the system
// Used for associating projects with clients and managing client information
type Client struct {
	ID       primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ClientID string             `bson:"client_id" json:"client_id"`       // Unique client identifier
	Email    string             `bson:"email" json:"email"`               // Client email address
	Name     string             `bson:"name" json:"name"`                 // Client full name
	Company  string             `bson:"company" json:"company"`           // Client company name
	Phone    string             `bson:"phone,omitempty" json:"phone"`     // Optional phone number
	Address  string             `bson:"address,omitempty" json:"address"` // Optional address

	// Subscription & Project Management
	TotalProjects  int      `bson:"total_projects" json:"total_projects"`   // Number of projects
	ActiveProjects int      `bson:"active_projects" json:"active_projects"` // Active projects count
	ProjectIDs     []string `bson:"project_ids" json:"project_ids"`         // Associated project IDs

	// Client Status & Preferences
	Status            string            `bson:"status" json:"status"`                         // active, suspended, inactive
	Timezone          string            `bson:"timezone,omitempty" json:"timezone"`           // Client timezone
	Language          string            `bson:"language,omitempty" json:"language"`           // Preferred language
	NotificationPrefs NotificationPrefs `bson:"notification_prefs" json:"notification_prefs"` // Notification preferences

	// Billing & Usage
	TotalTokensUsed int64     `bson:"total_tokens_used" json:"total_tokens_used"` // Cumulative token usage
	TotalCost       float64   `bson:"total_cost" json:"total_cost"`               // Total cost incurred
	LastBillingDate time.Time `bson:"last_billing_date,omitempty" json:"last_billing_date"`

	// Metadata
	CreatedAt   time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time `bson:"updated_at" json:"updated_at"`
	LastLoginAt time.Time `bson:"last_login_at,omitempty" json:"last_login_at"`
	IsActive    bool      `bson:"is_active" json:"is_active"`

	// Additional Information
	Notes string   `bson:"notes,omitempty" json:"notes"` // Admin notes
	Tags  []string `bson:"tags,omitempty" json:"tags"`   // Client tags for organization
}

// NotificationPrefs represents client notification preferences
type NotificationPrefs struct {
	EmailNotifications bool `bson:"email_notifications" json:"email_notifications"`
	SMSNotifications   bool `bson:"sms_notifications" json:"sms_notifications"`
	ExpiryReminders    bool `bson:"expiry_reminders" json:"expiry_reminders"`
	UsageAlerts        bool `bson:"usage_alerts" json:"usage_alerts"`
	MaintenanceUpdates bool `bson:"maintenance_updates" json:"maintenance_updates"`
}

// ClientSummary represents a simplified client view for listings
type ClientSummary struct {
	ClientID       string    `json:"client_id"`
	Name           string    `json:"name"`
	Email          string    `json:"email"`
	Company        string    `json:"company"`
	TotalProjects  int       `json:"total_projects"`
	ActiveProjects int       `json:"active_projects"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}

// ClientStats represents client usage statistics
type ClientStats struct {
	ClientID            string    `json:"client_id"`
	TotalProjects       int       `json:"total_projects"`
	ActiveProjects      int       `json:"active_projects"`
	TotalTokensUsed     int64     `json:"total_tokens_used"`
	TotalCost           float64   `json:"total_cost"`
	AverageTokensPerDay int64     `json:"average_tokens_per_day"`
	LastActivityDate    time.Time `json:"last_activity_date"`
}

// ClientCreateRequest represents the request structure for creating a new client
type ClientCreateRequest struct {
	Name              string            `json:"name" binding:"required"`
	Email             string            `json:"email" binding:"required,email"`
	Company           string            `json:"company"`
	Phone             string            `json:"phone"`
	Address           string            `json:"address"`
	Timezone          string            `json:"timezone"`
	Language          string            `json:"language"`
	NotificationPrefs NotificationPrefs `json:"notification_prefs"`
	Notes             string            `json:"notes"`
	Tags              []string          `json:"tags"`
}

// ClientUpdateRequest represents the request structure for updating client information
type ClientUpdateRequest struct {
	Name              string            `json:"name"`
	Company           string            `json:"company"`
	Phone             string            `json:"phone"`
	Address           string            `json:"address"`
	Timezone          string            `json:"timezone"`
	Language          string            `json:"language"`
	NotificationPrefs NotificationPrefs `json:"notification_prefs"`
	Status            string            `json:"status"`
	Notes             string            `json:"notes"`
	Tags              []string          `json:"tags"`
}

// Client status constants
const (
	ClientStatusActive    = "active"
	ClientStatusSuspended = "suspended"
	ClientStatusInactive  = "inactive"
	ClientStatusDeleted   = "deleted"
)

// Default notification preferences
var DefaultNotificationPrefs = NotificationPrefs{
	EmailNotifications: true,
	SMSNotifications:   false,
	ExpiryReminders:    true,
	UsageAlerts:        true,
	MaintenanceUpdates: true,
}

// Helper Methods

// IsValid checks if the client has valid required fields
func (c *Client) IsValid() bool {
	return c.Name != "" && c.Email != "" && c.ClientID != ""
}

// GetDisplayName returns the client's display name (name or email if name is empty)
func (c *Client) GetDisplayName() string {
	if c.Name != "" {
		return c.Name
	}
	return c.Email
}

// IsActiveStatus checks if the client status is active
func (c *Client) IsActiveStatus() bool {
	return c.Status == ClientStatusActive && c.IsActive
}

// AddProject adds a project ID to the client's project list
func (c *Client) AddProject(projectID string) {
	// Check if project ID already exists
	for _, id := range c.ProjectIDs {
		if id == projectID {
			return // Already exists
		}
	}

	c.ProjectIDs = append(c.ProjectIDs, projectID)
	c.TotalProjects = len(c.ProjectIDs)
	c.UpdatedAt = time.Now()
}

// RemoveProject removes a project ID from the client's project list
func (c *Client) RemoveProject(projectID string) {
	for i, id := range c.ProjectIDs {
		if id == projectID {
			c.ProjectIDs = append(c.ProjectIDs[:i], c.ProjectIDs[i+1:]...)
			break
		}
	}

	c.TotalProjects = len(c.ProjectIDs)
	c.UpdatedAt = time.Now()
}

// UpdateTokenUsage updates the client's total token usage
func (c *Client) UpdateTokenUsage(tokensUsed int64, cost float64) {
	c.TotalTokensUsed += tokensUsed
	c.TotalCost += cost
	c.UpdatedAt = time.Now()
}

// ToSummary converts the client to a summary view
func (c *Client) ToSummary() ClientSummary {
	return ClientSummary{
		ClientID:       c.ClientID,
		Name:           c.Name,
		Email:          c.Email,
		Company:        c.Company,
		TotalProjects:  c.TotalProjects,
		ActiveProjects: c.ActiveProjects,
		Status:         c.Status,
		CreatedAt:      c.CreatedAt,
	}
}

// ToStats converts the client to a stats view
func (c *Client) ToStats() ClientStats {
	// Calculate average tokens per day (simplified calculation)
	daysSinceCreation := time.Since(c.CreatedAt).Hours() / 24
	var avgTokensPerDay int64
	if daysSinceCreation > 0 {
		avgTokensPerDay = int64(float64(c.TotalTokensUsed) / daysSinceCreation)
	}

	return ClientStats{
		ClientID:            c.ClientID,
		TotalProjects:       c.TotalProjects,
		ActiveProjects:      c.ActiveProjects,
		TotalTokensUsed:     c.TotalTokensUsed,
		TotalCost:           c.TotalCost,
		AverageTokensPerDay: avgTokensPerDay,
		LastActivityDate:    c.LastLoginAt,
	}
}
