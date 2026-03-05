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

If you want to reuse an existing trace, pass a standard `traceparent` header. For quick demos, you can also pass `trace_id` and an optional `parent_span_id` as query parameters.

## OpenTelemetry configuration

Tracing is enabled by default and uses the OTLP HTTP exporter.

- `OTEL_EXPORTER_OTLP_ENDPOINT`
- `OTEL_EXPORTER_OTLP_HEADERS`
- `SERVICE_NAME` (default: `aca-tracer`)
- `PORT` (default: `8081`)

If you want to send data to Application Insights without the managed agent, run an OpenTelemetry Collector that uses the Azure Monitor exporter and point `OTEL_EXPORTER_OTLP_ENDPOINT` at the collector. The connection string belongs in the collector configuration, not in the app.

### Local collector proxy

This repo includes a minimal OpenTelemetry Collector config that accepts OTLP over HTTP and forwards traces to Application Insights.

1. Ensure `APPLICATIONINSIGHTS_CONNECTION_STRING` is set (the collector uses it).
2. Run the collector:

```bash
docker run --rm -p 4318:4318 \
  -e APPLICATIONINSIGHTS_CONNECTION_STRING="$APPLICATIONINSIGHTS_CONNECTION_STRING" \
  -v "$PWD/otel-collector-config.yaml:/etc/otelcol/config.yaml" \
  otel/opentelemetry-collector-contrib:latest \
  --config /etc/otelcol/config.yaml
```

3. Start the app with `OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318`.

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
  "service_name": "aca-tracer",
  "payload": {
    "order_id": 58311,
    "items": 3,
    "total": 142,
    "currency": "USD"
  }
}
```
