# SCTE-224 Orchestration Service

This repository contains a Go-based orchestration runtime that ingests SCTE-224 media schedules, orchestrates MediaPoint lifecycles with Temporal workflows, evaluates policy decisions, and manipulates manifests for downstream viewers via an HTTP API surface.

## Capabilities

- **Ingestion API** (`POST /v1/media`): parses SCTE-224 XML payloads delivered over an ESNI-style boundary and materialises `MediaPoint` definitions.
- **Temporal scheduling**: launches a Temporal workflow per MediaPoint, handling time-based triggers (`matchTime`) and reusable signal activations.
- **Real-Time Signaling** monitor: evaluates inbound SCTE-35 events against `MatchSignal` assertions (splice event IDs, segmentation types, private indicators, etc.).
- **Decision Making (SDS)**: maintains a per-audience policy stack and resolves the highest priority viewing policy when a trigger fires.
- **Manifest Replacement**: applies SDS decisions to per-audience manifest state, tracking current alternate/fallback URIs and logging transitions.

## Project Layout

- `cmd/scte224service/main.go` – CLI entrypoint bootstrapping config, Temporal client/worker, and the HTTP server.
- `internal/domain` – core domain entities (`Media`, `MediaPoint`, `Decision`, etc.).
- `internal/parser` – SCTE-224 XML parser (namespaced, action policies, durations).
- `internal/storage` – in-memory schedule catalogue.
- `internal/signals` – SCTE-35 assertion evaluation and trigger emission.
- `internal/decision` – SDS policy engine tracking per-audience state and removals.
- `internal/manifest` – manifest manipulator storing the latest per-audience playback state.
- `internal/service` – orchestration facade bridging ingest, Temporal workflows, decision engine, and manifest manipulator.
- `internal/temporal` – workflow definitions and worker harness.
- `internal/httpapi` – Chi-based HTTP handlers (`/v1/media`, `/v1/signals`, `/v1/policies/active`, `/v1/health`).

## Running the Demo

```bash
go run ./cmd/scte224service --config config.yaml
```

What happens at startup:

1. The service dials the local Temporal cluster (default `127.0.0.1:7233`, namespace `default`) and registers the MediaPoint worker on the configured task queue.
2. The HTTP API listens on `:8080` (configurable via YAML or environment variables).
3. POSTing `SCTE-224_example.xml` to `/v1/media` parses and persists the schedule, then launches/refreshes a Temporal workflow per `MediaPoint`.
4. POSTing SCTE-35 JSON payloads to `/v1/signals` fans matching events to the appropriate workflows, which in turn invoke SDS/manifest decisions.

Sample signal payload:

```json
{
  "eventId": "0x1234",
  "segmentationTypeId": "0x11",
  "segmentationEventId": "0x1234",
  "privateIndicator": "PrioritySwitch"
}
```

## Extending / Integrating

- **Alternate audiences**: set `audiences` in `config.yaml` or `DEFAULT_AUDIENCE` env to drive per-market policy stacks.
- **Temporal tuning**: customise task queue/namespace via `TEMPORAL_TASK_QUEUE`, `TEMPORAL_NAMESPACE`, or YAML config.
- **SCTE-35 ingestion**: POST decoded splice inserts to `/v1/signals` (fields `eventId`, `segmentationTypeId`, `segmentationEventId`, `privateIndicator`, optional `arrivedAt`).
- **Policy persistence**: swap `storage.ScheduleStore` with a durable backend if schedules need to survive restarts.
- **Manifest adapter**: extend `manifest.Manipulator` to write to an origin/packager API instead of in-memory logs.

## Configuration

Configuration can be supplied via environment variables or a YAML file (`--config`):

```yaml
http:
  address: ":8080"
temporal:
  hostPort: "127.0.0.1:7233"
  namespace: "default"
  taskQueue: "scte224-scheduler"
audiences:
  - "urn:scte:224:audience:us"
```

Environment overrides:

- `HTTP_ADDRESS`
- `TEMPORAL_HOST_PORT`
- `TEMPORAL_NAMESPACE`
- `TEMPORAL_TASK_QUEUE`
- `DEFAULT_AUDIENCE` (comma-separated)

## Testing & Validation

- `go build ./...` to ensure the code compiles.
- Add go tests around the parser (`internal/parser`) and decision engine to validate new policy constructs.

## Next Steps

- Add JSON ingestion parity (`MediaParser.ParseJSON`).
- Integrate with actual ad-decisioning / personalization services via gRPC or REST.
- Emit metrics and traces for observability (trigger latency, decision counts, manifest state transitions).