// Package redisclient connects to Redis.
package redisclient

import (
	"context"

	"github.com/redis/go-redis/v9"
)

// New returns a connected client, verified with a PING.
func New(ctx context.Context, addr string) (*redis.Client, error) {
	c := redis.NewClient(&redis.Options{Addr: addr})
	if err := c.Ping(ctx).Err(); err != nil {
		return nil, err
	}
	return c, nil
}
