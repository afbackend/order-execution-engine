# Architecture

## Overview

`order-execution-engine` is an order execution engine built in stages. The stage
implemented today is **pre-trade risk**: it ingests order intentions, applies
risk against an account's buying power, and emits a verdict for each order,
running as a continuous daemon. Execution — routing approved orders to a venue
and handling the fills that come back — is the committed next stage (see
Roadmap). The project is named for its target; this document is explicit about
which stage exists today.

Strategy research and validation live in a separate project
(`trading-research-engine`, Python); this repo is the Go execution-side
counterpart.

The guiding split:

- **Research (Python, separate repo):** validates strategies over historical
  data with walk-forward testing and look-ahead-bias controls. Answers *"does
  this strategy have edge?"*
- **Execution side (Go, this repo):** takes an order intention and answers *"can
  this order be accepted safely?"* — today via pre-trade risk; later, through
  execution against a venue.

Reimplementing strategy logic in Go would duplicate validated logic and risk
silent divergence — a correctness bug in a financial system. Go owns the
execution-side systems concerns: concurrency, atomic risk, service lifecycle.

## Current boundary

> **Today: INTENTION in → RISK VERDICT out.**

An intention (symbol, side, quantity, limit price) enters; a `RiskResult`
(approved, or rejected with a reason) comes out. There is no matching engine, no
simulated fills, and no real venue *yet* — those belong to the execution stage
on the roadmap.

**Why no simulated fills today.** An earlier design had a simulated venue
producing deterministic fills. It was removed: a locally-fabricated fill models
nothing real — in production, fills are *received* from an exchange, not
generated. Without a real venue or a post-trade stage to consume them, a
simulated fill is an orphan datum. Fills will enter with the execution stage,
where they originate from a venue (real or a faithful adapter), not from thin
air.

## Runtime: a continuous daemon

The system runs as a service, not a batch — it does not terminate when orders
are exhausted. It processes indefinitely and stops only on an OS signal
(SIGINT / SIGTERM), shutting down gracefully.

Three concurrent stages communicate over channels:

```
generator ──orders──▶ actor ──verdicts──▶ gateway ──▶ processor ──▶ stdout
 (produces)          (risk)              (forwards)   (serializes)
```

- **generator** builds a fixed, deterministic set of orders internally and
  submits them to the actor at a steady tick, looping until cancelled. It models
  continuous ingestion.
- **actor** owns the account state in a single goroutine; processes orders one
  at a time, applying pre-trade risk; emits a `RiskResult` per order.
- **gateway** reads verdicts from the actor and forwards each to a processor.
  It knows nothing about how a verdict is persisted. In the execution stage this
  is where routing to a venue would attach.
- **processor** (`DefaultProcessor`) serializes each verdict as JSON to an
  `io.Writer`, one object per line (JSON Lines).

## Core design: the account actor

The heart of the engine is the **account actor** — a single goroutine that
*owns* the account state (buying power + accumulated exposure). Orders arrive on
a channel and are processed one at a time, in sequence. This gives two
properties for free:

1. **Atomic risk without a mutex.** Because only the actor goroutine touches
   buying power and exposure, the check-and-debit (`Reserve`) is naturally
   atomic — no race, no lock.
2. **Preserved ordering.** Orders are processed in arrival order.

The actor produces a **`RiskResult`** (a new value describing the verdict)
rather than mutating the incoming order. Rejection carries a distinct
`RiskReason` (insufficient buying power vs. calculation error) — the reason a
downstream consumer would act on.

### Channel ownership and lifecycle

- The actor **owns the channels it produces** (results, done) and closes them on
  shutdown. The input channel is **never closed** — shutdown is driven by
  context cancellation, so there is no channel to close and no send-on-closed
  panic from a late producer.
- Consumers receive results as a **receive-only channel** (`<-chan`) — the
  direction is enforced by the compiler.
