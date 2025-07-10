package handlers

import (
	"context"
	"crypto/rand"
	"strconv"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"math"
"go.mongodb.org/mongo-driver/mongo/options"
	"time"
	"path/filepath"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	
	"jevi-chat/config"
	"jevi-chat/models"


)

// CreateProject - Enhanced project creation with OpenAI integration
func CreateProject(c *gin.Context) {
    userID := c.GetString("user_id")
    userEmail := c.GetString("user_email")
    userRole := c.GetString("user_role")
    
    if userID == "" || userRole != "admin" {
        c.JSON(http.StatusUnauthorized, gin.H{
            "error": "Admin authentication required",
        })
        return
    }

    // ✅ Handle multipart form data properly
    err := c.Request.ParseMultipartForm(32 << 20) // 32MB max
    if err != nil {
        log.Printf("❌ Failed to parse multipart form: %v", err)
        c.JSON(http.StatusBadRequest, gin.H{
            "error": "Failed to parse form data",
        })
        return
    }

    // ✅ Extract form values directly
    name := c.PostForm("name")
    description := c.PostForm("description")
    clientEmail := c.PostForm("client_email")
    welcomeMessage := c.PostForm("welcome_message")
    theme := c.PostForm("theme")
    primaryColor := c.PostForm("primary_color")
    
    // Parse monthly token limit
    monthlyTokenLimit := int64(100000) // default
    if limitStr := c.PostForm("monthly_token_limit"); limitStr != "" {
        if parsed, err := strconv.ParseInt(limitStr, 10, 64); err == nil {
            monthlyTokenLimit = parsed
        }
    }

    // ✅ Validate required fields
    if name == "" {
        log.Printf("❌ Project name is empty")
        c.JSON(http.StatusBadRequest, gin.H{
            "error": "Project name is required",
        })
        return
    }

    // Set defaults
    if welcomeMessage == "" {
        welcomeMessage = "Hello! How can I help you today?"
    }
    if theme == "" {
        theme = "default"
    }
    if primaryColor == "" {
        primaryColor = "#4f46e5"
    }

    // ✅ Handle PDF file uploads and processing
    form, _ := c.MultipartForm()
    files := form.File["pdf_files"]
    
    var pdfFiles []models.PDFFile
    var combinedPDFContent string
    
    for _, file := range files {
        // Validate file type
        if file.Header.Get("Content-Type") != "application/pdf" {
            c.JSON(http.StatusBadRequest, gin.H{
                "error": fmt.Sprintf("File %s is not a PDF", file.Filename),
            })
            return
        }
        
        // Validate file size (10MB max)
        if file.Size > 10*1024*1024 {
            c.JSON(http.StatusBadRequest, gin.H{
                "error": fmt.Sprintf("File %s exceeds 10MB limit", file.Filename),
            })
            return
        }
        
        // Generate unique filename and save
        fileID := primitive.NewObjectID().Hex()
        fileName := fmt.Sprintf("%s_%s", fileID, file.Filename)
        filePath := filepath.Join("uploads", "pdfs", fileName)
        
        // Create upload directory
        uploadDir := filepath.Dir(filePath)
        if err := os.MkdirAll(uploadDir, 0755); err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{
                "error": "Failed to create upload directory",
            })
            return
        }
        
        // Save file
        if err := c.SaveUploadedFile(file, filePath); err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{
                "error": fmt.Sprintf("Failed to save file %s", file.Filename),
            })
            return
        }
        
        // ✅ Extract PDF content for OpenAI processing
        pdfContent, err := extractPDFContent(filePath)
        if err != nil {
            log.Printf("⚠️ Failed to extract content from %s: %v", file.Filename, err)
            pdfContent = fmt.Sprintf("Content from %s could not be extracted", file.Filename)
        }
        
        // ✅ Process content with OpenAI for embeddings
        embeddings, err := generateOpenAIEmbeddings(pdfContent)
        if err != nil {
            log.Printf("⚠️ Failed to generate embeddings for %s: %v", file.Filename, err)
        }
        
        // Create PDF file record
        pdfFile := models.PDFFile{
            ID:           fileID,
            FileName:     file.Filename,
            FilePath:     filePath,
            FileSize:     file.Size,
            ContentType:  file.Header.Get("Content-Type"),
            Content:      pdfContent,
            Embeddings:   embeddings,
            UploadedAt:   time.Now(),
            ProcessedAt:  time.Now(),
            Status:       "processed",
        }
        
        pdfFiles = append(pdfFiles, pdfFile)
        combinedPDFContent += pdfContent + "\n\n"
    }

    // Generate unique project ID
    projectID := fmt.Sprintf("proj_%d_%s", time.Now().Unix(), generateRandomString(8))
    embedCode := generateEmbedCode(projectID)

    // Create project object
    project := models.Project{
        ID:                primitive.NewObjectID(),
        ProjectID:         projectID,
        Name:              name,
        Description:       description,
        Category:          "chatbot",
        ClientID:          clientEmail,
        StartDate:         time.Now(),
        ExpiryDate:        time.Now().AddDate(1, 0, 0),
        Status:            "active",
        TotalTokensUsed:   0,
        MonthlyTokenLimit: monthlyTokenLimit,
        EmbedCode:         embedCode,
        WidgetSettings: models.ProjectWidgetConfig{
            Theme:            theme,
            PrimaryColor:     primaryColor,
            WelcomeMessage:   welcomeMessage,
            Position:         "bottom-right",
            ShowBranding:     true,
            EnableFileUpload: len(pdfFiles) > 0,
            EnableRating:     true,
        },
        AIProvider:        "openai",
        OpenAIModel:       "gpt-4o",
        PDFFiles:          pdfFiles,
        PDFContent:        combinedPDFContent,
        CreatedAt:         time.Now(),
        UpdatedAt:         time.Now(),
        IsActive:          true,
    }

    // Insert project into database
    collection := config.GetProjectsCollection()
    result, err := collection.InsertOne(context.Background(), project)
    if err != nil {
        log.Printf("❌ Failed to create project: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "Failed to create project",
        })
        return
    }

    project.ID = result.InsertedID.(primitive.ObjectID)

    log.Printf("✅ Project created with %d PDF files: %s by %s", len(pdfFiles), project.Name, userEmail)

    c.JSON(http.StatusCreated, gin.H{
        "message": "Project created successfully",
        "project": gin.H{
            "id":                  project.ID.Hex(),
            "project_id":          project.ProjectID,
            "name":                project.Name,
            "description":         project.Description,
            "status":              project.Status,
            "total_tokens_used":   project.TotalTokensUsed,
            "monthly_token_limit": project.MonthlyTokenLimit,
            "pdf_files_count":     len(pdfFiles),
            "created_at":          project.CreatedAt,
            "expiry_date":         project.ExpiryDate,
        },
    })
}


