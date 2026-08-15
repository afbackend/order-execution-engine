package account

import (
	"errors"
	"math"
	"testing"

	"github.com/afbackend/pretrade-risk-engine/internal/order"
)

func TestNewAccount(t *testing.T) {
	_, err := NewAccount(0)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetBuyingPower(t *testing.T) {

	t.Run("Set BuyingPower", func(t *testing.T) {

		bp := int64(1000)

		acc, err := NewAccount(1)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		acc.SetBuyingPower(bp)

		if acc.buyingPower != bp {
			t.Fatalf("expected %d, got %d", bp, acc.buyingPower)
		}
	})

	t.Run("Set invalid BuyingPower", func(t *testing.T) {

		bp := int64(-1000) // Not allowed for now

		acc, err := NewAccount(1)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		err = acc.SetBuyingPower(bp)

		if !errors.Is(err, ErrInvalidBuyingPower) {
			t.Fatalf("expected %v, got %v", ErrInvalidBuyingPower, err)
		}

	})
}

func TestNotional(t *testing.T) {
	tests := []struct {
		name       string
		price, qty int64
		want       int64
		wantErr    error
	}{
		{"exact", 1000, 1, 10, nil},               // 1000/100 = 10, sem resto
		{"rounds up remainder", 1001, 1, 11, nil}, // ceil: 1001→11
		{"rounds up half", 1050, 1, 11, nil},      // ceil: 1050→11
		{"quantity multiplies", 1000, 3, 30, nil},
		{"overflow", 9_000_000_000_000_000_000, 1000, 0, ErrNotionalOverflow},
		{"max price computes correctly", math.MaxInt64, 1, 92233720368547759, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, err := notional(tt.price, tt.qty, order.SCALE)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}

			if tt.want != n {
				t.Fatalf("expected %d, got %d", tt.want, n)
			}
		})
	}
}

func TestRisk(t *testing.T) {
	t.Run("Test allow send order", func(t *testing.T) {
		acc, err := NewAccount(1)

		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		_ = acc.SetBuyingPower(1000)

		ord, err := order.NewOrder(1, 10, 1, "BTC", order.Long)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got, err := acc.Reserve(ord)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !got {
			t.Fatalf("expected true, BuyingPower: 1000 Order: %+v", ord)
		}

		expectedExposure, err := notional(ord.Price, ord.Qty, order.SCALE)
		if err != nil || acc.exposure != expectedExposure {
			t.Fatalf("expected %d, got %d", expectedExposure, acc.exposure)
		}
	})

	t.Run("Test reject send order", func(t *testing.T) {
		acc, err := NewAccount(1)

		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		_ = acc.SetBuyingPower(1000)

		ord, err := order.NewOrder(1, 20000, 10, "BTC", order.Long)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		before := acc.exposure
		got, err := acc.Reserve(ord)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got {
			t.Fatalf("expected false, BuyingPower: 1000 Order: %+v", ord)
		}

		if before != acc.exposure {
			t.Fatalf("expected %d, got %d", before, acc.exposure)
		}
	})

	t.Run("Test reject on accumulated exposure", func(t *testing.T) {
		acc, err := NewAccount(1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		_ = acc.SetBuyingPower(1000)

		ord1, err := order.NewOrder(1, 60000, 1, "BTC", order.Long)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got, err := acc.Reserve(ord1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !got {
			t.Fatalf("first order should be approved, exposure: %d BuyingPower: 1000", acc.exposure)
		}

		ord2, err := order.NewOrder(2, 60000, 1, "BTC", order.Long)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		before := acc.exposure
		got, err = acc.Reserve(ord2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got {
			t.Fatalf("second order should be rejected on accumulated exposure, exposure: %d", acc.exposure)
		}
		if before != acc.exposure {
			t.Fatalf("rejected order must not debit: expected %d, got %d", before, acc.exposure)
		}
	})

	t.Run("astronomical notional is rejected not approved", func(t *testing.T) {
		acc, err := NewAccount(1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		_ = acc.SetBuyingPower(1000)

		ord, _ := order.NewOrder(1, math.MaxInt64, 1, "BTC", order.Long)

		ok, err := acc.Reserve(ord)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok {
			t.Fatal("order with astronomical notional must be rejected, not approved")
		}
	})
}
