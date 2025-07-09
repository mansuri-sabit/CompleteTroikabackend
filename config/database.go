package config

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	DB     *mongo.Database
	Client *mongo.Client
)

// InitMongoDB - Enhanced MongoDB initialization with connection pooling and retry logic
func InitMongoDB() {
	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		log.Fatal("❌ MONGODB_URI not set in environment")
	}

	// Log connection attempt (hide password for security)
	safeURI := hideSensitiveInfo(uri)
	log.Printf("🔗 Connecting to MongoDB: %s", safeURI)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Enhanced client options for production
	clientOptions := options.Client().ApplyURI(uri)
	clientOptions.SetMaxPoolSize(10)
	clientOptions.SetMinPoolSize(1)
	clientOptions.SetMaxConnIdleTime(30 * time.Second)
	clientOptions.SetServerSelectionTimeout(10 * time.Second)

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Fatalf("❌ Failed to connect to MongoDB: %v", err)
	}

	// Test connection with retry logic
	if err := testConnection(ctx, client); err != nil {
		log.Fatalf("❌ Failed to establish MongoDB connection: %v", err)
	}

	// Get database name from environment or use default
	dbName := os.Getenv("MONGODB_DATABASE")
	if dbName == "" {
		dbName = "troika_tech"
		log.Printf("⚠️ MONGODB_DATABASE not set, using default: %s", dbName)
	}

	Client = client
	DB = client.Database(dbName)

	log.Printf("✅ Connected to MongoDB successfully (Database: %s)", dbName)

	// Verify collections and setup indexes
	if err := verifyCollections(ctx); err != nil {
		log.Printf("⚠️ Warning during collection verification: %v", err)
	}

	// Initialize subscription defaults for existing projects
	go func() {
		time.Sleep(2 * time.Second) // Wait for connection to stabilize
		if err := InitializeSubscriptionDefaults(); err != nil {
			log.Printf("⚠️ Warning during subscription initialization: %v", err)
		}
	}()
}

// testConnection - Test MongoDB connection with retry logic
func testConnection(ctx context.Context, client *mongo.Client) error {
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		if err := client.Ping(ctx, nil); err != nil {
			if i == maxRetries-1 {
				return fmt.Errorf("ping failed after %d attempts: %v", maxRetries, err)
			}
			log.Printf("⚠️ Ping attempt %d failed, retrying...", i+1)
			time.Sleep(time.Duration(i+1) * time.Second)
			continue
		}
		return nil
	}
	return nil
}

// hideSensitiveInfo - Hide sensitive information in connection strings
func hideSensitiveInfo(uri string) string {
	if strings.Contains(uri, "@") {
		parts := strings.Split(uri, "@")
		if len(parts) >= 2 {
			credPart := parts[0]
			if strings.Contains(credPart, ":") {
				credParts := strings.Split(credPart, ":")
				if len(credParts) >= 3 {
					return fmt.Sprintf("%s:%s:***@%s", credParts[0], credParts[1], parts[1])
				}
			}
		}
	}
	return uri
}

// verifyCollections - Verify required collections exist
func verifyCollections(ctx context.Context) error {
	requiredCollections := []string{
		"projects",
		"clients",
		"chat_messages",
		"chat_users",
		"widget_sessions",
		"widget_analytics",
		"openai_usage_logs",
		"notifications",
	}

	// List existing collections
	collections, err := DB.ListCollectionNames(ctx, bson.M{})
	if err != nil {
		return fmt.Errorf("failed to list collections: %v", err)
	}

	log.Printf("📊 Available collections: %v", collections)

	// Check for required collections
	existingMap := make(map[string]bool)
	for _, col := range collections {
		existingMap[col] = true
	}

	for _, required := range requiredCollections {
		if !existingMap[required] {
			log.Printf("⚠️ Collection '%s' does not exist, it will be created on first use", required)
		} else {
			log.Printf("✅ Collection '%s' found", required)
		}
	}

	// Setup indexes for better performance
	return setupIndexes(ctx)
}

