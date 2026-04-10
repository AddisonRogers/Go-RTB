package exchange

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	redis2 "github.com/AddisonRogers/Go-RTB/shared/redis"
	"github.com/bsm/openrtb/v3"
	"github.com/redis/go-redis/v9"
)

type DependencyService struct {
	cache redis2.Storer
}

func NewExchangeService(c redis2.Storer) *DependencyService {
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

	redisAdapter := redis2.NewRedisAdapter(rdb)

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

	desiredTags := convertToString(req.Site.Categories)
	blockedTags := convertToString(req.BlockedCategories)
	// TODO validate

	// TODO enrich
	// TODO Check the user against the bidreq to see how valuable they are

	// Syntax: @tags:{tag_name}
	query := fmt.Sprintf("@tags:{%s} -@tags:{%s}", desiredTags, blockedTags)

	res, err := s.cache.FTSearch(ctx, "idx:campaigns", query)
	if err != nil {
		log.Fatal(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var wg sync.WaitGroup
	results := make(chan string, len(res))

	// TODO something something loadbalancing across nodes and using that url

	client := &http.Client{}

	for _, item := range res {
		wg.Add(1)

		go func(item redis.Document) {
			defer wg.Done()

			// TODO fix and deserialize as I finish bibder
			req, err := http.NewRequestWithContext(ctx, "GET", item.ID, nil)

			if err != nil {
				results <- fmt.Sprintf("Error [%s]: %v", item.ID, err)
				return
			}

			resp, err := client.Do(req)
			if err != nil {
				results <- fmt.Sprintf("Error [%s]: %v", item.ID, err)
				return
			}

			results <- fmt.Sprintf("Success [%s]: %s", item.ID, resp.Status)

			defer resp.Body.Close()
		}(item)
	}

	fmt.Printf("Search Results: %v\n", res)
	// auction

}

func convertToString(categories []openrtb.ContentCategory) string {
	if len(categories) == 0 {
		return ""
	}

	parts := make([]string, 0, len(categories))
	for _, category := range categories {
		parts = append(parts, string(category))
	}

	return strings.Join(parts, "|")
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
