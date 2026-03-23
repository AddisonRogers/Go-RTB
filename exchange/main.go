package exchange

import (
	"encoding/json/v2"
	"log"
	"net/http"

	"github.com/bsm/openrtb/v3"
)

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /bid/request", handle)

	log.Print("Listening...")
	http.ListenAndServe(":3000", mux)
}

/*
Validates the request.

Enriches data (e.g., looks up User ID in a mock KV store).

The Fan-Out: Concurrently calls multiple Bidders via Protobuf or JSON.

The Auction: Aggregates responses, picks the highest bid_price, and handles "No Bids."

Returns: A 200 OK with the winning Bid response or a 204 No Content if no one bid high enough.
*/

func handle(w http.ResponseWriter, r *http.Request) {
	var req *openrtb.BidRequest
	err := json.UnmarshalRead(r.Body, &req)
	if err != nil {
		// handle this correctly
	}

	// validate

	// enrich

	// fan-out

	// auction

}
