package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aaguiarz/openfga-sync/config"
	"github.com/aaguiarz/openfga-sync/fetcher"
	"github.com/aaguiarz/openfga-sync/storage"
	"github.com/sirupsen/logrus"
)

func main() {
	// Create a simple logger
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	logger.SetFormatter(&logrus.TextFormatter{
		DisableColors: false,
		FullTimestamp: true,
	})

	// Demo SQLite adapter in both modes
	fmt.Println("🗄️  SQLite Adapter Demonstration")
	fmt.Println("================================")

	// Clean up any existing test database
	dbPath := "/tmp/openfga-sync-demo.db"
	os.Remove(dbPath)
	defer os.Remove(dbPath)

	// Test 1: Changelog Mode
	fmt.Println("\n📝 Testing Changelog Mode:")
	fmt.Println("--------------------------")

	changelogAdapter, err := storage.NewSQLiteAdapter(dbPath, config.StorageModeChangelog, logger)
	if err != nil {
		log.Fatalf("Failed to create changelog adapter: %v", err)
	}

	// Create some sample change events
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

	ctx := context.Background()

	// Write changes to changelog
	if err := changelogAdapter.WriteChanges(ctx, changes); err != nil {
		log.Fatalf("Failed to write changes: %v", err)
	}

	// Test continuation token functionality
	testToken := "changelog-token-123"
	if err := changelogAdapter.SaveContinuationToken(ctx, testToken); err != nil {
		log.Fatalf("Failed to save continuation token: %v", err)
	}

	retrievedToken, err := changelogAdapter.GetLastContinuationToken(ctx)
	if err != nil {
		log.Fatalf("Failed to get continuation token: %v", err)
	}

	fmt.Printf("✅ Stored continuation token: %s\n", retrievedToken)

	// Get changelog statistics
	stats, err := changelogAdapter.GetStats(ctx)
	if err != nil {
		log.Fatalf("Failed to get stats: %v", err)
	}

	fmt.Printf("✅ Changelog entries: %v\n", stats["changelog_entries"])
	fmt.Printf("✅ By change type: %v\n", stats["by_change_type"])

	changelogAdapter.Close()

	// Test 2: Stateful Mode
	fmt.Println("\n🔄 Testing Stateful Mode:")
	fmt.Println("-------------------------")

	// Use a different database file for stateful mode
	statefulDBPath := "/tmp/openfga-sync-stateful-demo.db"
	os.Remove(statefulDBPath)
	defer os.Remove(statefulDBPath)

	statefulAdapter, err := storage.NewSQLiteAdapter(statefulDBPath, config.StorageModeStateful, logger)
	if err != nil {
		log.Fatalf("Failed to create stateful adapter: %v", err)
	}
	defer statefulAdapter.Close()

	// Apply changes to stateful storage
	if err := statefulAdapter.ApplyChanges(ctx, changes); err != nil {
		log.Fatalf("Failed to apply changes: %v", err)
	}

	// Get stateful statistics
	statefulStats, err := statefulAdapter.GetStats(ctx)
	if err != nil {
		log.Fatalf("Failed to get stateful stats: %v", err)
	}

	fmt.Printf("✅ Current tuples: %v\n", statefulStats["current_tuples"])
	fmt.Printf("✅ By object type: %v\n", statefulStats["by_object_type"])

	// Test 3: In-memory SQLite
	fmt.Println("\n🧠 Testing In-Memory SQLite:")
	fmt.Println("----------------------------")

	memoryAdapter, err := storage.NewSQLiteAdapter(":memory:", config.StorageModeChangelog, logger)
	if err != nil {
		log.Fatalf("Failed to create memory adapter: %v", err)
	}
	defer memoryAdapter.Close()

	// Write some changes to memory database
	smallChanges := changes[:2] // Just the first two changes
	if err := memoryAdapter.WriteChanges(ctx, smallChanges); err != nil {
		log.Fatalf("Failed to write changes to memory: %v", err)
	}

	memoryStats, err := memoryAdapter.GetStats(ctx)
	if err != nil {
		log.Fatalf("Failed to get memory stats: %v", err)
	}

	fmt.Printf("✅ Memory database entries: %v\n", memoryStats["changelog_entries"])

	fmt.Println("\n🎉 SQLite Adapter Demo Complete!")
	fmt.Println("================================")
	fmt.Println("The SQLite adapter supports:")
	fmt.Println("• File-based and in-memory databases")
	fmt.Println("• Both changelog and stateful modes")
	fmt.Println("• Proper transaction handling")
	fmt.Println("• Continuation token persistence")
	fmt.Println("• Statistics and monitoring")
	fmt.Println("• WAL mode for better performance")
}
