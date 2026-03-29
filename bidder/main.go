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

func NewBidderService(c shared.Storer) *DependencyService {
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
	svc := NewBidderService(redisAdapter)

	mux := http.NewServeMux()

	// Route Registrations
	mux.HandleFunc("/health", healthCheck)
	mux.HandleFunc("POST /bid", svc.handleBid)

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

func (s *DependencyService) handleBid(w http.ResponseWriter, r *http.Request) {
	req := &shared.BidRequest{}
	err := json.UnmarshalRead(r.Body, req)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	/*
		validates request fields
		matches against campaign targeting
		computes bid price
		checks budget availability with Banker
		returns bid response if eligible
		Typical response
		200 OK with bid JSON if bidding
		204 No Content or 200 with no-bid payload if not bidding
	*/

	/*
		The bidder should:
		Receive a bid request from the Exchange
		Check whether the request matches any active campaign rules
		Compute a bid price
		Ask the Budget/Banker service whether it can spend that amount
		Return either:
		a valid bid response, or
		a no-bid / empty response
	*/
}
