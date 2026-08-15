package account

import (
	"context"
	"testing"

	"github.com/afbackend/pretrade-risk-engine/internal/order"
)

func TestActor(t *testing.T) {

	t.Run("Create", func(t *testing.T) {
		_, err := NewActor(1, 0)

		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}
	})
}

func TestSendOrder(t *testing.T) {
	ordReject, err := order.NewOrder(1, 1000, 1, "BTC", order.Long)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	ordAccept, err := order.NewOrder(2, 1000, 1, "BTC", order.Long)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	tests := []struct {
		name        string
		buyingPower int64
		want        RiskStatus
		wantReason  RiskReason
		ord         *order.Order
	}{
		{"Reject", 1, Rejected, InsufficientBuyingPower, ordReject},
		{"Accept", 1000, Accepted, None, ordAccept},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actor, err := NewActor(1, tt.buyingPower)
			if err != nil {
				t.Fatalf("unexpected error %v", err)
			}

			ctx := t.Context()
			go actor.Run(ctx)

			_ = actor.Submit(ctx, tt.ord)
			result := <-actor.Results()

			if result.Status != tt.want {
				t.Fatalf("expected status %d, got %d (order %+v)", tt.want, result.Status, tt.ord)
			}
			if result.Reason != tt.wantReason {
				t.Fatalf("expected reason %d, got %d", tt.wantReason, result.Reason)
			}
		})
	}
}
func TestActorShutdown(t *testing.T) {
	actor, err := NewActor(1, 1000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go actor.Run(ctx)

	cancel()

	for range actor.Results() {
	}

	err = actor.Submit(ctx, &order.Order{ID: 99})
	if err == nil {
		t.Fatal("Submit after shutdown should return an error, got nil")
	}
}
