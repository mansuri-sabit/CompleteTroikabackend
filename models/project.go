package models

import (
	"fmt"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"os"
	"time"
)

// Project represents a chatbot project in the system with comprehensive subscription management
type Project struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ProjectID   string             `bson:"project_id" json:"project_id"`
	Name        string             `bson:"name" json:"name"`
	Description string             `bson:"description" json:"description"`
	Category    string             `bson:"category" json:"category"`

	// Client Association
	ClientID string `bson:"client_id,omitempty" json:"client_id"`
	

	// Subscription Management
	StartDate         time.Time `bson:"start_date" json:"start_date"`
	ExpiryDate        time.Time `bson:"expiry_date" json:"expiry_date"`
	Status            string    `bson:"status" json:"status"`
	TotalTokensUsed   int64     `bson:"total_tokens_used" json:"total_tokens_used"`
	MonthlyTokenLimit int64     `bson:"monthly_token_limit" json:"monthly_token_limit"`

	// Widget & Embedding Configuration
	EmbedCode      string              `bson:"embed_code" json:"embed_code"`
	WidgetSettings ProjectWidgetConfig `bson:"widget_settings" json:"widget_settings"` // Renamed to avoid conflict

	// AI Provider Configuration
	AIProvider   string `bson:"ai_provider" json:"ai_provider"`
	OpenAIModel  string `bson:"openai_model" json:"openai_model"`
	OpenAIAPIKey string `bson:"openai_api_key,omitempty" json:"openai_api_key,omitempty"`

	// Document Management
	PDFFiles     []PDFFile `bson:"pdf_files" json:"pdf_files"`
	PDFContent   string    `bson:"pdf_content" json:"pdf_content"`
	DocumentPath string    `bson:"document_path" json:"document_path"`
	

	// Cost Tracking
	EstimatedCostToday float64 `bson:"estimated_cost_today" json:"estimated_cost_today"`
	EstimatedCostMonth float64 `bson:"estimated_cost_month" json:"estimated_cost_month"`
	TotalCost          float64 `bson:"total_cost" json:"total_cost"`

	// Notification Management
	ReminderSent     bool      `bson:"reminder_sent" json:"reminder_sent"`
	LastReminderDate time.Time `bson:"last_reminder_date" json:"last_reminder_date"`

	// Metadata
	CreatedAt time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time `bson:"updated_at" json:"updated_at"`
	IsActive              bool      `bson:"is_active" json:"is_active"` // Renamed field to avoid conflict
}

// ProjectWidgetConfig represents the embeddable widget configuration (renamed to avoid conflict)
// Add this to your models/project.go
type ProjectWidgetConfig struct {
    Theme            string `json:"theme" bson:"theme"`
    PrimaryColor     string `json:"primary_color" bson:"primary_color"`
    WelcomeMessage   string `json:"welcome_message" bson:"welcome_message"`
    Position         string `json:"position" bson:"position"`
    ShowBranding     bool   `json:"show_branding" bson:"show_branding"`
    EnableFileUpload bool   `json:"enable_file_upload" bson:"enable_file_upload"`
    EnableRating     bool   `json:"enable_rating" bson:"enable_rating"`
    Placeholder      string `json:"placeholder" bson:"placeholder"`
    Height           string `json:"height" bson:"height"`
    Width            string `json:"width" bson:"width"`
    EnableSound      bool   `json:"enable_sound" bson:"enable_sound"`
    AutoOpen         bool   `json:"auto_open" bson:"auto_open"`
    TriggerDelay     int    `json:"trigger_delay" bson:"trigger_delay"`
}


// PDFFile represents an uploaded PDF file
type PDFFile struct {
    ID           string    `bson:"id" json:"id"`
    FileName     string    `bson:"file_name" json:"file_name"`
    FilePath     string    `bson:"file_path" json:"file_path"`
    FileSize     int64     `bson:"file_size" json:"file_size"`
    ContentType  string    `bson:"content_type" json:"content_type"`
    Content      string    `bson:"content" json:"content"`
    Embeddings   []float64 `bson:"embeddings" json:"embeddings"`
    UploadedAt   time.Time `bson:"uploaded_at" json:"uploaded_at"`
    ProcessedAt  time.Time `bson:"processed_at" json:"processed_at"`
    Status       string    `bson:"status" json:"status"`
}

// Project status constants
const (
	ProjectStatusActive    = "active"
	ProjectStatusSuspended = "suspended"
	ProjectStatusExpired   = "expired"
	ProjectStatusDeleted   = "deleted"
)

// AI Provider constants
const (
	AIProviderOpenAI = "openai"
	AIProviderGemini = "gemini"
)

// PDF processing status constants
const (
	PDFStatusUploaded   = "uploaded"
	PDFStatusProcessing = "processing"
	PDFStatusProcessed  = "processed"
	PDFStatusError      = "error"
)

// Helper Methods

