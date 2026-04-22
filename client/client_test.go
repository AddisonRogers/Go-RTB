package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sharedRedis "github.com/AddisonRogers/Go-RTB/shared/redis"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestService(t *testing.T) (*DependencyService, func()) {
	t.Helper()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	svc := NewClientService(sharedRedis.NewRedisAdapter(rdb))

	cleanup := func() {
		_ = rdb.Close()
		mr.Close()
	}

	return svc, cleanup
}

func TestHandleTopUp(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()

	body := `{
"amount":100,
"currency":"USD",
"payment_method_id":"pm_1234567890"
}`
	req := httptest.NewRequest(http.MethodPost, "/accounts/123/topup", strings.NewReader(body))
	req.SetPathValue("id", "123")
	w := httptest.NewRecorder()

	svc.handleTopUp(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	expectedBody := "Top-up processed for account: 123\n"
	if got := w.Body.String(); got != expectedBody {
		t.Fatalf("unexpected body: got %q want %q", got, expectedBody)
	}

	gotBalance, err := svc.cache.Get(req.Context(), sharedRedis.CampaignBalanceKey("123"))
	if err != nil {
		t.Fatalf("expected redis value, got error: %v", err)
	}
	if gotBalance != "100" {
		t.Fatalf("unexpected redis value: got %q want %q", gotBalance, "100")
	}
}

func TestHandleTopUp_InvalidBody(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/accounts/123/topup", strings.NewReader(`{"amount":"nope"}`))
	req.SetPathValue("id", "123")
	w := httptest.NewRecorder()

	svc.handleTopUp(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}

	expectedBody := "Invalid request body\n"
	if got := w.Body.String(); got != expectedBody {
		t.Fatalf("unexpected body: got %q want %q", got, expectedBody)
	}
}

func TestHandleGetBalance(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()

	accountID := "123"
	balanceKey := sharedRedis.CampaignBalanceKey(accountID)
	_ = svc.cache.Set(t.Context(), balanceKey, "1234", 0)

	req := httptest.NewRequest(http.MethodGet, "/accounts/123/balance", nil)
	req.SetPathValue("id", accountID)
	w := httptest.NewRecorder()

	svc.handleGetBalance(w, req)

	if got, want := w.Body.String(), "1234"; got != want {
		t.Fatalf("unexpected body: got %q want %q", got, want)
	}
}

func TestCreateCampaign(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()

	accountID := "123"
	body := `{"amount": 1000, "length": 3600}` // length in seconds
	req := httptest.NewRequest(http.MethodPost, "/accounts/123/create", strings.NewReader(body))
	req.SetPathValue("id", accountID)
	w := httptest.NewRecorder()

	svc.createCampaign(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status OK, got %d", w.Code)
	}

	// Check balance
	balance, _ := svc.cache.Get(t.Context(), sharedRedis.CampaignBalanceKey(accountID))
	if balance != "1000" {
		t.Errorf("expected balance 1000, got %s", balance)
	}

	th, _ := svc.cache.Get(t.Context(), sharedRedis.CampaignTargetThroughputKey(accountID))
	if th != "1000" {
		t.Errorf("expected targetth 1000, got %s", th)
	}
}