- `Submit` is a **defensive send**: a `select` over the input channel,
  `ctx.Done()`, and `done`. If the actor has stopped or the context is
  cancelled, it returns an error instead of blocking or panicking. This is why
  the generator can never block submitting to a stopped actor.
- **Context cancellation is the only stop mechanism.** There is no `Stop()`
  method — that would duplicate what context provides. The generator and actor
  stop on `ctx.Done()`; the gateway stops when its input channel is closed,
  following the cascade. Only the producer side listens to the signal; the
  consumer follows the channel close.
- The input channel is **unbuffered**, making order handoff synchronous: a
  submit completes only when the actor receives, so no order sits pending in a
  buffer to be lost at shutdown.

### Shutdown cascade

```
signal → context cancelled
       → generator stops producing + actor stops processing
       → actor closes results
       → gateway drains remaining verdicts and exits
       → main's WaitGroup releases → process exits
```

The wait in `main` is **bounded by a deadline**: if a stage is wedged — for
example the processor's writer blocks (a full pipe) while the actor is mid-send
on its results channel — `main` forces exit rather than hanging until SIGKILL.
The deadline guarantees termination; see Known limitations for the residual
verdict-loss trade-off in that pathological case.

## Money representation

Monetary values (price, buying power, notional) and quantities are `int64` in a
fixed minimum unit (scale 100 — two decimal places). This follows the rule that
money is never floating point. `int64` (not `int`, which is platform-dependent)
keeps the arithmetic simple for the single-symbol, single-scale MVP. Named money
types (`Price`, `Quantity`, `Money`) were considered and deliberately dropped:
with a single scale, the compiler-enforced separation was ceremony without
payoff.

Notional uses **ceil rounding** — conservative for risk, since it never
*under*estimates cost. The ceil is computed by dividing first and adjusting for
the remainder (`product/scale`, then `+1` if there is a remainder) rather than
`(product + scale - 1)/scale`; the latter's addition can overflow int64 on a
large price and wrap to a negative notional, which `Reserve` would then approve
— a fail-open bug that this design avoids. An overflow guard covers the
`price*qty` multiplication itself. An astronomical price therefore yields a
correct large notional that is rejected on its merits — **fail-closed**.

**Planned evolution:** a dedicated `financial` package with a
`Currency{ value int64; scale int }` type carrying its scale with the value,
needed once multiple scales or currencies interact. Deferred: the v1 has a
single implicit scale.

## Simplifications today (and why)

- **Buying power is monotonically consumed.** Each approved order debits
  exposure; there is no release. This models a spot-style *daily capital limit*,
  not margin trading. A consequence: once buying power is exhausted, all further
  orders are rejected — by design, though it can look like the daemon has died.
  Position tracking and buying-power release arrive with the post-trade state on
  the roadmap.
- **Side is preserved but does not affect risk.** A long-only
  capital-consumption model: `Side` is carried for forward compatibility, but
  long and short consume buying power identically and no net position is tracked.
- **Single account, single symbol.** No routing, no sharding.

## Package layout

```
main.go              daemon entrypoint: signal handling, wiring, bounded shutdown
internal/
  order/             domain: Order, Side, sentinel errors, SCALE
  account/           account state, pre-trade risk, and the risk actor
  gateway/           RiskProcessor interface (consumer-side) + verdict forwarding
  processor/         DefaultProcessor: serializes verdicts (JSON Lines)
  generator/         builds and submits a deterministic order set at a tick
```

`internal/` is a compiler-enforced boundary. The layout is organized by
responsibility, not by technical category — there is no `models/` or `types/`
package, which is anti-idiomatic in Go. The `RiskProcessor` interface is
declared in the package that consumes it, not the one that implements it.

## Roadmap (committed next stages)

These are the stages that grow the project into its name. They are planned
direction, not a promise of dates.

