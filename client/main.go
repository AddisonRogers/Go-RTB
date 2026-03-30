package main

import (
	"encoding/json/v2"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/AddisonRogers/Go-RTB/shared"
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
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	defer func(rdb *redis.Client) {
		err := rdb.Close()
		if err != nil {
			fmt.Println("Error closing redis client", err)
		}
	}(rdb)

	redisAdapter := shared.NewRedisAdapter(rdb)

	// Inject the adapter (which implements Cacher) into our service
	svc := NewClientService(redisAdapter)

	mux := http.NewServeMux()

	// Route Registrations
	mux.HandleFunc("/health", healthCheck)
	mux.HandleFunc("POST /accounts/{id}", svc.handleTopUp)

	log.Print("Listening on :3000...")
	log.Fatal(http.ListenAndServe(":3000", mux))
}

// TODO auth + autho

func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte("OK"))
	if err != nil {
		return
	}
}

func (s *DependencyService) createAccount(w http.ResponseWriter, r *http.Request) {
	// heavily linked with the whole auth / autho
}

// POST /accounts/{id}/topup
func (s *DependencyService) handleTopUp(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	req := &shared.Topup{}
	err := json.UnmarshalRead(r.Body, req)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// TODO any extra validation on the account or what have you

	key := shared.AccountBalanceKey(id)
	newValue, err := s.cache.IncrBy(r.Context(), key, req.Amount)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if req.TTLExtension > 0 {
		err = s.cache.Set(r.Context(), key, strconv.FormatInt(newValue, 10), time.Duration(req.TTLExtension))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	timeRemaining, err := s.cache.TTL(r.Context(), key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	TenMins := int64((10 * time.Minute).Seconds())
	TimeRemainingValue := int64(timeRemaining.Seconds())
	CountOfTenMins := (TimeRemainingValue / TenMins) / 10
	if CountOfTenMins == 0 {
		CountOfTenMins = 1
	}
	Throughput := req.Amount / CountOfTenMins
	err = s.cache.Set(r.Context(), shared.AccountThroughputKey(id), strconv.FormatInt(Throughput, 10), 10*60)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "Top-up processed for account: %s\n", id)
}

// GET /accounts/{id}/balance
func (s *DependencyService) handleGetBalance(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if id == "" {
		http.Error(w, "Account ID cannot be empty", http.StatusBadRequest)
		return
	}

	balance, err := s.cache.Get(r.Context(), shared.AccountBalanceKey(id))

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

// POST /accounts/{id}/create
func (s *DependencyService) createCampaign(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "Account ID cannot be empty", http.StatusBadRequest)
		return
	}

	req := &shared.CreateAccount{}
	err := json.UnmarshalRead(r.Body, req)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
	}

	err = s.cache.Set(r.Context(), shared.AccountBalanceKey(id), strconv.FormatInt(req.Amount, 10), time.Duration(req.Length))
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
	err = s.cache.Set(r.Context(), shared.AccountTargetThroughputKey(id), strconv.FormatInt(Throughput, 10), time.Duration(req.Length))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = s.cache.Set(r.Context(), shared.AccountActualThroughputKey(id), strconv.FormatInt(0, 10), 10*60)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// this also includes topup actions
func (s *DependencyService) updateCampaign(w http.ResponseWriter, r *http.Request) {}
func (s *DependencyService) deleteCampaign(w http.ResponseWriter, r *http.Request) {}

func (s *DependencyService) getCampaigns(w http.ResponseWriter, r *http.Request) {}
func (s *DependencyService) getCampaign(w http.ResponseWriter, r *http.Request)  {}
func (s *DependencyService) getAccount(w http.ResponseWriter, r *http.Request)   {}
