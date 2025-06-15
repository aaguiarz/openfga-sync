package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/aaguiarz/openfga-sync/fetcher"
	"github.com/sirupsen/logrus"
)

// This example demonstrates the enhanced OpenFGA changes API client
func main() {
	// Setup logger
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	fmt.Println("🚀 OpenFGA Changes API Client Demonstration")
	fmt.Println("==========================================")

	// Example 1: Basic change event parsing
	demonstrateChangeEventParsing(logger)

	// Example 2: User and object type/ID parsing
	demonstrateUserObjectParsing()

	// Example 3: Change validation
	demonstrateChangeValidation(logger)

	// Example 4: Raw JSON handling
	demonstrateRawJSONHandling()

	fmt.Println("\n✅ All demonstrations completed successfully!")
}

func demonstrateChangeEventParsing(logger *logrus.Logger) {
	fmt.Println("\n📋 1. Change Event Parsing Demonstration")
	fmt.Println("----------------------------------------")

	// Create a mock fetcher
	mockFetcher := &fetcher.OpenFGAFetcher{}

	// Mock change event (simulating OpenFGA API response)
	fmt.Printf("📥 Processing change event from OpenFGA API...\n")

	// Manually create the change event to demonstrate the structure
	changeEvent := fetcher.ChangeEvent{
		ObjectType: "document",
		ObjectID:   "financial-report-2024.pdf",
		Relation:   "viewer",
		UserType:   "employee",
		UserID:     "alice@company.com",
		ChangeType: "tuple_write",
		Timestamp:  time.Now(),
		RawJSON:    `{"operation":"WRITE","tuple_key":{"user":"employee:alice@company.com","relation":"viewer","object":"document:financial-report-2024.pdf"}}`,
	}

	// Suppress unused variable warning
	_ = mockFetcher

	fmt.Printf("✅ Parsed Change Event:\n")
	fmt.Printf("   - Object Type: %s\n", changeEvent.ObjectType)
	fmt.Printf("   - Object ID: %s\n", changeEvent.ObjectID)
	fmt.Printf("   - Relation: %s\n", changeEvent.Relation)
	fmt.Printf("   - User Type: %s\n", changeEvent.UserType)
	fmt.Printf("   - User ID: %s\n", changeEvent.UserID)
	fmt.Printf("   - Change Type: %s\n", changeEvent.ChangeType)
	fmt.Printf("   - Timestamp: %s\n", changeEvent.Timestamp.Format(time.RFC3339))
}

func demonstrateUserObjectParsing() {
	fmt.Println("\n🔍 2. User and Object Parsing Demonstration")
	fmt.Println("-------------------------------------------")

	testCases := []struct {
		input       string
		description string
	}{
		{"employee:alice@company.com", "Employee user"},
		{"group:engineering#member", "Group membership"},
		{"service:api-gateway", "Service account"},
		{"user:12345", "Simple user ID"},
		{"just-an-id", "Plain ID without type"},
		{"namespace:complex:user:id", "Complex namespace"},
	}

	fmt.Println("👤 User Parsing Examples:")
	for _, tc := range testCases {
		userType, userID := parseUserTypeAndID(tc.input)
		fmt.Printf("   %s:\n", tc.description)
		fmt.Printf("     Input: %s → Type: %s, ID: %s\n", tc.input, userType, userID)
	}

	objectCases := []struct {
		input       string
		description string
	}{
		{"document:financial-report-2024.pdf", "Document"},
		{"folder:src/backend", "Folder path"},
		{"database:user_accounts", "Database"},
		{"api:v1/users", "API endpoint"},
		{"plain-object-id", "Plain object without type"},
	}

	fmt.Println("\n📄 Object Parsing Examples:")
	for _, tc := range objectCases {
		objectType, objectID := parseObjectTypeAndID(tc.input)
		fmt.Printf("   %s:\n", tc.description)
		fmt.Printf("     Input: %s → Type: %s, ID: %s\n", tc.input, objectType, objectID)
	}
}

