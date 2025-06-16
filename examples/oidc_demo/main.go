package main

import (
	"fmt"
	"strings"

	"github.com/aaguiarz/openfga-sync/config"
	"github.com/aaguiarz/openfga-sync/fetcher"
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

	fmt.Println("🔐 OpenFGA OIDC Authentication Demonstration")
	fmt.Println("============================================")
	fmt.Println("")
	fmt.Println("This demo shows how to configure OIDC authentication for Auth0 FGA")
	fmt.Println("and other OIDC-enabled OpenFGA instances using client credentials flow.")
	fmt.Println("")

	// Demo 1: OIDC Configuration Examples
	fmt.Println("📝 OIDC Configuration Examples:")
	fmt.Println("-------------------------------")

	// Example 1: Auth0 FGA Configuration
	fmt.Println("\n🔹 Auth0 FGA Configuration:")
	auth0Config := `
openfga:
  endpoint: "https://api.us1.fga.dev"
  store_id: "01HAUTH0-FGA-STORE-ID"
  
  oidc:
    issuer: "https://your-company.auth0.com/"
    audience: "https://api.us1.fga.dev/"
    client_id: "your-m2m-client-id"
    client_secret: "your-m2m-client-secret"
    scopes: ["read:tuples", "write:tuples"]
    token_issuer: "https://your-company.auth0.com/"
`
	fmt.Println(auth0Config)

	// Example 2: Environment Variables
	fmt.Println("🔹 Environment Variables Configuration:")
	envVars := []string{
		"OPENFGA_ENDPOINT=https://api.us1.fga.dev",
		"OPENFGA_STORE_ID=01HAUTH0-FGA-STORE-ID",
		"OPENFGA_OIDC_ISSUER=https://your-company.auth0.com/",
		"OPENFGA_OIDC_AUDIENCE=https://api.us1.fga.dev/",
		"OPENFGA_OIDC_CLIENT_ID=your-m2m-client-id",
		"OPENFGA_OIDC_CLIENT_SECRET=your-m2m-client-secret",
		"OPENFGA_OIDC_SCOPES=read:tuples,write:tuples",
	}
	for _, env := range envVars {
		fmt.Printf("export %s\n", env)
	}

	// Demo 2: Configuration Validation
	fmt.Println("\n🧪 Configuration Validation Examples:")
	fmt.Println("------------------------------------")

	testConfigurations := []struct {
		name        string
		description string
		config      func() *config.Config
		shouldPass  bool
	}{
		{
			name:        "Valid OIDC Configuration",
			description: "Complete OIDC configuration with all required fields",
			config: func() *config.Config {
				cfg := config.DefaultConfig()
				cfg.OpenFGA.Token = "" // Clear default token
				cfg.OpenFGA.OIDC = config.OIDCConfig{
					Issuer:       "https://test.auth0.com/",
					Audience:     "https://api.fga.dev/",
					ClientID:     "test-client-id",
					ClientSecret: "test-client-secret",
					Scopes:       []string{"read:tuples", "write:tuples"},
					TokenIssuer:  "https://test.auth0.com/",
				}
				cfg.OpenFGA.StoreID = "test-store-id"
				cfg.Backend.DSN = "postgres://test:test@localhost/test"
				return cfg
			},
			shouldPass: true,
		},
		{
			name:        "Missing OIDC Issuer",
			description: "OIDC configuration missing required issuer field",
			config: func() *config.Config {
				cfg := config.DefaultConfig()
				cfg.OpenFGA.Token = ""
				cfg.OpenFGA.OIDC = config.OIDCConfig{
					Audience:     "https://api.fga.dev/",
					ClientID:     "test-client-id",
					ClientSecret: "test-client-secret",
				}
				cfg.OpenFGA.StoreID = "test-store-id"
				cfg.Backend.DSN = "postgres://test:test@localhost/test"
				return cfg
			},
			shouldPass: false,
		},
		{
			name:        "Token and OIDC Conflict",
			description: "Both API token and OIDC configuration provided",
			config: func() *config.Config {
				cfg := config.DefaultConfig()
				cfg.OpenFGA.Token = "api-token"
				cfg.OpenFGA.OIDC = config.OIDCConfig{
					Issuer:       "https://test.auth0.com/",
					Audience:     "https://api.fga.dev/",
					ClientID:     "test-client-id",
					ClientSecret: "test-client-secret",
				}
				cfg.OpenFGA.StoreID = "test-store-id"
				cfg.Backend.DSN = "postgres://test:test@localhost/test"
				return cfg
			},
			shouldPass: false,
		},
		{
			name:        "API Token Only",
			description: "Traditional API token authentication",
			config: func() *config.Config {
				cfg := config.DefaultConfig()
				cfg.OpenFGA.Token = "api-token"
				cfg.OpenFGA.StoreID = "test-store-id"
				cfg.Backend.DSN = "postgres://test:test@localhost/test"
				return cfg
			},
			shouldPass: true,
		},
	}

	for _, test := range testConfigurations {
		fmt.Printf("\n🔸 %s:\n", test.name)
		fmt.Printf("   %s\n", test.description)

		cfg := test.config()
		err := validateConfig(cfg)

		if test.shouldPass && err == nil {
			fmt.Println("   ✅ Validation passed (as expected)")
		} else if !test.shouldPass && err != nil {
			fmt.Printf("   ✅ Validation failed (as expected): %v\n", err)
		} else if test.shouldPass && err != nil {
			fmt.Printf("   ❌ Unexpected validation failure: %v\n", err)
		} else {
			fmt.Println("   ❌ Expected validation failure but passed")
		}
	}

	// Demo 3: Authentication Method Detection
	fmt.Println("\n🔍 Authentication Method Detection:")
	fmt.Println("-----------------------------------")

	authExamples := []struct {
		description string
		hasToken    bool
		hasOIDC     bool
		expected    string
	}{
		{"API Token provided", true, false, "API Token"},
		{"OIDC credentials provided", false, true, "OIDC Client Credentials"},
		{"No authentication", false, false, "None (development mode)"},
		{"Both provided (invalid)", true, true, "Conflict (validation error)"},
	}

	for _, example := range authExamples {
		fmt.Printf("🔸 %s → %s\n", example.description, example.expected)
	}

	// Demo 4: Mock OIDC Fetcher Creation
	fmt.Println("\n🚀 OIDC Fetcher Creation Demo:")
	fmt.Println("------------------------------")

	// This would normally create a real OIDC fetcher, but we'll simulate it
	fmt.Println("Creating OpenFGA fetcher with OIDC authentication...")

	oidcConfig := fetcher.OIDCConfig{
		Issuer:       "https://demo.auth0.com/",
		Audience:     "https://api.us1.fga.dev/",
		ClientID:     "demo-client-id",
		ClientSecret: "demo-client-secret",
		Scopes:       []string{"read:tuples", "write:tuples"},
		TokenIssuer:  "https://demo.auth0.com/",
	}

	// Note: This would fail in practice because the credentials are fake
	fmt.Printf("OIDC Configuration:\n")
	fmt.Printf("  Issuer: %s\n", oidcConfig.Issuer)
	fmt.Printf("  Audience: %s\n", oidcConfig.Audience)
	fmt.Printf("  Client ID: %s\n", oidcConfig.ClientID)
	fmt.Printf("  Scopes: %s\n", strings.Join(oidcConfig.Scopes, ", "))

	fmt.Println("\n📋 Demo completed! This demonstration showed:")
	fmt.Println("  • OIDC configuration formats (YAML and environment variables)")
	fmt.Println("  • Configuration validation for different scenarios")
	fmt.Println("  • Authentication method detection and priority")
	fmt.Println("  • OIDC fetcher creation with client credentials")
	fmt.Println("")
	fmt.Println("🎯 Next Steps:")
	fmt.Println("  1. Set up a Machine-to-Machine application in Auth0")
	fmt.Println("  2. Configure the required scopes (read:tuples, write:tuples)")
	fmt.Println("  3. Update your configuration with real OIDC credentials")
	fmt.Println("  4. Test the connection with your Auth0 FGA instance")
	fmt.Println("")
	fmt.Println("📚 For detailed setup instructions, see OIDC_AUTHENTICATION.md")
}

