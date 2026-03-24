package banker

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleTopUp(t *testing.T) {
	// 1. Setup
	s := &Server{rdb: nil} // In a real test, use a miniredis or a real test DB

	body := `{"amount": 100}`
	req := httptest.NewRequest("POST", "/accounts/123/topup", strings.NewReader(body))

	// Crucial: PathValue only works if you set it manually in tests
	// or use the mux to route the request.
	req.SetPathValue("id", "123")

	w := httptest.NewRecorder()

	// 2. Execute
	s.handleTopUp(w, req)

	// 3. Assertions
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	expected := "Top-up processed for account: 123\n"
	if w.Body.String() != expected {
		t.Errorf("unexpected body: %s", w.Body.String())
	}
}
