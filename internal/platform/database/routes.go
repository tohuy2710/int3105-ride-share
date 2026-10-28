package database

import (
	"context"
	"database/sql"
	"github.com/gin-gonic/gin"
	"time"
)

func Register(r *gin.Engine, db *sql.DB, table string, timeout time.Duration) {
	r.Use(func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	r.GET("/readyz", func(c *gin.Context) {
		var exists bool
		err := db.QueryRowContext(c.Request.Context(), "SELECT to_regclass($1) IS NOT NULL", table).Scan(&exists)
		if err != nil || !exists {
			c.JSON(503, gin.H{"status": "unavailable", "postgres": "unavailable"})
			return
		}
		c.JSON(200, gin.H{"status": "ok", "postgres": "ok"})
	})
}
