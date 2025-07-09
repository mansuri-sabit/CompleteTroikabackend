package utils

import (
    "context"
    "log"
    "os"
    "time"

    "golang.org/x/crypto/bcrypt"
    "go.mongodb.org/mongo-driver/bson"
    "go.mongodb.org/mongo-driver/bson/primitive"
    
    "jevi-chat/config"
    "jevi-chat/models"
)

// CreateDefaultAdmin creates the default admin user if it doesn't exist
func CreateDefaultAdmin() error {
    collection := config.GetCollection("users")
    
    adminEmail := os.Getenv("ADMIN_EMAIL")
    adminPassword := os.Getenv("ADMIN_PASSWORD")
    adminName := os.Getenv("ADMIN_NAME")
    
    if adminEmail == "" || adminPassword == "" {
        log.Println("⚠️ Admin credentials not set in environment variables")
        return nil
    }
    
    // Check if admin already exists
    var existingAdmin models.User
    err := collection.FindOne(context.Background(), bson.M{
        "email": adminEmail,
        "role":  "admin",
    }).Decode(&existingAdmin)
    
    if err == nil {
        log.Printf("✅ Admin user already exists: %s", adminEmail)
        return nil
    }
    
    // Hash password
    hashedPassword, err := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost)
    if err != nil {
        return err
    }
    
    // Create admin user
    admin := models.User{
        ID:            primitive.NewObjectID(),
        Name:          adminName,
        Email:         adminEmail,
        Password:      string(hashedPassword),
        Role:          "admin",
        IsActive:      true,
        EmailVerified: true,
        NotificationPrefs: models.DefaultUserNotificationPrefs,
        CreatedAt:     time.Now(),
        UpdatedAt:     time.Now(),
    }
    
    _, err = collection.InsertOne(context.Background(), admin)
    if err != nil {
        return err
    }
    
    log.Printf("✅ Default admin user created: %s", adminEmail)
    return nil
}
