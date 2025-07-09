package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

// User represents a user in the system with authentication and role management
type User struct {
	ID       primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name     string             `bson:"name" json:"name"`
	Email    string             `bson:"email" json:"email"`
	Password string             `bson:"password" json:"-"` // Hidden from JSON
	Role     string             `bson:"role" json:"role"`  // admin, user
	IsActive bool               `bson:"is_active" json:"is_active"`

	// Profile Information
	Company string `bson:"company,omitempty" json:"company"`
	Phone   string `bson:"phone,omitempty" json:"phone"`
	Avatar  string `bson:"avatar,omitempty" json:"avatar"`

	// Authentication & Security
	EmailVerified       bool      `bson:"email_verified" json:"email_verified"`
	EmailVerifyToken    string    `bson:"email_verify_token,omitempty" json:"-"`
	PasswordResetToken  string    `bson:"password_reset_token,omitempty" json:"-"`
	PasswordResetExpiry time.Time `bson:"password_reset_expiry,omitempty" json:"-"`

	// Activity Tracking
	LastLoginAt   time.Time `bson:"last_login_at,omitempty" json:"last_login_at"`
	LastLoginIP   string    `bson:"last_login_ip,omitempty" json:"last_login_ip"`
	LoginAttempts int       `bson:"login_attempts" json:"login_attempts"`
	LockedUntil   time.Time `bson:"locked_until,omitempty" json:"locked_until"`

	// Preferences
	Timezone          string                `bson:"timezone,omitempty" json:"timezone"`
	Language          string                `bson:"language,omitempty" json:"language"`
	NotificationPrefs UserNotificationPrefs `bson:"notification_prefs" json:"notification_prefs"`

	// Metadata
	CreatedAt time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time `bson:"updated_at" json:"updated_at"`
	CreatedBy string    `bson:"created_by,omitempty" json:"created_by"`
}

// UserNotificationPrefs represents user notification preferences
type UserNotificationPrefs struct {
	EmailNotifications bool `bson:"email_notifications" json:"email_notifications"`
	SMSNotifications   bool `bson:"sms_notifications" json:"sms_notifications"`
	PushNotifications  bool `bson:"push_notifications" json:"push_notifications"`
	MarketingEmails    bool `bson:"marketing_emails" json:"marketing_emails"`
	SecurityAlerts     bool `bson:"security_alerts" json:"security_alerts"`
}

// UserCreateRequest represents the request structure for creating a new user
type UserCreateRequest struct {
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	Role     string `json:"role"`
	Company  string `json:"company"`
	Phone    string `json:"phone"`
}

// UserUpdateRequest represents the request structure for updating user information
type UserUpdateRequest struct {
	Name              string                `json:"name"`
	Company           string                `json:"company"`
	Phone             string                `json:"phone"`
	Timezone          string                `json:"timezone"`
	Language          string                `json:"language"`
	NotificationPrefs UserNotificationPrefs `json:"notification_prefs"`
}

// UserLoginRequest represents the login request structure
type UserLoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// UserLoginResponse represents the login response structure
type UserLoginResponse struct {
	Token     string    `json:"token"`
	User      User      `json:"user"`
	ExpiresAt time.Time `json:"expires_at"`
}

// PasswordChangeRequest represents the password change request structure
type PasswordChangeRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=8"`
}

// PasswordResetRequest represents the password reset request structure
type PasswordResetRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// PasswordResetConfirmRequest represents the password reset confirmation structure
type PasswordResetConfirmRequest struct {
	Token       string `json:"token" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

// User role constants
const (
	UserRoleAdmin = "admin"
	UserRoleUser  = "user"
)

// User status constants
const (
	UserStatusActive   = "active"
	UserStatusInactive = "inactive"
	UserStatusLocked   = "locked"
	UserStatusDeleted  = "deleted"
)

// Authentication constants
const (
	MaxLoginAttempts = 5
	LockoutDuration  = 30 * time.Minute
	TokenExpiry      = 24 * time.Hour
)

// Default notification preferences
var DefaultUserNotificationPrefs = UserNotificationPrefs{
	EmailNotifications: true,
	SMSNotifications:   false,
	PushNotifications:  true,
	MarketingEmails:    false,
	SecurityAlerts:     true,
}

// Helper Methods

// IsValid checks if the user has valid required fields
func (u *User) IsValid() bool {
	return u.Name != "" && u.Email != "" && u.Password != ""
}

// IsAdmin checks if the user has admin role
func (u *User) IsAdmin() bool {
	return u.Role == UserRoleAdmin
}

// IsLocked checks if the user account is currently locked
func (u *User) IsLocked() bool {
	return !u.LockedUntil.IsZero() && time.Now().Before(u.LockedUntil)
}

// CanLogin checks if the user can login (active, not locked, email verified)
func (u *User) CanLogin() bool {
	return u.IsActive && !u.IsLocked() && u.EmailVerified
}

// IncrementLoginAttempts increments failed login attempts and locks if necessary
func (u *User) IncrementLoginAttempts() {
	u.LoginAttempts++
	if u.LoginAttempts >= MaxLoginAttempts {
		u.LockedUntil = time.Now().Add(LockoutDuration)
	}
	u.UpdatedAt = time.Now()
}

// ResetLoginAttempts resets failed login attempts after successful login
func (u *User) ResetLoginAttempts() {
	u.LoginAttempts = 0
	u.LockedUntil = time.Time{}
	u.LastLoginAt = time.Now()
	u.UpdatedAt = time.Now()
}

// SetLastLogin updates the last login information
func (u *User) SetLastLogin(ip string) {
	u.LastLoginAt = time.Now()
	u.LastLoginIP = ip
	u.UpdatedAt = time.Now()
}

// GeneratePasswordResetToken generates a password reset token
func (u *User) GeneratePasswordResetToken() string {
	// In a real implementation, generate a secure random token
	token := generateSecureToken(32)
	u.PasswordResetToken = token
	u.PasswordResetExpiry = time.Now().Add(1 * time.Hour) // 1 hour expiry
	u.UpdatedAt = time.Now()
	return token
}

// IsPasswordResetTokenValid checks if the password reset token is valid
func (u *User) IsPasswordResetTokenValid(token string) bool {
	return u.PasswordResetToken == token &&
		!u.PasswordResetExpiry.IsZero() &&
		time.Now().Before(u.PasswordResetExpiry)
}

// ClearPasswordResetToken clears the password reset token after use
func (u *User) ClearPasswordResetToken() {
	u.PasswordResetToken = ""
	u.PasswordResetExpiry = time.Time{}
	u.UpdatedAt = time.Now()
}

// ToSafeUser returns a user object safe for API responses (no sensitive data)
func (u *User) ToSafeUser() User {
	safeUser := *u
	safeUser.Password = ""
	safeUser.PasswordResetToken = ""
	safeUser.EmailVerifyToken = ""
	return safeUser
}

// Helper function to generate secure tokens (simplified implementation)
func generateSecureToken(length int) string {
	// This is a simplified implementation
	// In production, use crypto/rand for secure token generation
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(result)
}
