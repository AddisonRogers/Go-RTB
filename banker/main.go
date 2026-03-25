package banker

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

func NewBankerService(c shared.Storer) *DependencyService {
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

	redisAdapter := NewRedisAdapter(rdb)

	// Inject the adapter (which implements Cacher) into our service
	svc := NewBankerService(redisAdapter)

	mux := http.NewServeMux()

	// Route Registrations
	mux.HandleFunc("POST /accounts/{id}/topup", svc.handleTopUp)
	mux.HandleFunc("POST /accounts/{id}/authorize", svc.handleAuthorize)
	mux.HandleFunc("POST /accounts/{id}/clear", svc.handleClear)
	mux.HandleFunc("GET /accounts/{id}/balance", svc.handleGetBalance)
	mux.HandleFunc("DELETE /accounts/{id}", svc.handleDeleteAccount)

	log.Print("Listening on :3000...")
	log.Fatal(http.ListenAndServe(":3000", mux))
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

	key := fmt.Sprintf("%s:balance", id)
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
	Throughput := req.Amount / CountOfTenMins
	err = s.cache.Set(r.Context(), fmt.Sprintf("%s:throughput", id), strconv.FormatInt(Throughput, 10), 10*60)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "Top-up processed for account: %s\n", id)
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

	// check time since previous and calculate the campaign / second

	// move fund into hold

	fmt.Fprintf(w, "Transaction authorized for account: %s\n", id)
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

	err = s.cache.Set(r.Context(), fmt.Sprintf("%s:balance", id), strconv.FormatInt(req.Amount, 10), time.Duration(req.Length))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	TenMins := int64((10 * time.Minute).Seconds())
	CountOfTenMins := (req.Length / TenMins) / 10
	Throughput := req.Amount / CountOfTenMins
	err = s.cache.Set(r.Context(), fmt.Sprintf("%s:throughput", id), strconv.FormatInt(Throughput, 10), time.Duration(req.Length))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
	val, err := s.cache.Get(r.Context(), fmt.Sprintf("%s:%s:hold", id, req.AuthorizeId))
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
		_, err = s.cache.IncrBy(r.Context(), fmt.Sprintf("%s:balance", id), req.FinalAmount-holdAmount)

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		err := s.cache.Delete(r.Context(), fmt.Sprintf("%s:%s:hold", id, req.AuthorizeId))

		// TODO Big issue here, if the delete fails, we're in a bad state
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		return
	}

	// TODO handle all erros by retrying the operation
	remaining, err := s.cache.DecrBy(r.Context(), fmt.Sprintf("%s:%s:hold", id, req.AuthorizeId), req.FinalAmount)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = s.cache.IncrBy(r.Context(), fmt.Sprintf("%s:balance", id), remaining)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = s.cache.Delete(r.Context(), fmt.Sprintf("%s:%s:hold", id, req.AuthorizeId))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = s.cache.Set(r.Context(), fmt.Sprintf("%s:%s:campaign", id, req.AuthorizeId), strconv.FormatInt(req.FinalAmount, 10), 10*60)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// GET /accounts/{id}/balance
func (s *DependencyService) handleGetBalance(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if id == "" {
		http.Error(w, "Account ID cannot be empty", http.StatusBadRequest)
		return
	}

	balance, err := s.cache.Get(r.Context(), fmt.Sprintf("%s:balance", id))

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

// TODO hhhhhh autho
// DELETE /accounts/{id}
func (s *DependencyService) handleDeleteAccount(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	w.WriteHeader(http.StatusNoContent)
	log.Printf("Account %s deleted", id)
}
