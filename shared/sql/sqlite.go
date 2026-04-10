package sql

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
}

// Open opens a SQLite database at the given path.
func Open(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	// Enable WAL mode for better concurrency
	if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys=ON;"); err != nil {
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	return &DB{db}, nil
}

// Migrate ensures the database schema is up to date.
func (db *DB) Migrate() error {
	const schema = `
	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		
	);

	CREATE TABLE IF NOT EXISTS users_campaigns (
	    PRIMARY KEY (user_id, campaign_id),
	    
	)
	
	CREATE TABLE IF NOT EXISTS campaigns (
	    
	)
	
	
	`
	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("failed to migrate schema: %w", err)
	}
	return nil
}
