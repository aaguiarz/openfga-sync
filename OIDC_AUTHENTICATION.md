# OpenFGA Sync Service - OIDC Authentication Guide

## Overview

The OpenFGA Sync Service now supports OIDC (OpenID Connect) authentication using the client credentials flow. This enables secure authentication with Auth0 FGA and other OIDC-enabled OpenFGA instances without requiring API tokens.

## Configuration

### OIDC Configuration Structure

```yaml
openfga:
  endpoint: "https://api.us1.fga.dev"
  store_id: "01HXXX-YOUR-STORE-ID"
  
  oidc:
    issuer: "https://your-auth0-domain.auth0.com/"
    audience: "https://api.us1.fga.dev/"
    client_id: "your-client-id"
    client_secret: "your-client-secret"
    scopes: ["read:tuples", "write:tuples"]
    token_issuer: "https://your-auth0-domain.auth0.com/"
```

### Configuration Parameters

| Parameter | Required | Description | Example |
|-----------|----------|-------------|---------|
| `issuer` | Yes | The OIDC issuer URL | `https://your-domain.auth0.com/` |
| `audience` | Yes | The API audience for token validation | `https://api.us1.fga.dev/` |
| `client_id` | Yes | The machine-to-machine application client ID | `abc123xyz789` |
| `client_secret` | Yes | The machine-to-machine application client secret | `secret_value` |
| `scopes` | No | List of required scopes for API access | `["read:tuples", "write:tuples"]` |
| `token_issuer` | No | The token issuer URL (defaults to issuer) | `https://your-domain.auth0.com/` |

## Auth0 FGA Setup

### 1. Create Machine-to-Machine Application

1. Go to your Auth0 Dashboard
2. Navigate to **Applications** → **Create Application**
3. Choose **Machine to Machine Applications**
4. Name it (e.g., "OpenFGA Sync Service")
5. Select the **FGA API** as the authorized API
6. Grant required scopes:
   - `read:tuples` - Required for reading changelog
   - `write:tuples` - Required for writing to target OpenFGA instance
   - `read:stores` - Optional, for store metadata access

### 2. Note Your Configuration Values

After creating the application, note these values:
- **Client ID**: Found in application settings
- **Client Secret**: Found in application settings
- **Domain**: Your Auth0 domain (e.g., `your-domain.auth0.com`)
- **API Audience**: Usually `https://api.us1.fga.dev/`

## Environment Variables

You can configure OIDC authentication using environment variables:

```bash
# OpenFGA Configuration
export OPENFGA_ENDPOINT="https://api.us1.fga.dev"
export OPENFGA_STORE_ID="01HXXX-YOUR-STORE-ID"

# OIDC Configuration
export OPENFGA_OIDC_ISSUER="https://your-auth0-domain.auth0.com/"
export OPENFGA_OIDC_AUDIENCE="https://api.us1.fga.dev/"
export OPENFGA_OIDC_CLIENT_ID="your-client-id"
export OPENFGA_OIDC_CLIENT_SECRET="your-client-secret"
export OPENFGA_OIDC_SCOPES="read:tuples,write:tuples"
export OPENFGA_OIDC_TOKEN_ISSUER="https://your-auth0-domain.auth0.com/"
```

## Authentication Priority

The service will use authentication in this order of preference:

1. **OIDC**: If both `client_id` and `client_secret` are provided
2. **API Token**: If `token` is provided
3. **No Authentication**: If neither is configured (for local development)

**Note**: You cannot use both OIDC and API token authentication simultaneously. The configuration validation will reject configs that specify both.

## Configuration Examples

### Complete Auth0 FGA Configuration

```yaml
# config.yaml
server:
  port: 8080

openfga:
  endpoint: "https://api.us1.fga.dev"
  store_id: "01HAUTH0FGA-STORE-ID"
  
  oidc:
    issuer: "https://your-company.auth0.com/"
    audience: "https://api.us1.fga.dev/"
    client_id: "abc123def456ghi789"
    client_secret: "very-secret-client-secret"
    scopes: ["read:tuples", "write:tuples", "read:stores"]
    token_issuer: "https://your-company.auth0.com/"

backend:
  type: "postgres"
  dsn: "postgres://user:pass@localhost:5432/openfga_sync"
  mode: "changelog"

service:
  poll_interval: "10s"
  batch_size: 100

logging:
  level: "info"
  format: "json"
```

### Docker Compose with OIDC

