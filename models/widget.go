package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

// WidgetConfig represents the configuration settings for the embeddable chatbot widget
type WidgetConfig struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ProjectID string             `bson:"project_id" json:"project_id"` // Associated project ID

	// Appearance & Styling
	Theme          string `bson:"theme" json:"theme"`                     // default, dark, light, custom
	PrimaryColor   string `bson:"primary_color" json:"primary_color"`     // Hex color code
	SecondaryColor string `bson:"secondary_color" json:"secondary_color"` // Hex color code
	AccentColor    string `bson:"accent_color" json:"accent_color"`       // Hex color code
	FontFamily     string `bson:"font_family" json:"font_family"`         // Font family name
	FontSize       string `bson:"font_size" json:"font_size"`             // small, medium, large
	BorderRadius   string `bson:"border_radius" json:"border_radius"`     // Border radius in px

	// Widget Positioning & Behavior
	Position        string `bson:"position" json:"position"`                   // bottom-right, bottom-left, top-right, top-left
	OffsetX         int    `bson:"offset_x" json:"offset_x"`                   // Horizontal offset in pixels
	OffsetY         int    `bson:"offset_y" json:"offset_y"`                   // Vertical offset in pixels
	Width           int    `bson:"width" json:"width"`                         // Widget width in pixels
	Height          int    `bson:"height" json:"height"`                       // Widget height in pixels
	MinimizeOnStart bool   `bson:"minimize_on_start" json:"minimize_on_start"` // Start minimized

	// Messages & Content
	WelcomeMessage  string `bson:"welcome_message" json:"welcome_message"`   // Initial greeting
	PlaceholderText string `bson:"placeholder_text" json:"placeholder_text"` // Input placeholder
	HeaderTitle     string `bson:"header_title" json:"header_title"`         // Widget header title
	HeaderSubtitle  string `bson:"header_subtitle" json:"header_subtitle"`   // Widget header subtitle

	// Branding
	Logo         string `bson:"logo" json:"logo"`                   // Logo URL or base64
	CompanyName  string `bson:"company_name" json:"company_name"`   // Company name
	ShowBranding bool   `bson:"show_branding" json:"show_branding"` // Show "Powered by" branding
	CustomCSS    string `bson:"custom_css" json:"custom_css"`       // Custom CSS overrides

	// Functionality
	EnableFileUpload bool `bson:"enable_file_upload" json:"enable_file_upload"` // Allow file uploads
	EnableRating     bool `bson:"enable_rating" json:"enable_rating"`           // Enable message rating
	EnableTyping     bool `bson:"enable_typing" json:"enable_typing"`           // Show typing indicators
	EnableSound      bool `bson:"enable_sound" json:"enable_sound"`             // Enable notification sounds
	AutoExpand       bool `bson:"auto_expand" json:"auto_expand"`               // Auto-expand on page load

	// Quick Actions
	QuickActions []QuickAction `bson:"quick_actions" json:"quick_actions"` // Predefined quick action buttons

	// Security & Privacy
	AllowedDomains  []string `bson:"allowed_domains" json:"allowed_domains"`     // Domains where widget can be embedded
	CollectUserInfo bool     `bson:"collect_user_info" json:"collect_user_info"` // Collect user name/email
	RequireAuth     bool     `bson:"require_auth" json:"require_auth"`           // Require user authentication

	// Analytics & Tracking
	EnableAnalytics bool `bson:"enable_analytics" json:"enable_analytics"` // Enable usage analytics
	TrackUserAgent  bool `bson:"track_user_agent" json:"track_user_agent"` // Track browser info
	TrackLocation   bool `bson:"track_location" json:"track_location"`     // Track user location

	// Rate Limiting (Widget Level)
	MessagesPerHour int `bson:"messages_per_hour" json:"messages_per_hour"` // Messages per hour limit
	MessagesPerDay  int `bson:"messages_per_day" json:"messages_per_day"`   // Messages per day limit

	// Metadata
	CreatedAt time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time `bson:"updated_at" json:"updated_at"`
	IsActive  bool      `bson:"is_active" json:"is_active"`
	Version   string    `bson:"version" json:"version"` // Widget version
}

