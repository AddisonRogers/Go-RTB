package shared

type Topup struct {
	amount            int64
	currency          string
	payment_method_id string
}

type Authorize struct {
	amount          int64
	currency        string
	merchant_id     string
	transaction_ref string
}

type Clear struct {
	authorize_id string
	final_amount int64
}