// setupIndexes - Setup database indexes for optimal performance
func setupIndexes(ctx context.Context) error {
	// Projects collection indexes
	projectsCol := DB.Collection("projects")
	_, err := projectsCol.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{"project_id", 1}},
			Options: options.Index().SetBackground(true).SetUnique(true),
		},
		{
			Keys:    bson.D{{"name", 1}},
			Options: options.Index().SetBackground(true),
		},
		{
			Keys:    bson.D{{"status", 1}},
			Options: options.Index().SetBackground(true),
		},
		{
			Keys:    bson.D{{"expiry_date", 1}},
			Options: options.Index().SetBackground(true),
		},
		{
			Keys:    bson.D{{"total_tokens_used", 1}},
			Options: options.Index().SetBackground(true),
		},
		{
			Keys:    bson.D{{"status", 1}, {"expiry_date", 1}},
			Options: options.Index().SetBackground(true),
		},
		{
			Keys:    bson.D{{"client_id", 1}},
			Options: options.Index().SetBackground(true),
		},
		{
			Keys:    bson.D{{"created_at", -1}},
			Options: options.Index().SetBackground(true),
		},
	})
	if err != nil {
		log.Printf("⚠️ Failed to create projects indexes: %v", err)
	}

	// Clients collection indexes
	clientsCol := DB.Collection("clients")
	_, err = clientsCol.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{"client_id", 1}},
			Options: options.Index().SetBackground(true).SetUnique(true),
		},
		{
			Keys:    bson.D{{"email", 1}},
			Options: options.Index().SetBackground(true).SetUnique(true),
		},
		{
			Keys:    bson.D{{"status", 1}},
			Options: options.Index().SetBackground(true),
		},
		{
			Keys:    bson.D{{"created_at", -1}},
			Options: options.Index().SetBackground(true),
		},
	})
	if err != nil {
		log.Printf("⚠️ Failed to create clients indexes: %v", err)
	}

	// Chat messages collection indexes
	chatCol := DB.Collection("chat_messages")
	_, err = chatCol.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{"project_id", 1}, {"session_id", 1}},
			Options: options.Index().SetBackground(true),
		},
		{
			Keys:    bson.D{{"timestamp", -1}},
			Options: options.Index().SetBackground(true),
		},
		{
			Keys:    bson.D{{"project_id", 1}, {"timestamp", -1}},
			Options: options.Index().SetBackground(true),
		},
	})
	if err != nil {
		log.Printf("⚠️ Failed to create chat_messages indexes: %v", err)
	}

	// Widget sessions collection indexes
	widgetCol := DB.Collection("widget_sessions")
	_, err = widgetCol.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{"session_id", 1}},
			Options: options.Index().SetBackground(true).SetUnique(true),
		},
		{
			Keys:    bson.D{{"project_id", 1}, {"started_at", -1}},
			Options: options.Index().SetBackground(true),
		},
		{
			Keys:    bson.D{{"is_active", 1}},
			Options: options.Index().SetBackground(true),
		},
	})
	if err != nil {
		log.Printf("⚠️ Failed to create widget_sessions indexes: %v", err)
	}

	// OpenAI usage logs collection indexes
	usageCol := DB.Collection("openai_usage_logs")
	_, err = usageCol.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{"project_id", 1}, {"timestamp", -1}},
			Options: options.Index().SetBackground(true),
		},
		{
			Keys:    bson.D{{"timestamp", -1}},
			Options: options.Index().SetBackground(true),
		},
		{
			Keys:    bson.D{{"project_id", 1}, {"success", 1}},
			Options: options.Index().SetBackground(true),
		},
	})
	if err != nil {
		log.Printf("⚠️ Failed to create openai_usage_logs indexes: %v", err)
	}

	// Notifications collection indexes
	notificationsCol := DB.Collection("notifications")
	_, err = notificationsCol.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{"project_id", 1}, {"sent_at", -1}},
			Options: options.Index().SetBackground(true),
		},
		{
			Keys:    bson.D{{"type", 1}},
			Options: options.Index().SetBackground(true),
		},
		{
			Keys:    bson.D{{"sent_at", -1}},
			Options: options.Index().SetBackground(true),
		},
	})
	if err != nil {
		log.Printf("⚠️ Failed to create notifications indexes: %v", err)
	}

	log.Println("📈 Database indexes setup completed")
	return nil
}

// Enhanced collection access with validation
func GetCollection(collectionName string) *mongo.Collection {
	if DB == nil {
		log.Fatal("❌ Database not initialized. Call InitMongoDB() first.")
	}

	if collectionName == "" {
		log.Fatal("❌ Collection name cannot be empty")
	}

	return DB.Collection(collectionName)
}

// Convenience functions for commonly used collections
func GetProjectsCollection() *mongo.Collection {
	return GetCollection("projects")
}

func GetClientsCollection() *mongo.Collection {
	return GetCollection("clients")
}

func GetChatMessagesCollection() *mongo.Collection {
	return GetCollection("chat_messages")
}

func GetChatUsersCollection() *mongo.Collection {
	return GetCollection("chat_users")
}

func GetWidgetSessionsCollection() *mongo.Collection {
	return GetCollection("widget_sessions")
}

