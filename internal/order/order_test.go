package order

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

func TestNewOrder(t *testing.T) {
	_, err := NewOrder(1, 1, 1, "BTC", Long)

	if err != nil {
		t.Fatalf("error %v", err)
	}

}