// QuickAction represents a predefined quick action button
type QuickAction struct {
	ID       string `bson:"id" json:"id"`               // Unique action ID
	Label    string `bson:"label" json:"label"`         // Button text
	Message  string `bson:"message" json:"message"`     // Message to send when clicked
	Icon     string `bson:"icon" json:"icon"`           // Icon name or URL
	Color    string `bson:"color" json:"color"`         // Button color
	Order    int    `bson:"order" json:"order"`         // Display order
	IsActive bool   `bson:"is_active" json:"is_active"` // Whether action is enabled
}

// WidgetSession represents an active widget session
type WidgetSession struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	SessionID string             `bson:"session_id" json:"session_id"`     // Unique session identifier
	ProjectID string             `bson:"project_id" json:"project_id"`     // Associated project
	UserID    string             `bson:"user_id,omitempty" json:"user_id"` // Optional user identifier

	// Session Information
	IPAddress string `bson:"ip_address" json:"ip_address"` // Client IP address
	UserAgent string `bson:"user_agent" json:"user_agent"` // Browser user agent
	Referrer  string `bson:"referrer" json:"referrer"`     // Page referrer
	Domain    string `bson:"domain" json:"domain"`         // Website domain

	// User Information (if collected)
	UserName  string `bson:"user_name,omitempty" json:"user_name"`
	UserEmail string `bson:"user_email,omitempty" json:"user_email"`

	// Session Statistics
	MessageCount int   `bson:"message_count" json:"message_count"` // Messages in this session
	TokensUsed   int64 `bson:"tokens_used" json:"tokens_used"`     // Tokens consumed
	Duration     int64 `bson:"duration" json:"duration"`           // Session duration in seconds

	// Timestamps
	StartedAt    time.Time `bson:"started_at" json:"started_at"`
	LastActivity time.Time `bson:"last_activity" json:"last_activity"`
	EndedAt      time.Time `bson:"ended_at,omitempty" json:"ended_at"`

	// Status
	IsActive  bool   `bson:"is_active" json:"is_active"`
	EndReason string `bson:"end_reason,omitempty" json:"end_reason"` // timeout, user_closed, error
}

// WidgetAnalytics represents widget usage analytics
type WidgetAnalytics struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ProjectID string             `bson:"project_id" json:"project_id"`
	Date      time.Time          `bson:"date" json:"date"` // Analytics date (daily aggregation)

	// Usage Metrics
	TotalSessions   int     `bson:"total_sessions" json:"total_sessions"`
	UniqueSessions  int     `bson:"unique_sessions" json:"unique_sessions"`
	TotalMessages   int     `bson:"total_messages" json:"total_messages"`
	AverageMessages float64 `bson:"average_messages" json:"average_messages"`
	TotalTokens     int64   `bson:"total_tokens" json:"total_tokens"`

	// Performance Metrics
	AverageResponse float64 `bson:"average_response_time" json:"average_response_time"` // Response time in seconds
	SuccessRate     float64 `bson:"success_rate" json:"success_rate"`                   // Success rate percentage
	ErrorRate       float64 `bson:"error_rate" json:"error_rate"`                       // Error rate percentage

	// User Engagement
	BounceRate     float64 `bson:"bounce_rate" json:"bounce_rate"`                           // Single message sessions
	AverageSession float64 `bson:"average_session_duration" json:"average_session_duration"` // Duration in minutes
	ReturnUsers    int     `bson:"return_users" json:"return_users"`                         // Returning users count

	// Geographic Data
	TopCountries []CountryStats `bson:"top_countries" json:"top_countries"`
	TopDomains   []DomainStats  `bson:"top_domains" json:"top_domains"`

	// Device & Browser
	DeviceTypes  []DeviceStats  `bson:"device_types" json:"device_types"`
	BrowserTypes []BrowserStats `bson:"browser_types" json:"browser_types"`

	CreatedAt time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time `bson:"updated_at" json:"updated_at"`
}

// CountryStats represents country-wise usage statistics
type CountryStats struct {
	Country string `bson:"country" json:"country"`
	Count   int    `bson:"count" json:"count"`
}

// DomainStats represents domain-wise usage statistics
type DomainStats struct {
	Domain string `bson:"domain" json:"domain"`
	Count  int    `bson:"count" json:"count"`
}

// DeviceStats represents device type statistics
type DeviceStats struct {
	DeviceType string `bson:"device_type" json:"device_type"`
	Count      int    `bson:"count" json:"count"`
}

// BrowserStats represents browser usage statistics
type BrowserStats struct {
	Browser string `bson:"browser" json:"browser"`
	Count   int    `bson:"count" json:"count"`
}
