# SCTE-224 Orchestration Service

This repository contains a Go-based orchestration service that ingests SCTE-224 media schedules, manages time-based and signal-based triggers, evaluates policy decisions, and manipulates manifests for downstream viewers.

## Capabilities

- **Ingestion** via an ESNI-style API boundary: parses XML (or JSON) SCTE-224 payloads and materialises `MediaPoint` definitions.
- **Scheduling** with configurable clock/time-scaling: registers timers for `matchTime` media points and persists the active schedule in-memory.
- **Real-Time Signaling** monitor: evaluates inbound SCTE-35 events against `MatchSignal` assertions (splice event IDs, segmentation types, private indicators, etc.).
- **Decision Making (SDS)**: maintains a per-audience policy stack and resolves the highest priority viewing policy when a trigger fires.
- **Manifest Replacement**: applies SDS decisions to per-audience manifest state, tracking current alternate/fallback URIs and logging transitions.

## Project Layout

- `cmd/scte224service/main.go` – executable demo wiring the service together using `SCTE-224_example.xml` and simulated SCTE-35 signals.
- `internal/domain` – core domain entities (`Media`, `MediaPoint`, `Decision`, etc.).
- `internal/parser` – SCTE-224 XML parser (namespaced, action policies, durations).
- `internal/storage` – in-memory schedule catalogue.
- `internal/scheduler` – time-based trigger manager with optional time compression.
- `internal/signals` – SCTE-35 assertion evaluation and trigger emission.
- `internal/decision` – SDS policy engine tracking per-audience state and removals.
- `internal/manifest` – manifest manipulator storing the latest per-audience playback state.
- `internal/service` – orchestration facade that coordinates all modules.

## Running the Demo

```bash
go run ./cmd/scte224service
```

What the demo does:

1. Ingests `SCTE-224_example.xml` and logs the number of media points discovered.
2. Schedules any past-due media points for immediate evaluation and waits briefly for timer callbacks.
3. Injects simulated SCTE-35 start, stop, and high-priority override signals to exercise the signal monitor.
4. Prints the resulting manifest state per audience (alternate URI, fallback URI, action, and priority).

Sample output snippet:

```
Ingested media urn:scte:224:media:base-channel with 18 media points
manifest: 2025/10/30 00:00:00 audience=urn:scte:224:audience:us apply policy=urn:scte:224:policy:base-stream source=stream://base-stream alt=stream://base-stream
...
Audience urn:scte:224:audience:us manifest -> alt=stream://local-override-stream fallback=stream://base-stream action=apply priority=100
```

## Extending / Integrating

- **Alternate audiences**: supply `service.Config{Audiences: []string{"audience-id"}}` when constructing the service to drive per-market manifests.
- **Time compression**: pass `service.Config{TimeScale: 3600}` to accelerate scheduled triggers during testing (1 hour becomes 1 second).
- **SCTE-35 ingestion**: replace the demo's synthetic `ProcessSignal` calls with events decoded from your transport stream handler; populate `SCTE35Event` fields (`EventID`, `SegmentationType`, `SegmentationEventID`, `PrivateIndicator`).
- **Policy persistence**: swap `storage.ScheduleStore` with a durable backend if schedules need to survive restarts.
- **Manifest adapter**: extend `manifest.Manipulator` to write to an origin/packager API instead of in-memory logs.

## Testing & Validation

- `go build ./...` to ensure the code compiles.
- Add go tests around the parser (`internal/parser`) and decision engine to validate new policy constructs.

## Next Steps

- Add JSON ingestion parity (`MediaParser.ParseJSON`).
- Integrate with actual ad-decisioning / personalization services via gRPC or REST.
- Emit metrics and traces for observability (trigger latency, decision counts, manifest state transitions).