package shared

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
	return r.client.DecrBy(ctx, key, value).Result()
}

// TTL returns the remaining time to live of a key.
func (r *RedisAdapter) TTL(ctx context.Context, key string) (time.Duration, error) {
	ttl, err := r.client.TTL(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	return ttl, nil
}

// FindKeysByValue returns all keys that have the given value.
func (r *RedisAdapter) FindKeysByValue(ctx context.Context, target string) ([]string, error) {
	var (
		cursor uint64
		keys   []string
	)

	for {
		batch, nextCursor, err := r.client.Scan(ctx, cursor, "*", 1000).Result()
		if err != nil {
			return nil, err
		}

		for _, key := range batch {
			val, err := r.client.Get(ctx, key).Result()
			if errors.Is(err, redis.Nil) {
				continue
			}
			if err != nil {
				return nil, err
			}

			if val == target {
				keys = append(keys, key)
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	return keys, nil
}

// ZRangeArgs returns a range of elements from a sorted set.
func (r *RedisAdapter) ZRangeArgs(ctx context.Context, args redis.ZRangeArgs) ([]string, error) {
	return r.client.ZRangeArgs(ctx, args).Result()
}

// ZRem removes one or more members from a sorted set.
func (r *RedisAdapter) ZRem(ctx context.Context, key string, members ...interface{}) *redis.IntCmd {
	return r.client.ZRem(ctx, key, members...)
}

// ZAdd adds one or more members to a sorted set, or updates its score if it already exists.
func (r *RedisAdapter) ZAdd(ctx context.Context, key string, members ...redis.Z) *redis.IntCmd {
	return r.client.ZAdd(ctx, key, members...)
}

// Close closes the Redis client connection.
func (r *RedisAdapter) Close() error {
	return r.client.Close()
}

// TODO convert the response to something a bit more useful
// FTSearch performs a full-text search on a Redis search index.
func (r *RedisAdapter) FTSearch(ctx context.Context, index string, query string) ([]redis.Document, error) {
	results, err := r.client.FTSearch(ctx, index, query).Result()
	if err != nil {
		return nil, err
	}
	return results.Docs, nil
}

func (r *RedisAdapter) Do(ctx context.Context, cmd string, args ...interface{}) (reply interface{}, err error) {
	return r.client.Do(ctx, cmd, args).Result()
}

func (r *RedisAdapter) HSet(ctx context.Context, key string, value interface{}) (int64, error) {
	return r.client.HSet(ctx, key, value).Result()
}

func (r *RedisAdapter) HGet(ctx context.Context, key string, field string) (string, error) {
	return r.client.HGet(ctx, key, field).Result()
}

func (r *RedisAdapter) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return r.client.HGetAll(ctx, key).Result()
}

func (r *RedisAdapter) FindAllHashes(ctx context.Context, partialKey string) ([]string, error) {
	var (
		cursor uint64
		keys   []string
	)

	keys = make([]string, 0)

	for {
		batch, nextCursor, err := r.client.Scan(ctx, cursor, partialKey, 1000).Result()
		if err != nil {
			return nil, err
		}

		keys = append(keys, batch...)

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	return keys, nil
}
