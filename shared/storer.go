package shared

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type Storer interface {
	Set(ctx context.Context, key, value string, ttl time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	Delete(ctx context.Context, key string) error
	IncrBy(ctx context.Context, key string, value int64) (int64, error)
	DecrBy(ctx context.Context, key string, value int64) (int64, error)
	Exists(ctx context.Context, key string) (bool, error)
	Incr(ctx context.Context, key string) (int64, error)
	Decr(ctx context.Context, key string) (int64, error)
	TTL(ctx context.Context, key string) (time.Duration, error)
	FindKeysByValue(ctx context.Context, target string) ([]string, error)
	ZRangeArgs(ctx context.Context, args redis.ZRangeArgs) ([]string, error)
	ZRem(ctx context.Context, key string, members ...interface{}) *redis.IntCmd
	ZAdd(ctx context.Context, key string, members ...redis.Z) *redis.IntCmd
	Close() error
	FTSearch(ctx context.Context, index string, query string) ([]redis.Document, error)
	Do(ctx context.Context, cmd string, args ...interface{}) (reply interface{}, err error)
	HSet(ctx context.Context, key string, value interface{}) (int64, error)
	HGet(ctx context.Context, key string, field string) (string, error)
	HGetAll(ctx context.Context, key string) (map[string]string, error)
	FindAllHashes(ctx context.Context, partialKey string) ([]string, error)
}
