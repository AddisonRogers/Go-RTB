//package repositories
//
//import (
//	"github.com/AddisonRogers/Go-RTB/shared"
//)
//
//type DBRepository struct {
//	db shared.DBTX
//}
//
//// TODO fetch if userip has already sold ad in the last minute
//
//// TODO
//func (db DBRepository) AddNewAccount(request shared.CreateAccountRequest) {
//
//}
//
//// TODO
//func (db DBRepository) AddNewCampaign(request shared.CampaignRequest) {
//
//}
//
//func (db DBRepository) AddNewExchangeRequest(request shared.ExchangeRequest) {
//	// this takes the exchange request and adds it to the database
//	// Timestamp, UserIP, Site, State(Pending, Failed, Complete), CampaignIDWinner
//}
//
//func (db DBRepository) FinishExchangeRequest(request shared.ExchangeRequest) {
//	// Needs to edit the exchange request to reflect the exchange
//
//	// Needs to add a new record to the Website - Campaign - Count
//
//}
//
//func (db DBRepository) AddNewWebsite(request shared.WebsiteRequest) {}
