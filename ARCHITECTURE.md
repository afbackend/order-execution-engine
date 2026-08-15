# Architecture

## Overview

`order-execution-engine` is the pre-trade risk stage of a trading system,
running as a continuous daemon. It ingests order intentions, applies pre-trade
risk against an account's buying power, and emits a risk verdict for each order.
Strategy research and validation live in a separate project
(`trading-research-engine`, Python); this repo is the Go execution-side
counterpart.

The guiding split:

- **Research (Python, separate repo):** validates strategies over historical
  data with walk-forward testing and look-ahead-bias controls. Produces a
  *validated strategy* — the "what" and "when" of a trade.
- **Execution side (Go, this repo):** takes an order intention and applies
  pre-trade risk, deciding whether the order may proceed. What happens after
  approval — routing to a gateway, sending to a real venue — is a later stage,
  out of scope for v1.

Reimplementing strategy logic in Go would duplicate validated logic and risk
silent divergence — a correctness bug in a financial system. Go owns the
execution-side systems concerns: concurrency, atomic risk, service lifecycle.

## The v1 boundary

> **v1 starts at the INTENTION and ends at the RISK VERDICT.**

An intention (symbol, side, quantity, limit price) enters; a `RiskResult`
(approved, or rejected with a reason) comes out. Everything after the verdict
(a gateway sending approved orders to a real venue, fills coming back) is out of
scope. No matching engine, no simulated fills, no real venue.

**Why the verdict, not a fill.** An earlier design had a simulated venue
producing deterministic fills. It was removed: a locally-fabricated fill models
nothing real — in production, fills are *received* from an exchange, not
generated. Without a real matching engine or a post-trade stage to consume them,
a simulated fill is an orphan datum. The honest, complete unit of work here is
the pre-trade risk verdict.

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
  It knows nothing about how a verdict is persisted.
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

## Simplifications in v1 (and why)

- **Buying power is monotonically consumed.** Each approved order debits
  exposure; there is no release. This models a spot-style *daily capital limit*,
  not margin trading. A consequence: once buying power is exhausted, all further
  orders are rejected — by design, though it can look like the daemon has died.
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

## Known limitations & future work

None affect correctness of the v1 risk path; they are quality and completeness
improvements, ranked roughly by value.

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
  approve and *credit* to exposure; a `scale` of zero panics on division. Neither
  is reachable through the v1 path — `NewOrder` rejects a non-positive price, and
  `SCALE` is a constant — so the function is safe as *called*, not as *written*.
  Left as a deliberate deferral, not an oversight: the guard belongs in
  `notional` rather than depending on its caller.
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
- **Graceful drain of in-flight orders on shutdown.** v1 aborts on signal
  (bounded by the deadline); a fuller service would finish accepted orders first.

### Scope (deliberately out of v1)
- Real venue / exchange integration; fills originate there, not here.
- Post-trade: positions, P&L, mark-to-market.
- Multiple accounts / symbols; routing; sharding.
- Live market data / signal evaluation (stays in the Python research repo).
- Orders read from an external feed (v1 generates them internally).
