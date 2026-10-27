package metrics

import "sync/atomic"

type Counters struct{ sqlAttempts, redisRejected, redisFallback atomic.Uint64 }
type Snapshot struct {
	SQLAttempts   uint64 `json:"sqlAttempts"`
	RedisRejected uint64 `json:"redisRejected"`
	RedisFallback uint64 `json:"redisFallback"`
}

func (c *Counters) SQLAttempt()    { c.sqlAttempts.Add(1) }
func (c *Counters) RedisRejected() { c.redisRejected.Add(1) }
func (c *Counters) RedisFallback() { c.redisFallback.Add(1) }
func (c *Counters) Snapshot() Snapshot {
	return Snapshot{c.sqlAttempts.Load(), c.redisRejected.Load(), c.redisFallback.Load()}
}