// UpdateProject - Update project settings
func UpdateProject(c *gin.Context) {
	projectID := c.Param("id")

	var updateData struct {
		Name              string `json:"name"`
		Description       string `json:"description"`
		MonthlyTokenLimit int64  `json:"monthly_token_limit"`
		WelcomeMessage    string `json:"welcome_message"`
		Theme             string `json:"theme"`
		PrimaryColor      string `json:"primary_color"`
		Status            string `json:"status"`
	}

	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid update data"})
		return
	}

	collection := config.DB.Collection("projects")

	update := bson.M{
		"$set": bson.M{
			"updated_at": time.Now(),
		},
	}

	// Update fields if provided
	if updateData.Name != "" {
		update["$set"].(bson.M)["name"] = updateData.Name
	}
	if updateData.Description != "" {
		update["$set"].(bson.M)["description"] = updateData.Description
	}
	if updateData.MonthlyTokenLimit > 0 {
		update["$set"].(bson.M)["monthly_token_limit"] = updateData.MonthlyTokenLimit
	}
	if updateData.WelcomeMessage != "" {
		update["$set"].(bson.M)["widget_settings.welcome_message"] = updateData.WelcomeMessage
	}
	if updateData.Theme != "" {
		update["$set"].(bson.M)["widget_settings.theme"] = updateData.Theme
	}
	if updateData.PrimaryColor != "" {
		update["$set"].(bson.M)["widget_settings.primary_color"] = updateData.PrimaryColor
	}
	if updateData.Status != "" && isValidStatus(updateData.Status) {
		update["$set"].(bson.M)["status"] = updateData.Status
	}

	result, err := collection.UpdateOne(context.Background(),
		bson.M{"project_id": projectID}, update)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update project"})
		return
	}

	if result.ModifiedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Project updated successfully",
	})
}

// SuspendProject - Suspend project access
func SuspendProject(c *gin.Context) {
	projectID := c.Param("id")

	err := updateProjectStatus(projectID, "suspended")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to suspend project"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Project suspended successfully",
		"status":  "suspended",
	})
}

// ReactivateProject - Reactivate suspended project
func ReactivateProject(c *gin.Context) {
	projectID := c.Param("id")

	// Check if project is not expired
	project, err := getProjectByID(projectID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	if time.Now().After(project.ExpiryDate) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Cannot reactivate expired project. Please renew first.",
		})
		return
	}

	err = updateProjectStatus(projectID, "active")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reactivate project"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Project reactivated successfully",
		"status":  "active",
	})
}

