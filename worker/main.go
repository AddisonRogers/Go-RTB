package worker

import (
	"context"
	"fmt"
	"net/http"

	"github.com/AddisonRogers/Go-RTB/shared"
	"github.com/redis/go-redis/v9"
)

type DependencyService struct {
	cache shared.Storer
}

func NewWorkerService(c shared.Storer) *DependencyService {
	return &DependencyService{
		cache: c,
	}
}

func main() {
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	defer func(rdb *redis.Client) {
		err := rdb.Close()
		if err != nil {
			fmt.Println("Error closing redis client", err)
		}
	}(rdb)

	ctx := context.Background()
	pubsub := rdb.PSubscribe(ctx, "__keyevent@0__:expired")
	defer func(pubsub *redis.PubSub) {
		err := pubsub.Close()
		if err != nil {
			fmt.Println("Error closing redis pubsub", err)
		}
	}(pubsub)

	redisAdapter := shared.NewRedisAdapter(rdb)

	svc := NewWorkerService(redisAdapter)

	ch := pubsub.Channel()

	for msg := range ch {
		fmt.Println("Received message:", msg.Channel, msg.Payload)
		fmt.Println("expired key:", msg.Payload)
		svc.checkThroughput(ctx, msg.Payload)
	}
}

// todo - run a loop that checks for expired keys and deletes them
func (s *DependencyService) checkThroughput(ctx context.Context, key string) {
	if key == "" {
		return
	}

	values, err := s.cache.FindKeysByValue(ctx, fmt.Sprintf("%s:campaign:*", id))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	held, err := s.cache.FindKeysByValue(ctx, fmt.Sprintf("%s:hold:*", id))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

}
