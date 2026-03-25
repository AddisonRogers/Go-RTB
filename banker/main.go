package banker

import (
	"fmt"
	"log"
	"net/http"

	"encoding/json/v2"

	"github.com/AddisonRogers/Go-RTB/shared"
	"github.com/redis/go-redis/v9"
)

type DependencyService struct {
	cache Cacher
}

func NewBankerService(c Cacher) *DependencyService {
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

	_, err = s.cache.IncrBy(r.Context(), id, req.Amount)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "Top-up processed for account: %s\n", id)
}

// POST /accounts/{id}/authorize
func (s *DependencyService) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	fmt.Fprintf(w, "Transaction authorized for account: %s\n", id)
}

// POST /accounts/{id}/clear
func (s *DependencyService) handleClear(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	fmt.Fprintf(w, "Funds cleared for account: %s\n", id)
}

// GET /accounts/{id}/balance
func (s *DependencyService) handleGetBalance(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	fmt.Fprintf(w, "Current balance for account %s: $0.00\n", id)
}

// DELETE /accounts/{id}
func (s *DependencyService) handleDeleteAccount(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	w.WriteHeader(http.StatusNoContent)
	log.Printf("Account %s deleted", id)
}