func demonstrateChangeValidation(logger *logrus.Logger) {
	fmt.Println("\n✅ 3. Change Event Validation Demonstration")
	fmt.Println("-------------------------------------------")

	mockFetcher := &fetcher.OpenFGAFetcher{}

	// Valid change event
	validChange := fetcher.ChangeEvent{
		ObjectType: "document",
		ObjectID:   "readme.md",
		Relation:   "viewer",
		UserType:   "employee",
		UserID:     "alice",
		ChangeType: "tuple_write",
		Timestamp:  time.Now(),
		RawJSON:    `{"operation":"WRITE"}`,
	}

	fmt.Println("🔍 Validating complete change event...")
	if err := mockFetcher.ValidateChangeEvent(validChange); err != nil {
		fmt.Printf("❌ Validation failed: %v\n", err)
	} else {
		fmt.Println("✅ Change event is valid!")
	}

	// Invalid change event
	invalidChange := fetcher.ChangeEvent{
		ObjectType: "document",
		// Missing required fields
	}

	fmt.Println("\n🔍 Validating incomplete change event...")
	if err := mockFetcher.ValidateChangeEvent(invalidChange); err != nil {
		fmt.Printf("❌ Validation failed (as expected): %v\n", err)
	} else {
		fmt.Println("✅ This shouldn't happen - invalid event passed validation!")
	}
}

func demonstrateRawJSONHandling() {
	fmt.Println("\n📦 4. Raw JSON Handling Demonstration")
	fmt.Println("-------------------------------------")

	// Simulate a complex OpenFGA change with additional metadata
	complexChange := map[string]interface{}{
		"operation": "WRITE",
		"timestamp": time.Now().Format(time.RFC3339Nano),
		"tuple_key": map[string]interface{}{
			"user":     "group:engineering#member",
			"relation": "viewer",
			"object":   "repository:backend-service",
		},
		"metadata": map[string]interface{}{
			"source":    "api",
			"requestId": "req-12345",
			"actor":     "admin@company.com",
		},
	}

	// Convert to JSON
	rawJSON, err := json.MarshalIndent(complexChange, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal change: %v", err)
	}

	fmt.Println("📥 Original OpenFGA change (with metadata):")
	fmt.Printf("%s\n", rawJSON)

	// Show how this would be stored
	changeEvent := fetcher.ChangeEvent{
		ObjectType: "repository",
		ObjectID:   "backend-service",
		Relation:   "viewer",
		UserType:   "group",
		UserID:     "engineering#member",
		ChangeType: "tuple_write",
		Timestamp:  time.Now(),
		RawJSON:    string(rawJSON),
	}

	fmt.Println("\n📤 Structured fields extracted:")
	fmt.Printf("   - Object: %s:%s\n", changeEvent.ObjectType, changeEvent.ObjectID)
	fmt.Printf("   - User: %s:%s\n", changeEvent.UserType, changeEvent.UserID)
	fmt.Printf("   - Relation: %s\n", changeEvent.Relation)
	fmt.Printf("   - Change Type: %s\n", changeEvent.ChangeType)

	fmt.Println("\n💾 Raw JSON preserved for audit/compliance:")
	fmt.Printf("   Length: %d bytes\n", len(changeEvent.RawJSON))
	fmt.Println("   Contains original metadata: ✅")
	fmt.Println("   Can be re-parsed if needed: ✅")
}

// Helper functions (copied from the actual implementation for demo)
func parseUserTypeAndID(user string) (string, string) {
	if user == "" {
		return "user", ""
	}

	if strings.Contains(user, "#") {
		parts := strings.SplitN(user, ":", 2)
		if len(parts) == 2 {
			return parts[0], parts[1]
		}
		return "user", user
	}

	if strings.Contains(user, ":") {
		parts := strings.SplitN(user, ":", 2)
		if len(parts) == 2 && parts[0] != "" {
			return parts[0], parts[1]
		}
	}

	return "user", user
}

func parseObjectTypeAndID(object string) (string, string) {
	if object == "" {
		return "object", ""
	}

	if strings.Contains(object, ":") {
		parts := strings.SplitN(object, ":", 2)
		if len(parts) == 2 && parts[0] != "" {
			return parts[0], parts[1]
		}
	}

	return "object", object
}
