package main

import (
	"encoding/json/v2"
	"fmt"
	"log"
	"net/http"

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

func (s *DependencyService) createCampaign(w http.ResponseWriter, r *http.Request) {

}

// this also includes topup actions
func (s *DependencyService) updateCampaign(w http.ResponseWriter, r *http.Request) {}
func (s *DependencyService) deleteCampaign(w http.ResponseWriter, r *http.Request) {}

func (s *DependencyService) getCampaigns(w http.ResponseWriter, r *http.Request) {}
func (s *DependencyService) getCampaign(w http.ResponseWriter, r *http.Request)  {}
func (s *DependencyService) getAccount(w http.ResponseWriter, r *http.Request)   {}
