package banker

import (
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

func NewBankerService(c shared.Storer) *DependencyService {
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

	svc := NewBankerService(redisAdapter)

	mux := http.NewServeMux()

	mux.HandleFunc("POST /accounts/{id}/authorize", svc.handleAuthorize)
	mux.HandleFunc("POST /accounts/{id}/clear", svc.handleClear)
	mux.HandleFunc("/health", healthCheck)

	log.Print("Listening on :3000...")
	log.Fatal(http.ListenAndServe(":3000", mux))
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte("OK"))
	if err != nil {
		return
	}
}

// POST /accounts/{id}/authorize
func (s *DependencyService) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if id == "" {
		http.Error(w, "Account ID cannot be empty", http.StatusBadRequest)
		return
	}

	req := &shared.Authorize{}
	err := json.UnmarshalRead(r.Body, req)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	actualth, err := s.cache.Get(r.Context(), shared.CampaignActualThroughputKey(id))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	actualthInt, err := strconv.ParseInt(actualth, 10, 64)
	if err != nil {
		http.Error(w, "Failed to parse actual throughput", http.StatusInternalServerError)
		return
	}

	targetth, err := s.cache.Get(r.Context(), shared.CampaignTargetThroughputKey(id))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	targetthInt, err := strconv.ParseInt(targetth, 10, 64)
	if err != nil {
		http.Error(w, "Failed to parse target throughput", http.StatusInternalServerError)
		return
	}

	if actualthInt >= targetthInt {
		http.Error(w, "Account throughput limit reached", http.StatusTooManyRequests)
		return
	}

	if req.Amount > targetthInt-actualthInt {
		http.Error(w, "Insufficient throughput capacity", http.StatusPaymentRequired)
		return
	}

	authorizeID := uuid.NewString()

	err = s.cache.Set(r.Context(), shared.CampaignHoldKey(id, authorizeID), strconv.FormatInt(req.Amount, 10), 100)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = s.cache.IncrBy(r.Context(), shared.CampaignActualThroughputKey(id), req.Amount)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = s.cache.DecrBy(r.Context(), shared.CampaignBalanceKey(id), req.Amount)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	resp := shared.AuthorizeResponse{
		AuthorizeID: authorizeID,
	}

	b, err := json.Marshal(resp)
	if err != nil {
		return
	}

	_, err = w.Write(b)
	if err != nil {
		return
	}
}

// POST /accounts/{id}/clear
func (s *DependencyService) handleClear(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if id == "" {
		http.Error(w, "Account ID cannot be empty", http.StatusBadRequest)
		return
	}

	req := &shared.Clear{}
	err := json.UnmarshalRead(r.Body, req)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// confirm that the hold is higher than
	val, err := s.cache.Get(r.Context(), shared.CampaignHoldKey(id, req.AuthorizeId))
	if err != nil {
		return
	}

	if val == "" {
		http.Error(w, "Hold not found", http.StatusNotFound)
	}

	holdAmount, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		http.Error(w, "Invalid hold value", http.StatusInternalServerError)
		return
	}

	// This *shouldnt* happen, but just in case
	if holdAmount < req.FinalAmount {
		http.Error(w, "Hold is not high enough", http.StatusBadRequest)
		_, err = s.cache.IncrBy(r.Context(), shared.CampaignBalanceKey(id), req.FinalAmount-holdAmount)

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		err := s.cache.Delete(r.Context(), shared.CampaignHoldKey(id, req.AuthorizeId))

		// TODO Big issue here, if the delete fails, we're in a bad state
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		return
	}

	// TODO handle all erros by retrying the operation
	remaining := holdAmount - req.FinalAmount
	err = s.cache.Delete(r.Context(), shared.CampaignHoldKey(id, req.AuthorizeId))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if remaining < 0 {
		_, err = s.cache.IncrBy(r.Context(), shared.CampaignBalanceKey(id), remaining)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}

	err = s.cache.Delete(r.Context(), shared.CampaignHoldKey(id, req.AuthorizeId))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = s.cache.Set(r.Context(), shared.AccountCampaignKey(id, req.AuthorizeId), strconv.FormatInt(req.FinalAmount, 10), 10*60)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = s.cache.IncrBy(r.Context(), shared.CampaignActualThroughputKey(id), req.FinalAmount)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	member, err := json.Marshal(shared.Campaign{
		AccountID: id,
		Amount:    req.FinalAmount,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = s.cache.ZAdd(r.Context(), shared.AccountCampaignsKey(id),
		redis.Z{
			Score:  float64(time.Now().Add(10 * time.Minute).Unix()),
			Member: member,
		}).Result()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
