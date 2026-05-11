package main

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/AddisonRogers/Go-RTB/shared"
	sharedRedis "github.com/AddisonRogers/Go-RTB/shared/redis"
	sharedVector "github.com/AddisonRogers/Go-RTB/shared/vector"
	"github.com/google/uuid"
	"github.com/qdrant/go-client/qdrant"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
)

type DependencyService struct {
	cache  sharedRedis.Storer
	tei    sharedVector.TEIClient
	qdrant sharedVector.QdrantClient
}

const name = "client"

var (
	tracer = otel.Tracer(name)
	meter  = otel.Meter(name)
	logger = otelslog.NewLogger(name)

	bidderErrors, _ = meter.Int64Counter(
		"exchange.bidder.errors",
	)

	bidderRequestDuration, _ = meter.Float64Histogram(
		"exchange.bidder_request_duration_ms",
	)
)

func NewClientService(c sharedRedis.Storer, tei sharedVector.TEIClient, qdrant qdrant.Client) *DependencyService {
	return &DependencyService{
		cache:  c,
		tei:    tei,
		qdrant: *sharedVector.NewQdrantClient(&qdrant), // TODO this is a temporary workaround
	}
}

// TODO move url to env
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

	teiURLEnv := os.Getenv("TEI_URL")
	teiURL, err := url.ParseRequestURI(teiURLEnv)
	if err != nil {
		logger.ErrorContext(ctx, "invalid TEI_URL", slog.Any("error", err), slog.String("value", teiURLEnv))
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
			logger.ErrorContext(ctx, "error closing redis client", slog.Any("error", err))
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
	teiClient := sharedVector.NewTEIClient(teiURL.String())

	svc := NewClientService(redisAdapter, *teiClient, *qdrantClient)

	svc.setupIndex(context.Background())

	mux := http.NewServeMux()

	mux.HandleFunc("/health", healthCheck)
	mux.HandleFunc("POST /campaigns/{id}/topup", svc.handleTopUp)

	logger.InfoContext(ctx, "starting server", slog.String("address", ":3000"))
	err = http.ListenAndServe(":3000", mux)
	if err != nil {
		return
	}
}

// TODO auth + autho

func (s *DependencyService) setupIndex(ctx context.Context) {
	_, err := s.cache.Do(ctx, "FT.CREATE", "idx:campaigns",
		"ON", "HASH",
		"PREFIX", "1", "campaign:",
		"SCHEMA",
		"name", "TEXT",
		"tags", "TAG", // TAG type is optimized for CSV-like strings
		"budget", "NUMERIC",
	)

	if err != nil {
		logger.ErrorContext(ctx, "error creating index", slog.Any("error", err))
	}
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte("OK"))
	if err != nil {
		logger.ErrorContext(r.Context(), "error writing health check response", slog.Any("error", err))
		return
	}
}

// Put /accounts/{id}
func (s *DependencyService) createAccount(w http.ResponseWriter, r *http.Request) {
	// heavily linked with the whole auth / autho
}

