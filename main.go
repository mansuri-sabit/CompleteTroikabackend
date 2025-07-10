// main.go
//
// Troika Chatbot SaaS – entry-point.
// Spin-ups MongoDB, loads environment, mounts all middle-ware / routes,
// starts an HTTPS/HTTP server, and shuts everything down gracefully.

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"jevi-chat/config"
	"jevi-chat/handlers"
	"jevi-chat/middleware"
	"jevi-chat/utils"
)

// getDomain returns the appropriate domain based on environment
func getDomain() string {
	if domain := os.Getenv("DOMAIN"); domain != "" {
		return domain
	}
	if os.Getenv("ENVIRONMENT") == "production" {
		return "https://completetroikabackend.onrender.com"
	}
	return "http://localhost:8080"
}

func main() {
	/*───────────────────────────────────────────*
	| 1. ENV-VARS & DATABASE                    |
	*───────────────────────────────────────────*/
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️  .env not found – relying on container / host env")
	}

	// Set default environment if not specified
	if os.Getenv("ENVIRONMENT") == "" {
		os.Setenv("ENVIRONMENT", "production")
	}

	// Set default domain if not specified
	if os.Getenv("DOMAIN") == "" {
		os.Setenv("DOMAIN", "https://completetroikabackend.onrender.com")
	}

	// Initialise MongoDB (panics on failure)
	config.InitMongoDB()
	defer config.CloseMongoDB()

	// Create default admin user
	if err := utils.CreateDefaultAdmin(); err != nil {
		log.Printf("❌ Failed to create default admin: %v", err)
	}

	/*───────────────────────────────────────────*
	| 2. GIN ENGINE & GLOBAL MIDDLEWARE         |
	*───────────────────────────────────────────*/
	gin.SetMode(os.Getenv("GIN_MODE")) // release | debug (default)
	r := gin.New()

