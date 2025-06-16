package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/aaguiarz/openfga-sync/config"
	"github.com/aaguiarz/openfga-sync/fetcher"
	"github.com/aaguiarz/openfga-sync/storage"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
)

func main() {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	adapter, err := storage.NewSQLiteAdapter(":memory:", config.StorageModeStateful, logger)
	if err != nil {
		log.Fatalf("Failed to create adapter: %v", err)
	}
	defer adapter.Close()

	ctx := context.Background()

	// Step 1: Insert two tuples
	changes := []fetcher.ChangeEvent{
		{
			Operation:  "WRITE",
			ObjectType: "document",
			ObjectID:   "readme",
			Relation:   "viewer",
			UserType:   "user",
			UserID:     "alice",
			Timestamp:  time.Now(),
		},
		{
			Operation:  "WRITE",
			ObjectType: "document",
			ObjectID:   "readme",
			Relation:   "editor",
			UserType:   "user",
			UserID:     "bob",
			Timestamp:  time.Now(),
		},
	}

	fmt.Println("=== Step 1: Insert 2 tuples ===")
	err = adapter.ApplyChanges(ctx, changes)
	if err != nil {
		log.Fatalf("ApplyChanges() error = %v", err)
	}

	// Query the database directly to see what's there
	sqliteAdapter := adapter.(*storage.SQLiteAdapter)
	rows, err := sqliteAdapter.DB().QueryContext(ctx, "SELECT object_type, object_id, relation, user_type, user_id FROM fga_tuples")
	if err != nil {
		log.Fatalf("Failed to query tuples: %v", err)
	}
	defer rows.Close()

	fmt.Println("Tuples in database after insert:")
	count := 0
	for rows.Next() {
		var objectType, objectID, relation, userType, userID string
		if err := rows.Scan(&objectType, &objectID, &relation, &userType, &userID); err != nil {
			log.Fatalf("Failed to scan row: %v", err)
		}
		fmt.Printf("  %s:%s#%s@%s:%s\n", objectType, objectID, relation, userType, userID)
		count++
	}
	fmt.Printf("Total count: %d\n\n", count)

	// Step 2: Delete one tuple
	deleteChanges := []fetcher.ChangeEvent{
		{
			Operation:  "DELETE",
			ObjectType: "document",
			ObjectID:   "readme",
			Relation:   "viewer",
			UserType:   "user",
			UserID:     "alice",
			Timestamp:  time.Now(),
		},
	}

	fmt.Println("=== Step 2: Delete alice's viewer tuple ===")
	err = adapter.ApplyChanges(ctx, deleteChanges)
	if err != nil {
		log.Fatalf("ApplyChanges() delete error = %v", err)
	}

	// Query again to see what's left
	rows2, err := sqliteAdapter.DB().QueryContext(ctx, "SELECT object_type, object_id, relation, user_type, user_id FROM fga_tuples")
	if err != nil {
		log.Fatalf("Failed to query tuples after delete: %v", err)
	}
	defer rows2.Close()

	fmt.Println("Tuples in database after delete:")
	count2 := 0
	for rows2.Next() {
		var objectType, objectID, relation, userType, userID string
		if err := rows2.Scan(&objectType, &objectID, &relation, &userType, &userID); err != nil {
			log.Fatalf("Failed to scan row: %v", err)
		}
		fmt.Printf("  %s:%s#%s@%s:%s\n", objectType, objectID, relation, userType, userID)
		count2++
	}
	fmt.Printf("Total count: %d\n\n", count2)

	// Step 3: Check GetStats
	fmt.Println("=== Step 3: Check GetStats ===")
	stats, err := adapter.GetStats(ctx)
	if err != nil {
		log.Fatalf("GetStats() error = %v", err)
	}

	fmt.Printf("GetStats current_tuples: %v\n", stats["current_tuples"])
	fmt.Printf("All stats: %+v\n", stats)
}
