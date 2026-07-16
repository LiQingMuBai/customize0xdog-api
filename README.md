# customize-teldog-api

`customize-teldog-api` is a lightweight Go HTTP service that wraps the Teldog OpenAPI and exposes a clean external-facing API for balance lookup, country/operator/product queries, order creation, order status lookup, and callback verification.

## Features

- Loads runtime configuration from a local `.env` file
- Proxies requests to the Teldog agent API with `X-API-Key`
- Exposes health check endpoints for monitoring
- Verifies Teldog callback signatures with HMAC-SHA256
- Includes basic unit tests

## Requirements

- Go 1.22 or later

## Configuration

Create a `.env` file in the project root:

```env
TELDOG_BASE_URL=https://api.example.com
TELDOG_API_KEY=agt_xxxxxxxxxxxxxxxxxxxx
LISTEN_ADDR=:8080
HTTP_TIMEOUT=12s
```

Environment variables:

- `TELDOG_BASE_URL`: Teldog upstream base URL
- `TELDOG_API_KEY`: API key used in the `X-API-Key` header
- `LISTEN_ADDR`: local listen address, default `:8080`
- `HTTP_TIMEOUT`: upstream request timeout, default `12s`

## Run

```bash
go run ./cmd/server
```

The service starts on `LISTEN_ADDR`.

## API Endpoints

### Health

- `GET /health`
  - Returns JSON:

```json
{
  "status": "ok",
  "service": "customize-teldog-api",
  "timestamp": "2026-07-16T12:34:56Z"
}
```

- `GET /healthz`
  - Returns `204 No Content`

### Teldog Proxy Endpoints

- `GET /api/teldog/balance`
- `GET /api/teldog/countries`
- `GET /api/teldog/operators?country_iso=US`
- `GET /api/teldog/products?country_iso=US&operator_code=att`
- `POST /api/teldog/orders`
- `GET /api/teldog/orders/{agent_order_id}`

These endpoints forward requests to the corresponding Teldog OpenAPI routes under `/openapi/agent/*`.

## Callback Verification

Endpoint:

- `POST /api/teldog/callback`

Required headers:

- `X-Callback-Timestamp`
- `X-Callback-Signature`

Signature rule:

```text
HMAC_SHA256_HEX(api_key, "timestamp.raw_body")
```

The service returns `204 No Content` when the signature is valid.

## Test

```bash
go test ./...
```

## Project Structure

```text
cmd/server/            application entrypoint
internal/config/       configuration loading and .env support
internal/server/       HTTP routing, proxy handlers, callback verification
internal/teldog/       upstream Teldog HTTP client
```

## Notes

- `.env` is ignored by Git and should not be committed.
- `.env.example` provides a safe configuration template.
