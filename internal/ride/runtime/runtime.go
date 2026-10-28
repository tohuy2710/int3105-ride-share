// Package runtime is the composition root for infrastructure-dependent Ride setup.
package runtime

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"
	"int3105-ride-share/internal/platform/api"
	"int3105-ride-share/internal/platform/config"
	"int3105-ride-share/internal/platform/database"
	"int3105-ride-share/internal/platform/metrics"
	"int3105-ride-share/internal/ride"
	"int3105-ride-share/internal/ride/postgres"
	"int3105-ride-share/internal/ride/redisgate"
	"time"
)

type Runtime struct {
	DB      *sql.DB
	Redis   *redis.Client
	App     *ride.Application
	Metrics *metrics.Counters
	Mode    string
}

func New() (*Runtime, error) {
	mode := config.Env("ACCEPT_MODE", "redis_sql")
	if mode != "sql" && mode != "redis_sql" {
		return nil, fmt.Errorf("ACCEPT_MODE must be sql or redis_sql")
	}
	dbTimeout, err := config.Duration("DB_TIMEOUT", 2*time.Second)
	if err != nil {
		return nil, err
	}
	redisTimeout, err := config.Duration("REDIS_TIMEOUT", 100*time.Millisecond)
	if err != nil {
		return nil, err
	}
	ttl, err := config.Duration("REDIS_LOCK_TTL", 5*time.Second)
	if err != nil {
		return nil, err
	}
	opts, err := redis.ParseURL(config.Env("REDIS_URL", "redis://localhost:6379/0"))
	if err != nil {
		return nil, err
	}
	opts.DialTimeout = redisTimeout
	opts.ReadTimeout = redisTimeout
	opts.WriteTimeout = redisTimeout
	opts.PoolTimeout = redisTimeout
	opts.MaxRetries = -1
	opts.ContextTimeoutEnabled = true
	opts.MaintNotificationsConfig = &maintnotifications.Config{Mode: maintnotifications.ModeDisabled}
	db, err := database.Open()
	if err != nil {
		return nil, err
	}
	rt := &Runtime{DB: db, Metrics: &metrics.Counters{}, Mode: mode}
	rt.App = &ride.Application{Repo: &postgres.Repository{DB: db}, Metrics: rt.Metrics, DBTimeout: dbTimeout}
	if mode == "redis_sql" {
		rt.Redis = redis.NewClient(opts)
		rt.App.Gate = &redisgate.Gate{Client: rt.Redis, TTL: ttl, Timeout: redisTimeout}
	}
	return rt, nil
}
func (rt *Runtime) Close() {
	rt.DB.Close()
	if rt.Redis != nil {
		rt.Redis.Close()
	}
}
func (rt *Runtime) Register(r *gin.Engine) {
	ride.RegisterRoutes(r, rt.App)
	r.GET("/internal/metrics", api.Internal(), func(c *gin.Context) { c.JSON(200, rt.Metrics.Snapshot()) })
	r.GET("/readyz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), rt.App.DBTimeout)
		defer cancel()
		var exists bool
		err := rt.DB.QueryRowContext(ctx, "SELECT to_regclass('ride.rides') IS NOT NULL").Scan(&exists)
		if err != nil || !exists {
			c.JSON(503, gin.H{"status": "unavailable", "postgres": "unavailable"})
			return
		}
		if rt.Redis != nil {
			ctx, cancel := context.WithTimeout(c.Request.Context(), 100*time.Millisecond)
			defer cancel()
			if rt.Redis.Ping(ctx).Err() != nil {
				c.JSON(200, gin.H{"status": "degraded", "postgres": "ok", "redis": "unavailable"})
				return
			}
		}
		c.JSON(200, gin.H{"status": "ok", "postgres": "ok"})
	})
}
