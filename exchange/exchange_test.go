package exchange

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/AddisonRogers/Go-RTB/shared"
	sharedRedis "github.com/AddisonRogers/Go-RTB/shared/redis"
	sharedVector "github.com/AddisonRogers/Go-RTB/shared/vector"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	qdrantTesting "github.com/testcontainers/testcontainers-go/modules/qdrant"
)

func newTestService(t *testing.T) (*DependencyService, func(), <-chan shared.BidRequest) {
	t.Helper()
	ctx := context.Background()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	qdrantContainer, err := qdrantTesting.Run(ctx, "qdrant/qdrant:latest")
	if err != nil {
		_ = rdb.Close()
		mr.Close()
		t.Skipf("failed to start qdrant container: %v", err)
	}

	qdrantEndpoint, err := qdrantContainer.GRPCEndpoint(ctx)
	if err != nil {
		_ = testcontainers.TerminateContainer(qdrantContainer)
		_ = rdb.Close()
		mr.Close()
		t.Fatalf("failed to get qdrant endpoint: %v", err)
	}

	fmt.Println(qdrantEndpoint)

	qdrantURL, err := url.Parse(qdrantEndpoint)
	if err != nil {
		_ = testcontainers.TerminateContainer(qdrantContainer)
		_ = rdb.Close()
		mr.Close()
		t.Fatalf("failed to parse qdrant endpoint: %v", err)
	}

	fmt.Println(qdrantURL.String())

	bidRequests := make(chan shared.BidRequest, 10)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bid" {
			http.NotFound(w, r)
			return
		}

		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req shared.BidRequest
		if err := json.UnmarshalRead(r.Body, &req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		bidRequests <- req

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"request_id":"` + req.RequestId + `","bidder_id":"test-bidder","price":1.25,"creative_id":"creative-1","ad_markup":"<div>ad</div>"}`))
	}))

	qdrantClient := sharedVector.NewQdrantClient(qdrantURL)
	redisClient := sharedRedis.NewRedisAdapter(rdb)
	httpClient := &http.Client{}

	svc := NewExchangeService(redisClient, *qdrantClient, *httpClient, server.URL)

	cleanup := func() {
		server.Close()

		if err := testcontainers.TerminateContainer(qdrantContainer); err != nil {
			log.Printf("failed to terminate container: %s", err)
		}

		_ = rdb.Close()
		mr.Close()
	}

	return svc, cleanup, bidRequests
}

func testEmbedding1024(primaryIndex int, primaryValue float32, secondaryIndex int, secondaryValue float32) []float32 {
	embedding := make([]float32, 1024)
	embedding[primaryIndex] = primaryValue
	embedding[secondaryIndex] = secondaryValue
	return embedding
}

func (s *DependencyService) DummyDataSetup(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	type adSeed struct {
		AccountID  string
		CampaignID string
		Name       string
		Tags       []string
		Website    string
		Embedding  []float32
	}

	ads := []adSeed{
		{
			AccountID:  "acct-1",
			CampaignID: "camp-1",
			Name:       "Gaming Laptop Campaign",
			Tags:       []string{"gaming", "laptop", "tech"},
			Website:    "https://example.com/gaming",
			Embedding:  testEmbedding1024(0, 0.99, 1, 0.01),
		},
		{
			AccountID:  "acct-2",
			CampaignID: "camp-2",
			Name:       "Travel Backpack Campaign",
			Tags:       []string{"travel", "backpack", "outdoors"},
			Website:    "https://example.com/travel",
			Embedding:  testEmbedding1024(1, 0.99, 0, 0.01),
		},
		{
			AccountID:  "acct-3",
			CampaignID: "camp-3",
			Name:       "Coffee Subscription Campaign",
			Tags:       []string{"coffee", "subscription", "lifestyle"},
			Website:    "https://example.com/coffee",
			Embedding:  testEmbedding1024(2, 0.99, 0, 0.01),
		},
	}

	// Fake website embedding the exchange will read from Redis
	websiteEmbedding := testEmbedding1024(0, 1.0, 1, 0.01)
	websiteJSON, err := json.Marshal(websiteEmbedding)
	if err != nil {
		t.Fatalf("failed to marshal website embedding: %v", err)
	}

	err = s.cache.Set(ctx, sharedRedis.WebsiteKey("https://publisher.test/article-1"), string(websiteJSON), -1)
	if err != nil {
		t.Fatalf("failed to store website embedding in redis: %v", err)
	}

	s.qdrant.CreateWebsiteCollection()

	err = s.qdrant.AddWebsiteVectorToQdrant("https://publisher.test/article-1", websiteEmbedding)
	if err != nil {
		t.Fatalf("failed to upsert website vector into qdrant: %v data being: %v", err, websiteEmbedding)
	}

	for _, ad := range ads {
		err = s.qdrant.AddAdVectorToQdrant(ad.AccountID, ad.CampaignID, ad.Tags, ad.Embedding)
		if err != nil {
			t.Fatalf("failed to upsert ad vector into qdrant: %v data being: %v", err, ad)
		}
	}
}

func TestExchangeHandleCallsBidderWithMatchingAds(t *testing.T) {
	svc, cleanup, bidRequests := newTestService(t)
	defer cleanup()

	svc.DummyDataSetup(t)

	exchangeReq := shared.ExchangeRequest{
		Tags:        []string{"gaming", "tech"},
		BlockedTags: []string{"coffee"},
		Site:        "https://publisher.test/article-1",
		User: shared.User{
			Id:     "user-1",
			Age:    31,
			Gender: "unknown",
			Income: 75000,
			Region: "US",
		},
		TimeMax: 1000,
	}

	body, err := json.Marshal(exchangeReq)
	if err != nil {
		t.Fatalf("failed to marshal exchange request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/bid/request", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	svc.handle(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var received []shared.BidRequest
	for {
		select {
		case bidReq := <-bidRequests:
			received = append(received, bidReq)
		default:
			goto done
		}
	}

done:
	if len(received) == 0 {
		t.Fatal("expected at least one bid request to be sent to bidder")
	}

	var sawGamingCampaign bool
	for _, bidReq := range received {
		if bidReq.RequestId == "" {
			t.Fatal("expected bid request to include request id")
		}

		if bidReq.Site != exchangeReq.Site {
			t.Fatalf("expected site %q, got %q", exchangeReq.Site, bidReq.Site)
		}

		if bidReq.User.Id != exchangeReq.User.Id {
			t.Fatalf("expected user id %q, got %q", exchangeReq.User.Id, bidReq.User.Id)
		}

		if bidReq.AccountId == "acct-3" || bidReq.CampaignId == "camp-3" {
			t.Fatalf("blocked coffee campaign should not have been sent to bidder: %+v", bidReq)
		}

		if bidReq.AccountId == "acct-1" && bidReq.CampaignId == "camp-1" {
			sawGamingCampaign = true
		}
	}

	if !sawGamingCampaign {
		t.Fatalf("expected matching gaming campaign to be sent to bidder, got %+v", received)
	}
}
