# azure-container-apps-tracing

This project demonstrates distributed tracing with OpenTelemetry in Go, focused on running in Azure Container Apps. The API returns random payloads but uses fixed, per-endpoint delays so trace timelines are consistent and easy to compare.

## Concept

The service is a small Gin-based API that:

- Emits traces for each request and internal step
- Uses fixed delays to create predictable span durations
- Returns random payload data so each call looks distinct

## Endpoints and fixed delays

Each endpoint produces a consistent total delay (sum of its internal steps):

- `GET /api/users/:id` (120ms)
- `GET /api/orders/:id` (220ms)
- `GET /api/catalog/search` (180ms)
- `POST /api/checkout` (450ms)
- `GET /api/recommendations` (320ms)

Responses include `delay_ms` and `trace_id` to make it easy to locate the trace for a given request.

## OpenTelemetry configuration

Tracing is enabled by default and uses the OTLP HTTP exporter. Set standard OpenTelemetry environment variables to send spans to Azure:

- `OTEL_EXPORTER_OTLP_ENDPOINT`
- `OTEL_EXPORTER_OTLP_HEADERS`
- `SERVICE_NAME` (default: `slow-api`)
- `PORT` (default: `8081`)

For Azure Monitor via the OpenTelemetry Collector, set `OTEL_EXPORTER_OTLP_ENDPOINT` to your collector endpoint and pass the Azure connection string as an OTLP header if required by your setup.

## Usage

```bash
go run main.go
```

### Example response

```json
{
  "status": "ok",
  "timestamp": "2026-03-05T10:30:45Z",
  "delay_ms": 220,
  "trace_id": "bdb0a8c2b900f1422a93ef6c1f38b0e6",
  "endpoint": "/api/orders/:id",
  "service_name": "slow-api",
  "payload": {
    "order_id": 58311,
    "items": 3,
    "total": 142,
    "currency": "USD"
  }
}
```
