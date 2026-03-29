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
	mux.HandleFunc("PUT /accounts/{id}/topup", svc.handleTopUp)
	mux.HandleFunc("POST /accounts/{id}/authorize", svc.handleAuthorize)
	mux.HandleFunc("POST /accounts/{id}/clear", svc.handleClear)
	mux.HandleFunc("GET /accounts/{id}/balance", svc.handleGetBalance)
	mux.HandleFunc("DELETE /accounts/{id}", svc.handleDeleteAccount)
	mux.HandleFunc("POST /accounts/{id}/create", svc.createCampaign)
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
