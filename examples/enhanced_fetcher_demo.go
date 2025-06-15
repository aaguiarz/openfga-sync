package main

import (
	"fmt"
	"time"

	"github.com/aaguiarz/openfga-sync/fetcher"
	"github.com/sirupsen/logrus"
)

func main() {
	// Create a logger
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	fmt.Println("OpenFGA Enhanced Fetcher Demo")
	fmt.Println("=============================")

	// Demo 1: Basic fetcher creation with default options
	fmt.Println("\n1. Creating basic fetcher with default options:")
	basicFetcher, err := fetcher.NewOpenFGAFetcher(
		"http://localhost:8080", // API URL
		"test-store-id",         // Store ID
		"",                      // No token for demo
		logger,
	)
	if err != nil {
		fmt.Printf("Error creating basic fetcher: %v\n", err)
	} else {
		fmt.Println("✓ Basic fetcher created successfully")
		stats := basicFetcher.GetStats()
		fmt.Printf("  Initial stats: TotalRequests=%d, SuccessRequests=%d\n", stats.TotalRequests, stats.SuccessRequests)
		basicFetcher.Close()
	}

	// Demo 2: Advanced fetcher with custom options
	fmt.Println("\n2. Creating advanced fetcher with custom options:")

	customOptions := fetcher.FetchOptions{
		PageSize:         50,                     // Smaller page size
		MaxChanges:       1000,                   // Limit to 1000 changes
		Timeout:          60 * time.Second,       // 1 minute timeout
		RateLimitDelay:   100 * time.Millisecond, // Rate limiting
		EnableValidation: true,                   // Enable validation
		RetryConfig: fetcher.RetryConfig{
			MaxRetries:    5,                      // More retries
			InitialDelay:  200 * time.Millisecond, // Longer initial delay
			MaxDelay:      10 * time.Second,       // Higher max delay
			BackoffFactor: 1.5,                    // Gentler backoff
		},
	}

	advancedFetcher, err := fetcher.NewOpenFGAFetcherWithOptions(
		"http://localhost:8080",
		"test-store-id",
		"",
		logger,
		customOptions,
	)
	if err != nil {
		fmt.Printf("Error creating advanced fetcher: %v\n", err)
	} else {
		fmt.Println("✓ Advanced fetcher created successfully")
		fmt.Printf("  Options: PageSize=%d, MaxChanges=%d, Timeout=%v\n",
			customOptions.PageSize, customOptions.MaxChanges, customOptions.Timeout)
		fmt.Printf("  Retry: MaxRetries=%d, InitialDelay=%v, BackoffFactor=%.1f\n",
			customOptions.RetryConfig.MaxRetries, customOptions.RetryConfig.InitialDelay, customOptions.RetryConfig.BackoffFactor)
		advancedFetcher.Close()
	}

	// Demo 3: Configuration examples
	fmt.Println("\n3. Configuration examples:")

	defaultConfig := fetcher.DefaultFetchOptions()
	fmt.Printf("  Default FetchOptions:\n")
	fmt.Printf("    PageSize: %d\n", defaultConfig.PageSize)
	fmt.Printf("    MaxChanges: %d\n", defaultConfig.MaxChanges)
	fmt.Printf("    Timeout: %v\n", defaultConfig.Timeout)
	fmt.Printf("    EnableValidation: %t\n", defaultConfig.EnableValidation)
	fmt.Printf("    RateLimitDelay: %v\n", defaultConfig.RateLimitDelay)

	defaultRetryConfig := fetcher.DefaultRetryConfig()
	fmt.Printf("\n  Default RetryConfig:\n")
	fmt.Printf("    MaxRetries: %d\n", defaultRetryConfig.MaxRetries)
	fmt.Printf("    InitialDelay: %v\n", defaultRetryConfig.InitialDelay)
	fmt.Printf("    MaxDelay: %v\n", defaultRetryConfig.MaxDelay)
	fmt.Printf("    BackoffFactor: %.1f\n", defaultRetryConfig.BackoffFactor)

	fmt.Println("\n✓ Demo completed successfully!")
	fmt.Println("\nKey Features Demonstrated:")
	fmt.Println("- Enhanced fetcher with customizable options")
	fmt.Println("- Retry logic with exponential backoff")
	fmt.Println("- Rate limiting and timeout support")
	fmt.Println("- Statistics tracking")
	fmt.Println("- Change event validation")
	fmt.Println("- Resource cleanup")
}
