# pretrade-risk-engine

A pre-trade risk daemon written in Go. It ingests order intentions, applies risk
checks against an account's buying power, and emits a verdict for each order —
running as a continuous service that shuts down gracefully on signal.

This is the execution-side counterpart to a separate Python strategy-research
project. Strategy logic (evaluating market conditions) stays in Python where it
is validated; this repo owns the systems concerns: concurrency, atomic risk,
and service lifecycle.

## What it does

```
generator ──orders──▶ actor ──verdicts──▶ gateway ──▶ processor ──▶ stdout
 (produces)          (risk)              (forwards)   (serializes)
```

Three concurrent stages communicate over channels:

- **generator** builds a deterministic set of orders and submits them to the
  actor at a steady tick, looping continuously.
- **actor** owns the account state (buying power + exposure) in a single
  goroutine, applying pre-trade risk one order at a time. Because only this
  goroutine touches the state, the check-and-debit is atomic without a mutex.
- **gateway** forwards each verdict to a processor.
- **processor** serializes each verdict as JSON (one object per line) to a
  writer — stdout in the daemon.

The boundary is deliberate: **intention in → risk verdict out.** No matching
engine, no simulated fills, no real venue. See [ARCHITECTURE.md](ARCHITECTURE.md)
for the design rationale and what is intentionally out of scope.

## Running it

```
make run          # or: go run .
```

The daemon emits a JSON verdict per processed order and runs until it receives
SIGINT (Ctrl-C) or SIGTERM, then shuts down gracefully. Verdicts go to stdout,
logs to stderr, so `make run > verdicts.jsonl` keeps the stream clean:

```
{"OrderID":1,"Status":1,"Reason":0}
{"OrderID":2,"Status":1,"Reason":0}
{"OrderID":3,"Status":2,"Reason":1}
{"OrderID":4,"Status":1,"Reason":0}
^C
2026/08/15 03:11:30 shutting down
2026/08/15 03:11:30 shutdown complete
```

`Status`: 1 = Accepted, 2 = Rejected. `Reason`: 0 = None,
1 = InsufficientBuyingPower, 2 = UnexpectedError.

> **Note:** buying power is consumed monotonically and never released (a
> spot-style daily capital limit). Once the account's buying power is exhausted,
> every subsequent order is rejected — this is by design, not a fault. With the
> default configuration the daemon transitions to all-rejections after a couple
> of minutes.

## Testing

```
make check      # gofmt, go vet, staticcheck, go test -race
```

Every package under `internal/` is unit-tested, including the money arithmetic
(ceil rounding and integer-overflow boundaries), the risk logic (approval,
rejection, accumulated exposure), and the actor lifecycle (verdict emission and
graceful shutdown). Tests run under the race detector; CI (GitHub Actions) runs
`make check` — the same target, not a copy of it — on every push and pull
request.

## Design highlights

- **Actor model for atomic risk.** A single goroutine owns account state, so
  check-and-debit needs no mutex — the channel serializes access.
- **Money is never floating point.** Values are `int64` in a fixed minimum unit
  (scale 100), with ceil rounding (conservative for risk) and an overflow guard
  that fails closed.
- **Context is the only stop mechanism.** No ad-hoc `Stop()`; the generator and
  actor stop on context cancellation, and the gateway follows the channel-close
  cascade. Shutdown is bounded by a deadline so a wedged stage cannot hang the
  process.

## Project layout

```
main.go              daemon entrypoint: signal handling, wiring, bounded shutdown
internal/
  order/             domain: Order, Side, validation, SCALE
  account/           account state, pre-trade risk, and the risk actor
  gateway/           forwards risk verdicts to a processor
  processor/         serializes verdicts (JSON Lines)
  generator/         builds and submits a deterministic order set at a tick
```

## Status

Complete for what it sets out to do: the daemon runs, applies pre-trade risk,
and shuts down gracefully. See [ARCHITECTURE.md](ARCHITECTURE.md) for the design
rationale, the known limitations, and where the project could go next.