- **Execution gateway / venue adapter.** The stage the name points at: an API
  that receives approved verdicts and routes orders to a venue (or a faithful
  simulated one), then handles the acks and fills that come back. This is where
  fills genuinely originate — received, not fabricated — and it brings
  idempotency, reconciliation, and reconnect concerns.
- **Position & post-trade state.** Positions, gross/net exposure, realized and
  unrealized PnL, buying-power release. The richer account state that turns the
  risk model from a capital counter into a real risk engine, and where `Side`
  starts to matter.
- **Concurrent risk invariants.** Property tests asserting that across any
  concurrent sequence of orders, exposure never exceeds its limit — proving
  correctness under concurrency rather than measuring throughput.
- **Command / event separation with replay.** Orders as commands, verdicts as
  events; an event log the account state can be reconstructed from by replay.
  Connects concurrency, determinism, and financial correctness.

## Known limitations & future work

None affect correctness of the current risk path; they are quality and
completeness improvements, ranked roughly by value.

### Testing gaps
- **`Gateway.Run()` has no test, and there is no end-to-end test.** The
  gateway's forwarding is covered via `process`, but the `Run` loop is not. An
  end-to-end test driving the full pipeline through a bounded shutdown would
  cover the emergent lifecycle behaviour unit tests can't.
- **`ErrActorStopped` is never exercised.** The shutdown test asserts only
  `err != nil`; it doesn't distinguish `ErrActorStopped` from `ctx.Err()`.

### Output contract
- **`RiskResult` serializes without JSON tags; enums emit as bare integers.**
  The verdict is the product, and `{"Status":1,"Reason":0}` is opaque. Struct
  tags plus `String()`/`MarshalJSON` on the enums (so verdicts read as
  `"accepted"` / `"insufficient_buying_power"`), plus a timestamp and symbol,
  would make the output self-describing.

### Money arithmetic
- **`notional` does not validate its own inputs.** A negative price yields a
  negative notional (`notional(-1000, 1, 100)` → `-10`), which `Reserve` would
  approve and *credit* to exposure; a `scale` of zero panics on division.
  Neither is reachable through the current path — `NewOrder` rejects a
  non-positive price, and `SCALE` is a constant — so the function is safe as
  *called*, not as *written*. A deliberate deferral, not an oversight: the guard
  belongs in `notional` rather than depending on its caller.
- **`Reserve` sums before comparing.** `exposure + n > buyingPower` can wrap with
  a buying power near `MaxInt64`; `n > buyingPower - exposure` cannot, since
  `exposure <= buyingPower` is an invariant of the method. Unreachable at any
  realistic capital limit.

### Idiomatic / API
- **`NewDefault` returns a value whose methods have pointer receivers**, forcing
  callers to take its address. The constructor should return a pointer.
- **`log.Fatalf` inside `internal/generator`.** A library package should not
  terminate the process; it should return an error and let the daemon decide.
- **`account` risk methods could surface richer typed errors** for clearer
  propagation and testing.

### Lifecycle robustness
- **Bare send `a.out <- r` depends on the gateway draining until close.** This
  holds today (single consumer, no `ctx.Done()` in the gateway), and a wedged
  writer is bounded by the shutdown deadline. Note that the obvious hardening —
  wrapping the send in a `select` with `ctx.Done()` — is *worse*: on SIGTERM
  both cases are ready at once, and Go's random choice would discard verdicts on
  perfectly healthy shutdowns. The alternative worth considering is a *limited
  drain*: the actor attempting the send with its own timeout before giving up,
  which loses nothing on a normal shutdown and bounds the pathological one.
  Deferred, as the deadline already guarantees termination.
- **Graceful drain of in-flight orders on shutdown.** Today aborts on signal
  (bounded by the deadline); a fuller service would finish accepted orders first.

## Out of scope (not planned for this project)

- Live market data / signal evaluation — that stays in the Python research repo.
- An ultra-low-latency execution core — a different language and a different
  project.
