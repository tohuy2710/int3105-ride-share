// Package redisgate is a best-effort contention gate, never the data authority.
package redisgate

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"github.com/redis/go-redis/v9"
	"time"
)

type Gate struct {
	Client       *redis.Client
	TTL, Timeout time.Duration
}

func (g *Gate) Acquire(ctx context.Context, id string) (func(), bool, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return nil, false, err
	}
	token := hex.EncodeToString(value[:])
	key := "ride:accept:" + id
	timeout := g.Timeout
	if timeout <= 0 {
		timeout = 100 * time.Millisecond
	}
	ttl := g.TTL
	if ttl <= 0 {
		ttl = 5 * time.Second
	}
	request, cancel := context.WithTimeout(ctx, timeout)
	ok, err := g.Client.SetNX(request, key, token, ttl).Result()
	cancel()
	if err != nil || !ok {
		return nil, ok, err
	}
	return func() {
		cleanup, done := context.WithTimeout(context.Background(), timeout)
		defer done()
		g.Client.Eval(cleanup, `if redis.call('GET',KEYS[1]) == ARGV[1] then return redis.call('DEL',KEYS[1]) else return 0 end`, []string{key}, token)
	}, true, nil
}