// IsValid checks if the project has valid required fields
func (p *Project) IsValid() bool {
	return p.Name != "" && p.ProjectID != "" && p.MonthlyTokenLimit > 0
}

// IsActive checks if the project is currently active (method renamed to avoid conflict)
func (p *Project) IsProjectActive() bool {
	return p.Status == ProjectStatusActive && p.IsActive && time.Now().Before(p.ExpiryDate)
}

// IsExpired checks if the project subscription has expired
func (p *Project) IsExpired() bool {
	return time.Now().After(p.ExpiryDate) || p.Status == ProjectStatusExpired
}

// GetUsagePercentage calculates the current token usage percentage
func (p *Project) GetUsagePercentage() float64 {
	if p.MonthlyTokenLimit == 0 {
		return 0
	}
	return float64(p.TotalTokensUsed) / float64(p.MonthlyTokenLimit) * 100
}

// GetRemainingTokens returns the number of tokens remaining in the monthly limit
func (p *Project) GetRemainingTokens() int64 {
	remaining := p.MonthlyTokenLimit - p.TotalTokensUsed
	if remaining < 0 {
		return 0
	}
	return remaining
}

// GetDaysUntilExpiry returns the number of days until the subscription expires
func (p *Project) GetDaysUntilExpiry() float64 {
	return time.Until(p.ExpiryDate).Hours() / 24
}

// CanUseTokens checks if the project can use the specified number of tokens
func (p *Project) CanUseTokens(tokensNeeded int64) bool {
	return p.IsProjectActive() && (p.TotalTokensUsed+tokensNeeded) <= p.MonthlyTokenLimit
}

// AddTokenUsage adds token usage to the project
func (p *Project) AddTokenUsage(tokensUsed int64) {
	p.TotalTokensUsed += tokensUsed
	p.UpdatedAt = time.Now()
}

// ResetTokenUsage resets the monthly token usage (for renewals)
func (p *Project) ResetTokenUsage() {
	p.TotalTokensUsed = 0
	p.UpdatedAt = time.Now()
}

// ExtendSubscription extends the subscription by the specified number of months
func (p *Project) ExtendSubscription(months int) {
	if p.IsExpired() {
		p.ExpiryDate = time.Now().AddDate(0, months, 0)
	} else {
		p.ExpiryDate = p.ExpiryDate.AddDate(0, months, 0)
	}
	p.Status = ProjectStatusActive
	p.ReminderSent = false
	p.UpdatedAt = time.Now()
}

// Suspend suspends the project
func (p *Project) Suspend() {
	p.Status = ProjectStatusSuspended
	p.UpdatedAt = time.Now()
}

// Reactivate reactivates a suspended project (if not expired)
func (p *Project) Reactivate() error {
	if p.IsExpired() {
		return fmt.Errorf("cannot reactivate expired project")
	}
	p.Status = ProjectStatusActive
	p.UpdatedAt = time.Now()
	return nil
}

// MarkAsExpired marks the project as expired
func (p *Project) MarkAsExpired() {
	p.Status = ProjectStatusExpired
	p.UpdatedAt = time.Now()
}

// SoftDelete performs a soft delete of the project
func (p *Project) SoftDelete() {
	p.Status = ProjectStatusDeleted
	p.IsActive = false
	p.UpdatedAt = time.Now()
}

// GetAIModel returns the appropriate AI model based on provider
func (p *Project) GetAIModel() string {
	switch p.AIProvider {
	case AIProviderOpenAI:
		if p.OpenAIModel != "" {
			return p.OpenAIModel
		}
		return "gpt-4o"
	case AIProviderGemini:
		return "gemini-1.5-flash"
	default:
		return "gpt-4o"
	}
}

// GetAPIKey returns the appropriate API key based on provider
func (p *Project) GetAPIKey() string {
	switch p.AIProvider {
	case AIProviderOpenAI:
		if p.OpenAIAPIKey != "" {
			return p.OpenAIAPIKey
		}
		return os.Getenv("OPENAI_API_KEY")
	case AIProviderGemini:
		return os.Getenv("GEMINI_API_KEY")
	default:
		return os.Getenv("OPENAI_API_KEY")
	}
}

// HasPDFContent checks if the project has PDF content available
func (p *Project) HasPDFContent() bool {
	return len(p.PDFContent) > 0 || len(p.PDFFiles) > 0
}

// GetProcessedPDFCount returns the number of successfully processed PDF files
func (p *Project) GetProcessedPDFCount() int {
	count := 0
	for _, pdf := range p.PDFFiles {
		if pdf.Status == PDFStatusProcessed {
			count++
		}
	}
	return count
}

// NeedsReminderNotification checks if a reminder notification should be sent
func (p *Project) NeedsReminderNotification() bool {
	if p.ReminderSent {
		return false
	}

	reminderDate := p.ExpiryDate.AddDate(0, 0, -3)
	return time.Now().After(reminderDate) && time.Now().Before(p.ExpiryDate)
}
