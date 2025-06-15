package fetcher

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/openfga/go-sdk/client"
	"github.com/openfga/go-sdk/credentials"
	"github.com/sirupsen/logrus"
)

// ChangeEvent represents a change event from OpenFGA
type ChangeEvent struct {
	TupleKey   TupleKey  `json:"tuple_key"`
	Operation  string    `json:"operation"`
	Timestamp  time.Time `json:"timestamp"`
	ChangeType string    `json:"change_type"`
	RawEvent   string    `json:"raw_event,omitempty"` // For changelog mode
}

// TupleKey represents a tuple key from OpenFGA with parsed user
type TupleKey struct {
	// Original user field from OpenFGA
	User string `json:"user"`
	// Parsed user components
	UserType string `json:"user_type"`
	UserID   string `json:"user_id"`
	// Other fields
	Relation   string `json:"relation"`
	Object     string `json:"object"`
	ObjectType string `json:"object_type"`
	ObjectID   string `json:"object_id"`
}

// OpenFGAFetcher handles fetching changes from OpenFGA
type OpenFGAFetcher struct {
	client  *client.OpenFgaClient
	storeID string
	logger  *logrus.Logger
}

// NewOpenFGAFetcher creates a new OpenFGA fetcher
func NewOpenFGAFetcher(apiURL, storeID, apiToken string, logger *logrus.Logger) (*OpenFGAFetcher, error) {
	configuration := &client.ClientConfiguration{
		ApiUrl:  apiURL,
		StoreId: storeID,
	}

	if apiToken != "" {
		creds, err := credentials.NewCredentials(credentials.Credentials{
			Method: credentials.CredentialsMethodApiToken,
			Config: &credentials.Config{
				ApiToken: apiToken,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create credentials: %w", err)
		}
		configuration.Credentials = creds
	}

	fgaClient, err := client.NewSdkClient(configuration)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenFGA client: %w", err)
	}

	return &OpenFGAFetcher{
		client:  fgaClient,
		storeID: storeID,
		logger:  logger,
	}, nil
}

// FetchChanges fetches changes from OpenFGA starting from a continuation token
func (f *OpenFGAFetcher) FetchChanges(ctx context.Context, continuationToken string) ([]ChangeEvent, string, error) {
	f.logger.WithField("continuation_token", continuationToken).Debug("Fetching changes from OpenFGA")

	options := client.ClientReadChangesOptions{}
	if continuationToken != "" {
		options.ContinuationToken = &continuationToken
	}

	response, err := f.client.ReadChanges(ctx).Options(options).Execute()
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch changes: %w", err)
	}

	var changes []ChangeEvent
	for _, change := range response.Changes {
		tupleKey := f.parseTupleKey(change.TupleKey.User, change.TupleKey.Relation, change.TupleKey.Object)
		
		changeEvent := ChangeEvent{
			TupleKey:   tupleKey,
			Operation:  string(change.Operation),
			Timestamp:  change.Timestamp,
			ChangeType: "tuple_change",
		}
		changes = append(changes, changeEvent)
	}

	nextToken := ""
	if response.ContinuationToken != nil {
		nextToken = *response.ContinuationToken
	}

	f.logger.WithFields(logrus.Fields{
		"changes_count": len(changes),
		"next_token":    nextToken,
	}).Info("Successfully fetched changes from OpenFGA")

	return changes, nextToken, nil
}

// parseTupleKey parses a tuple key and splits user and object into type and ID components
func (f *OpenFGAFetcher) parseTupleKey(user, relation, object string) TupleKey {
	userType, userID := parseUserTypeAndID(user)
	objectType, objectID := parseObjectTypeAndID(object)

	return TupleKey{
		User:       user,
		UserType:   userType,
		UserID:     userID,
		Relation:   relation,
		Object:     object,
		ObjectType: objectType,
		ObjectID:   objectID,
	}
}

// parseUserTypeAndID parses a user string into type and ID
// Expected format: "user_type:user_id" or just "user_id"
func parseUserTypeAndID(user string) (string, string) {
	if strings.Contains(user, ":") {
		parts := strings.SplitN(user, ":", 2)
		if len(parts) == 2 {
			return parts[0], parts[1]
		}
	}
	// If no type prefix, assume it's just an ID
	return "user", user
}

// parseObjectTypeAndID parses an object string into type and ID
// Expected format: "object_type:object_id"
func parseObjectTypeAndID(object string) (string, string) {
	if strings.Contains(object, ":") {
		parts := strings.SplitN(object, ":", 2)
		if len(parts) == 2 {
			return parts[0], parts[1]
		}
	}
	// If no type prefix, assume it's just an ID
	return "object", object
}
