package venue

import (
	"testing"

	"github.com/afbackend/order-execution-engine/internal/order"
)

func TestExecution(t *testing.T) {

	tests := []struct {
		name   string
		id     int64
		price  int64
		qty    int64
		symbol string
		side   order.Side
	}{
		{"Long single", 0, 1, 1, "BTC", order.Long},
		{"Long quantity", 1, 1, 2, "BTC", order.Long},
		{"Long price", 2, 20, 1, "BTC", order.Long},
		{"Short single", 3, 1, 1, "BTC", order.Short},
		{"Short quantity", 4, 1, 2, "BTC", order.Short},
		{"Short price", 5, 20, 1, "BTC", order.Short},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ord, err := order.NewOrder(tt.id, tt.price, tt.qty, tt.symbol, tt.side)

			if err != nil {
				t.Fatalf("error %v", err)
			}

			fill, err := Execute(ord)

			if err != nil {
				t.Fatalf("error %v", err)
			}

			if fill.Price != ord.Price || fill.Qty != ord.Qty {
				t.Errorf("expected price: %d e quantity: %d, got %+v", ord.Price, ord.Qty, fill)
			}
		})
	}
}
