package exchange

import (
	"testing"

	sharedRedis "github.com/AddisonRogers/Go-RTB/shared/redis"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	qdrantTesting "github.com/testcontainers/testcontainers-go/modules/qdrant"
)

func newTestService(t *testing.T) (*DependencyService, func()) {
	t.Helper()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	qdrant, err := qdrantTesting.Run()
	if err != nil {
		t.Fatalf("failed to start qdrant: %v", err)
	}

	svc := NewExchangeService(sharedRedis.NewRedisAdapter(rdb))

	cleanup := func() {
		_ = rdb.Close()
		mr.Close()
	}

	return svc, cleanup
}
