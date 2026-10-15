package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

func init() {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		if err := v.RegisterValidation("notblank", func(fl validator.FieldLevel) bool { return strings.TrimSpace(fl.Field().String()) != "" }); err != nil {
			panic(err)
		}
	}
}

type ErrorBody struct {
	Code    string   `json:"code"`
	Message string   `json:"message"`
	Details []string `json:"details"`
}
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

func Fail(c *gin.Context, status int, code, message string, details ...string) {
	if details == nil {
		details = []string{}
	}
	c.AbortWithStatusJSON(status, ErrorResponse{Error: ErrorBody{code, message, details}})
}

// BindJSON validates one JSON document. Pointers distinguish zero/false from missing fields.
func BindJSON(c *gin.Context, dst any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20))
	err := decoder.Decode(dst)
	if err == nil {
		var extra any
		if next := decoder.Decode(&extra); next != io.EOF {
			err = errors.New("body must contain exactly one JSON document")
		}
	}
	if err == nil {
		err = binding.Validator.ValidateStruct(dst)
	}
	if err != nil {
		Fail(c, 400, "VALIDATION_ERROR", "Invalid request body", err.Error())
		return false
	}
	return true
}
func BindQuery(c *gin.Context, dst any) bool {
	if err := c.ShouldBindQuery(dst); err != nil {
		Fail(c, 400, "VALIDATION_ERROR", "Invalid query parameters", err.Error())
		return false
	}
	return true
}
func RideID(c *gin.Context) (string, bool) {
	id := c.Param("rideId")
	if strings.TrimSpace(id) == "" {
		Fail(c, 400, "VALIDATION_ERROR", "Invalid rideId", "rideId must not be blank")
		return "", false
	}
	return id, true
}
func Identity(key, value string) gin.HandlerFunc {
	return func(c *gin.Context) { c.Set(key, value); c.Next() }
}

type Coordinates struct {
	Lat  *float64 `json:"lat" binding:"required,gte=-90,lte=90"`
	Long *float64 `json:"long" binding:"required,gte=-180,lte=180"`
}

func NewRouter(service string, logger *slog.Logger) *gin.Engine {
	r := gin.New()
	r.HandleMethodNotAllowed = true
	r.Use(func(c *gin.Context) {
		id := c.GetHeader("X-Request-ID")
		if id == "" || len(id) > 128 {
			var b [16]byte
			if _, err := rand.Read(b[:]); err != nil {
				panic(err)
			}
			id = hex.EncodeToString(b[:])
		}
		c.Header("X-Request-ID", id)
		start := time.Now()
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error("request panic", "requestId", id, "panic", fmt.Sprint(recovered))
				Fail(c, 500, "INTERNAL_ERROR", "Internal server error")
			}
			logger.Info("http request", "service", service, "requestId", id, "method", c.Request.Method, "path", c.Request.URL.Path, "status", c.Writer.Status(), "duration", time.Since(start))
		}()
		c.Next()
	})
	r.GET("/healthz", func(c *gin.Context) { c.JSON(200, gin.H{"service": service, "status": "ok"}) })
	r.NoRoute(func(c *gin.Context) { Fail(c, 404, "NOT_FOUND", "Route not found") })
	r.NoMethod(func(c *gin.Context) { Fail(c, 405, "METHOD_NOT_ALLOWED", "Method not allowed") })
	return r
}
