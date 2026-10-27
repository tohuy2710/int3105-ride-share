package auth

import (
	"github.com/gin-gonic/gin"
	"int3105-ride-share/internal/platform/api"
)

type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,max=72"`
}
type registerRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8,max=72"`
	Role     string `json:"role" binding:"required,oneof=RIDER DRIVER"`
}

func RegisterRoutes(r *gin.Engine, s Service) {
	r.POST("/auth/login", func(c *gin.Context) {
		var input loginRequest
		if !api.BindJSON(c, &input) {
			return
		}
		out, err := s.Login(c.Request.Context(), input.Email, input.Password)
		if err != nil {
			api.ServiceError(c, err)
			return
		}
		c.JSON(200, out)
	})
	r.POST("/auth/register", func(c *gin.Context) {
		var input registerRequest
		if !api.BindJSON(c, &input) {
			return
		}
		c.JSON(201, s.Register(input.Email, input.Role))
	})
	r.GET("/auth/me", api.Auth(""), func(c *gin.Context) { c.JSON(200, api.User(c)) })
}
