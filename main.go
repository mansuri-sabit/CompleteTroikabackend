// main.go
//
// Troika Chatbot SaaS – entry-point.
// Spin-ups MongoDB, loads environment, mounts all middle-ware / routes,
// starts an HTTPS/HTTP server, and shuts everything down gracefully.

package main

import (
	"context"
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

func main() {
	/*───────────────────────────────────────────*
	| 1. ENV-VARS & DATABASE                    |
	*───────────────────────────────────────────*/
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️  .env not found – relying on container / host env")
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

	// Global middleware – order matters
	r.Use(
		middleware.LoggingMiddleware(),         // request log
		gin.Recovery(),                         // panic recovery (gin's built-in)
		middleware.CORSMiddleware(),            // CORS
		middleware.SecurityHeadersMiddleware(), // basic hardening
		middleware.RefreshTokenMiddleware(),    // auto refresh soon-to-expire JWT
	)

	/*───────────────────────────────────────────*
	| 3. PUBLIC ENDPOINTS                       |
	*───────────────────────────────────────────*/
	public := r.Group("/api")
	{
		public.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
		public.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") })

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

		// Widget JS (served from ./static/widget.js)
		r.Static("/static", "./static")
		r.GET("/widget.js", func(c *gin.Context) {
    	c.Header("Content-Type", "application/javascript")
    	c.Header("Cache-Control", "public, max-age=3600")
    	c.File("./static/widget.js")
})
	}

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

		// Embed / docs
		admin.GET("/projects/:id/embed", handlers.GetEmbedCode)
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
