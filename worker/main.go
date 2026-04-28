package main

import (
	"context"
	"encoding/json/v2"
	"log/slog"
	_ "net/http"
	"net/url"
	"os"
	"os/signal"
	_ "strings"
	"time"

	"github.com/AddisonRogers/Go-RTB/shared"
	sharedRedis "github.com/AddisonRogers/Go-RTB/shared/redis"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
)

type DependencyService struct {
	cache sharedRedis.Storer
}

const name = "worker"

var (
	meter  = otel.Meter(name)
	logger = otelslog.NewLogger(name)

	BinnedHistories, _ = meter.Int64Counter(
		"worker.banker.histories.binned",
	)
)

func NewWorkerService(c sharedRedis.Storer) *DependencyService {
	return &DependencyService{
		cache: c,
	}
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	otelShutdown, err := shared.SetupOTelSDK(ctx, shared.OTelConfig{
		ServiceName: name,
	})
	if err != nil {
		logger.ErrorContext(ctx, "failed to set up OpenTelemetry", slog.Any("error", err))
		os.Exit(1)
	}
	defer func() {
		if shutdownErr := otelShutdown(context.Background()); shutdownErr != nil {
			logger.ErrorContext(context.Background(), "failed to shut down OpenTelemetry", slog.Any("error", shutdownErr))
		}
	}()

	redisURLEnv := os.Getenv("REDIS_URL")
	redisURL, err := url.ParseRequestURI(redisURLEnv)
	if err != nil {
		logger.ErrorContext(ctx, "failed to parse REDIS_URL", slog.Any("error", err))
		return
	}

	redisPasswordEnv := os.Getenv("REDIS_PASSWORD")
	if redisPasswordEnv == "" {
		logger.ErrorContext(ctx, "REDIS_PASSWORD environment variable is not set")
		return
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     redisURL.String(),
		Password: redisPasswordEnv,
		DB:       0,
	})
	defer func(rdb *redis.Client) {
		err := rdb.Close()
		if err != nil {
			logger.ErrorContext(ctx, "failed to close redis client", slog.Any("error", err))
		}
	}(rdb)

	redisAdapter := sharedRedis.NewRedisAdapter(rdb)

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
				Key:     sharedRedis.BadHistoryKey(),
				Start:   "-inf",
				Stop:    now,
				ByScore: true,
				Offset:  0,
				Count:   10,
			})

			if err != nil || len(jobs) == 0 {
				continue
			}

			BinnedHistories.Add(ctx, int64(len(jobs)))

			for _, jobID := range jobs {
				removed, _ := s.cache.ZRem(ctx, sharedRedis.BadHistoryKey(), jobID).Result()
				if removed > 0 {
					job := &shared.CampaignAdRecord{}
					err := json.Unmarshal([]byte(jobID), job)
					if err != nil {
						logger.ErrorContext(ctx, "failed to unmarshal job", slog.Any("error", err))
						continue
					}

					_, err = s.cache.DecrBy(ctx, sharedRedis.CampaignActualThroughputKey(job.AccountID, job.CampaignID), job.Amount)
					if err != nil {
						logger.ErrorContext(ctx, "failed to decrease actual throughput", slog.Any("error", err))
						continue
					}
				}
			}
		}
	}
}
