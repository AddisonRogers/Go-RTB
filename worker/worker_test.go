package main

import (
	"context"
	"encoding/json/v2"
	"testing"
	"time"

	"github.com/AddisonRogers/Go-RTB/shared"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestService(t *testing.T) (*DependencyService, *redis.Client, *miniredis.Miniredis, func()) {
	t.Helper()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	svc := NewWorkerService(shared.NewRedisAdapter(rdb))

	cleanup := func() {
		_ = rdb.Close()
		mr.Close()
	}

	return svc, rdb, mr, cleanup
}

func TestPollDelayedJobsProcessesExpiredJob(t *testing.T) {
	svc, _, mr, cleanup := newTestService(t)
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	job := shared.Campaign{
		AccountID: "acct-123",
		Amount:    25,
	}

	jobBytes, err := json.Marshal(job)
	if err != nil {
		t.Fatalf("failed to marshal job: %v", err)
	}

	now := time.Now().Unix()
	_, err = svc.cache.ZAdd(ctx, shared.DelayedJobsKey, redis.Z{
		Score:  float64(now - 1),
		Member: string(jobBytes),
	}).Result()

	if err != nil {
		t.Fatalf("failed to seed delayed job: %v", err)
	}

	if err := svc.cache.Set(ctx, shared.CampaignActualThroughputKey(job.AccountID), "100", 0); err != nil {
		t.Fatalf("failed to seed actual throughput: %v", err)
	}

	done := make(chan struct{})
	go func() {
		svc.PollDelayedJobs(ctx)
		close(done)
	}()

	deadline := time.After(3 * time.Second)
	for {
		val, err := svc.cache.Get(ctx, shared.CampaignActualThroughputKey(job.AccountID))
		if err == nil && val == "75" {
			break
		}

		select {
		case <-deadline:
			t.Fatalf("timed out waiting for delayed job to be processed; redis=%s", mr.Addr())
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}

	exists, err := svc.cache.Exists(ctx, shared.DelayedJobsKey)
	if err != nil {
		t.Fatalf("failed to check delayed_jobs existence: %v", err)
	}
	if exists {
		members, err := svc.cache.ZRangeArgs(ctx, redis.ZRangeArgs{
			Key:     shared.DelayedJobsKey,
			Start:   "-inf",
			Stop:    "+inf",
			ByScore: true,
		})
		if err != nil {
			t.Fatalf("failed to read delayed jobs: %v", err)
		}
		if len(members) != 0 {
			t.Fatalf("expected delayed_jobs to be empty, got %v", members)
		}
	}

	cancel()
	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("poller did not stop after context cancellation")
	}
}

func TestPollDelayedJobsIgnoresFutureJob(t *testing.T) {
	svc, _, _, cleanup := newTestService(t)
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	job := shared.Campaign{
		AccountID: "acct-456",
		Amount:    10,
	}
	jobBytes, err := json.Marshal(job)
	if err != nil {
		t.Fatalf("failed to marshal job: %v", err)
	}

	futureScore := float64(time.Now().Add(2 * time.Second).Unix())
	if _, err := svc.cache.ZAdd(ctx, shared.DelayedJobsKey, redis.Z{
		Score:  futureScore,
		Member: string(jobBytes),
	}).Result(); err != nil {
		t.Fatalf("failed to seed delayed job: %v", err)
	}

	if err := svc.cache.Set(ctx, shared.CampaignActualThroughputKey(job.AccountID), "100", 0); err != nil {
		t.Fatalf("failed to seed actual throughput: %v", err)
	}

	done := make(chan struct{})
	go func() {
		svc.PollDelayedJobs(ctx)
		close(done)
	}()

	time.Sleep(1200 * time.Millisecond)

	val, err := svc.cache.Get(ctx, shared.CampaignActualThroughputKey(job.AccountID))
	if err != nil {
		t.Fatalf("failed to read actual throughput: %v", err)
	}
	if val != "100" {
		t.Fatalf("expected throughput to stay at 100, got %q", val)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("poller did not stop after context cancellation")
	}
}
