package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

// Logger is a middleware that logs each request
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)

		clientIP := c.ClientIP()
		statusCode := c.Writer.Status()

		responseSize := c.Writer.Size()

		method := c.Request.Method

		if raw != "" {
			path = path + "?" + raw
		}

		switch {
		case statusCode >= 500:
			log.Error().
				Str("method", method).
				Str("path", path).
				Int("status", statusCode).
				Str("ip", clientIP).
				Dur("latency", latency).
				Int("size", responseSize).
				Msg("HTTP Request")
		case statusCode >= 400:
			log.Warn().
				Str("method", method).
				Str("path", path).
				Int("status", statusCode).
				Str("ip", clientIP).
				Dur("latency", latency).
				Int("size", responseSize).
				Msg("HTTP Request")
		default:
			log.Info().
				Str("method", method).
				Str("path", path).
				Int("status", statusCode).
				Str("ip", clientIP).
				Dur("latency", latency).
				Int("size", responseSize).
				Msg("HTTP Request")
		}
	}
}

// Recovery is a middleware that recovers from panics
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.Error().
					Str("method", c.Request.Method).
					Str("path", c.Request.URL.Path).
					Str("ip", c.ClientIP()).
					Interface("error", err).
					Msg("HTTP Panic")

				c.AbortWithStatusJSON(500, gin.H{
					"error": "Internal server error",
				})
			}
		}()

		c.Next()
	}
}
