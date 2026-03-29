package shared

type BidRequest struct {
	RequestId      string   `json:"request_id"`
	TagsOfInterest []string `json:"tags_of_interest"`
	Imp            []struct {
		Id    string `json:"id"`
		Image struct {
			W int `json:"w"`
			H int `json:"h"`
		} `json:"image"`
		Bidfloor float64 `json:"bidfloor"`
	} `json:"imp"`
}

type BidResponse struct {
	RequestId  string  `json:"request_id"`
	BidderId   string  `json:"bidder_id"`
	Price      float64 `json:"price"`
	CreativeId string  `json:"creative_id"`
	AdMarkup   string  `json:"ad_markup"`
}