func GetWidgetAnalyticsCollection() *mongo.Collection {
	return GetCollection("widget_analytics")
}

func GetOpenAIUsageLogsCollection() *mongo.Collection {
	return GetCollection("openai_usage_logs")
}

func GetNotificationsCollection() *mongo.Collection {
	return GetCollection("notifications")
}

// Health check and connection monitoring
func HealthCheck() error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test connection
	if err := Client.Ping(ctx, nil); err != nil {
		return fmt.Errorf("database ping failed: %v", err)
	}

	// Test a simple query
	collection := GetCollection("projects")
	count, err := collection.CountDocuments(ctx, bson.M{})
	if err != nil {
		return fmt.Errorf("database query failed: %v", err)
	}

	log.Printf("💚 Database health check passed (Projects: %d)", count)
	return nil
}

// GetDatabaseStats - Get comprehensive database statistics
func GetDatabaseStats() map[string]interface{} {
	if DB == nil {
		return map[string]interface{}{"error": "database not initialized"}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stats := make(map[string]interface{})

	// Get collection counts
	collections := []string{
		"projects",
		"clients",
		"chat_messages",
		"chat_users",
		"widget_sessions",
		"widget_analytics",
		"openai_usage_logs",
		"notifications",
	}

	for _, colName := range collections {
		count, err := GetCollection(colName).CountDocuments(ctx, bson.M{})
		if err != nil {
			stats[colName] = "error"
		} else {
			stats[colName] = count
		}
	}

	// Add connection info
	stats["database_name"] = DB.Name()
	stats["connected"] = true
	stats["timestamp"] = time.Now().Format(time.RFC3339)

	return stats
}

// Graceful shutdown
func CloseMongoDB() {
	if Client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := Client.Disconnect(ctx); err != nil {
			log.Printf("❌ Error disconnecting from MongoDB: %v", err)
		} else {
			log.Println("✅ Disconnected from MongoDB successfully")
		}
	}
}

// FixProjectLimits - Enhanced project limits fixing with configurable defaults
func FixProjectLimits() error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	collection := GetProjectsCollection()

	// Find projects with zero limits or missing subscription fields
	filter := bson.M{
		"$or": []bson.M{
			{"status": bson.M{"$exists": false}},
			{"expiry_date": bson.M{"$exists": false}},
			{"total_tokens_used": bson.M{"$exists": false}},
			{"monthly_token_limit": bson.M{"$exists": false}},
			{"start_date": bson.M{"$exists": false}},
			{"project_id": bson.M{"$exists": false}},
			{"status": ""},
		},
	}

	// Get configurable defaults from environment
	defaultTokenLimit := getEnvInt64("DEFAULT_MONTHLY_TOKEN_LIMIT", 100000)

	update := bson.M{
		"$set": bson.M{
			"updated_at": time.Now(),
			// Subscription fields
			"status":              "active",
			"start_date":          time.Now(),
			"expiry_date":         time.Now().AddDate(0, 1, 0), // 1 month from now
			"monthly_token_limit": defaultTokenLimit,
			// OpenAI configuration
			"ai_provider":  "openai",
			"openai_model": "gpt-4o",
		},
		"$setOnInsert": bson.M{
			"total_tokens_used": int64(0), // Only set if field doesn't exist
		},
	}

	result, err := collection.UpdateMany(ctx, filter, update)
	if err != nil {
		log.Printf("❌ Database error in FixProjectLimits: %v", err)
		return fmt.Errorf("failed to fix project limits: %v", err)
	}

	if result.ModifiedCount == 0 {
		log.Printf("ℹ️ No projects needed subscription field updates")
	} else {
		log.Printf("✅ Fixed limits and subscription fields for %d projects", result.ModifiedCount)
		log.Printf("📊 Applied defaults: Tokens=%d", defaultTokenLimit)
	}

	return nil
}

// InitializeSubscriptionDefaults - Initialize subscription defaults for existing projects
func InitializeSubscriptionDefaults() error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	collection := GetProjectsCollection()

	// Find projects missing subscription fields
	filter := bson.M{
		"$or": []bson.M{
			{"status": bson.M{"$exists": false}},
			{"expiry_date": bson.M{"$exists": false}},
			{"total_tokens_used": bson.M{"$exists": false}},
			{"monthly_token_limit": bson.M{"$exists": false}},
			{"start_date": bson.M{"$exists": false}},
			{"ai_provider": bson.M{"$exists": false}},
		},
	}

	update := bson.M{
		"$set": bson.M{
			"status":              "active",
			"start_date":          time.Now(),
			"expiry_date":         time.Now().AddDate(0, 1, 0), // 1 month from now
			"total_tokens_used":   int64(0),
			"monthly_token_limit": int64(100000), // 100k tokens default
			"ai_provider":         "openai",
			"openai_model":        "gpt-4o",
			"updated_at":          time.Now(),
		},
	}

	result, err := collection.UpdateMany(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to initialize subscription defaults: %v", err)
	}

	log.Printf("✅ Initialized subscription defaults for %d projects", result.ModifiedCount)
	return nil
}

