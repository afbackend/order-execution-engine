package venue

import (
	"errors"
	"testing"
)

func TestOrderErr(t *testing.T) {
	tests := []struct {
		name   string
		id     int64
		price  int64
		qty    int64
		symbol string
		side   Side
		err    error
	}{
		{"Invalid price", 0, -1, 1, "BTC", Long, ErrInvalidPrice},
		{"Invalid Quantity", 1, 1, 0, "BTC", Long, ErrInvalidQty},
		{"Invalid Symbol", 2, 1, 1, "", Long, ErrInvalidSymbol},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewOrder(tt.id, tt.price, tt.qty, tt.symbol, tt.side)

			if !errors.Is(err, tt.err) {
				t.Fatalf("expected %v, got %v", tt.err, err)
			}
		})
	}
}

func TestExecution(t *testing.T) {

	tests := []struct {
		name   string
		id     int64
		price  int64
		qty    int64
		symbol string
		side   Side
	}{
		{"Long single", 0, 1, 1, "BTC", Long},
		{"Long quantity", 1, 1, 2, "BTC", Long},
		{"Long price", 2, 20, 1, "BTC", Long},
		{"Short single", 3, 1, 1, "BTC", Short},
		{"Short quantity", 4, 1, 2, "BTC", Short},
		{"Short price", 5, 20, 1, "BTC", Short},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			order, err := NewOrder(tt.id, tt.price, tt.qty, tt.symbol, tt.side)

			if err != nil {
				t.Fatalf("error %v", err)
			}

			fill, err := Execute(order)

			if err != nil {
				t.Fatalf("error %v", err)
			}

			if fill.price != order.price || fill.qty != order.qty {
				t.Errorf("expected price: %d e quantity: %d, got %+v", order.price, order.qty, fill)
			}
		})
	}
}
