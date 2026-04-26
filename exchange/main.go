package exchange

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/AddisonRogers/Go-RTB/shared"
	sharedRedis "github.com/AddisonRogers/Go-RTB/shared/redis"
	sharedVector "github.com/AddisonRogers/Go-RTB/shared/vector"
	"github.com/google/uuid"
	"github.com/qdrant/go-client/qdrant"
	"github.com/redis/go-redis/v9"
)

type DependencyService struct {
	cache      sharedRedis.Storer
	qdrant     sharedVector.QdrantClient
	httpClient http.Client
	bidderURL  string
}

func NewExchangeService(c sharedRedis.Storer, q sharedVector.QdrantClient, h http.Client, bidderURL string) *DependencyService {
	return &DependencyService{
		cache:      c,
		qdrant:     q,
		httpClient: h,
		bidderURL:  bidderURL,
	}
}

func main() {
	bidderEnv := os.Getenv("BIDDER_ENV")
	bidderURL, err := url.ParseRequestURI(bidderEnv)
	if err != nil {
		fmt.Println(fmt.Sprintf("Invalid BIDDER_ENV: %v", err), http.StatusInternalServerError)
		return
	}

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

	requestURI, err := url.ParseRequestURI("http://localhost:6333")
	if err != nil {
		fmt.Println(fmt.Sprintf("Invalid URL: %v", err), http.StatusInternalServerError)
		return
	}

	qdrantClient := sharedVector.NewQdrantClient(requestURI)
	httpClient := http.Client{}

	svc := NewExchangeService(redisAdapter, *qdrantClient, httpClient, bidderURL.String())

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

func (s *DependencyService) handle(w http.ResponseWriter, r *http.Request) {
	var req *shared.ExchangeRequest
	err := json.UnmarshalRead(r.Body, &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if req == nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	ctx, cancel := bidContext(r.Context(), time.Duration(req.TimeMax)*time.Millisecond)
	defer cancel()

	// TODO DB integration to have a quick return if available
	// TODO enrich with data from cookies

	blockedTags := make(map[string]struct{})
	for _, blocked := range req.BlockedTags {
		blocked = strings.TrimSpace(blocked)
		if blocked == "" {
			continue
		}
		blockedTags[blocked] = struct{}{}
	}

	// TODO this doesnt check that the website is already embedded
	// ^ if it is not embedded then it should reject the request as it needs to call client first
	// ^^ could also make it so that the request payload requires the embedding
	embeddingStr, err := s.cache.Get(ctx, sharedRedis.WebsiteKey(req.Site))
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

	var wg sync.WaitGroup
	results := make(chan string, len(candidates))

	requestId := uuid.New().String()

	baseBody := shared.BidRequest{
		RequestId:      requestId,
		TagsOfInterest: req.Tags,
		Site:           req.Site,
		User:           req.User,
	}

	for _, item := range candidates {
		wg.Add(1)

		go func(item *qdrant.ScoredPoint) {
			defer wg.Done()

			// filter here against blocked tags
			tags := item.Payload["tags"].String()
			if tags != "" {
				for _, t := range strings.Split(tags, "|") {
					_, found := blockedTags[strings.TrimSpace(t)]
					if found {
						return
					}
				}
			}

			body := baseBody
			body.AccountId = item.Payload["accountKey"].String()
			body.CampaignId = item.Payload["campaignKey"].String()

			bodyJson, err := json.Marshal(body)
			if err != nil {
				results <- fmt.Sprintf("Error [%s]: %v", item.Id, err)
				return
			}

			resp, err := s.httpClient.Post(
				s.bidderURL+"/bid",
				fmt.Sprintf("application/json; charset=utf-8"),
				bytes.NewReader(bodyJson),
			)

			if err != nil {
				results <- fmt.Sprintf("Error [%s]: %v", item.Id, err)
				return
			}
			defer resp.Body.Close()

			results <- fmt.Sprintf("Success [%s]: %s", item.Id, resp.Status)
		}(item)
	}

	wg.Wait()
	close(results)

	out := make([]string, 0, len(results))
	for result := range results {
		out = append(out, result)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.MarshalWrite(w, out); err != nil {
		log.Printf("failed to write exchange response: %v", err)
	}
}

// This creates/shortens a context with a timeout
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
