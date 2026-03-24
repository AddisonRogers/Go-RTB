package shared

import (
	"context"
	"time"
)

type Topup struct {
	Amount            int64
	Currency          string
	Payment_method_id string
}

type Authorize struct {
	amount          int64
	currency        string
	merchant_id     string
	transaction_ref string
}

type Clear struct {
	authorize_id string
	final_amount int64
}

type Storer interface {
	Init(addr string) (*Storer, error)
	Close()
	Set(ctx context.Context, key, value string, ttl time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	Delete(ctx context.Context, keys ...string) error
	Exists(ctx context.Context, key string) (bool, error)
	Incr(ctx context.Context, key string) (int64, error)
	Decr(ctx context.Context, key string) (int64, error)
}
