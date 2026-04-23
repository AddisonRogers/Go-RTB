//package database
//
//import (
//	"database/sql"
//	"fmt"
//
//	"github.com/AddisonRogers/Go-RTB/shared"
//	_ "modernc.org/sqlite"
//)
//
//// Open opens a SQLite database at the given path.
//func Open(path string) (*shared.Database, error) {
//	db, err := sql.Open("sqlite", path)
//	if err != nil {
//		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
//	}
//
//	// Enable WAL mode for better concurrency
//	if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
//		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
//	}
//
//	// Enable foreign keys
//	if _, err := db.Exec("PRAGMA foreign_keys=ON;"); err != nil {
//		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
//	}
//
//	return &shared.Database{Db: db}, nil
//}
//
//// TODO decide table structure
//
//// Migrate ensures the database schema is up to date.
//func Migrate(db *sql.DB) error {
//	const schema = `
//	CREATE TABLE IF NOT EXISTS users (
//		id TEXT PRIMARY KEY,
//		name TEXT NOT NULL,
//
//	);
//
//	CREATE TABLE IF NOT EXISTS users_campaigns (
//	    PRIMARY KEY (user_id, campaign_id),
//
//	)
//
//	CREATE TABLE IF NOT EXISTS campaigns (
//
//	)
//
//	CREATE TABLE IF NOT EXISTS exchange_requests ()
//
//
//	`
//	if _, err := db.Exec(schema); err != nil {
//		return fmt.Errorf("failed to migrate schema: %w", err)
//	}
//	return nil
//}
