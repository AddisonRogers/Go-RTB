package banker

import (
	"encoding/json/v2"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/AddisonRogers/Go-RTB/shared"
	redis2 "github.com/AddisonRogers/Go-RTB/shared/redis"
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

	svc := NewBankerService(redis2.NewRedisAdapter(rdb))

	cleanup := func() {
		_ = rdb.Close()
		mr.Close()
	}

	return svc, cleanup
}

func TestHandleAuthorize(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()

	accountID := "123"
	campaignID := "camp-456"
	actualTHKey := redis2.CampaignActualThroughputKey(accountID, campaignID)
	targetTHKey := redis2.CampaignTargetThroughputKey(accountID, campaignID)
	balanceKey := redis2.CampaignBalanceKey(accountID, campaignID)

	// Seed state so authorize can succeed.
	_ = svc.cache.Set(t.Context(), actualTHKey, "10", 0)
	_ = svc.cache.Set(t.Context(), targetTHKey, "100", 0)
	_ = svc.cache.Set(t.Context(), balanceKey, "500", 0)

	body := `{"amount": 50}`
	req := httptest.NewRequest(http.MethodPost, "/accounts/123/authorize", strings.NewReader(body))
	req.SetPathValue("id", accountID)
	w := httptest.NewRecorder()

	svc.handleAuthorize(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, w.Code, w.Body.String())
	}

	var resp shared.AuthorizeResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.AuthorizeID == "" {
		t.Fatal("expected non-empty authorize_id")
	}

	holdKey := redis2.CampaignHoldKey(accountID, campaignID, resp.AuthorizeID)
	holdAmount, err := svc.cache.Get(t.Context(), holdKey)
	if err != nil {
		t.Fatalf("expected hold key to exist: %v", err)
	}
	if holdAmount != "50" {
		t.Fatalf("unexpected hold amount: got %q want %q", holdAmount, "50")
	}

	actualTH, err := svc.cache.Get(t.Context(), actualTHKey)
	if err != nil {
		t.Fatalf("expected actual throughput key: %v", err)
	}
	if actualTH != "60" {
		t.Fatalf("unexpected actual throughput: got %q want %q", actualTH, "60")
	}

	balance, err := svc.cache.Get(t.Context(), balanceKey)
	if err != nil {
		t.Fatalf("expected balance key: %v", err)
	}
	if balance != "450" {
		t.Fatalf("unexpected balance: got %q want %q", balance, "450")
	}
}

func TestHandleClear(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()

	accountID := "123"
	campaignKey := "camp-456"
	authID := "auth_456"
	holdKey := redis2.CampaignHoldKey(accountID, campaignKey, authID)
	balanceKey := redis2.CampaignBalanceKey(accountID, campaignKey)
	actualTHKey := redis2.CampaignActualThroughputKey(accountID, campaignKey)

	// Setup: initial hold and balance
	_ = svc.cache.Set(t.Context(), holdKey, "100", 0)
	_ = svc.cache.Set(t.Context(), balanceKey, "500", 0)

	body := fmt.Sprintf(`{"authorize_id": "%s", "final_amount": 70}`, authID)
	req := httptest.NewRequest(http.MethodPost, "/accounts/123/clear", strings.NewReader(body))
	req.SetPathValue("id", accountID)
	w := httptest.NewRecorder()

	svc.handleClear(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status OK, got %d: %s", w.Code, w.Body.String())
	}

	// Verify balance stays unchanged for the normal clear path
	val, _ := svc.cache.Get(t.Context(), balanceKey)
	if val != "500" {
		t.Errorf("expected balance 500, got %s", val)
	}

	// Verify actual throughput updated
	actualTH, err := svc.cache.Get(t.Context(), actualTHKey)
	if err != nil {
		t.Fatalf("expected actual throughput key: %v", err)
	}
	if actualTH != "70" {
		t.Errorf("expected actual throughput 70, got %s", actualTH)
	}

	// Verify hold deleted
	exists, _ := svc.cache.Exists(t.Context(), holdKey)
	if exists {
		t.Error("expected hold to be deleted")
	}
}

func TestHandleClear_HoldTooLow(t *testing.T) {
	svc, cleanup := newTestService(t)
	defer cleanup()

	accountID := "123"
	campaignKey := "camp-456"
	authID := "auth_456"
	holdKey := redis2.CampaignHoldKey(accountID, campaignKey, authID)
	balanceKey := redis2.CampaignBalanceKey(accountID, campaignKey)
	actualTHKey := redis2.CampaignActualThroughputKey(accountID, campaignKey)

	_ = svc.cache.Set(t.Context(), holdKey, "50", 0)
	_ = svc.cache.Set(t.Context(), balanceKey, "500", 0)
	_ = svc.cache.Set(t.Context(), actualTHKey, "10", 0)

	body := fmt.Sprintf(`{"authorize_id": "%s", "final_amount": 70}`, authID)
	req := httptest.NewRequest(http.MethodPost, "/accounts/123/clear", strings.NewReader(body))
	req.SetPathValue("id", accountID)
	w := httptest.NewRecorder()

	svc.handleClear(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}

	if got, want := w.Body.String(), "Hold is not high enough\n"; got != want {
		t.Fatalf("unexpected body: got %q want %q", got, want)
	}

	// Balance should be restored by the difference: 500 + (70 - 50) = 520
	val, err := svc.cache.Get(t.Context(), balanceKey)
	if err != nil {
		t.Fatalf("expected balance key: %v", err)
	}
	if val != "520" {
		t.Errorf("expected balance 520, got %s", val)
	}

	// Hold should be deleted
	exists, err := svc.cache.Exists(t.Context(), holdKey)
	if err != nil {
		t.Fatalf("expected exists check to succeed: %v", err)
	}
	if exists {
		t.Error("expected hold to be deleted")
	}

	// Actual throughput should remain unchanged in this branch
	actualTH, err := svc.cache.Get(t.Context(), actualTHKey)
	if err != nil {
		t.Fatalf("expected actual throughput key: %v", err)
	}
	if actualTH != "10" {
		t.Errorf("expected actual throughput 10, got %s", actualTH)
	}
}