// POST  /accounts/{accountKey}/campaigns/{campaignKey}/topup
func (s *DependencyService) handleTopUp(w http.ResponseWriter, r *http.Request) {
	accountKey := r.PathValue("id")
	campaignKey := r.PathValue("id")

	if accountKey == "" || campaignKey == "" {
		http.Error(w, "Account ID and Campaign ID cannot be empty", http.StatusBadRequest)
		return
	}

	req := &shared.Topup{}
	err := json.UnmarshalRead(r.Body, req)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// TODO any extra validation on the account or what have you

	key := sharedRedis.CampaignBalanceKey(accountKey, campaignKey)
	newValue, err := s.cache.IncrBy(r.Context(), key, req.Amount)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	timeRemaining, err := s.cache.TTL(r.Context(), key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	TTL := timeRemaining.Seconds()

	if req.TTLExtension > 0 {
		fmt.Println("Extending TTL")
		TTL = TTL + req.TTLExtension
		err = s.cache.Set(r.Context(), key, strconv.FormatInt(newValue, 10), time.Duration(TTL))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	TenMins := (10 * time.Minute).Seconds()
	CountOfTenMins := (TTL / TenMins) / 10
	if CountOfTenMins == 0 {
		CountOfTenMins = 1
	}

	Throughput := float64(newValue) / CountOfTenMins
	err = s.cache.Set(r.Context(), sharedRedis.CampaignTargetThroughputKey(accountKey, campaignKey), strconv.FormatInt(int64(Throughput), 10), 10*60)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// GET /accounts/{accountKey}/campaigns/{campaignKey}/balance
func (s *DependencyService) handleGetBalance(w http.ResponseWriter, r *http.Request) {
	accountKey := r.PathValue("id")
	campaignKey := r.PathValue("id")

	if accountKey == "" || campaignKey == "" {
		http.Error(w, "Account ID and Campaign ID cannot be empty", http.StatusBadRequest)
		return
	}

	balance, err := s.cache.Get(r.Context(), sharedRedis.CampaignBalanceKey(accountKey, campaignKey))

	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if balance == "" {
		http.Error(w, "Account not found", http.StatusNotFound)
		return
	}

	// TODO return a json
	_, err = w.Write([]byte(balance))
	if err != nil {
		return
	}
	return
}

// PUT /accounts/{accountKey}/campaigns
func (s *DependencyService) createCampaign(w http.ResponseWriter, r *http.Request) {
	accountKey := r.PathValue("id")
	if accountKey == "" {
		http.Error(w, "Account ID cannot be empty", http.StatusBadRequest)
		return
	}

	req := &shared.CampaignRequest{}
	err := json.UnmarshalRead(r.Body, req)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	campaignKeyUUID, _ := uuid.NewUUID()
	campaignKey := campaignKeyUUID.String()

	// TODO make a request to url to download html
	embedding, err := s.tei.GetEmbedding(r.Context(), req.Name+" "+req.Desc)

	err = s.qdrant.AddAdVectorToQdrant(accountKey, campaignKey, req.Tags, embedding)
	if err != nil {
		http.Error(w, "Failed to add campaign to qdrant", http.StatusInternalServerError)
		return
	}

	//accountCampaignKey := sharedRedis.AccountCampaignKey(accountKey, campaignKey)
	//_, err = s.cache.HSet(r.Context(), accountCampaignKey, map[string]interface{}{
	//	"name":      req.Name,
	//	"tags":      req.Tags,
	//	"embedding": embedding,
	//})

	//if err != nil {
	//	http.Error(w, err.Error(), http.StatusInternalServerError)
	//	return
	//}

	// Managing the throughput
	err = s.cache.Set(r.Context(), sharedRedis.CampaignBalanceKey(accountKey, campaignKey), strconv.FormatInt(req.Amount, 10), time.Duration(req.Length))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	TenMins := int64((10 * time.Minute).Seconds())
	CountOfTenMins := (req.Length / TenMins) / 10
	if CountOfTenMins == 0 {
		CountOfTenMins = 1
	}
	Throughput := req.Amount / CountOfTenMins
	err = s.cache.Set(r.Context(), sharedRedis.CampaignTargetThroughputKey(accountKey, campaignKey), strconv.FormatInt(Throughput, 10), time.Duration(req.Length))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = s.cache.Set(r.Context(), sharedRedis.CampaignActualThroughputKey(accountKey, campaignKey), strconv.FormatInt(0, 10), 10*60)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Println("Campaign created successfully!")

	// TODO return success
}

// PUT /websites
func (s *DependencyService) createWebsite(w http.ResponseWriter, r *http.Request) {
	// This is to be used by the webiste partners to register their website with us
	req := &shared.WebsiteRequest{}
	err := json.UnmarshalRead(r.Body, req)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	embedding, err := s.tei.GetEmbedding(r.Context(), req.Name+" "+req.Desc)
	if err != nil {
		http.Error(w, "Failed to get embedding for website", http.StatusInternalServerError)
		return
	}

	val, err := json.Marshal(embedding)
	if err != nil {
		http.Error(w, "Failed to marshal embedding", http.StatusInternalServerError)
		return
	}

	err = s.cache.Set(r.Context(), sharedRedis.WebsiteKey(req.Url), string(val), -1)
	if err != nil {
		http.Error(w, "Failed to cache website embedding", http.StatusInternalServerError)
		return
	}

	err = s.qdrant.AddWebsiteVectorToQdrant(req.Url, embedding)
	if err != nil {
		http.Error(w, "Failed to add website to qdrant", http.StatusInternalServerError)
		return
	}

	// TODO return success
}

// this also includes topup actions
//// TODO
//// PATCH /accounts/{accountKey}/campaigns/{id}
//func (s *DependencyService) updateCampaign(w http.ResponseWriter, r *http.Request) {
//	accountKey := r.PathValue("id")
//	campaignKey := r.PathValue("id")
//
//	if accountKey == "" || campaignKey == "" {
//		http.Error(w, "Account ID and Campaign ID cannot be empty", http.StatusBadRequest)
//		return
//	}
//
//	req := &shared.Campaign{}
//	err := json.UnmarshalRead(r.Body, req)
//	if err != nil {
//		http.Error(w, "Invalid request body", http.StatusBadRequest)
//	}
//
//	s.cache.HSet(r.Context(), shared.AccountCampaignKey(accountKey, campaignKey), preExistingCampaign)
//}

// TODO
// DELETE /accounts/{accountKey}/campaigns/{id}
func (s *DependencyService) deleteCampaign(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "Campaign ID cannot be empty", http.StatusBadRequest)
		return
	}
}

// GET /accounts/{accountKey}/campaigns
func (s *DependencyService) getCampaigns(w http.ResponseWriter, r *http.Request) {
	accountKey := r.PathValue("id")
	if accountKey == "" {
		http.Error(w, "Account ID cannot be empty", http.StatusBadRequest)
		return
	}

	keys, err := s.cache.FindAllHashes(r.Context(), accountKey+":campaign:*")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type campaignResponse struct {
		Key    string            `json:"key"`
		Fields map[string]string `json:"fields"`
	}

	returnJSON := make([]campaignResponse, 0, len(keys))
	for _, key := range keys {
		campaign, err := s.cache.HGetAll(r.Context(), key)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		returnJSON = append(returnJSON, campaignResponse{
			Key:    key,
			Fields: campaign,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.MarshalWrite(w, returnJSON); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// GET /accounts/{accountKey}/campaigns/{id}
func (s *DependencyService) getCampaign(w http.ResponseWriter, r *http.Request) {
	accountKey := r.PathValue("id")
	campaignKey := r.PathValue("id")

	if accountKey == "" || campaignKey == "" {
		http.Error(w, "Account ID and Campaign ID cannot be empty", http.StatusBadRequest)
		return
	}

	campaign, err := s.cache.HGetAll(r.Context(), sharedRedis.AccountCampaignKey(accountKey, campaignKey))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.MarshalWrite(w, campaign); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

//// GET /accounts/{id}
//func (s *DependencyService) getAccount(w http.ResponseWriter, r *http.Request) {
//	id := r.PathValue("id")
//	if id == "" {
//		http.Error(w, "Account ID cannot be empty", http.StatusBadRequest)
//		return
//	}
//}
//
//// GET /accounts
//func (s *DependencyService) getAccounts(w http.ResponseWriter, r *http.Request) {}
