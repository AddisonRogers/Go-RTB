package banker

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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

	svc := NewBankerService(NewRedisAdapter(rdb))

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

	gotBalance, err := svc.cache.Get(req.Context(), "123")
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

func TestHandleAuthorize(t *testing.T) {
	svc := &DependencyService{}

	req := httptest.NewRequest(http.MethodPost, "/accounts/123/authorize", nil)
	req.SetPathValue("id", "123")
	w := httptest.NewRecorder()

	svc.handleAuthorize(w, req)

	if got, want := w.Body.String(), "Transaction authorized for account: 123\n"; got != want {
		t.Fatalf("unexpected body: got %q want %q", got, want)
	}
}

func TestHandleClear(t *testing.T) {
	svc := &DependencyService{}

	req := httptest.NewRequest(http.MethodPost, "/accounts/123/clear", nil)
	req.SetPathValue("id", "123")
	w := httptest.NewRecorder()

	svc.handleClear(w, req)

	if got, want := w.Body.String(), "Funds cleared for account: 123\n"; got != want {
		t.Fatalf("unexpected body: got %q want %q", got, want)
	}
}

func TestHandleGetBalance(t *testing.T) {
	svc := &DependencyService{}

	req := httptest.NewRequest(http.MethodGet, "/accounts/123/balance", nil)
	req.SetPathValue("id", "123")
	w := httptest.NewRecorder()

	svc.handleGetBalance(w, req)

	if got, want := w.Body.String(), "Current balance for account 123: $0.00\n"; got != want {
		t.Fatalf("unexpected body: got %q want %q", got, want)
	}
}

func TestHandleDeleteAccount(t *testing.T) {
	svc := &DependencyService{}

	req := httptest.NewRequest(http.MethodDelete, "/accounts/123", nil)
	req.SetPathValue("id", "123")
	w := httptest.NewRecorder()

	svc.handleDeleteAccount(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, resp.StatusCode)
	}
}
