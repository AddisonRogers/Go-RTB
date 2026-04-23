package shared

import "database/sql"

type Database struct {
	Db DBTX
}

type DBTX interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

type ExchangeRequest struct {
	Tags        []string
	BlockedTags []string
	Site        string
	User        User
	TimeMax     int64
}

type FinishExchangeRequest struct {
	AccountID  string
	campaignID string
	Site       string
}
