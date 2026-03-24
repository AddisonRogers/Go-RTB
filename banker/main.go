package banker

import (
	"fmt"
	"log"
	"net/http"

	"encoding/json/v2"

	"github.com/AddisonRogers/Go-RTB/shared"
	"github.com/redis/go-redis/v9"
)

func main() {
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	defer rdb.Close()

	mux := http.NewServeMux()

	// Route Registrations
	mux.HandleFunc("POST /accounts/{id}/topup", handleTopUp)
	mux.HandleFunc("POST /accounts/{id}/authorize", handleAuthorize)
	mux.HandleFunc("POST /accounts/{id}/clear", handleClear)
	mux.HandleFunc("GET /accounts/{id}/balance", handleGetBalance)
	mux.HandleFunc("DELETE /accounts/{id}", handleDeleteAccount)

	log.Print("Listening on :3000...")
	log.Fatal(http.ListenAndServe(":3000", mux))
}

// POST /accounts/{id}/topup
func handleTopUp(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	req := &shared.Topup{}
	err := json.UnmarshalRead(r.Body, &req)

	fmt.Fprintf(w, "Top-up processed for account: %s\n", id)
}

// POST /accounts/{id}/authorize
func handleAuthorize(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	fmt.Fprintf(w, "Transaction authorized for account: %s\n", id)
}

// POST /accounts/{id}/clear
func handleClear(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	fmt.Fprintf(w, "Funds cleared for account: %s\n", id)
}

// GET /accounts/{id}/balance
func handleGetBalance(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	fmt.Fprintf(w, "Current balance for account %s: $0.00\n", id)
}

// DELETE /accounts/{id}
func handleDeleteAccount(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	w.WriteHeader(http.StatusNoContent)
	log.Printf("Account %s deleted", id)
}
