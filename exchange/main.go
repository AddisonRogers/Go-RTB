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

	sharedRedis "github.com/AddisonRogers/Go-RTB/shared/redis"
	sharedVector "github.com/AddisonRogers/Go-RTB/shared/vector"
	"github.com/bsm/openrtb/v3"
	"github.com/redis/go-redis/v9"
)

type DependencyService struct {
	cache  sharedRedis.Storer
	qdrant sharedVector.QdrantClient
}

func NewExchangeService(c sharedRedis.Storer, q sharedVector.QdrantClient) *DependencyService {
	return &DependencyService{
		cache:  c,
		qdrant: q,
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

	redisAdapter := sharedRedis.NewRedisAdapter(rdb)
	qdrantClient := sharedVector.NewQdrantClient("localhost:6333")

	svc := NewExchangeService(redisAdapter, qdrantClient)

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

	blockedTagsString := convertToString(req.BlockedCategories)
	blockedTags := make(map[string]struct{})
	for _, blocked := range strings.Split(blockedTagsString, "|") {
		blocked = strings.TrimSpace(blocked)
		if blocked == "" {
			continue
		}
		blockedTags[blocked] = struct{}{}
	}

	// TODO enrich
	// TODO Check the user against the bidreq to see how valuable they are

	embeddingStr, err := s.cache.Get(ctx, sharedRedis.WebsiteKey(req.Site.Domain))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var embedding []float32
	err = json.Unmarshal([]byte(embeddingStr), &embedding)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	candidates, err := s.qdrant.FindBestAdsForWebsite(embedding)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	candidateKeys := make([]string, len(candidates))
	for i, item := range candidates {
		accountCampaignKey := sharedRedis.AccountCampaignKey(item.Payload["accountkey"].(string), item.Payload["campaignkey"].(string))
		candidateKeys[i] = accountCampaignKey
	}

	res, err := s.cache.MGet(ctx, candidateKeys...)
	if err != nil {
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

			// filter here against blocked tags
			tags, _ := item.Fields["tags"].(string)
			if tags != "" {
				for _, t := range strings.Split(tags, "|") {
					_, found := blockedTags[strings.TrimSpace(t)]
					if found {
						return
					}
				}
			}

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
