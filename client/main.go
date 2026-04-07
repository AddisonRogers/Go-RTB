package main

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/AddisonRogers/Go-RTB/shared"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type DependencyService struct {
	cache shared.Storer
}

func NewClientService(c shared.Storer) *DependencyService {
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

	redisAdapter := shared.NewRedisAdapter(rdb)

	svc := NewClientService(redisAdapter)

	svc.setupIndex(context.Background())

	mux := http.NewServeMux()

	mux.HandleFunc("/health", healthCheck)
	mux.HandleFunc("POST /campaigns/{id}/topup", svc.handleTopUp)

	log.Print("Listening on :3000...")
	log.Fatal(http.ListenAndServe(":3000", mux))
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
		fmt.Println("Index might already exist:", err)
	}
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte("OK"))
	if err != nil {
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

	key := shared.CampaignBalanceKey(accountKey, campaignKey)
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
	err = s.cache.Set(r.Context(), shared.CampaignTargetThroughputKey(accountKey, campaignKey), strconv.FormatInt(int64(Throughput), 10), 10*60)
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

	balance, err := s.cache.Get(r.Context(), shared.CampaignBalanceKey(accountKey, campaignKey))

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

	req := &shared.Campaign{}
	err := json.UnmarshalRead(r.Body, req)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
	}

	campaignKeyUUID, _ := uuid.NewUUID()
	campaignKey := campaignKeyUUID.String()

	accountCampaignKey := shared.AccountCampaignKey(accountKey, campaignKey)
	_, err = s.cache.HSet(r.Context(), accountCampaignKey, map[string]interface{}{
		"name": req.Name,
		"tags": req.Tags,
	})

	if err != nil {
		log.Fatal(err)
	}

	// Managing the throughput
	err = s.cache.Set(r.Context(), shared.CampaignBalanceKey(accountKey, campaignKey), strconv.FormatInt(req.Amount, 10), time.Duration(req.Length))
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
	err = s.cache.Set(r.Context(), shared.CampaignTargetThroughputKey(accountKey, campaignKey), strconv.FormatInt(Throughput, 10), time.Duration(req.Length))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = s.cache.Set(r.Context(), shared.CampaignActualThroughputKey(accountKey, campaignKey), strconv.FormatInt(0, 10), 10*60)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Println("Campaign created successfully!")

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

	campaign, err := s.cache.HGetAll(r.Context(), shared.AccountCampaignKey(accountKey, campaignKey))
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
