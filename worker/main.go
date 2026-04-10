package main

import (
	"context"
	"encoding/json/v2"
	"fmt"
	_ "net/http"
	_ "strings"
	"time"

	"github.com/AddisonRogers/Go-RTB/shared"
	redis2 "github.com/AddisonRogers/Go-RTB/shared/redis"
	"github.com/redis/go-redis/v9"
)

type DependencyService struct {
	cache redis2.Storer
}

func NewWorkerService(c redis2.Storer) *DependencyService {
	return &DependencyService{
		cache: c,
	}
}

func main() {
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	defer func(rdb *redis.Client) {
		err := rdb.Close()
		if err != nil {
			fmt.Println("Error closing redis client", err)
		}
	}(rdb)

	ctx := context.Background()

	redisAdapter := redis2.NewRedisAdapter(rdb)

	svc := NewWorkerService(redisAdapter)

	svc.PollHistories(ctx)
}

// TODO implement a crawl and tag system to validate their tags

func (s *DependencyService) PollHistories(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:

			now := time.Now().Unix()

			jobs, err := s.cache.ZRangeArgs(ctx, redis.ZRangeArgs{
				Key:     redis2.BadHistoryKey(),
				Start:   "-inf",
				Stop:    now,
				ByScore: true,
				Offset:  0,
				Count:   10,
			})

			if err != nil || len(jobs) == 0 {
				continue
			}

			for _, jobID := range jobs {
				removed, _ := s.cache.ZRem(ctx, redis2.BadHistoryKey(), jobID).Result()
				if removed > 0 {
					job := &shared.CampaignAdRecord{}
					err := json.Unmarshal([]byte(jobID), job)
					if err != nil {
						fmt.Println("Error unmarshalling job", err)
						continue
					}

					_, err = s.cache.DecrBy(ctx, redis2.CampaignActualThroughputKey(job.AccountID, job.CampaignID), job.Amount)
					if err != nil {
						fmt.Println("Error decreasing actual throughput", err)
						continue
					}
				}
			}
		}
	}
}
