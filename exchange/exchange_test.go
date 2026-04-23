package exchange

import (
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	sharedRedis "github.com/AddisonRogers/Go-RTB/shared/redis"
	sharedVector "github.com/AddisonRogers/Go-RTB/shared/vector"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	qdrantTesting "github.com/testcontainers/testcontainers-go/modules/qdrant"
)

func newTestService(t *testing.T) (*DependencyService, func()) {
	t.Helper()
	ctx := context.Background()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	qdrantContainer, err := qdrantTesting.Run(ctx, "qdrant/qdrant:latest")
	if err != nil {
		log.Printf("failed to start container: %s", err)
		return nil, nil
	}

	_, err = qdrantContainer.State(ctx)
	if err != nil {
		log.Printf("failed to get container state: %s", err)
		return nil, nil
	}

	qdrantEndpoint, err := qdrantContainer.RESTEndpoint(ctx)
	if err != nil {
		log.Printf("failed to get qdrant endpoint: %s", err)
		return nil, nil
	}

	qdrantClient := sharedVector.NewQdrantClient(qdrantEndpoint)
	redisClient := sharedRedis.NewRedisAdapter(rdb)

	// TODO fix the return
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	httpClient := &http.Client{}
	svc := NewExchangeService(redisClient, *qdrantClient, *httpClient, server.URL)

	cleanup := func() {
		if err := testcontainers.TerminateContainer(qdrantContainer); err != nil {
			log.Printf("failed to terminate container: %s", err)
		}
		_ = rdb.Close()
		mr.Close()
	}

	return svc, cleanup
}

func DummyDataSetup(t *testing.T, svc *DependencyService) {

}

func Test(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()

	DummyDataSetup(t, svc)

}
