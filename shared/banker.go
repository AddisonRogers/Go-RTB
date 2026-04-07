package shared

type Topup struct {
	Amount          int64   `json:"amount"`
	Currency        string  `json:"currency"`
	PaymentMethodId string  `json:"payment_method_id"`
	TTLExtension    float64 `json:"ttl_extension"`
}

type Authorize struct {
	Amount int64 `json:"amount"`
}

type AuthorizeResponse struct {
	AuthorizeID string `json:"authorize_id"`
}

type Clear struct {
	AuthorizeId string `json:"authorize_id"`
	FinalAmount int64  `json:"final_amount"`
}

type Balance struct {
	Currency string `json:"currency"`
	Amount   int64  `json:"amount"`
}

type Campaign struct {
	Name   string   `json:"name"`
	Amount int64    `json:"amount"`
	Length int64    `json:"length"`
	Tags   []string `json:"tags"`
}

type CampaignAdRecord struct {
	AccountID  string `json:"account_id"`
	CampaignID string `json:"campaign_id"`
	Amount     int64  `json:"amount"`
}
