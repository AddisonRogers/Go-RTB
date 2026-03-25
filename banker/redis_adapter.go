package banker

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisAdapter struct {
	client *redis.Client
}

func NewRedisAdapter(client *redis.Client) *RedisAdapter {
	return &RedisAdapter{
		client: client,
	}
}

// Get retrieves a value by key. Returns an error if the key does not exist.
func (r *RedisAdapter) Get(ctx context.Context, key string) (string, error) {
	return r.client.Get(ctx, key).Result()
}

// Set stores a key-value pair with an optional expiration time.
func (r *RedisAdapter) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	return r.client.Set(ctx, key, value, ttl).Err()
}

// Delete removes a key from Redis.
func (r *RedisAdapter) Delete(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}

// Exists checks if a key exists in Redis.
func (r *RedisAdapter) Exists(ctx context.Context, key string) (bool, error) {
	// Exists returns the number of keys that exist from the ones provided.
	// Since we only provide one key, it will return 1 if it exists, or 0 if it doesn't.
	count, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// Incr increments the integer value of a key by one.
func (r *RedisAdapter) Incr(ctx context.Context, key string) (int64, error) {
	return r.client.Incr(ctx, key).Result()
}

// IncrBy increments the integer value of a key by the given amount.
func (r *RedisAdapter) IncrBy(ctx context.Context, key string, value int64) (int64, error) {
	if value == 0 {
		return 0, errors.New("value cannot be zero")
	}
	return r.client.IncrBy(ctx, key, value).Result()
}

// Decr decrements the integer value of a key by one.
func (r *RedisAdapter) Decr(ctx context.Context, key string) (int64, error) {
	return r.client.Decr(ctx, key).Result()
}

// DecrBy decrements the integer value of a key by the given amount.
func (r *RedisAdapter) DecrBy(ctx context.Context, key string, value int64) (int64, error) {
	if value == 0 {
		return 0, errors.New("value cannot be zero")
	}
	return r.client.IncrBy(ctx, key, value).Result()
}

// TTL returns the remaining time to live of a key.
func (r *RedisAdapter) TTL(ctx context.Context, key string) (time.Duration, error) {
	ttl, err := r.client.TTL(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	return ttl, nil
}
