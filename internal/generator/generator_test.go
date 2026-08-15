package generator

import (
	"context"
	"testing"
	"time"

	"github.com/afbackend/order-execution-engine/internal/account"
)

func TestGeneratorSubmitsOrders(t *testing.T) {
	actor, err := account.NewActor(1, 1_000_000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go actor.Run(ctx)

	gen := NewGenerator(actor, time.Millisecond)
	go gen.Run(ctx)

	// The generator builds and submits its own orders; draining Results proves
	// the flow works end to end.
	const want = 3
	for i := 0; i < want; i++ {
		select {
		case <-actor.Results():
			// received a verdict — the generator submitted an order
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for result %d", i+1)
		}
	}
}

func TestGeneratorStopsOnContextCancel(t *testing.T) {
	actor, err := account.NewActor(1, 1_000_000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	go actor.Run(ctx)

	// The actor emits on Results() with a bare send; something must drain it
	// or the actor blocks. Drain in the background until the channel closes.
	go func() {
		for range actor.Results() {
		}
	}()

	gen := NewGenerator(actor, time.Millisecond)

	done := make(chan struct{})
	go func() {
		gen.Run(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
		// generator returned after cancel — correct
	case <-time.After(time.Second):
		t.Fatal("generator did not stop within 1s of context cancellation")
	}
}