// Add this BEFORE your existing middleware in main.go
r.Use(func(c *gin.Context) {
    origin := c.Request.Header.Get("Origin")
    
    // Log CORS requests for debugging
    log.Printf("🌐 CORS Request - Origin: %s, Method: %s, Path: %s", 
        origin, c.Request.Method, c.Request.URL.Path)
    
    // Define allowed origins
    allowedOrigins := []string{
        "https://troika-admin-dashborad.onrender.com",
        "https://troikacompletefrontend.onrender.com",
        "https://admin.troikatech.com",
        "http://localhost:3000",
        "http://localhost:3001",
    }
    
    // Check if origin is allowed
    isAllowed := false
    for _, allowedOrigin := range allowedOrigins {
        if origin == allowedOrigin {
            isAllowed = true
            break
        }
    }
    
    if isAllowed || os.Getenv("ENVIRONMENT") == "development" {
        c.Header("Access-Control-Allow-Origin", origin)
        log.Printf("✅ CORS Allowed for origin: %s", origin)
    }
    
    c.Header("Access-Control-Allow-Credentials", "true")
    c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With, Accept, Origin, Cache-Control")
    
    // 🔥 CRITICAL FIX: Include ALL HTTP methods including PATCH
    c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS, HEAD")
    
    c.Header("Access-Control-Max-Age", "86400")
    
    // Handle preflight requests
    if c.Request.Method == "OPTIONS" {
        log.Printf("🔄 CORS Preflight request handled for %s", c.Request.URL.Path)
        c.AbortWithStatus(http.StatusNoContent)
        return
    }
    
    c.Next()
})


	// Global middleware – order matters
	r.Use(
		middleware.LoggingMiddleware(),         // request log
		gin.Recovery(),                         // panic recovery (gin's built-in)
		middleware.CORSMiddleware(),            // Your existing CORS middleware (backup)
		middleware.SecurityHeadersMiddleware(), // basic hardening
		middleware.RefreshTokenMiddleware(),    // auto refresh soon-to-expire JWT
	)

	/*───────────────────────────────────────────*
	| 3. PUBLIC ENDPOINTS                       |
	*───────────────────────────────────────────*/
	public := r.Group("/api")
	{
		// 🔥 ENHANCED: Health check with more detailed information
		public.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"ok":        true,
				"timestamp": time.Now(),
				"service":   "troika-chatbot-api",
				"version":   "1.0.0",
				"domain":    getDomain(),
			})
		})

		public.GET("/ping", func(c *gin.Context) {
			c.String(http.StatusOK, "pong")
		})

		// 🔥 NEW: CORS test endpoint for debugging
		public.GET("/cors-test", func(c *gin.Context) {
			origin := c.Request.Header.Get("Origin")
			c.JSON(http.StatusOK, gin.H{
				"message":   "CORS test successful",
				"origin":    origin,
				"timestamp": time.Now(),
				"headers":   c.Request.Header,
			})
		})

		// Authentication routes
		public.POST("/auth/login", handlers.Login)
		public.POST("/auth/register", handlers.Register)
		public.POST("/auth/logout", handlers.Logout)
		public.GET("/auth/verify", handlers.VerifyToken)

		// Chat / widget (project-first). Extra middle-wares per request:
		public.POST("/projects/:projectId/chat",
			middleware.SubscriptionValidator(),
			middleware.TokenLimitValidator(),
			middleware.RateLimitValidator(),
			middleware.SubscriptionHeaders(),
			handlers.ProjectChatMessage,
		)

		public.GET("/projects/:projectId/history", handlers.GetChatHistory)

		// Subscription status (used by widget UI)
		public.GET("/projects/:projectId/subscription", handlers.GetSubscriptionStatus)

		// Embed routes
		public.GET("/embed/:projectId", handlers.EmbedChat)
		public.POST("/embed/:projectId/auth", handlers.EmbedAuth)
		public.GET("/embed/:projectId/chat", handlers.IframeChatInterface)
		public.GET("/embed/:projectId/auth", handlers.ShowEmbedAuth)
		public.GET("/embed/health", handlers.EmbedHealth)
	}

	// 🔥 ENHANCED: Widget.js route with proper CORS headers for embedding
	r.Static("/static", "./static")
	r.GET("/widget.js", func(c *gin.Context) {
		c.Header("Content-Type", "application/javascript")
		c.Header("Cache-Control", "public, max-age=3600")
		c.Header("Access-Control-Allow-Origin", "*") // Allow embedding on any domain
		c.Header("Access-Control-Allow-Methods", "GET")
		c.Header("Access-Control-Allow-Headers", "Content-Type")

		// Check if widget.js file exists
		if _, err := os.Stat("./static/widget.js"); os.IsNotExist(err) {
			log.Printf("⚠️ Widget.js file not found at ./static/widget.js")
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Widget file not found",
				"path":  "./static/widget.js",
			})
			return
		}

		c.File("./static/widget.js")
	})

	/*───────────────────────────────────────────*
	| 4. AUTHENTICATED ROUTES (USER PANEL)      |
	*───────────────────────────────────────────*/
	user := r.Group("/api/user")
	user.Use(
		middleware.AuthMiddleware(), // require JWT
		middleware.SubscriptionLogger(),
	)
	{
		user.GET("/profile", handlers.GetUserProfile)
		user.PUT("/profile", handlers.UpdateUserProfile)
		user.POST("/change-password", handlers.ChangePassword)
	}

	/*───────────────────────────────────────────*
	| 5. ADMIN ROUTES                           |
	*───────────────────────────────────────────*/
	admin := r.Group("/api/admin")
	admin.Use(
		middleware.AuthMiddleware(),  // JWT
		middleware.AdminMiddleware(), // must be role = admin
	)
	{
		// Dashboard & system
		admin.GET("/dashboard", handlers.AdminDashboard)
		admin.GET("/stats", handlers.GetSystemStats)
		admin.GET("/notifications", handlers.GetNotificationHistory)

		// Project CRUD
		admin.GET("/projects", handlers.GetProjectsDashboard)
		admin.POST("/projects", handlers.CreateProject)
		admin.GET("/projects/:id", handlers.GetProjectDetails)
		admin.PATCH("/projects/:id", handlers.UpdateProject)
		admin.DELETE("/projects/:id", handlers.DeleteProject)

		// 🔥 ENHANCED: Embed / docs with proper domain handling
		admin.GET("/projects/:id/embed", func(c *gin.Context) {
			projectID := c.Param("id")
			domain := getDomain()

			// Generate proper embed code with actual domain
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
				"domain":     domain,
				"iframe_url": fmt.Sprintf("%s/embed/%s", domain, projectID),
			})
		})

		admin.POST("/projects/:id/embed/regenerate", handlers.RegenerateEmbedCode)

		// Subscription actions
		admin.POST("/projects/:id/renew", handlers.RenewProject)
		admin.PATCH("/projects/:id/status", handlers.UpdateProjectStatus)
		admin.POST("/projects/:id/suspend", handlers.SuspendProject)
		admin.POST("/projects/:id/reactivate", handlers.ReactivateProject)

		// Token / usage tools
		admin.GET("/projects/:id/usage", handlers.GetProjectUsage)
		admin.POST("/projects/:id/limit", handlers.UpdateTokenLimit)
		admin.POST("/projects/:id/usage/reset", handlers.ResetTokenUsage)

		// Notifications
		admin.GET("/projects/:id/notifications", handlers.GetProjectNotifications)
		admin.POST("/projects/:id/notifications/test", handlers.TestNotification)
	}

	/*───────────────────────────────────────────*
	| 6. BACKGROUND MAINTENANCE JOBS            |
	*───────────────────────────────────────────*/
	go func() {
		// Daily subscription maintenance & expiry sweep
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		for range ticker.C {
			if err := config.RunSubscriptionMaintenance(); err != nil {
				log.Printf("⚠️  Subscription maintenance failed: %v", err)
			}
		}
	}()

	/*───────────────────────────────────────────*
	| 7. START SERVER + GRACEFUL SHUTDOWN       |
	*───────────────────────────────────────────*/
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:           ":" + port,
		Handler:        r,
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1 MiB
	}

	// Log startup information
	domain := getDomain()
	log.Printf("🚀  Troika Chatbot API starting...")
	log.Printf("📍  Domain: %s", domain)
	log.Printf("🌍  Environment: %s", os.Getenv("ENVIRONMENT"))
	log.Printf("🎯  Port: %s", port)
	log.Printf("📡  Widget URL: %s/widget.js", domain)

	go func() {
		log.Printf("🚀  Troika Chatbot API listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌  ListenAndServe: %v", err)
		}
	}()

	// Wait for interrupt → graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, os.Kill)
	<-quit
	log.Println("🛑  Shutting down server…")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("❌  Server forced to shutdown: %v", err)
	}

	log.Println("✅  Server exiting")
}
