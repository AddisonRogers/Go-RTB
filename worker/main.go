package main

import (
	"context"
	"encoding/json/v2"
	"fmt"
	_ "net/http"
	_ "strings"
	"time"

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

	redisAdapter := shared.NewRedisAdapter(rdb)

	svc := NewWorkerService(redisAdapter)

	svc.PollDelayedJobs(ctx)
}

func (s *DependencyService) PollDelayedJobs(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:

			now := time.Now().Unix()

			jobs, err := s.cache.ZRangeArgs(ctx, redis.ZRangeArgs{
				Key:     "delayed_jobs",
				Start:   "-inf", // Start from the oldest possible job
				Stop:    now,    // Stop at "Right Now"
				ByScore: true,   // Crucial: treat Start/Stop as scores, not ranks
				Offset:  0,
				Count:   10,
			})

			if err != nil || len(jobs) == 0 {
				continue
			}

			for _, jobID := range jobs {
				removed, _ := s.cache.ZRem(ctx, "delayed_jobs", jobID).Result()
				if removed > 0 {
					job := &shared.Campaign{}
					err := json.Unmarshal([]byte(jobID), job)
					if err != nil {
						fmt.Println("Error unmarshalling job", err)
						continue
					}

					_, err = s.cache.DecrBy(ctx, fmt.Sprintf("%s:actualth", job.AccountID), job.Amount)
					if err != nil {
						fmt.Println("Error decreasing actual throughput", err)
						continue
					}
				}
			}
		}
	}
}
