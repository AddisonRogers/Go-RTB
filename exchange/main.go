package exchange

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/AddisonRogers/Go-RTB/shared"
	sharedRedis "github.com/AddisonRogers/Go-RTB/shared/redis"
	sharedVector "github.com/AddisonRogers/Go-RTB/shared/vector"
	"github.com/google/uuid"
	"github.com/qdrant/go-client/qdrant"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type DependencyService struct {
	cache      sharedRedis.Storer
	qdrant     sharedVector.QdrantClient
	httpClient http.Client
	bidderURL  string
}

const name = "exchange"

var (
	tracer = otel.Tracer(name)
	meter  = otel.Meter(name)
	logger = otelslog.NewLogger(name)

	bidRequests, _ = meter.Int64Counter(
		"exchange.bid.requests",
	)

	bidRequestClientErrors, _ = meter.Int64Counter(
		"exchange.bid.request.client_errors",
	)

	bidRequestServerErrors, _ = meter.Int64Counter(
		"exchange.bid.request.server_errors",
	)

	candidateCount, _ = meter.Int64Histogram(
		"exchange.candidates.count",
	)

	bidderErrors, _ = meter.Int64Counter(
		"exchange.bidder.errors",
	)

	bidderRequestDuration, _ = meter.Float64Histogram(
		"exchange.bidder_request_duration_ms",
	)
)

func NewExchangeService(c sharedRedis.Storer, q qdrant.Client, h http.Client, bidderURL string) *DependencyService {
	return &DependencyService{
		cache:      c,
		qdrant:     *sharedVector.NewQdrantClient(&q), // TODO this is a temporary workaround
		httpClient: h,
		bidderURL:  bidderURL,
	}
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	otelShutdown, err := shared.SetupOTelSDK(ctx, shared.OTelConfig{
		ServiceName: "exchange",
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

	bidderEnv := os.Getenv("BIDDER_ENV")
	bidderURL, err := url.ParseRequestURI(bidderEnv)
	if err != nil {
		logger.ErrorContext(ctx, "invalid BIDDER_ENV", slog.Any("error", err), slog.String("value", bidderEnv))
		return
	}

	redisURLEnv := os.Getenv("REDIS_URL")
	redisURL, err := url.ParseRequestURI(redisURLEnv)
	if err != nil {
		logger.ErrorContext(ctx, "invalid REDIS_URL", slog.Any("error", err), slog.String("value", redisURLEnv))
		return
	}

	redisPasswordEnv := os.Getenv("REDIS_PASSWORD")
	if redisPasswordEnv == "" {
		logger.ErrorContext(ctx, "REDIS_PASSWORD is empty", slog.String("value", redisPasswordEnv))
		return
	}

	qdrantURLEnv := os.Getenv("QDRANT_URL")
	qdrantURL, err := url.ParseRequestURI(qdrantURLEnv)
	if err != nil {
		logger.ErrorContext(ctx, "invalid QDRANT_URL", slog.Any("error", err), slog.String("value", qdrantURLEnv))
		return
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     redisURL.Host,
		Password: redisPasswordEnv,
		DB:       0,
	})
	defer func(rdb *redis.Client) {
		err := rdb.Close()
		if err != nil {
			logger.ErrorContext(ctx, "failed to start redis client", slog.Any("error", err))
		}
	}(rdb)

	port := strings.Split(qdrantURL.String(), ":")[1]
	portInt, err := strconv.ParseInt(port, 10, 32)
	if err != nil {
		logger.ErrorContext(ctx, "failed to parse qdrant port", slog.Any("error", err))
		return
	}

	config := &qdrant.Config{
		Host:     qdrantURL.Host,
		Port:     int(portInt),
		PoolSize: 1,
	}

	qdrantClient, err := qdrant.NewClient(config)
	if err != nil {
		logger.ErrorContext(ctx, "failed to start qdrant client", slog.Any("error", err))
		return
	}

	redisAdapter := sharedRedis.NewRedisAdapter(rdb)
	httpClient := http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}

	svc := NewExchangeService(redisAdapter, *qdrantClient, httpClient, bidderURL.String())

	srv := &http.Server{
		Addr:         ":8080",
		BaseContext:  func(net.Listener) context.Context { return ctx },
		ReadTimeout:  time.Second,
		WriteTimeout: 10 * time.Second,
		Handler:      newHTTPHandler(svc),
	}

	srvErr := make(chan error, 1)
	go func() {
		logger.InfoContext(ctx, "starting exchange HTTP server", slog.String("addr", srv.Addr))
		srvErr <- srv.ListenAndServe()
	}()

	select {
	case err = <-srvErr:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.ErrorContext(ctx, "exchange HTTP server failed", slog.Any("error", err))
			os.Exit(1)
		}
	case <-ctx.Done():
		stop()
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err = srv.Shutdown(shutdownCtx); err != nil {
		logger.ErrorContext(ctx, "failed to gracefully shut down exchange HTTP server", slog.Any("error", err))
		os.Exit(1)
	}

	logger.InfoContext(ctx, "exchange HTTP server stopped")
}