// GetExpiredProjects - Get projects with expired subscriptions
func GetExpiredProjects() ([]primitive.ObjectID, error) {
	if DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	collection := GetProjectsCollection()

	filter := bson.M{
		"expiry_date": bson.M{"$lt": time.Now()},
		"status":      bson.M{"$ne": "expired"},
	}

	cursor, err := collection.Find(ctx, filter, options.Find().SetProjection(bson.M{"_id": 1}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var expiredProjects []primitive.ObjectID
	for cursor.Next(ctx) {
		var project struct {
			ID primitive.ObjectID `bson:"_id"`
		}
		if err := cursor.Decode(&project); err != nil {
			continue
		}
		expiredProjects = append(expiredProjects, project.ID)
	}

	return expiredProjects, nil
}

// UpdateExpiredProjects - Mark expired projects as expired
func UpdateExpiredProjects() error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	collection := GetProjectsCollection()

	filter := bson.M{
		"expiry_date": bson.M{"$lt": time.Now()},
		"status":      bson.M{"$ne": "expired"},
	}

	update := bson.M{
		"$set": bson.M{
			"status":     "expired",
			"updated_at": time.Now(),
		},
	}

	result, err := collection.UpdateMany(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update expired projects: %v", err)
	}

	log.Printf("✅ Marked %d projects as expired", result.ModifiedCount)
	return nil
}

// RunSubscriptionMaintenance - Run automated subscription maintenance
func RunSubscriptionMaintenance() error {
	log.Println("🔄 Running subscription maintenance...")

	// Update expired projects
	if err := UpdateExpiredProjects(); err != nil {
		log.Printf("❌ Failed to update expired projects: %v", err)
		return err
	}

	// Fix any projects with missing limits
	if err := FixProjectLimits(); err != nil {
		log.Printf("❌ Failed to fix project limits: %v", err)
		return err
	}

	log.Println("✅ Subscription maintenance completed")
	return nil
}

// LogNotification - Log notification events to database
func LogNotification(projectID primitive.ObjectID, notificationType, message string) error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	collection := GetNotificationsCollection()

	notification := bson.M{
		"project_id": projectID,
		"type":       notificationType,
		"message":    message,
		"sent_at":    time.Now(),
		"status":     "sent",
	}

	_, err := collection.InsertOne(ctx, notification)
	if err != nil {
		log.Printf("❌ Failed to log notification: %v", err)
		return err
	}

	log.Printf("✅ Notification logged: %s for project %s", notificationType, projectID.Hex())
	return nil
}

// WasNotificationRecentlySent - Check if notification was recently sent
func WasNotificationRecentlySent(projectID primitive.ObjectID, notificationType string, hours int) (bool, error) {
	if DB == nil {
		return false, fmt.Errorf("database not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	collection := GetNotificationsCollection()

	filter := bson.M{
		"project_id": projectID,
		"type":       notificationType,
		"sent_at": bson.M{
			"$gte": time.Now().Add(-time.Duration(hours) * time.Hour),
		},
	}

	count, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// Helper functions for environment variable parsing
func getEnvInt(key string, defaultValue int) int {
	if envValue := os.Getenv(key); envValue != "" {
		if parsed, err := strconv.Atoi(envValue); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvInt64(key string, defaultValue int64) int64 {
	if envValue := os.Getenv(key); envValue != "" {
		if parsed, err := strconv.ParseInt(envValue, 10, 64); err == nil {
			return parsed
		}
	}
	return defaultValue
}

// Subscription status constants
const (
	StatusActive    = "active"
	StatusExpired   = "expired"
	StatusSuspended = "suspended"
	StatusInactive  = "inactive"
	StatusDeleted   = "deleted"
)

// AI Provider constants
const (
	AIProviderOpenAI = "openai"
	AIProviderGemini = "gemini"
)

// Notification type constants
const (
	NotificationMonthlyLimit = "monthly_limit"
	NotificationUsageWarning = "usage_warning"
	NotificationExpired      = "expired"
	NotificationRenewal      = "renewal"
	NotificationTest         = "test"
)
