package venue

import "github.com/afbackend/order-execution-engine/internal/order"

func Execute(ord *order.Order) (*order.Fill, error) {
	return &order.Fill{ID: 1, Price: ord.Price, Qty: ord.Qty, OrderID: ord.ID}, nil
}