func newHTTPHandler(svc *DependencyService) http.Handler {
	mux := http.NewServeMux()

	// Register handlers.
	mux.HandleFunc("/health", healthCheck)
	mux.HandleFunc("POST /bid/request", svc.handle)

	// Add HTTP instrumentation for the whole server.
	handler := otelhttp.NewHandler(mux, "/")
	return handler
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte("OK"))
	if err != nil {
		return
	}
}

func (s *DependencyService) handle(w http.ResponseWriter, r *http.Request) {
	var req *shared.ExchangeRequest
	err := json.UnmarshalRead(r.Body, &req)
	if err != nil {
		bidRequestClientErrors.Add(r.Context(), 1, metric.WithAttributes(
			attribute.String("bidder.url", s.bidderURL),
		))
		logger.ErrorContext(r.Context(), "failed to unmarshal request body", slog.Any("error", err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req == nil {
		bidRequestClientErrors.Add(r.Context(), 1, metric.WithAttributes(
			attribute.String("bidder.url", s.bidderURL),
		))
		logger.ErrorContext(r.Context(), "invalid request body", slog.Any("error", err))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	ctx, cancel := bidContext(r.Context(), time.Duration(req.TimeMax)*time.Millisecond)
	defer cancel()

	bidRequests.Add(ctx, 1, metric.WithAttributes(
		attribute.String("bidder.url", s.bidderURL),
	))

	logger.InfoContext(ctx,
		"received bid request",
		slog.String("site", req.Site),
		slog.Duration("time_max", time.Duration(req.TimeMax)*time.Millisecond),
		slog.String("tags", strings.Join(req.Tags, ",")),
		slog.String("blocked_tags", strings.Join(req.BlockedTags, ",")),
	)

	ctx, redisSpan := tracer.Start(ctx, "redis.get_website_embedding")
	embeddingStr, err := s.cache.Get(ctx, sharedRedis.WebsiteKey(req.Site))
	if err != nil {
		redisSpan.RecordError(err)
		redisSpan.End()
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	redisSpan.End()

	blockedTags := make(map[string]struct{})
	for _, blocked := range req.BlockedTags {
		blocked = strings.TrimSpace(blocked)
		if blocked == "" {
			continue
		}
		blockedTags[blocked] = struct{}{}
	}

	if embeddingStr == "" {
		bidRequestServerErrors.Add(ctx, 1, metric.WithAttributes(
			attribute.String("bidder.url", s.bidderURL),
		))
		logger.ErrorContext(ctx, "website embedding not found in cache", slog.String("site", req.Site))
		http.Error(w, "website embedding not found in cache", http.StatusNotFound)
		return
	}

	var embedding []float32
	err = json.Unmarshal([]byte(embeddingStr), &embedding)
	if err != nil {
		bidRequestServerErrors.Add(ctx, 1, metric.WithAttributes(
			attribute.String("bidder.url", s.bidderURL),
		))
		logger.ErrorContext(ctx, "failed to unmarshal website embedding from cache", slog.Any("error", err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ctx, qdrantSpan := tracer.Start(ctx, "qdrant.find_best_ads_for_website")
	candidates, err := s.qdrant.FindBestAdsForWebsite(embedding)
	if err != nil {
		qdrantSpan.RecordError(err)
		qdrantSpan.End()
		bidRequestServerErrors.Add(ctx, 1, metric.WithAttributes(
			attribute.String("bidder.url", s.bidderURL),
		))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	qdrantSpan.End()

	candidateCount.Record(ctx, int64(len(candidates)))
	var wg sync.WaitGroup
	results := make(chan string, len(candidates))

	requestId := uuid.New().String()

	baseBody := shared.BidRequest{
		RequestId:      requestId,
		TagsOfInterest: req.Tags,
		Site:           req.Site,
		User:           req.User,
	}

	for _, item := range candidates {
		wg.Add(1)

		go func(item *qdrant.ScoredPoint) {
			defer wg.Done()

			bidCtx, span := tracer.Start(ctx, "exchange.request_bidder")
			defer span.End()

			// filter here against blocked tags
			tags := item.Payload["tags"].String()
			if tags != "" {
				for _, t := range strings.Split(tags, "|") {
					_, found := blockedTags[strings.TrimSpace(t)]
					if found {
						return
					}
				}
			}

			body := baseBody
			body.AccountId = item.Payload["accountKey"].String()
			body.CampaignId = item.Payload["campaignKey"].String()

			bodyJson, err := json.Marshal(body)
			if err != nil {
				results <- fmt.Sprintf("Error [%s]: %v", item.Id, err)
				return
			}

			bidReq, err := http.NewRequestWithContext(
				bidCtx,
				http.MethodPost,
				s.bidderURL+"/bid",
				bytes.NewReader(bodyJson),
			)
			if err != nil {
				results <- fmt.Sprintf("Error [%s]: %v", item.Id, err)
				return
			}
			bidReq.Header.Set("Content-Type", "application/json; charset=utf-8")

			logger.InfoContext(ctx,
				"sending bidder request",
				slog.String("request_id", requestId),
				slog.String("account_id", body.AccountId),
				slog.String("campaign_id", body.CampaignId),
			)

			start := time.Now()

			resp, err := s.httpClient.Do(bidReq)

			duration := time.Since(start).Seconds()

			statusCode := 0
			if resp != nil {
				statusCode = resp.StatusCode
			}

			bidderRequestDuration.Record(ctx, duration,
				metric.WithAttributes(
					attribute.Int("http.status_code", statusCode),
					attribute.Bool("error", err != nil),
				),
			)

			if err != nil {
				bidderErrors.Add(ctx, 1, metric.WithAttributes(
					attribute.String("bidder.url", s.bidderURL),
					attribute.String("account_id", body.AccountId),
					attribute.String("campaign_id", body.CampaignId),
					attribute.Int("http.status_code", statusCode),
				))
				results <- fmt.Sprintf("Error [%s]: %v", item.Id, err)
				return
			}
			defer resp.Body.Close()

			results <- fmt.Sprintf("Success [%s]: %s", item.Id, resp.Status)
		}(item)
	}

	wg.Wait()
	close(results)

	out := make([]string, 0, len(results))
	for result := range results {
		out = append(out, result)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.MarshalWrite(w, out); err != nil {
		logger.ErrorContext(r.Context(), "failed to write exchange response", slog.Any("error", err))
		return
	}
}

// This creates/shortens a context with a timeout
func bidContext(parent context.Context, tmax time.Duration) (context.Context, context.CancelFunc) {
	deadline, present := parent.Deadline()
	if present {
		remaining := time.Until(deadline)
		if remaining < tmax {
			tmax = remaining
		}
	}

	if tmax <= 0 {
		ctx, cancel := context.WithCancel(parent)
		cancel()
		return ctx, cancel
	}

	return context.WithTimeout(parent, tmax)
}
