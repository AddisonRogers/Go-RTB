package shared

import (
	"context"
	"time"
)

type Topup struct {
	Amount          int64  `json:"amount"`
	Currency        string `json:"currency"`
	PaymentMethodId string `json:"payment_method_id"`
}

type Authorize struct {
	Amount         int64  `json:"amount"`
	Currency       string `json:"currency"`
	MerchantId     string `json:"merchant_id"`
	TransactionRef string `json:"transaction_ref"`
}

type Clear struct {
	AuthorizeId string `json:"authorize_id"`
	FinalAmount int64  `json:"final_amount"`
}

type Balance struct {
	Currency string `json:"currency"`
	Amount   int64  `json:"amount"`
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
