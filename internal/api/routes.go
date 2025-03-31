package api

import (
	"github.com/gin-gonic/gin"

	"github.com/rishavmehra/indexer/internal/api/handlers"
	"github.com/rishavmehra/indexer/internal/api/middleware"
)

// SetupRoutes configures all the API routes
func SetupRoutes(
	router *gin.Engine,
	authHandler *handlers.AuthHandler,
	userHandler *handlers.UserHandler,
	indexerHandler *handlers.IndexerHandler,
	mw middleware.MiddlewareConfig,
) {
	router.Use(middleware.Logger())
	router.Use(middleware.Recovery())
	router.Use(gin.Recovery())

	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "ok",
		})
	})

	v1 := router.Group("/api/v1")
	{
		authHandler.RegisterRoutes(v1)
		userHandler.RegisterRoutes(v1, mw)
		indexerHandler.RegisterRoutes(v1, mw)
	}

	indexerHandler.RegisterWebhookRoute(router)
}
