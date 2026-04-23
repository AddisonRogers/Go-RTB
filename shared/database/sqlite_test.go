//package database
//
//import (
//	"os"
//	"testing"
//)
//
//func TestSQLiteService(t *testing.T) {
//	dbPath := "test.db"
//	defer os.Remove(dbPath)
//
//	db, err := Open(dbPath)
//	if err != nil {
//		t.Fatalf("failed to open db: %v", err)
//	}
//	defer db.Close()
//
//	if err := db.Migrate(); err != nil {
//		t.Fatalf("failed to migrate db: %v", err)
//	}
//}