// GetEmbedCode - Get embeddable widget code
func GetEmbedCode(c *gin.Context) {
    projectID := c.Param("id")
    
    // Get the actual domain from environment or request
    domain := os.Getenv("DOMAIN")
    if domain == "" {
        // Fallback to your actual backend domain
        domain = "https://completetroikabackend.onrender.com"
    }
    
    // Generate proper embed code
    embedCode := fmt.Sprintf(`<script>
(function() {
    var script = document.createElement('script');
    script.src = '%s/widget.js';
    script.setAttribute('data-project-id', '%s');
    script.async = true;
    document.head.appendChild(script);
})();
</script>`, domain, projectID)
    
    c.JSON(http.StatusOK, gin.H{
        "embed_code": embedCode,
        "widget_url": fmt.Sprintf("%s/widget.js", domain),
        "project_id": projectID,
    })
}


// RegenerateEmbedCode - Generate new embed code
func RegenerateEmbedCode(c *gin.Context) {
	projectID := c.Param("id")

	newEmbedCode := generateEmbedCode(projectID)

	collection := config.DB.Collection("projects")
	update := bson.M{
		"$set": bson.M{
			"embed_code": newEmbedCode,
			"updated_at": time.Now(),
		},
	}

	result, err := collection.UpdateOne(context.Background(),
		bson.M{"project_id": projectID}, update)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to regenerate embed code"})
		return
	}

	if result.ModifiedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Embed code regenerated successfully",
		"embed_code": newEmbedCode,
	})
}

// DeleteProject - Soft delete project

// Helper Functions

// generateUniqueProjectID - Generate unique project identifier
func generateUniqueProjectID() string {
	timestamp := time.Now().Unix()
	randomPart := generateRandomString(8)
	return fmt.Sprintf("proj_%d_%s", timestamp, randomPart)
}

// generateRandomString - Generate random string for IDs
func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, length)

	for i := range result {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		result[i] = charset[num.Int64()]
	}

	return string(result)
}

// generateEmbedCode - Generate embeddable widget code
func generateEmbedCode(projectID string) string {
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "https://your-domain.com"
	}

	return fmt.Sprintf(`<script>
(function() {
    var script = document.createElement('script');
    script.src = '%s/widget.js';
    script.setAttribute('data-project-id', '%s');
    script.async = true;
    document.head.appendChild(script);
})();
</script>`, baseURL, projectID)
}

// createClientRecord - Create client record
func createClientRecord(email, name, company, projectID string) string {
	clientID := fmt.Sprintf("client_%d_%s", time.Now().Unix(), generateRandomString(6))

	client := models.Client{
		ID:        primitive.NewObjectID(),
		ClientID:  clientID,
		Email:     email,
		Name:      name,
		Company:   company,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	collection := config.DB.Collection("clients")
	_, err := collection.InsertOne(context.Background(), client)
	if err != nil {
		log.Printf("Failed to create client record: %v", err)
		return ""
	}

	log.Printf("✅ Client created: %s (%s)", name, email)
	return clientID
}

// getStringOrDefault - Helper to get string value or default
func getStringOrDefault(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}

// DeleteProject - Soft delete project
func DeleteProject(c *gin.Context) {
    projectID := c.Param("id")
    
    if projectID == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Project ID is required"})
        return
    }

    collection := config.GetProjectsCollection()
    
    // Perform soft delete by updating status and is_active fields
    update := bson.M{
        "$set": bson.M{
            "status":     "deleted",
            "is_active":  false,
            "updated_at": time.Now(),
        },
    }

    result, err := collection.UpdateOne(context.Background(), 
        bson.M{"project_id": projectID}, update)
    if err != nil {
        log.Printf("❌ Failed to delete project %s: %v", projectID, err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete project"})
        return
    }

    if result.ModifiedCount == 0 {
        c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
        return
    }

    // Log deletion action
    config.LogNotification(primitive.NilObjectID, "deletion", 
        fmt.Sprintf("Project %s was deleted", projectID))

    log.Printf("⚠️ Project soft deleted: %s", projectID)

    c.JSON(http.StatusOK, gin.H{
        "message": "Project deleted successfully",
        "project_id": projectID,
    })
}



// GET /api/admin/projects?page=1&limit=10
func GetProjects(c *gin.Context) {
    page,  _ := strconv.Atoi(c.DefaultQuery("page",  "1"))
    limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

    skip := (page - 1) * limit

    opts  := options.Find().SetSkip(int64(skip)).SetLimit(int64(limit)).
                     SetSort(bson.D{{"created_at", -1}})

    cur, _ := config.GetProjectsCollection().Find(context.Background(), bson.M{}, opts)

    var projects []models.Project
    cur.All(c, &projects)

    total, _ := config.GetProjectsCollection().CountDocuments(c, bson.M{})

    c.JSON(http.StatusOK, gin.H{
        "projects": projects,
        "pagination": gin.H{
            "page": page,
            "limit": limit,
            "total": total,
            "pages": int(math.Ceil(float64(total)/float64(limit))),
        },
    })
}
