//go:build debug

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
			Operation: "WRITE",
			TupleKey: fetcher.TupleKey{
				ObjectType: "document",
				ObjectID:   "readme",
				Relation:   "viewer",
				UserType:   "user",
				UserID:     "alice",
			},
			Timestamp: time.Now(),
		},
		{
			Operation: "WRITE",
			TupleKey: fetcher.TupleKey{
				ObjectType: "document",
				ObjectID:   "readme",
				Relation:   "editor",
				UserType:   "user",
				UserID:     "bob",
			},
			Timestamp: time.Now(),
		},
	}

	fmt.Println("=== Step 1: Insert 2 tuples ===")
	err = adapter.ApplyChanges(ctx, changes)
	if err != nil {
		log.Fatalf("ApplyChanges() error = %v", err)
	}

	// Check stats after insert
	stats, err := adapter.GetStats(ctx)
	if err != nil {
		log.Fatalf("GetStats() error = %v", err)
	}
	fmt.Printf("After insert - current_tuples: %v\n", stats["current_tuples"])

	// Step 2: Delete one tuple
	deleteChanges := []fetcher.ChangeEvent{
		{
			Operation: "DELETE",
			TupleKey: fetcher.TupleKey{
				ObjectType: "document",
				ObjectID:   "readme",
				Relation:   "viewer",
				UserType:   "user",
				UserID:     "alice",
			},
			Timestamp: time.Now(),
		},
	}

	fmt.Println("\n=== Step 2: Delete alice's viewer tuple ===")
	err = adapter.ApplyChanges(ctx, deleteChanges)
	if err != nil {
		log.Fatalf("ApplyChanges() delete error = %v", err)
	}

	// Check stats after delete
	stats2, err := adapter.GetStats(ctx)
	if err != nil {
		log.Fatalf("GetStats() error = %v", err)
	}
	fmt.Printf("After delete - current_tuples: %v\n", stats2["current_tuples"])
	fmt.Printf("All stats: %+v\n", stats2)
}
