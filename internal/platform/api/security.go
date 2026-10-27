package api

import (
	"crypto/subtle"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"int3105-ride-share/internal/contracts"
	"int3105-ride-share/internal/platform/fault"
	"int3105-ride-share/internal/platform/security"
)

func Auth(role string) gin.HandlerFunc {
	manager, err := security.FromEnv()
	if err != nil {
		panic(err)
	}
	return func(c *gin.Context) {
		parts := strings.Fields(c.GetHeader("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			Fail(c, 401, "UNAUTHORIZED", "Bearer token required")
			return
		}
		user, err := manager.Parse(parts[1])
		if err != nil {
			Fail(c, 401, "UNAUTHORIZED", "Invalid or expired token")
			return
		}
		if role != "" && user.Role != role {
			Fail(c, 403, "FORBIDDEN", "Access denied")
			return
		}
		c.Set("user", user)
		if user.Role == "RIDER" {
			c.Set("riderId", user.UserID)
		} else {
			c.Set("driverId", user.UserID)
		}
		c.Next()
	}
}
func User(c *gin.Context) contracts.User {
	value, _ := c.Get("user")
	user, _ := value.(contracts.User)
	return user
}
func Internal() gin.HandlerFunc {
	token := os.Getenv("INTERNAL_TOKEN")
	if len(token) < 16 {
		panic("INTERNAL_TOKEN must have at least 16 bytes")
	}
	return func(c *gin.Context) {
		if subtle.ConstantTimeCompare([]byte(c.GetHeader("X-Internal-Token")), []byte(token)) != 1 {
			Fail(c, 401, "UNAUTHORIZED", "Invalid internal token")
			return
		}
		c.Next()
	}
}
func ServiceError(c *gin.Context, err error) {
	code := fault.Code(err)
	status := 500
	switch code {
	case "UNAUTHORIZED":
		status = 401
	case "FORBIDDEN":
		status = 403
	case "RIDE_NOT_FOUND":
		status = 404
	case "INVALID_RIDE_STATE", "RIDE_ALREADY_ACCEPTED", "ACCEPT_IN_PROGRESS":
		status = 409
	case "DEPENDENCY_UNAVAILABLE":
		status = 503
	}
	message := "Internal server error"
	if status != 500 {
		message = err.Error()
	}
	Fail(c, status, code, message)
}