```yaml
version: '3.8'
services:
  openfga-sync:
    image: openfga-sync:latest
    environment:
      - OPENFGA_ENDPOINT=https://api.us1.fga.dev
      - OPENFGA_STORE_ID=01HAUTH0FGA-STORE-ID
      - OPENFGA_OIDC_ISSUER=https://your-company.auth0.com/
      - OPENFGA_OIDC_AUDIENCE=https://api.us1.fga.dev/
      - OPENFGA_OIDC_CLIENT_ID=abc123def456ghi789
      - OPENFGA_OIDC_CLIENT_SECRET=very-secret-client-secret
      - OPENFGA_OIDC_SCOPES=read:tuples,write:tuples
      - BACKEND_TYPE=postgres
      - BACKEND_DSN=postgres://user:pass@postgres:5432/openfga_sync
```

## Validation and Testing

### Configuration Validation

The service validates OIDC configuration at startup:

- Ensures required fields are present
- Prevents conflicts between token and OIDC authentication
- Validates issuer and audience URLs

### Testing Your Configuration

1. **Dry Run**: Use the health endpoint to verify configuration:
   ```bash
   curl http://localhost:8080/health
   ```

2. **Authentication Test**: Check logs for authentication success:
   ```bash
   docker logs openfga-sync 2>&1 | grep -i "auth\|oidc\|token"
   ```

3. **Manual Token Test**: Use your client credentials to manually obtain a token:
   ```bash
   curl -X POST https://your-domain.auth0.com/oauth/token \
     -H "Content-Type: application/json" \
     -d '{
       "client_id": "your-client-id",
       "client_secret": "your-client-secret",
       "audience": "https://api.us1.fga.dev/",
       "grant_type": "client_credentials"
     }'
   ```

## Security Best Practices

### 1. Secure Secret Management

- **Never commit secrets to version control**
- Use environment variables or secret management systems
- Rotate client secrets regularly
- Use least-privilege scopes

### 2. Network Security

- Use HTTPS endpoints only
- Validate SSL certificates
- Implement proper firewall rules
- Monitor authentication logs

### 3. Monitoring and Alerting

- Monitor authentication failures
- Set up alerts for token expiration
- Track unusual API usage patterns
- Log authentication events

## Troubleshooting

### Common Issues

#### 1. "Invalid Client" Error
```
failed to create OIDC credentials: invalid_client
```
**Solution**: Verify your client ID and secret are correct.

#### 2. "Invalid Audience" Error
```
failed to create OIDC credentials: invalid_audience
```
**Solution**: Ensure the audience matches your FGA API audience.

#### 3. "Insufficient Scope" Error
```
access denied: insufficient scope
```
**Solution**: Add required scopes to your machine-to-machine application.

#### 4. "Token Expired" Error
```
authentication failed: token expired
```
**Solution**: The SDK handles token refresh automatically. Check network connectivity.

### Debugging Steps

1. **Enable Debug Logging**:
   ```yaml
   logging:
     level: "debug"
   ```

2. **Check Client Configuration** in Auth0 Dashboard:
   - Verify application type is "Machine to Machine"
   - Ensure FGA API is authorized
   - Check granted scopes

3. **Test Network Connectivity**:
   ```bash
   curl -v https://your-domain.auth0.com/.well-known/openid_configuration
   ```

4. **Validate Token Manually**:
   Use tools like [jwt.io](https://jwt.io) to decode and verify tokens.

## Migration from API Tokens

If you're currently using API tokens and want to migrate to OIDC:

### 1. Parallel Configuration (Testing)
```yaml
openfga:
  # Keep existing token for fallback
  token: "existing-api-token"
  
  # Add OIDC configuration
  oidc:
    issuer: "https://your-domain.auth0.com/"
    # ... other OIDC settings
```

### 2. Test OIDC Configuration
Remove the token temporarily to test OIDC-only authentication.

### 3. Remove API Token
Once OIDC is working, remove the token configuration.

## Performance Considerations

- **Token Caching**: The SDK automatically caches and refreshes tokens
- **Network Latency**: OIDC adds one initial network call for token acquisition
- **Token Lifetime**: Tokens are automatically refreshed before expiration
- **Rate Limiting**: Auth0 has rate limits on token endpoint calls

## Further Reading

- [Auth0 FGA Documentation](https://auth0.com/docs/fga)
- [OpenFGA Go SDK Documentation](https://github.com/openfga/go-sdk)
- [OAuth 2.0 Client Credentials Flow](https://auth0.com/docs/get-started/authentication-and-authorization-flow/client-credentials-flow)
- [OIDC Specification](https://openid.net/connect/)
