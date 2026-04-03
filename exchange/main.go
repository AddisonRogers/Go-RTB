package exchange

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/AddisonRogers/Go-RTB/shared"
	"github.com/bsm/openrtb/v3"
	"github.com/redis/go-redis/v9"
)

type DependencyService struct {
	cache shared.Storer
}

func NewExchangeService(c shared.Storer) *DependencyService {
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

	svc := NewExchangeService(redisAdapter)

	mux := http.NewServeMux()

	mux.HandleFunc("/health", healthCheck)
	mux.HandleFunc("POST /bid/request", svc.handle)

	log.Print("Listening...")
	http.ListenAndServe(":3000", mux)
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte("OK"))
	if err != nil {
		return
	}
}

/*
Validates the request.

Enriches data (e.g., looks up User ID in a mock KV store).

The Fan-Out: Concurrently calls multiple Bidders via Protobuf or JSON.

The Auction: Aggregates responses, picks the highest bid_price, and handles "No Bids."

Returns: A 200 OK with the winning Bid response or a 204 No Content if no one bid high enough.
*/

func (s *DependencyService) handle(w http.ResponseWriter, r *http.Request) {
	var req *openrtb.BidRequest
	err := json.UnmarshalRead(r.Body, &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if req == nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.AuctionType != 2 {
		// We only support auction type 2
		http.Error(w, "Invalid auction type", http.StatusBadRequest)
		return
	}

	ctx, cancel := bidContext(r.Context(), time.Duration(req.TimeMax)*time.Millisecond)
	defer cancel()

	// TODO implement a crawl and tag system to validate their tags

	req.Site.Categories
	req.BlockedCategories
	// TODO validate

	// TODO enrich

	// fan-out

	// Syntax: @tags:{tag_name}
	query := "@tags:{beauty}"

	res, err := s.cache.FTSearch(ctx, "idx:campaigns", query)
	if err != nil {
		log.Fatal(err)
	}

	for _, item := range res {
		go func(item redis.Document) {
			// TODO fan-out
			// send a request to the bidders

			http.Get(fmt.Sprintf("/%s", item.ID))
		}(item)
	}

	fmt.Printf("Search Results: %v\n", res)
	// auction

}

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