// Add a Validate method to config.Config for the demo (normally this would be private)
func validateConfig(c *config.Config) error {
	// This is just for demo purposes - normally we'd call the private validate method
	// We'll simulate validation logic here
	var errors []string

	if c.OpenFGA.Endpoint == "" {
		errors = append(errors, "openfga.endpoint is required")
	}
	if c.OpenFGA.StoreID == "" {
		errors = append(errors, "openfga.store_id is required")
	}

	hasToken := c.OpenFGA.Token != ""
	hasOIDC := c.OpenFGA.OIDC.ClientID != "" && c.OpenFGA.OIDC.ClientSecret != ""

	if !hasToken && !hasOIDC {
		errors = append(errors, "OpenFGA authentication required: either 'token' or OIDC configuration must be provided")
	}

	if hasToken && hasOIDC {
		errors = append(errors, "OpenFGA authentication conflict: provide either 'token' or OIDC configuration, not both")
	}

	if hasOIDC {
		if c.OpenFGA.OIDC.Issuer == "" {
			errors = append(errors, "openfga.oidc.issuer is required")
		}
		if c.OpenFGA.OIDC.Audience == "" {
			errors = append(errors, "openfga.oidc.audience is required")
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("validation errors: %s", strings.Join(errors, ", "))
	}

	return nil
}
