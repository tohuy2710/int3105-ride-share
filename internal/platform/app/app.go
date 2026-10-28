package app

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"int3105-ride-share/internal/platform/api"
	"int3105-ride-share/internal/platform/security"
	"int3105-ride-share/internal/platform/swagger"
)

type Config struct {
	Port  string
	Mode  string
	Level slog.Level
}

func LoadConfig(defaultPort string) (Config, error) {
	cfg := Config{Port: defaultPort, Mode: gin.ReleaseMode, Level: slog.LevelInfo}
	if s := os.Getenv("PORT"); s != "" {
		cfg.Port = s
	}
	port, err := strconv.Atoi(cfg.Port)
	if err != nil || port < 1 || port > 65535 {
		return cfg, fmt.Errorf("PORT must be between 1 and 65535")
	}
	if s := os.Getenv("GIN_MODE"); s != "" {
		cfg.Mode = s
	}
	if cfg.Mode != gin.DebugMode && cfg.Mode != gin.ReleaseMode && cfg.Mode != gin.TestMode {
		return cfg, fmt.Errorf("invalid GIN_MODE %q", cfg.Mode)
	}
	if s := os.Getenv("LOG_LEVEL"); s != "" {
		if err := cfg.Level.UnmarshalText([]byte(s)); err != nil {
			return cfg, fmt.Errorf("invalid LOG_LEVEL: %w", err)
		}
	}
	return cfg, nil
}
func Run(name, port string, register func(*gin.Engine)) error {
	if _, err := security.FromEnv(); err != nil {
		return err
	}
	if len(os.Getenv("INTERNAL_TOKEN")) < 16 {
		return fmt.Errorf("INTERNAL_TOKEN must have at least 16 bytes")
	}
	cfg, err := LoadConfig(port)
	if err != nil {
		return err
	}
	gin.SetMode(cfg.Mode)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.Level}))
	router := api.NewRouter(name, logger)
	if err := swagger.Register(router, name); err != nil {
		return err
	}
	register(router)
	hasReadiness := false
	for _, route := range router.Routes() {
		if route.Method == "GET" && route.Path == "/readyz" {
			hasReadiness = true
		}
	}
	if !hasReadiness {
		router.GET("/readyz", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })
	}
	server := &http.Server{Addr: net.JoinHostPort("", cfg.Port), Handler: router, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	done := make(chan error, 1)
	go func() {
		logger.Info("server started", "service", name, "port", cfg.Port)
		done <- server.ListenAndServe()
	}()
	select {
	case err := <-done:
		if err != http.ErrServerClosed {
			return err
		}
		return nil
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			_ = server.Close()
			return err
		}
		return nil
	}
}
