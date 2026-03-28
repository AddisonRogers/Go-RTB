package shared

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type Topup struct {
	Amount          int64  `json:"amount"`
	Currency        string `json:"currency"`
	PaymentMethodId string `json:"payment_method_id"`
	TTLExtension    int64  `json:"ttl_extension"`
}

type Authorize struct {
	Amount int64 `json:"amount"`
}

type Clear struct {
	AuthorizeId string `json:"authorize_id"`
	FinalAmount int64  `json:"final_amount"`
}

type Balance struct {
	Currency string `json:"currency"`
	Amount   int64  `json:"amount"`
}

type CreateAccount struct {
	Amount int64 `json:"amount"`
	Length int64 `json:"length"`
	// AmountThroughput int    `json:"amount_throughput"`
}

type Campaign struct {
	AccountID string `json:"account_id"`
	Amount    int64  `json:"amount"`
}

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
}
